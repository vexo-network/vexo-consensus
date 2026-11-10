package consensus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/vexo-network/vexo-consensus/consensus/internal/safety"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrUnknownValidator    = errors.New("unknown validator")
	ErrConflictingVote     = errors.New("conflicting vote")
	ErrNoQuorum            = errors.New("not enough voting power for quorum")
	ErrInvalidProposal     = errors.New("invalid proposal")
	ErrStaleProposal       = errors.New("stale proposal")
	ErrInvalidVote         = errors.New("invalid vote")
	ErrUnknownVoteBlock    = errors.New("vote target block not found")
	ErrStaleVote           = errors.New("stale vote")
	ErrUnsafeProposal      = errors.New("unsafe proposal")
	ErrUnexpectedProposer  = errors.New("unexpected proposer")
	ErrUnsafeVote          = errors.New("unsafe vote")
	ErrAggregateRequired   = errors.New("aggregate signer is required")
	ErrMissingSignature    = errors.New("vote signature is required")
	ErrConflictingCommit   = safety.ErrConflictingCommit
	ErrInvalidQuorumCert   = errors.New("invalid quorum certificate")
	ErrConflictingProposal = errors.New("conflicting proposal in view")
	ErrOutOfWindow         = errors.New("message outside the accepted height and view window")
	ErrInvalidSafetyState  = errors.New("invalid safety state")
	// ErrValidatorSetPending reports a block whose validator set the registry holds only after a
	// committed block this node has not executed yet. The node executes its committed blocks and
	// the block becomes verifiable; it is not invalid.
	ErrValidatorSetPending = errors.New("validator set of the block is not executed yet")
	// ErrUnknownValidatorSet reports a registry height older than the validator sets kept.
	ErrUnknownValidatorSet = errors.New("validator set of the registry height is not kept")
	// ErrUnknownParent reports a proposal whose parent block the machine does not hold. The
	// validator set of a block derives from its parent header, so the parent must arrive first.
	ErrUnknownParent = errors.New("parent block not found")
)

const (
	// maxHeightWindow bounds how far above the finalized height a block, vote or timeout
	// vote is accepted.
	maxHeightWindow types.Height = 256
	// maxViewLookahead bounds how far above the current view a vote or timeout vote is
	// accepted.
	maxViewLookahead types.Round = 256
	// timeoutViewRetention is how many views behind the current view timeout votes stay
	// available for equivocation detection.
	timeoutViewRetention types.Round = 32
	// maxChainView and maxChainHeight reject values whose successor would overflow.
	maxChainView   types.Round  = 1 << 62
	maxChainHeight types.Height = 1 << 62
	// maxVerifiedQCCache bounds the cache of already verified certificates.
	maxVerifiedQCCache = 1024
	// maxValidatorSetHistory bounds the validator set snapshots kept for verifying
	// certificates of earlier heights.
	maxValidatorSetHistory = 64
)

// MaxBlockBytes bounds the transaction bytes of a block: a validator refuses a larger proposal, so a
// leader cannot make the validators fetch, execute and store more than one block's worth per view,
// whatever it names. A whole proposal of that size fits a transport message.
const MaxBlockBytes = 2 << 20

// BlockTxBytes returns the transaction bytes of block.
func BlockTxBytes(block types.Block) int {
	total := 0
	for _, tx := range block.Txs {
		total += len(tx)
	}
	return total
}

type StateMachineConfig struct {
	ChainID      string
	ValidatorSet validator.Set
	HashBlock    func(types.Block) types.Hash
	// Signatures verifies individual proposal, vote and timeout vote signatures.
	Signatures signatureVerifier
	// Aggregator combines individual signatures into certificates.
	Aggregator aggregateSigner
	// AggregateVerifier verifies certificate signatures. Domain separation is applied by
	// the state machine, so the verifier must not wrap the message itself.
	AggregateVerifier        aggregateVerifier
	EnforceProposerSelection bool
	// AppHashes returns the header app hash (finality.HeaderAppHash) of the state after executing
	// block height (the genesis state for 0). With it, a proposal commits the app hash of the
	// height finality.AppHashHeight names
	// in its header; without it, as for a machine that runs no application, the field stays zero.
	AppHashes func(height types.Height) (types.Hash, error)
}

// SafetyState is the part of the consensus state that must survive a restart for voting to
// stay safe: the lock, the highest certificate seen, and the last view voted in.
type SafetyState struct {
	LockedQC        finality.QuorumCert
	HighQC          finality.QuorumCert
	LastVotedHeight types.Height
	LastVotedRound  types.Round
}

// registeredValidatorSet is the validator set the registry holds from registry height from on.
type registeredValidatorSet struct {
	from types.Height
	set  validator.Set
}

type viewSlot struct {
	blockHash types.Hash
	height    types.Height
}

type StateMachine struct {
	mu            sync.Mutex
	chainID       string
	validatorSets []registeredValidatorSet
	// validatorSetsThrough is the highest registry height the history covers. Zero means the
	// machine runs on the configured set alone, which then certifies every block.
	validatorSetsThrough     types.Height
	hashBlock                func(types.Block) types.Hash
	signatures               signatureVerifier
	aggregator               aggregateSigner
	aggregates               aggregateVerifier
	enforceProposerSelection bool
	appHashes                func(types.Height) (types.Hash, error)
	status                   Status
	votes                    map[types.Height]map[types.Round]map[types.Hash]map[types.ValidatorID]Vote
	votedVotes               map[types.Height]map[types.Round]map[types.ValidatorID]Vote
	// evidence is the window of the latest offences recorded, evidenceBase the number recorded
	// before it, and offences the keys of the offences in it.
	evidence        []slashing.Evidence
	evidenceBase    uint64
	offences        map[string]struct{}
	timeouts        *TimeoutCollector
	timeoutCerts    map[types.Round]finality.TimeoutCert
	pacemaker       *Pacemaker
	blockTree       *BlockTree
	viewSlots       map[types.Round]viewSlot
	lockedQC        finality.QuorumCert
	lastVotedHeight types.Height
	lastVotedRound  types.Round
	commitRule      TwoChainCommitRule
	committed       []CommitDecision
	committedSet    map[types.Hash]struct{}
	commitIndex     safety.CommitIndex
	pendingFinal    map[types.Hash]CommitDecision
	floorHeight     types.Height
	floorHash       types.Hash
	verifiedQCs     map[[sha256.Size]byte]struct{}
}

func NewStateMachine(config StateMachineConfig) (*StateMachine, error) {
	if config.ChainID == "" {
		return nil, errors.New("chain id is required")
	}
	if config.ValidatorSet == nil {
		return nil, errors.New("validator set is required")
	}
	if config.Signatures == nil {
		return nil, errors.New("signature verifier is required")
	}
	if config.Aggregator == nil {
		return nil, errors.New("aggregate signer is required")
	}
	if config.AggregateVerifier == nil {
		return nil, errors.New("aggregate verifier is required")
	}
	hashBlock := config.HashBlock
	if hashBlock == nil {
		hashBlock = HashBlock
	}

	machine := &StateMachine{
		chainID:                  config.ChainID,
		validatorSets:            []registeredValidatorSet{{from: finality.FirstValidatorSetHeight, set: config.ValidatorSet}},
		hashBlock:                hashBlock,
		signatures:               config.Signatures,
		aggregator:               config.Aggregator,
		aggregates:               config.AggregateVerifier,
		enforceProposerSelection: config.EnforceProposerSelection,
		appHashes:                config.AppHashes,
		status: Status{
			ChainID:          config.ChainID,
			Phase:            PhasePropose,
			ValidatorSetHash: config.ValidatorSet.Hash(),
		},
		votes:        make(map[types.Height]map[types.Round]map[types.Hash]map[types.ValidatorID]Vote),
		votedVotes:   make(map[types.Height]map[types.Round]map[types.ValidatorID]Vote),
		evidence:     make([]slashing.Evidence, 0),
		offences:     make(map[string]struct{}),
		timeoutCerts: make(map[types.Round]finality.TimeoutCert),
		pacemaker:    NewPacemaker(0, 0),
		blockTree:    NewBlockTree(),
		viewSlots:    make(map[types.Round]viewSlot),
		commitRule:   TwoChainCommitRule{},
		committed:    make([]CommitDecision, 0),
		committedSet: make(map[types.Hash]struct{}),
		commitIndex:  safety.NewCommitIndex(),
		pendingFinal: make(map[types.Hash]CommitDecision),
		verifiedQCs:  make(map[[sha256.Size]byte]struct{}),
	}
	machine.timeouts = newTimeoutCollector(machine.validatorSetForHeight, config.Aggregator)
	return machine, nil
}

// StartRound moves the machine to at least the given height and view. Views are global:
// neither the height nor the view ever moves backwards.
func (machine *StateMachine) StartRound(height types.Height, round types.Round) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	machine.advanceView(height, round)
	machine.status.Phase = PhasePropose
}

// advanceView raises the pacemaker and mirrors it into the status.
func (machine *StateMachine) advanceView(height types.Height, round types.Round) {
	if !machine.pacemaker.Advance(height, round) {
		return
	}
	machine.status.Height = machine.pacemaker.Height()
	machine.status.Round = machine.pacemaker.Round()
	machine.status.Phase = PhasePropose
	if machine.status.Round > timeoutViewRetention {
		machine.timeouts.Prune(machine.status.Round - timeoutViewRetention)
	}
}

// UpdateValidatorSetFromRegistry records the registry's validator sets through registry height
// height: the sets the registry holds after executing every block up to height-1. The node calls
// it once the block at height-1 is executed. The first call replaces the configured set with the
// registry history from height on; later calls fill every registry height up to height, so the
// history never skips a set the registry held. A block is verified with the set of the registry
// height its header names, never with a set chosen by its own height.
func (machine *StateMachine) UpdateValidatorSetFromRegistry(ctx context.Context, registry validator.Registry, height types.Height) error {
	if registry == nil {
		return errors.New("validator registry is required")
	}
	if height == 0 {
		return errors.New("registry height is required")
	}
	machine.mu.Lock()
	first := machine.validatorSetsThrough + 1
	if machine.validatorSetsThrough == 0 {
		first = height
	}
	machine.mu.Unlock()
	if first > height {
		return nil
	}
	sets := make([]registeredValidatorSet, 0, height-first+1)
	for registryHeight := first; registryHeight <= height; registryHeight++ {
		validatorSet, err := registry.ValidatorSet(ctx, registryHeight)
		if err != nil {
			return err
		}
		if validatorSet == nil {
			return errors.New("validator set is required")
		}
		sets = append(sets, registeredValidatorSet{from: registryHeight, set: validatorSet})
	}

	machine.mu.Lock()
	defer machine.mu.Unlock()
	for _, entry := range sets {
		machine.recordValidatorSet(entry.from, entry.set)
	}
	return nil
}

// recordValidatorSet records the set of one registry height. Heights the history already covers
// are executed and final, so the registry holds the same set for them and a repeat is ignored.
func (machine *StateMachine) recordValidatorSet(height types.Height, validatorSet validator.Set) {
	if machine.validatorSetsThrough == 0 {
		machine.validatorSets = []registeredValidatorSet{{from: height, set: validatorSet}}
		machine.validatorSetsThrough = height
		machine.status.ValidatorSetHash = validatorSet.Hash()
		return
	}
	if height != machine.validatorSetsThrough+1 {
		return
	}
	latest := machine.validatorSets[len(machine.validatorSets)-1]
	if latest.set.Hash() != validatorSet.Hash() {
		machine.validatorSets = append(machine.validatorSets, registeredValidatorSet{from: height, set: validatorSet})
		if len(machine.validatorSets) > maxValidatorSetHistory {
			machine.validatorSets = append([]registeredValidatorSet(nil), machine.validatorSets[len(machine.validatorSets)-maxValidatorSetHistory:]...)
		}
	}
	machine.validatorSetsThrough = height
	machine.status.ValidatorSetHash = validatorSet.Hash()
}

// validatorSetAt returns the set the registry holds at registry height. A height above the
// executed history is pending, and one below the kept history is unknown.
func (machine *StateMachine) validatorSetAt(height types.Height) (validator.Set, error) {
	if height == 0 {
		return nil, fmt.Errorf("%w: registry height 0", ErrUnknownValidatorSet)
	}
	if machine.validatorSetsThrough == 0 {
		return machine.validatorSets[0].set, nil
	}
	if height > machine.validatorSetsThrough {
		return nil, fmt.Errorf("%w: registry height %d, executed through %d", ErrValidatorSetPending, height, machine.validatorSetsThrough)
	}
	for index := len(machine.validatorSets) - 1; index >= 0; index-- {
		if machine.validatorSets[index].from <= height {
			return machine.validatorSets[index].set, nil
		}
	}
	return nil, fmt.Errorf("%w: registry height %d", ErrUnknownValidatorSet, height)
}

// ValidatorSetAt returns the validator set of a registry height, the height a block header names.
func (machine *StateMachine) ValidatorSetAt(height types.Height) (validator.Set, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.validatorSetAt(height)
}

// validatorSetForHeight returns the set for a message of height whose block the machine does not
// hold: the set of the highest registry height a block of that height can name, as far as it is
// executed. A block the machine holds is always verified with the set its header names.
func (machine *StateMachine) validatorSetForHeight(height types.Height) validator.Set {
	registryHeight := finality.FirstValidatorSetHeight
	if height > registryHeight {
		registryHeight = height - 1
	}
	if machine.validatorSetsThrough > 0 && registryHeight > machine.validatorSetsThrough {
		registryHeight = machine.validatorSetsThrough
	}
	validatorSet, err := machine.validatorSetAt(registryHeight)
	if err != nil {
		return machine.validatorSets[0].set
	}
	return validatorSet
}

// validatorSetForCert returns the set that verifies a certificate or vote for blockHash at height:
// the set the block header names when the machine holds the block.
func (machine *StateMachine) validatorSetForCert(height types.Height, blockHash types.Hash) (validator.Set, bool, error) {
	node, found := machine.blockTree.Get(blockHash)
	if !found || node.Block.Header.Height != height {
		return machine.validatorSetForHeight(height), false, nil
	}
	validatorSet, err := machine.validatorSetAt(node.Block.Header.ValidatorSetHeight)
	return validatorSet, true, err
}

// chainHeights are the registry height of the validator set a header names and the height of the
// state it commits.
type chainHeights struct {
	set   types.Height
	state types.Height
}

// expectedChainHeights returns the set and state heights the header of a new block must name. They
// derive from the parent header, so the machine must hold the parent.
func (machine *StateMachine) expectedChainHeights(header types.Header) (chainHeights, error) {
	if header.Height <= 1 {
		return chainHeights{set: finality.FirstValidatorSetHeight}, nil
	}
	parent, found := machine.blockTree.Get(header.PreviousBlockHash)
	if !found {
		return chainHeights{}, fmt.Errorf("%w: height %d parent %x", ErrUnknownParent, header.Height, header.PreviousBlockHash)
	}
	if parent.Block.Header.Height+1 != header.Height {
		return chainHeights{}, fmt.Errorf("%w: parent height %d does not precede height %d", ErrInvalidProposal, parent.Block.Header.Height, header.Height)
	}
	return chainHeights{set: finality.ChildValidatorSetHeight(parent.Block.Header), state: finality.ChildStateHeight(parent.Block.Header)}, nil
}

// checkProposalHeader checks the header of a proposal whose signature verified against the rules
// its parent header and the replicated validator sets fix for it: its heights, chain and time
// against the parent the machine holds, its validator set hash against validatorSet, the set of the
// height the chain fixes, and its next validator set hash against nextSet (nil for a block at or
// below the finalized height, whose next set may be pruned). The proposer signed the header, so a
// broken rule is recorded as evidence against it, which every node judges alike.
func (machine *StateMachine) checkProposalHeader(proposal Proposal, validatorSet validator.Set, nextSet validator.Set) error {
	header := proposal.Block.Header
	var parent *types.Header
	if header.Height > 1 {
		if held, found := machine.blockTree.Get(header.PreviousBlockHash); found {
			parentHeader := held.Block.Header
			parent = &parentHeader
		}
	}
	if parent != nil || header.Height <= 1 {
		if fault := ParentLinkageFault(header, parent); fault != nil {
			machine.recordProposalEvidence(NewParentLinkageEvidence(proposal, parent))
			return fault
		}
	} else if header.TimeUnixNano <= 0 {
		return fmt.Errorf("%w: block time %d", ErrInvalidProposal, header.TimeUnixNano)
	}
	if header.ValidatorSetHash != validatorSet.Hash() {
		machine.recordProposalEvidence(NewInvalidProposalHashEvidence(proposal, InvalidProposalReasonValidatorSetHash, validatorSet.Hash()))
		return fmt.Errorf("%w: validator set hash mismatch", ErrInvalidProposal)
	}
	if nextSet != nil && header.NextValidatorSetHash != nextSet.Hash() {
		machine.recordProposalEvidence(NewInvalidProposalHashEvidence(proposal, InvalidProposalReasonNextValidatorSetHash, nextSet.Hash()))
		return fmt.Errorf("%w: next validator set hash mismatch", ErrInvalidProposal)
	}
	return nil
}

// maxRetainedEvidence bounds the offences a machine keeps for its readers. Every offence is signed
// by its offender, so a faulty validator could otherwise grow the window without end; the node
// publishes each offence as it is recorded, well before it leaves the window.
const maxRetainedEvidence = 1024

// recordProposalEvidence records evidence against a proposer.
func (machine *StateMachine) recordProposalEvidence(evidence slashing.Evidence, err error) {
	if err != nil {
		return
	}
	machine.recordEvidence(evidence)
}

// recordEvidence records an offence once: an offender retransmits its messages, peers relay them,
// and the same fault is one offence.
func (machine *StateMachine) recordEvidence(evidence slashing.Evidence) {
	offence, err := slashing.OffenceKey(evidence)
	if err != nil {
		return
	}
	if _, recorded := machine.offences[offence]; recorded {
		return
	}
	if len(machine.evidence) == maxRetainedEvidence {
		if oldest, err := slashing.OffenceKey(machine.evidence[0]); err == nil {
			delete(machine.offences, oldest)
		}
		machine.evidence[0] = slashing.Evidence{}
		machine.evidence = machine.evidence[1:]
		machine.evidenceBase++
	}
	machine.offences[offence] = struct{}{}
	machine.evidence = append(machine.evidence, evidence)
}

// parentTime returns the time of the parent of the block with header, zero for the first block or
// a parent the machine does not hold. Block time is strictly increasing along the chain: a block's
// time is after its parent's, so the time of a certified header bounds the time of every block
// below it.
func (machine *StateMachine) parentTime(header types.Header) int64 {
	if header.Height <= 1 {
		return 0
	}
	parent, found := machine.blockTree.Get(header.PreviousBlockHash)
	if !found {
		return 0
	}
	return parent.Block.Header.TimeUnixNano
}

func (machine *StateMachine) currentValidatorSet() validator.Set {
	return machine.validatorSets[len(machine.validatorSets)-1].set
}

// ProposalValidatorSet returns the validator set of the next block the machine would propose: the
// child of the highest certified block. It is the set the proposer of the view is scheduled from.
func (machine *StateMachine) ProposalValidatorSet() (validator.Set, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	highQC := machine.blockTree.HighQC()
	header := types.Header{Height: highQC.Height + 1, PreviousBlockHash: highQC.BlockHash}
	if highQC.Height < machine.floorHeight {
		header = types.Header{Height: machine.floorHeight + 1, PreviousBlockHash: machine.floorHash}
	}
	heights, err := machine.expectedChainHeights(header)
	if err != nil {
		return nil, err
	}
	return machine.validatorSetAt(heights.set)
}

// CreateProposal builds the proposal for a view. A block above height 1 must extend a
// justify QC (the highest known QC when none is given), and a view that does not directly
// follow the QC's view carries the timeout certificate of the previous view.
func (machine *StateMachine) CreateProposal(block types.Block, round types.Round, proposer types.ValidatorID, justifyQC finality.QuorumCert) (Proposal, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if justifyQC.Height == 0 {
		justifyQC = machine.blockTree.HighQC()
	}
	expectedHeight := justifyQC.Height + 1
	if block.Header.Height == 0 {
		block.Header.Height = expectedHeight
	}
	if block.Header.Height != expectedHeight {
		return Proposal{}, fmt.Errorf("%w: height %d does not extend justify qc height %d", ErrInvalidProposal, block.Header.Height, justifyQC.Height)
	}
	if block.Header.PreviousBlockHash == (types.Hash{}) && justifyQC.Height > 0 {
		block.Header.PreviousBlockHash = justifyQC.BlockHash
	}
	if block.Header.PreviousBlockHash != justifyQC.BlockHash {
		return Proposal{}, fmt.Errorf("%w: justify qc must match parent block", ErrInvalidProposal)
	}
	block.Header.Round = round
	block.Header.JustifyRound = justifyQC.Round
	if parentTime := machine.parentTime(block.Header); block.Header.TimeUnixNano <= parentTime {
		block.Header.TimeUnixNano = parentTime + 1
	}
	heights, err := machine.expectedChainHeights(block.Header)
	if err != nil {
		return Proposal{}, err
	}
	block.Header.ValidatorSetHeight = heights.set
	block.Header.StateHeight = heights.state
	validatorSet, err := machine.validatorSetAt(heights.set)
	if err != nil {
		return Proposal{}, err
	}
	nextSet, err := machine.validatorSetAt(heights.state + 1)
	if err != nil {
		return Proposal{}, err
	}
	block.Header.NextValidatorSetHash = nextSet.Hash()
	if _, found := validatorSet.Get(proposer); !found {
		return Proposal{}, ErrUnknownValidator
	}
	if err := machine.validateProposer(validatorSet, block.Header.Height, round, proposer); err != nil {
		return Proposal{}, err
	}

	var timeoutCert finality.TimeoutCert
	direct := directRound(justifyQC)
	if round < direct {
		return Proposal{}, fmt.Errorf("%w: view %d does not follow justify qc view %d", ErrInvalidProposal, round, justifyQC.Round)
	}
	if round > direct {
		var found bool
		timeoutCert, found = machine.timeoutCerts[round-1]
		if !found {
			return Proposal{}, fmt.Errorf("%w: view %d needs a timeout certificate for view %d", ErrInvalidTimeoutCert, round, round-1)
		}
	}

	block.Header.ChainID = machine.chainID
	block.Header.ValidatorSetHash = validatorSet.Hash()
	if machine.appHashes != nil {
		appHashHeight, _ := finality.AppHashHeight(block.Header)
		appHash, err := machine.appHashes(appHashHeight)
		if err != nil {
			return Proposal{}, fmt.Errorf("app hash of height %d: %w", appHashHeight, err)
		}
		block.Header.AppHash = appHash
	}
	block.Txs = fairordering.SortTxsWithSalt(block.Txs, machine.orderingSalt(block.Header.Height))
	block.Header.TxRoot = TxRoot(block.Txs)
	if block.Header.ConsensusHash == (types.Hash{}) && len(block.Txs) > 0 {
		block = dataavailability.AttachCommitment(block)
	}

	return Proposal{
		Block:       block,
		Round:       round,
		Proposer:    proposer,
		JustifyQC:   justifyQC,
		TimeoutCert: timeoutCert,
	}, nil
}

// OnProposal validates a proposal completely before it touches any state: structure and
// view rules, the proposer signature, the block body, the justify QC, and the timeout
// certificate. A nil return means the proposal is safe to vote for. ErrStaleProposal and
// ErrUnsafeProposal are returned only for fully verified proposals.
func (machine *StateMachine) OnProposal(ctx context.Context, proposal Proposal) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	machine.mu.Lock()
	defer machine.mu.Unlock()

	header := proposal.Block.Header
	if header.Height == 0 {
		return fmt.Errorf("%w: missing height", ErrInvalidProposal)
	}
	if header.Height > maxChainHeight || proposal.Round > maxChainView {
		return fmt.Errorf("%w: height or view out of range", ErrInvalidProposal)
	}
	if header.ChainID != machine.chainID {
		return fmt.Errorf("%w: chain id mismatch: %s", ErrInvalidProposal, header.ChainID)
	}
	if size := BlockTxBytes(proposal.Block); size > MaxBlockBytes {
		return fmt.Errorf("%w: the block carries %d transaction bytes, more than %d", ErrInvalidProposal, size, MaxBlockBytes)
	}
	if header.Round != proposal.Round || header.JustifyRound != proposal.JustifyQC.Round {
		return fmt.Errorf("%w: header views %d/%d do not match the proposal views %d/%d", ErrInvalidProposal, header.Round, header.JustifyRound, proposal.Round, proposal.JustifyQC.Round)
	}
	if err := machine.checkProposalChain(proposal); err != nil {
		return err
	}
	if header.Height > machine.floorHeight+maxHeightWindow {
		return fmt.Errorf("%w: proposal height %d", ErrOutOfWindow, header.Height)
	}
	heights, err := machine.proposalChainHeights(proposal)
	if err != nil {
		return err
	}
	validatorSet, err := machine.validatorSetAt(heights.set)
	if errors.Is(err, ErrValidatorSetPending) {
		return machine.observePendingJustify(proposal, err)
	}
	if err != nil {
		return err
	}
	var nextSet validator.Set
	if header.Height > machine.floorHeight {
		nextSet, err = machine.validatorSetAt(heights.state + 1)
		if errors.Is(err, ErrValidatorSetPending) {
			return machine.observePendingJustify(proposal, err)
		}
		if err != nil {
			return err
		}
	}
	proposerInfo, found := validatorSet.Get(proposal.Proposer)
	if !found {
		return ErrUnknownValidator
	}
	if err := machine.validateProposer(validatorSet, header.Height, proposal.Round, proposal.Proposer); err != nil {
		return err
	}
	if err := machine.verifyProposalSignature(proposal, proposerInfo); err != nil {
		return err
	}
	if err := machine.checkProposalHeader(proposal, validatorSet, nextSet); err != nil {
		return err
	}
	if err := VerifyBlockBody(proposal.Block); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidProposal, err)
	}
	if proposal.JustifyQC.Height > 0 {
		if err := machine.verifyQuorumCert(proposal.JustifyQC); err != nil {
			return err
		}
	}
	hasTimeoutCert := !isZeroTimeoutCert(proposal.TimeoutCert)
	if hasTimeoutCert {
		if err := machine.verifyTimeoutCert(proposal.TimeoutCert); err != nil {
			return err
		}
		if isBetterQC(proposal.TimeoutCert.HighQC, proposal.JustifyQC) {
			return fmt.Errorf("%w: justify qc is older than the timeout certificate high qc", ErrInvalidProposal)
		}
	}

	blockHash := machine.hashBlock(proposal.Block)
	if slot, taken := machine.viewSlots[proposal.Round]; taken && slot.blockHash != blockHash {
		return fmt.Errorf("%w: view=%d", ErrConflictingProposal, proposal.Round)
	}
	if header.Height < machine.floorHeight {
		return ErrStaleProposal
	}
	if header.Height == machine.floorHeight {
		if machine.floorHash != (types.Hash{}) && blockHash != machine.floorHash {
			return fmt.Errorf("%w: conflicts with the finalized block at height %d", ErrUnsafeProposal, header.Height)
		}
		return ErrStaleProposal
	}

	machine.blockTree.Insert(proposal.Block, blockHash, proposal.JustifyQC)
	machine.viewSlots[proposal.Round] = viewSlot{blockHash: blockHash, height: header.Height}
	if err := machine.observeQuorumCert(proposal.JustifyQC); err != nil {
		return err
	}
	if err := machine.evaluateCommit(blockHash); err != nil {
		return err
	}
	if hasTimeoutCert {
		machine.recordTimeoutCert(proposal.TimeoutCert)
		machine.advanceView(0, proposal.TimeoutCert.Round+1)
	}

	if proposal.Round < machine.status.Round {
		return ErrStaleProposal
	}
	if machine.lastVotedHeight > 0 && proposal.Round <= machine.lastVotedRound {
		return ErrStaleProposal
	}
	if !machine.isSafeProposal(proposal) {
		return ErrUnsafeProposal
	}

	machine.updateLockedQC(proposal.JustifyQC)
	machine.lastVotedHeight = header.Height
	machine.lastVotedRound = proposal.Round
	machine.advanceView(header.Height, proposal.Round)
	machine.status.Phase = PhaseVote
	return nil
}

// checkProposalChain checks the cheap structural rules that tie a proposal to its justify
// QC and to the view sequence, before any signature work.
func (machine *StateMachine) checkProposalChain(proposal Proposal) error {
	header := proposal.Block.Header
	justifyQC := proposal.JustifyQC
	if justifyQC.Height == 0 {
		if !isZeroQC(justifyQC) {
			return fmt.Errorf("%w: malformed justify qc", ErrInvalidProposal)
		}
		if header.Height != 1 {
			return fmt.Errorf("%w: height %d requires a justify qc", ErrInvalidProposal, header.Height)
		}
	} else {
		if justifyQC.Round > maxChainView || justifyQC.Height > maxChainHeight {
			return fmt.Errorf("%w: justify qc out of range", ErrInvalidProposal)
		}
		if justifyQC.Height+1 != header.Height {
			return fmt.Errorf("%w: height %d does not extend justify qc height %d", ErrInvalidProposal, header.Height, justifyQC.Height)
		}
		if justifyQC.BlockHash != header.PreviousBlockHash {
			return fmt.Errorf("%w: justify qc must match parent block", ErrInvalidProposal)
		}
	}
	direct := directRound(justifyQC)
	if proposal.Round < direct {
		return fmt.Errorf("%w: view %d does not follow justify qc view %d", ErrInvalidProposal, proposal.Round, justifyQC.Round)
	}
	hasTimeoutCert := !isZeroTimeoutCert(proposal.TimeoutCert)
	if proposal.Round == direct {
		if hasTimeoutCert {
			return fmt.Errorf("%w: unexpected timeout certificate for view %d", ErrInvalidTimeoutCert, proposal.Round)
		}
		return nil
	}
	if !hasTimeoutCert {
		return fmt.Errorf("%w: view %d needs a timeout certificate for view %d", ErrInvalidTimeoutCert, proposal.Round, proposal.Round-1)
	}
	if proposal.TimeoutCert.Round != proposal.Round-1 {
		return fmt.Errorf("%w: certificate is for view %d, not view %d", ErrInvalidTimeoutCert, proposal.TimeoutCert.Round, proposal.Round-1)
	}
	return nil
}

// proposalChainHeights returns the set and state heights the proposal's block must name. A block
// the machine already holds keeps the height it was accepted with: its parent may be pruned once
// it is finalized. A block at or below the finalized height can only be stale or conflict with the
// finalized chain; it is authenticated against the kept set its header names, and is never
// inserted. Above the finalized height the parent must be known; a proposal whose parent is not
// has its justify certificate checked first, so a forged certificate is reported as such.
func (machine *StateMachine) proposalChainHeights(proposal Proposal) (chainHeights, error) {
	header := proposal.Block.Header
	heights, err := machine.expectedChainHeights(header)
	if !errors.Is(err, ErrUnknownParent) {
		return heights, err
	}
	if held, found := machine.blockTree.Get(machine.hashBlock(proposal.Block)); found {
		return chainHeights{set: held.Block.Header.ValidatorSetHeight, state: held.Block.Header.StateHeight}, nil
	}
	if header.Height <= machine.floorHeight {
		return chainHeights{set: header.ValidatorSetHeight, state: header.StateHeight}, nil
	}
	if proposal.JustifyQC.Height > 0 {
		if qcErr := machine.verifyQuorumCert(proposal.JustifyQC); qcErr != nil {
			return chainHeights{}, qcErr
		}
	}
	return chainHeights{}, err
}

// observePendingJustify handles a proposal whose validator set is the set after a block this node
// has not executed. The justify certificate is verified with the set of the parent and observed:
// it can be exactly what finalizes the block the set depends on. The proposal itself is judged
// once that block is executed.
func (machine *StateMachine) observePendingJustify(proposal Proposal, pending error) error {
	if proposal.JustifyQC.Height == 0 {
		return pending
	}
	if err := machine.verifyQuorumCert(proposal.JustifyQC); err != nil {
		return err
	}
	if err := machine.observeQuorumCert(proposal.JustifyQC); err != nil {
		return err
	}
	return pending
}

func (machine *StateMachine) validateProposer(validatorSet validator.Set, height types.Height, round types.Round, proposer types.ValidatorID) error {
	if !machine.enforceProposerSelection {
		return nil
	}
	expected, ok := validator.SelectProposer(validatorSet.List(), round)
	if !ok {
		return fmt.Errorf("%w: no active validators", ErrUnexpectedProposer)
	}
	if expected != proposer {
		return fmt.Errorf("%w: height=%d round=%d expected=%s actual=%s", ErrUnexpectedProposer, height, round, expected, proposer)
	}
	return nil
}

func (machine *StateMachine) orderingSalt(height types.Height) []byte {
	return fairordering.HeightSalt(machine.chainID, height)
}

// OnVote records a signed vote. A vote counts only after its signature verifies; a second
// vote of the same validator for another block in the same view is equivocation and is
// kept as evidence.
func (machine *StateMachine) OnVote(ctx context.Context, vote Vote) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	machine.mu.Lock()
	defer machine.mu.Unlock()

	validatorSet, _, err := machine.validatorSetForCert(vote.Height, vote.BlockHash)
	if err != nil {
		return err
	}
	validatorInfo, found := validatorSet.Get(vote.ValidatorID)
	if !found {
		return ErrUnknownValidator
	}
	if vote.Height == 0 {
		return fmt.Errorf("%w: missing height", ErrInvalidVote)
	}
	if vote.BlockHash == (types.Hash{}) {
		return fmt.Errorf("%w: missing block hash", ErrInvalidVote)
	}
	if err := machine.verifyVoteSignature(vote, validatorInfo); err != nil {
		return err
	}
	if previous, found := machine.votedVote(vote); found && previous.BlockHash != vote.BlockHash {
		if evidence, err := NewConflictingVoteEvidence(previous, vote); err == nil {
			machine.recordEvidence(evidence)
		}
		return ErrConflictingVote
	}
	if vote.Round < machine.status.Round || vote.Height <= machine.floorHeight {
		return ErrStaleVote
	}
	if vote.Round > machine.status.Round+maxViewLookahead || vote.Height > machine.floorHeight+maxHeightWindow {
		return fmt.Errorf("%w: vote height=%d round=%d", ErrOutOfWindow, vote.Height, vote.Round)
	}
	node, found := machine.blockTree.Get(vote.BlockHash)
	if !found {
		return fmt.Errorf("%w: %w", ErrInvalidVote, ErrUnknownVoteBlock)
	}
	if node.Block.Header.Height != vote.Height {
		return fmt.Errorf("%w: target height mismatch", ErrInvalidVote)
	}
	if node.Block.Header.Round != vote.Round {
		return fmt.Errorf("%w: block is proposed in view %d, not view %d", ErrInvalidVote, node.Block.Header.Round, vote.Round)
	}
	if !machine.isSafeVoteTarget(node) {
		return ErrUnsafeVote
	}

	machine.recordVote(vote)
	qc, err := machine.buildQuorumCert(vote.Height, vote.Round, vote.BlockHash)
	if err != nil {
		// The vote is recorded; BuildQuorumCert reports why no certificate exists yet.
		return nil
	}
	if err := machine.blockTree.SetQuorumCert(qc); err != nil && !errors.Is(err, ErrBlockNotFound) {
		return err
	}
	if err := machine.observeQuorumCert(qc); err != nil {
		return err
	}
	machine.status.Phase = PhaseCommit
	return nil
}

// OnTimeoutVote records a signed timeout vote whose high QC verifies. When the votes of
// the view reach quorum the timeout certificate is returned and the machine leaves the
// view.
func (machine *StateMachine) OnTimeoutVote(ctx context.Context, vote TimeoutVote) (finality.TimeoutCert, error) {
	select {
	case <-ctx.Done():
		return finality.TimeoutCert{}, ctx.Err()
	default:
	}
	machine.mu.Lock()
	defer machine.mu.Unlock()

	validatorInfo, found := machine.validatorSetForHeight(vote.Height).Get(vote.ValidatorID)
	if !found {
		return finality.TimeoutCert{}, ErrUnknownValidator
	}
	if vote.Height == 0 || vote.Height > maxChainHeight || vote.Round > maxChainView {
		return finality.TimeoutCert{}, fmt.Errorf("%w: timeout vote height or view out of range", ErrInvalidVote)
	}
	if vote.HighQC.Height > 0 && vote.HighQC.Round >= vote.Round {
		return finality.TimeoutCert{}, fmt.Errorf("%w: high qc view %d is not below timeout view %d", ErrInvalidVote, vote.HighQC.Round, vote.Round)
	}
	if err := machine.verifyTimeoutVoteSignature(vote, validatorInfo); err != nil {
		return finality.TimeoutCert{}, err
	}
	if err := machine.verifyOptionalQuorumCert(vote.HighQC); err != nil {
		return finality.TimeoutCert{}, err
	}
	previous, conflicting := machine.timeouts.ConflictingVote(vote)
	if conflicting {
		if evidence, err := NewConflictingTimeoutVoteEvidence(previous, vote); err == nil {
			machine.recordEvidence(evidence)
		}
		return finality.TimeoutCert{}, ErrConflictingTimeoutVote
	}
	if vote.Round < machine.status.Round {
		return finality.TimeoutCert{}, ErrStaleTimeoutCert
	}
	if vote.Round > machine.status.Round+maxViewLookahead ||
		vote.Height > machine.blockTree.HighQC().Height+1+maxHeightWindow {
		return finality.TimeoutCert{}, fmt.Errorf("%w: timeout vote height=%d round=%d", ErrOutOfWindow, vote.Height, vote.Round)
	}
	if err := machine.timeouts.AddVote(vote); err != nil {
		return finality.TimeoutCert{}, err
	}
	if err := machine.observeQuorumCert(vote.HighQC); err != nil {
		return finality.TimeoutCert{}, err
	}

	timeoutCert, err := machine.timeouts.BuildTimeoutCert(vote.Round)
	if err != nil {
		return finality.TimeoutCert{}, err
	}
	if err := machine.verifyTimeoutCert(timeoutCert); err != nil {
		return finality.TimeoutCert{}, err
	}
	if err := machine.pacemaker.AdvanceRound(timeoutCert); err != nil {
		return finality.TimeoutCert{}, err
	}
	machine.recordTimeoutCert(timeoutCert)
	machine.status.Height = machine.pacemaker.Height()
	machine.status.Round = machine.pacemaker.Round()
	machine.status.Phase = PhasePropose
	machine.status.LastTimeoutCert = timeoutCert
	if machine.status.Round > timeoutViewRetention {
		machine.timeouts.Prune(machine.status.Round - timeoutViewRetention)
	}
	return timeoutCert, nil
}

// recordTimeoutCert keeps a verified timeout certificate so a proposal for the next view
// can carry it.
func (machine *StateMachine) recordTimeoutCert(timeoutCert finality.TimeoutCert) {
	machine.timeoutCerts[timeoutCert.Round] = timeoutCert
	if machine.status.Round > timeoutViewRetention {
		for round := range machine.timeoutCerts {
			if round+timeoutViewRetention < machine.status.Round {
				delete(machine.timeoutCerts, round)
			}
		}
	}
}

func (machine *StateMachine) BuildQuorumCert(height types.Height, round types.Round, blockHash types.Hash) (finality.QuorumCert, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.buildQuorumCert(height, round, blockHash)
}

func (machine *StateMachine) HighQC(ctx context.Context) finality.QuorumCert {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if ctx == nil {
		return machine.blockTree.HighQC()
	}
	select {
	case <-ctx.Done():
		return finality.QuorumCert{}
	default:
		return machine.blockTree.HighQC()
	}
}

// VerifyQuorumCert reports whether qc is a valid certificate: unique signers known in the
// validator set of the certified height, quorum voting power, and a valid aggregate
// signature over the vote sign bytes.
func (machine *StateMachine) VerifyQuorumCert(qc finality.QuorumCert) error {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.verifyQuorumCert(qc)
}

// VerifyTimeoutCert reports whether timeoutCert proves that a quorum timed out in its view.
func (machine *StateMachine) VerifyTimeoutCert(timeoutCert finality.TimeoutCert) error {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.verifyTimeoutCert(timeoutCert)
}

// ObserveCommittedBlock records a block the node has executed as committed. The block body
// and the QC are verified, the finalized height advances to the block, and everything below
// it is pruned.
func (machine *StateMachine) ObserveCommittedBlock(block types.Block, quorumCert finality.QuorumCert) error {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	blockHash := machine.hashBlock(block)
	if quorumCert.Height != block.Header.Height || quorumCert.BlockHash != blockHash {
		return ErrInvalidBlockCert
	}
	if err := VerifyBlockBody(block); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBlockCert, err)
	}
	if quorumCert.Round != block.Header.Round {
		return fmt.Errorf("%w: block is proposed in view %d, not view %d", ErrInvalidBlockCert, block.Header.Round, quorumCert.Round)
	}
	validatorSet, err := machine.validatorSetAt(block.Header.ValidatorSetHeight)
	if err != nil {
		return err
	}
	if block.Header.ValidatorSetHash != validatorSet.Hash() {
		return fmt.Errorf("%w: validator set hash mismatch", ErrInvalidBlockCert)
	}
	if err := machine.verifyQuorumCertWithSet(quorumCert, validatorSet); err != nil {
		return err
	}
	height := block.Header.Height
	if height < machine.floorHeight {
		machine.pruneDecisions(height)
		return nil
	}
	if err := machine.commitIndex.Record(height, blockHash); err != nil {
		return err
	}
	machine.blockTree.Insert(block, blockHash, finality.QuorumCert{})
	if err := machine.blockTree.SetQuorumCert(quorumCert); err != nil {
		return err
	}
	if err := machine.observeQuorumCert(quorumCert); err != nil {
		return err
	}
	machine.updateLockedQC(quorumCert)
	if height > machine.floorHeight {
		machine.setFloor(height, blockHash)
	}
	machine.pruneDecisions(height)
	return machine.emitFinalized()
}

// AnchorCommittedBlock records a block the node committed and executed, but whose certificate it
// does not hold, as the finalized root. The machine needs the root block itself: the validator set
// of its children derives from its header. The body was judged when the block was committed.
func (machine *StateMachine) AnchorCommittedBlock(block types.Block) error {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	height := block.Header.Height
	if height == 0 || height < machine.floorHeight {
		return nil
	}
	blockHash := machine.hashBlock(block)
	if height == machine.floorHeight && machine.floorHash != (types.Hash{}) && machine.floorHash != blockHash {
		return ErrConflictingCommit
	}
	if err := machine.commitIndex.Record(height, blockHash); err != nil {
		return err
	}
	machine.blockTree.Insert(block, blockHash, finality.QuorumCert{})
	if height > machine.floorHeight {
		machine.setFloor(height, blockHash)
	} else {
		machine.floorHash = blockHash
	}
	machine.pruneDecisions(height)
	return machine.emitFinalized()
}

// Prune discards consensus state strictly below committedHeight. The block at
// committedHeight stays as the root of the tree. It is idempotent and never lowers the
// finalized height.
func (machine *StateMachine) Prune(committedHeight types.Height) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if committedHeight > machine.floorHeight {
		blockHash, _ := machine.commitIndex.Get(committedHeight)
		machine.setFloor(committedHeight, blockHash)
	}
	machine.pruneDecisions(committedHeight)
}

// SafetyState returns the state a validator must persist before it signs a vote.
func (machine *StateMachine) SafetyState() SafetyState {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return SafetyState{
		LockedQC:        machine.lockedQC,
		HighQC:          machine.blockTree.HighQC(),
		LastVotedHeight: machine.lastVotedHeight,
		LastVotedRound:  machine.lastVotedRound,
	}
}

// RestoreSafetyState reinstates a persisted safety state after a restart. Both
// certificates are verified again, and the state only ever raises the machine: the lock,
// the highest QC and the last voted view never move backwards.
func (machine *StateMachine) RestoreSafetyState(state SafetyState) error {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if state.LastVotedHeight == 0 && state.LastVotedRound != 0 {
		return fmt.Errorf("%w: last voted view without a height", ErrInvalidSafetyState)
	}
	if err := machine.verifyOptionalQuorumCert(state.LockedQC); err != nil {
		return err
	}
	if err := machine.verifyOptionalQuorumCert(state.HighQC); err != nil {
		return err
	}
	if err := machine.observeQuorumCert(state.LockedQC); err != nil {
		return err
	}
	machine.updateLockedQC(state.LockedQC)
	if err := machine.observeQuorumCert(state.HighQC); err != nil {
		return err
	}
	if state.LastVotedHeight > 0 && (machine.lastVotedHeight == 0 || state.LastVotedRound > machine.lastVotedRound) {
		machine.lastVotedHeight = state.LastVotedHeight
		machine.lastVotedRound = state.LastVotedRound
	}
	machine.advanceView(machine.lastVotedHeight, machine.lastVotedRound)
	return nil
}

func (machine *StateMachine) Status(ctx context.Context) Status {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if ctx == nil {
		return machine.status
	}
	select {
	case <-ctx.Done():
		return machine.status
	default:
		return machine.status
	}
}

// Evidence returns the offences the machine keeps, oldest first.
func (machine *StateMachine) Evidence() []slashing.Evidence {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return append([]slashing.Evidence(nil), machine.evidence...)
}

// EvidenceCursor returns the number of offences recorded so far.
func (machine *StateMachine) EvidenceCursor() uint64 {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.evidenceBase + uint64(len(machine.evidence))
}

// EvidenceSince returns the offences recorded from cursor on that the machine still keeps, and
// the cursor after them.
func (machine *StateMachine) EvidenceSince(cursor uint64) ([]slashing.Evidence, uint64) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	next := machine.evidenceBase + uint64(len(machine.evidence))
	if cursor >= next {
		return nil, next
	}
	start := uint64(0)
	if cursor > machine.evidenceBase {
		start = cursor - machine.evidenceBase
	}
	return append([]slashing.Evidence(nil), machine.evidence[start:]...), next
}

func (machine *StateMachine) CommitDecisions() []CommitDecision {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return append([]CommitDecision(nil), machine.committed...)
}

// ApplyCommitRule evaluates a commit candidate. Both certificates are verified first, so a
// caller cannot finalize a block with certificates the validator set never signed.
func (machine *StateMachine) ApplyCommitRule(candidate CommitCandidate) (CommitDecision, error) {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	if err := machine.verifyQuorumCert(candidate.BlockQC); err != nil {
		return CommitDecision{}, err
	}
	if err := machine.verifyQuorumCert(candidate.ChildQC); err != nil {
		return CommitDecision{}, err
	}
	return machine.applyCommitRule(candidate)
}

func (machine *StateMachine) IsSafeProposal(proposal Proposal) bool {
	machine.mu.Lock()
	defer machine.mu.Unlock()
	return machine.isSafeProposal(proposal)
}

// IsAncestorOfHighQC reports whether blockHash is on the certified branch.
// Certified ancestors must remain available until the 2-chain commit rule
// finalizes them, even after the state machine advances to a higher height.
func (machine *StateMachine) IsAncestorOfHighQC(blockHash types.Hash) bool {
	machine.mu.Lock()
	defer machine.mu.Unlock()
	highQC := machine.blockTree.HighQC()
	if highQC.BlockHash == (types.Hash{}) || blockHash == (types.Hash{}) {
		return false
	}
	return blockHash == highQC.BlockHash || machine.blockTree.Extends(highQC.BlockHash, blockHash)
}

// isSafeProposal is the voting rule: the justify QC must be at least as recent, by view,
// as the locked QC.
func (machine *StateMachine) isSafeProposal(proposal Proposal) bool {
	return isAtLeastAsRecent(proposal.JustifyQC, machine.lockedQC)
}

func (machine *StateMachine) isSafeVoteTarget(node BlockNode) bool {
	return isAtLeastAsRecent(node.JustifyQC, machine.lockedQC)
}

// isAtLeastAsRecent reports whether justifyQC does not predate lockedQC by view. The empty
// certificate of a genesis proposal predates every real QC.
func isAtLeastAsRecent(justifyQC finality.QuorumCert, lockedQC finality.QuorumCert) bool {
	if lockedQC.Height == 0 {
		return true
	}
	if justifyQC.Height == 0 {
		return false
	}
	return justifyQC.Round >= lockedQC.Round
}

func (machine *StateMachine) updateLockedQC(qc finality.QuorumCert) {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return
	}
	if isBetterQC(qc, machine.lockedQC) {
		machine.lockedQC = qc
	}
}

// observeQuorumCert feeds a verified QC into the tree and the pacemaker: seeing the QC of a
// view moves the machine to the next height and view.
func (machine *StateMachine) observeQuorumCert(qc finality.QuorumCert) error {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return nil
	}
	machine.blockTree.ObserveQuorumCert(qc)
	machine.advanceView(qc.Height+1, qc.Round+1)
	return machine.evaluateCommit(qc.BlockHash)
}

// evaluateCommit applies the commit rule to the pair formed by the block and its justify QC.
func (machine *StateMachine) evaluateCommit(childHash types.Hash) error {
	candidate, found := machine.blockTree.CommitCandidate(childHash)
	if !found {
		return nil
	}
	if _, err := machine.applyCommitRule(candidate); err != nil && !errors.Is(err, ErrCommitRuleNotSatisfied) {
		return err
	}
	return nil
}

func (machine *StateMachine) applyCommitRule(candidate CommitCandidate) (CommitDecision, error) {
	decision, err := machine.commitRule.Decide(candidate)
	if err != nil {
		return CommitDecision{}, err
	}
	if decision.CommittedHeight < machine.floorHeight {
		return decision, nil
	}
	if decision.CommittedHeight == machine.floorHeight {
		if machine.floorHash != (types.Hash{}) && decision.CommittedBlockHash != machine.floorHash {
			return CommitDecision{}, ErrConflictingCommit
		}
		return decision, nil
	}
	if _, found := machine.committedSet[decision.CommittedBlockHash]; found {
		return decision, nil
	}
	machine.pendingFinal[decision.CommittedBlockHash] = decision
	if err := machine.emitFinalized(); err != nil {
		return CommitDecision{}, err
	}
	return decision, nil
}

// emitFinalized publishes every pending finalized block whose chain of ancestors down to
// the finalized root is present, lowest height first.
func (machine *StateMachine) emitFinalized() error {
	tips := make([]CommitDecision, 0, len(machine.pendingFinal))
	for _, decision := range machine.pendingFinal {
		tips = append(tips, decision)
	}
	sort.Slice(tips, func(first, second int) bool {
		if tips[first].CommittedHeight != tips[second].CommittedHeight {
			return tips[first].CommittedHeight < tips[second].CommittedHeight
		}
		return bytes.Compare(tips[first].CommittedBlockHash[:], tips[second].CommittedBlockHash[:]) < 0
	})
	for _, tip := range tips {
		if tip.CommittedHeight <= machine.floorHeight {
			delete(machine.pendingFinal, tip.CommittedBlockHash)
			continue
		}
		complete, err := machine.emitChain(tip)
		if err != nil {
			return err
		}
		if complete {
			delete(machine.pendingFinal, tip.CommittedBlockHash)
		}
	}
	return nil
}

// emitChain finalizes the tip and every ancestor above the finalized root. It reports
// false, without changing anything, while an ancestor block is still missing.
func (machine *StateMachine) emitChain(tip CommitDecision) (bool, error) {
	type chainStep struct {
		node BlockNode
		qc   finality.QuorumCert
	}
	var path []chainStep
	currentHash := tip.CommittedBlockHash
	currentQC := tip.CommitQC
	expectedHeight := tip.CommittedHeight
	for {
		node, found := machine.blockTree.Get(currentHash)
		if !found {
			return false, nil
		}
		height := node.Block.Header.Height
		if height != expectedHeight {
			return false, nil
		}
		if height <= machine.floorHeight {
			if height == machine.floorHeight && machine.floorHash != (types.Hash{}) && currentHash != machine.floorHash {
				return false, ErrConflictingCommit
			}
			break
		}
		path = append(path, chainStep{node: node, qc: currentQC})
		if height == 1 {
			break
		}
		if node.JustifyQC.Height == 0 || node.JustifyQC.BlockHash != node.ParentHash {
			return false, nil
		}
		currentHash = node.ParentHash
		currentQC = node.JustifyQC
		expectedHeight = height - 1
	}
	if len(path) == 0 {
		return true, nil
	}
	for index := len(path) - 1; index >= 0; index-- {
		step := path[index]
		height := step.node.Block.Header.Height
		if err := machine.commitIndex.Record(height, step.node.Hash); err != nil {
			return false, err
		}
		machine.committed = append(machine.committed, CommitDecision{
			CommittedHeight:    height,
			CommittedBlockHash: step.node.Hash,
			CommitQC:           step.qc,
		})
		machine.committedSet[step.node.Hash] = struct{}{}
		machine.status.LastFinalized = step.node.Hash
	}
	machine.setFloor(path[0].node.Block.Header.Height, path[0].node.Hash)
	return true, nil
}

// setFloor advances the finalized root and drops all state below it.
func (machine *StateMachine) setFloor(height types.Height, blockHash types.Hash) {
	if height <= machine.floorHeight {
		return
	}
	machine.floorHeight = height
	machine.floorHash = blockHash
	machine.blockTree.Prune(height)
	machine.commitIndex.Prune(height)
	for voteHeight := range machine.votes {
		if voteHeight < height {
			delete(machine.votes, voteHeight)
		}
	}
	for voteHeight := range machine.votedVotes {
		if voteHeight < height {
			delete(machine.votedVotes, voteHeight)
		}
	}
	for round, slot := range machine.viewSlots {
		if slot.height < height {
			delete(machine.viewSlots, round)
		}
	}
	for hash, decision := range machine.pendingFinal {
		if decision.CommittedHeight <= height {
			delete(machine.pendingFinal, hash)
		}
	}
}

// pruneDecisions drops published commit decisions below height; the node has executed them.
func (machine *StateMachine) pruneDecisions(height types.Height) {
	kept := machine.committed[:0]
	for _, decision := range machine.committed {
		if decision.CommittedHeight < height {
			delete(machine.committedSet, decision.CommittedBlockHash)
			continue
		}
		kept = append(kept, decision)
	}
	machine.committed = kept
}

func directRound(justifyQC finality.QuorumCert) types.Round {
	if justifyQC.Height == 0 {
		return 0
	}
	return justifyQC.Round + 1
}

func isZeroQC(qc finality.QuorumCert) bool {
	return qc.Height == 0 && qc.Round == 0 && qc.BlockHash == (types.Hash{}) &&
		len(qc.Signers) == 0 && len(qc.Signature) == 0 && qc.VotingPower == 0
}

func isZeroTimeoutCert(timeoutCert finality.TimeoutCert) bool {
	return timeoutCert.Height == 0 && timeoutCert.Round == 0 && isZeroQC(timeoutCert.HighQC) &&
		len(timeoutCert.Signers) == 0 && len(timeoutCert.Signature) == 0
}

// verifyOptionalQuorumCert verifies qc, or accepts the zero certificate that stands for
// "no QC yet".
func (machine *StateMachine) verifyOptionalQuorumCert(qc finality.QuorumCert) error {
	if qc.Height == 0 {
		if !isZeroQC(qc) {
			return fmt.Errorf("%w: malformed empty certificate", ErrInvalidQuorumCert)
		}
		return nil
	}
	return machine.verifyQuorumCert(qc)
}

func (machine *StateMachine) verifyQuorumCert(qc finality.QuorumCert) error {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return fmt.Errorf("%w: missing height or block hash", ErrInvalidQuorumCert)
	}
	if qc.Height > maxChainHeight || qc.Round > maxChainView {
		return fmt.Errorf("%w: height or view out of range", ErrInvalidQuorumCert)
	}
	validatorSet, known, err := machine.validatorSetForCert(qc.Height, qc.BlockHash)
	if err != nil {
		return err
	}
	if known {
		if node, _ := machine.blockTree.Get(qc.BlockHash); node.Block.Header.Round != qc.Round {
			return fmt.Errorf("%w: block is proposed in view %d, not view %d", ErrInvalidQuorumCert, node.Block.Header.Round, qc.Round)
		}
	}
	return machine.verifyQuorumCertWithSet(qc, validatorSet)
}

// verifyQuorumCertWithSet verifies qc against validatorSet.
func (machine *StateMachine) verifyQuorumCertWithSet(qc finality.QuorumCert, validatorSet validator.Set) error {
	if qc.Height == 0 || qc.BlockHash == (types.Hash{}) {
		return fmt.Errorf("%w: missing height or block hash", ErrInvalidQuorumCert)
	}
	if qc.Height > maxChainHeight || qc.Round > maxChainView {
		return fmt.Errorf("%w: height or view out of range", ErrInvalidQuorumCert)
	}
	if len(qc.Signature) == 0 {
		return fmt.Errorf("%w: missing aggregate signature", ErrInvalidQuorumCert)
	}
	cacheKey := quorumCertCacheKey(qc, validatorSet.Hash())
	if _, verified := machine.verifiedQCs[cacheKey]; verified {
		return nil
	}
	signers, err := finality.ParseSigners(qc.Signers)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidQuorumCert, err)
	}
	if len(signers) == 0 {
		return fmt.Errorf("%w: no signers", ErrInvalidQuorumCert)
	}
	seen := make(map[types.ValidatorID]struct{}, len(signers))
	publicKeys := make([]types.PublicKey, 0, len(signers))
	var votingPower types.VotingPower
	for _, signer := range signers {
		if _, duplicate := seen[signer]; duplicate {
			return fmt.Errorf("%w: duplicate signer %s", ErrInvalidQuorumCert, signer)
		}
		seen[signer] = struct{}{}
		validatorInfo, found := validatorSet.Get(signer)
		if !found {
			return fmt.Errorf("%w: unknown signer %s", ErrInvalidQuorumCert, signer)
		}
		votingPower, err = types.AddVotingPower(votingPower, validatorInfo.VotingPower)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidQuorumCert, err)
		}
		publicKeys = append(publicKeys, validatorInfo.PublicKey)
	}
	if !hasQuorum(votingPower, validatorSet.TotalVotingPower()) {
		return fmt.Errorf("%w: %w", ErrInvalidQuorumCert, ErrNoQuorum)
	}
	if qc.VotingPower != 0 && qc.VotingPower != votingPower {
		return fmt.Errorf("%w: voting power mismatch", ErrInvalidQuorumCert)
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusVote, VoteSignBytes(Vote{Height: qc.Height, Round: qc.Round, BlockHash: qc.BlockHash}))
	if err != nil {
		return err
	}
	if !machine.aggregates.VerifyAggregate(publicKeys, message, qc.Signature) {
		return fmt.Errorf("%w: invalid aggregate signature", ErrInvalidQuorumCert)
	}
	if len(machine.verifiedQCs) >= maxVerifiedQCCache {
		machine.verifiedQCs = make(map[[sha256.Size]byte]struct{})
	}
	machine.verifiedQCs[cacheKey] = struct{}{}
	return nil
}

// quorumCertCacheKey identifies a certificate by every field that influences its
// verification, including the signature and the validator set that verifies it.
func quorumCertCacheKey(qc finality.QuorumCert, validatorSetHash types.Hash) [sha256.Size]byte {
	hasher := sha256.New()
	hasher.Write(validatorSetHash[:])
	writeUint64(hasher, uint64(qc.Height))
	writeUint64(hasher, uint64(qc.Round))
	hasher.Write(qc.BlockHash[:])
	writeUint64(hasher, uint64(qc.VotingPower))
	writeUint64(hasher, uint64(len(qc.Signers)))
	hasher.Write(qc.Signers)
	writeUint64(hasher, uint64(len(qc.Signature)))
	hasher.Write(qc.Signature)
	var key [sha256.Size]byte
	copy(key[:], hasher.Sum(nil))
	return key
}

// verifyTimeoutCert checks a timeout certificate: its signatures are grouped by signed
// message, every group verifies against the validator set of the certificate height, every
// signer is unique, the signers reach quorum, and the claimed high QC is the best QC the
// groups reported.
func (machine *StateMachine) verifyTimeoutCert(timeoutCert finality.TimeoutCert) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidTimeoutCert, fmt.Sprintf(format, args...))
	}
	if timeoutCert.Height == 0 || timeoutCert.Height > maxChainHeight || timeoutCert.Round > maxChainView {
		return invalid("height or view out of range")
	}
	groups, err := decodeTimeoutSignatureGroups(timeoutCert.Signature)
	if err != nil {
		return invalid("%v", err)
	}
	var maxHeight types.Height
	for _, group := range groups {
		if group.Height > maxHeight {
			maxHeight = group.Height
		}
	}
	if timeoutCert.Height != maxHeight {
		return invalid("height %d is not the highest vote height %d", timeoutCert.Height, maxHeight)
	}
	validatorSet := machine.validatorSetForHeight(maxHeight)
	seen := make(map[types.ValidatorID]struct{})
	allSigners := make([]types.ValidatorID, 0)
	var votingPower types.VotingPower
	var bestQC finality.QuorumCert
	for _, group := range groups {
		if group.Height == 0 || group.Height > maxChainHeight || len(group.Signers) == 0 {
			return invalid("malformed signature group")
		}
		if group.HighQC.Height > 0 && group.HighQC.Round >= timeoutCert.Round {
			return invalid("high qc view %d is not below timeout view %d", group.HighQC.Round, timeoutCert.Round)
		}
		if err := machine.verifyOptionalQuorumCert(group.HighQC); err != nil {
			return fmt.Errorf("%w: high qc: %w", ErrInvalidTimeoutCert, err)
		}
		publicKeys := make([]types.PublicKey, 0, len(group.Signers))
		for _, signer := range group.Signers {
			if _, duplicate := seen[signer]; duplicate {
				return invalid("duplicate signer %s", signer)
			}
			seen[signer] = struct{}{}
			validatorInfo, found := validatorSet.Get(signer)
			if !found {
				return invalid("unknown signer %s", signer)
			}
			votingPower, err = types.AddVotingPower(votingPower, validatorInfo.VotingPower)
			if err != nil {
				return invalid("%v", err)
			}
			publicKeys = append(publicKeys, validatorInfo.PublicKey)
			allSigners = append(allSigners, signer)
		}
		message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(TimeoutVote{
			Height: group.Height,
			Round:  timeoutCert.Round,
			HighQC: group.HighQC,
		}))
		if err != nil {
			return err
		}
		if !machine.aggregates.VerifyAggregate(publicKeys, message, group.Signature) {
			return invalid("invalid aggregate signature")
		}
		if isBetterQC(group.HighQC, bestQC) {
			bestQC = group.HighQC
		}
	}
	if !hasQuorum(votingPower, validatorSet.TotalVotingPower()) {
		return fmt.Errorf("%w: %w", ErrInvalidTimeoutCert, ErrNoQuorum)
	}
	sort.Slice(allSigners, func(first, second int) bool { return allSigners[first] < allSigners[second] })
	if !bytes.Equal(finality.EncodeSigners(allSigners), timeoutCert.Signers) {
		return invalid("signers do not match the signature groups")
	}
	if !sameQC(bestQC, timeoutCert.HighQC) {
		return invalid("high qc is not the best qc of the signature groups")
	}
	return nil
}

func (machine *StateMachine) buildQuorumCert(height types.Height, round types.Round, blockHash types.Hash) (finality.QuorumCert, error) {
	blockVotes := machine.votesForBlock(height, round, blockHash)
	if len(blockVotes) == 0 {
		return finality.QuorumCert{}, ErrNoQuorum
	}

	validatorSet, _, err := machine.validatorSetForCert(height, blockHash)
	if err != nil {
		return finality.QuorumCert{}, err
	}
	type signedVote struct {
		validatorID types.ValidatorID
		signature   types.Signature
	}
	var votingPower types.VotingPower
	votes := make([]signedVote, 0, len(blockVotes))
	for validatorID, vote := range blockVotes {
		validatorInfo, found := validatorSet.Get(validatorID)
		if !found {
			continue
		}
		var err error
		votingPower, err = types.AddVotingPower(votingPower, validatorInfo.VotingPower)
		if err != nil {
			return finality.QuorumCert{}, err
		}
		votes = append(votes, signedVote{validatorID: validatorID, signature: vote.Signature})
	}

	if !hasQuorum(votingPower, validatorSet.TotalVotingPower()) {
		return finality.QuorumCert{}, ErrNoQuorum
	}

	sort.Slice(votes, func(firstIndex int, secondIndex int) bool {
		return votes[firstIndex].validatorID < votes[secondIndex].validatorID
	})
	signers := make([]string, 0, len(votes))
	signatures := make([]types.Signature, 0, len(votes))
	for _, vote := range votes {
		signers = append(signers, string(vote.validatorID))
		signatures = append(signatures, vote.signature)
	}
	if machine.aggregator == nil {
		return finality.QuorumCert{}, ErrAggregateRequired
	}
	if !allSignaturesPresent(signatures) {
		return finality.QuorumCert{}, ErrMissingSignature
	}
	aggregateSignature, err := machine.aggregator.Aggregate(signatures)
	if err != nil {
		return finality.QuorumCert{}, err
	}
	qc := finality.QuorumCert{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		Signers:     types.Bitmap(strings.Join(signers, ",")),
		Signature:   aggregateSignature,
		VotingPower: votingPower,
	}
	if err := machine.verifyQuorumCert(qc); err != nil {
		return finality.QuorumCert{}, err
	}
	return qc, nil
}

func (machine *StateMachine) verifyProposalSignature(proposal Proposal, proposer validator.Validator) error {
	if len(proposal.Signature) == 0 {
		return fmt.Errorf("%w: missing proposal signature", ErrInvalidProposal)
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusProposal, ProposalSignBytes(proposal))
	if err != nil {
		return err
	}
	if !machine.signatures.Verify(proposer.PublicKey, message, proposal.Signature) {
		return fmt.Errorf("%w: invalid proposal signature", ErrInvalidProposal)
	}
	return nil
}

func (machine *StateMachine) verifyVoteSignature(vote Vote, voter validator.Validator) error {
	if len(vote.Signature) == 0 {
		return fmt.Errorf("%w: missing vote signature", ErrInvalidVote)
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusVote, VoteSignBytes(vote))
	if err != nil {
		return err
	}
	if !machine.signatures.Verify(voter.PublicKey, message, vote.Signature) {
		return fmt.Errorf("%w: invalid vote signature", ErrInvalidVote)
	}
	return nil
}

func (machine *StateMachine) verifyTimeoutVoteSignature(vote TimeoutVote, voter validator.Validator) error {
	if len(vote.Signature) == 0 {
		return fmt.Errorf("%w: missing timeout vote signature", ErrInvalidVote)
	}
	message, err := vexocrypto.DomainMessage(vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(vote))
	if err != nil {
		return err
	}
	if !machine.signatures.Verify(voter.PublicKey, message, vote.Signature) {
		return fmt.Errorf("%w: invalid timeout vote signature", ErrInvalidVote)
	}
	return nil
}

func (machine *StateMachine) votedVote(vote Vote) (Vote, bool) {
	previous, found := machine.votedVotes[vote.Height][vote.Round][vote.ValidatorID]
	return previous, found
}

// recordVote stores a vote that already passed every check. Maps are allocated only here,
// after rejected votes have returned, so invalid traffic cannot grow the vote tables.
func (machine *StateMachine) recordVote(vote Vote) {
	machine.ensureVoteMaps(vote.Height, vote.Round, vote.BlockHash)
	machine.votedVotes[vote.Height][vote.Round][vote.ValidatorID] = vote
	machine.votes[vote.Height][vote.Round][vote.BlockHash][vote.ValidatorID] = vote
}

func (machine *StateMachine) votesForBlock(height types.Height, round types.Round, blockHash types.Hash) map[types.ValidatorID]Vote {
	return machine.votes[height][round][blockHash]
}

func (machine *StateMachine) ensureVoteMaps(height types.Height, round types.Round, blockHash types.Hash) {
	if _, found := machine.votes[height]; !found {
		machine.votes[height] = make(map[types.Round]map[types.Hash]map[types.ValidatorID]Vote)
	}
	if _, found := machine.votes[height][round]; !found {
		machine.votes[height][round] = make(map[types.Hash]map[types.ValidatorID]Vote)
	}
	if _, found := machine.votes[height][round][blockHash]; !found {
		machine.votes[height][round][blockHash] = make(map[types.ValidatorID]Vote)
	}

	if _, found := machine.votedVotes[height]; !found {
		machine.votedVotes[height] = make(map[types.Round]map[types.ValidatorID]Vote)
	}
	if _, found := machine.votedVotes[height][round]; !found {
		machine.votedVotes[height][round] = make(map[types.ValidatorID]Vote)
	}
}

func hasQuorum(power types.VotingPower, total types.VotingPower) bool {
	return types.HasTwoThirdsQuorum(power, total)
}

func allSignaturesPresent(signatures []types.Signature) bool {
	if len(signatures) == 0 {
		return false
	}
	for _, signature := range signatures {
		if len(signature) == 0 {
			return false
		}
	}
	return true
}
