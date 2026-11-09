package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/p2p"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrMissingApplication    = errors.New("application is required")
	ErrNodeAlreadyRunning    = errors.New("node already running")
	ErrNodeNotRunning        = errors.New("node is not running")
	ErrMissingValidatorID    = errors.New("validator id is required")
	ErrConsensusOffline      = errors.New("consensus reactor is unavailable")
	ErrInvalidCommitQC       = errors.New("invalid commit quorum certificate")
	ErrEmptyProposal         = errors.New("proposal has no transactions")
	ErrLoopAlreadyRunning    = errors.New("consensus loop already running")
	ErrLoopNotRunning        = errors.New("consensus loop is not running")
	ErrFinalityNotFound      = errors.New("finality proof not found")
	ErrInvalidLoopConfig     = errors.New("invalid consensus loop config")
	ErrUnsafeQCCommit        = errors.New("unsafe qc commit requires explicit unsafe API")
	ErrBlockAlreadyCommitted = errors.New("block height is already committed")
	ErrNonSequentialCommit   = errors.New("block commit height is not sequential")
	ErrLocalVoteRecorded     = errors.New("local vote already recorded for a different block")
	ErrValidatorKeyMismatch  = errors.New("validator signer public key does not match genesis validator public key")
	ErrInvalidCommitBlock    = errors.New("invalid commit block")
	ErrBlockParentMismatch   = errors.New("block does not extend the last committed block")
	// ErrAppHashMismatch reports a header whose app hash is not the one this node computed for the
	// height the header names: either the proposer lies or this node's state diverged.
	ErrAppHashMismatch  = errors.New("block header app hash does not match the local state")
	ErrProposalRejected = errors.New("application rejected the proposal")
	ErrUnauthorizedVote = errors.New("vote does not match an accepted proposal")
)

type Status struct {
	ChainID               string
	EVMChainID            uint64
	Running               bool
	StartedAt             time.Time
	LatestHeight          types.Height
	LatestAppHash         types.Hash
	LatestFinalizedHeight types.Height
	LatestFinalizedHash   types.Hash
	DataDir               string
	PeerCount             int
	ActivePeerCount       int
	ConfiguredPeerCount   int
	ScoredPeerCount       int
	BannedPeers           int
	Peers                 []p2p.PeerSnapshot
}

type Node struct {
	cfg     Config
	genesis Genesis
	app     app.Application
	wire    transport.Transport

	mu          sync.Mutex
	stepMu      sync.Mutex
	commitMu    sync.Mutex
	admissionMu sync.Mutex
	broadcastMu sync.Mutex
	runtime     *vexoruntime.Runtime
	consensus   *consensus.StateMachine
	reactor     *consensus.TransportReactor
	// overflowTxs holds gossiped transactions the pool had no room for, to rebuild compact
	// proposals with.
	overflowTxs    *overflowTxs
	localVoter     *autoVoteReactor
	txCancel       context.CancelFunc
	txGossipOnce   sync.Once
	txGossip       *txGossipBatcher
	txDone         chan struct{}
	commitCancel   context.CancelFunc
	commitDone     chan struct{}
	evidenceCancel context.CancelFunc
	evidenceDone   chan struct{}
	scoreCancel    context.CancelFunc
	scoreDone      chan struct{}
	loopCancel     context.CancelFunc
	loopDone       chan struct{}
	loopWake       chan struct{}
	loopConfig     ConsensusLoopConfig
	pending        map[types.Hash]consensus.Proposal
	proposed       map[proposalRound]time.Time
	timeoutVotes   map[proposalRound]consensus.TimeoutVote
	broadcasts     map[string]struct{}
	metrics        nodeMetrics
	store          store.Store
	consensusWAL   *consensus.WAL
	signer         vexocrypto.Signer
	eventLogger    EventLogger
	scoreDirty     bool
	scoreLastSaved time.Time
	running        bool
	startedAt      time.Time
	// reconciledHeight is the committed height whose post-commit steps have all completed.
	reconciledHeight types.Height
	// finalityPersisted is the height up to which every finality decision is durably stored.
	finalityPersisted types.Height
	// committedHeader is the header of the last block this node executed. checkParentBinding uses
	// it in place of the stored block when its hash is the latest state's block hash.
	committedHeader *types.Header
	recoveryGate    recoveryGate

	// evidenceMu guards deferredEvidence, the equivocation this node detected in a view the evidence
	// pool cannot judge yet. It is kept in memory only: the pool takes it over once it can judge it.
	evidenceMu       sync.Mutex
	deferredEvidence []slashing.Evidence
}

type proposalRound struct {
	height types.Height
	round  types.Round
}

type EventLogger func(event string, fields map[string]any)

func New(cfg Config, genesis Genesis, application app.Application) (*Node, error) {
	if application == nil {
		return nil, ErrMissingApplication
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := genesis.Validate(cfg.Chain.ChainID); err != nil {
		return nil, err
	}
	if cfg.RequireNetworkSafety {
		if err := cfg.Chain.ValidateNetworkSafety(); err != nil {
			return nil, err
		}
		if err := genesis.ValidateNetworkSafety(cfg.Chain); err != nil {
			return nil, err
		}
	}
	return &Node{
		cfg:          cfg,
		genesis:      genesis,
		app:          application,
		pending:      make(map[types.Hash]consensus.Proposal),
		proposed:     make(map[proposalRound]time.Time),
		timeoutVotes: make(map[proposalRound]consensus.TimeoutVote),
		broadcasts:   make(map[string]struct{}),
		overflowTxs:  newOverflowTxs(),
	}, nil
}

func (node *Node) WithTransport(wire transport.Transport) *Node {
	node.wire = wire
	return node
}

func (node *Node) WithSigner(signer vexocrypto.Signer) *Node {
	node.signer = signer
	return node
}

func (node *Node) WithEventLogger(logger EventLogger) *Node {
	node.mu.Lock()
	defer node.mu.Unlock()
	node.eventLogger = logger
	return node
}

func (node *Node) Start(ctx context.Context) error {
	if err := node.start(ctx); err != nil {
		return err
	}
	// The pool of verified evidence outlives a restart, the mempool does not, so the evidence is
	// queued for a block again once the node runs.
	node.queuePooledEvidence(ctx)
	return nil
}

func (node *Node) start(ctx context.Context) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	if node.running {
		return ErrNodeAlreadyRunning
	}
	if node.cfg.ValidatorID != "" && node.signer == nil {
		return ErrMissingSigner
	}
	if node.cfg.ValidatorID != "" && node.cfg.RequireNetworkSafety {
		if err := node.validateSignerMatchesGenesis(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(node.cfg.StoreDir(), 0o755); err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(node.cfg.StoreDir())
	if err != nil {
		return err
	}

	var runtime *vexoruntime.Runtime
	if node.cfg.RequireNetworkSafety {
		runtime, err = vexoruntime.NewNetworkSafeWithStoreContext(
			ctx,
			node.cfg.Chain,
			node.app,
			node.genesis.Validators,
			node.genesis.Governance,
			storage,
		)
	} else {
		runtime, err = vexoruntime.NewWithStoreContext(
			ctx,
			node.cfg.Chain,
			node.app,
			node.genesis.Validators,
			node.genesis.Governance,
			storage,
		)
	}
	if err != nil {
		storage.Close()
		return err
	}
	if runtime.P2PScore != nil {
		if err := runtime.P2PScore.LoadFile(node.cfg.PeerScorePath()); err != nil {
			storage.Close()
			return err
		}
	}
	startHeight, err := node.initializeRuntime(ctx, runtime, storage)
	if err != nil {
		return err
	}
	if err := node.reconcileEvidence(ctx, runtime); err != nil {
		storage.Close()
		return err
	}
	consensusState, anchored, err := node.newConsensusMachine(ctx, runtime, startHeight)
	if err != nil {
		storage.Close()
		return err
	}
	consensusWAL, err := consensus.OpenWAL(node.cfg.ConsensusWALPath())
	if err != nil {
		storage.Close()
		return err
	}
	if node.cfg.ValidatorID != "" {
		// A validator that cannot restore what it already signed must not sign anything new.
		if err := restoreSafetyState(consensusState, consensusWAL, startHeight-1, anchored); err != nil {
			consensusWAL.Close()
			storage.Close()
			return fmt.Errorf("restore consensus safety state: %w", err)
		}
	}
	consensusState.StartRound(startHeight, 0)
	var localVoter *autoVoteReactor
	if node.cfg.ValidatorID != "" {
		localVoter = &autoVoteReactor{
			machine:            consensusState,
			chainID:            node.cfg.Chain.ChainID,
			validatorID:        node.cfg.ValidatorID,
			signer:             node.signer,
			validateProposal:   node.processProposal,
			onProposalAccepted: node.cacheProposal,
			onProposalRejected: node.handleRejectedProposal,
			onVoteAccepted:     node.wakeConsensus,
			onEvidence:         node.handleLocalEvidence,
			onError: func(event string, err error) {
				node.logEvent(event, map[string]any{"error": err.Error()})
			},
			onProposalLatency: node.metrics.observeProposalLatency,
			onVoteLatency:     node.metrics.observeVoteLatency,
			onSigningFailure:  node.metrics.observeSigningFailure,
			wal:               consensusWAL,
		}
	}
	var reactor *consensus.TransportReactor
	if node.wire != nil {
		receiver := consensus.Reactor(consensusState)
		if localVoter != nil {
			receiver = localVoter
		}
		reactor = consensus.NewTransportReactor(node.wire, receiver)
		reactor.SetPeerScoring(node.admitPeerMessage, node.observePeerMessage)
		// Transactions reach every validator through gossip: proposals name them by short id.
		if runtime.Mempool != nil {
			reactor.EnableCompactProposals(proposalTxSource{pool: runtime.Mempool, overflow: node.overflowTxs})
		}
		node.configureTransportPeerGate(runtime)
		if voter, ok := receiver.(*autoVoteReactor); ok {
			voter.broadcastVote = func(ctx context.Context, vote consensus.Vote) error {
				node.broadcastConsensusAsync("vote_broadcast_failed", map[string]any{
					"height": vote.Height,
					"round":  vote.Round,
				}, func(ctx context.Context) error {
					return reactor.BroadcastVote(ctx, vote)
				})
				return nil
			}
		}
		if err := reactor.Start(ctx); err != nil {
			consensusWAL.Close()
			storage.Close()
			return err
		}
		if err := node.startTxGossip(ctx); err != nil {
			consensusWAL.Close()
			reactor.Stop(ctx)
			storage.Close()
			return err
		}
		if err := node.startCommitGossip(ctx); err != nil {
			consensusWAL.Close()
			node.txCancel()
			reactor.Stop(ctx)
			storage.Close()
			return err
		}
		if err := node.startEvidenceGossip(ctx); err != nil {
			consensusWAL.Close()
			node.commitCancel()
			node.txCancel()
			reactor.Stop(ctx)
			storage.Close()
			return err
		}
	}

	node.runtime = runtime
	node.consensus = consensusState
	node.consensusWAL = consensusWAL
	node.reactor = reactor
	node.localVoter = localVoter
	node.pending = make(map[types.Hash]consensus.Proposal)
	node.proposed = make(map[proposalRound]time.Time)
	node.store = storage
	node.running = true
	node.startedAt = time.Now().UTC()
	node.startPeerScoreWindowReset(ctx)
	return nil
}

func (node *Node) validateSignerMatchesGenesis() error {
	if node.signer == nil {
		return ErrMissingSigner
	}
	publicKey := node.signer.PublicKey()
	for _, validatorInfo := range node.genesis.Validators {
		if validatorInfo.ID != node.cfg.ValidatorID {
			continue
		}
		if len(validatorInfo.PublicKey) == 0 || string(validatorInfo.PublicKey) != string(publicKey) {
			return ErrValidatorKeyMismatch
		}
		return nil
	}
	return ErrValidatorKeyMismatch
}

func (node *Node) initializeRuntime(ctx context.Context, runtime *vexoruntime.Runtime, storage store.Store) (types.Height, error) {
	state, err := runtime.Recover(ctx)
	if err == nil {
		return state.Height + 1, nil
	}
	if !errors.Is(err, store.ErrStateNotFound) {
		storage.Close()
		return 0, err
	}
	if _, err := runtime.InitChain(ctx, app.InitChainRequest{
		ChainID:    node.genesis.ChainID,
		Genesis:    node.genesis.AppState,
		Validators: genesisValidatorUpdates(node.genesis.Validators),
	}); err != nil {
		storage.Close()
		return 0, err
	}
	return 1, nil
}

// genesisValidatorUpdates hands the genesis validator set to the modules that seed their state from it.
// Without it the staking module knows no genesis validator, so nothing could ever be slashed.
func genesisValidatorUpdates(validators []validator.Validator) []types.ValidatorUpdate {
	updates := make([]types.ValidatorUpdate, 0, len(validators))
	for _, info := range validators {
		updates = append(updates, types.ValidatorUpdate{
			ID:          info.ID,
			Address:     info.Address,
			PublicKey:   info.PublicKey,
			VotingPower: info.VotingPower,
			Stake:       info.Stake,
			Metadata:    info.Metadata,
		})
	}
	return updates
}

func (node *Node) Stop(ctx context.Context) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if !node.running {
		return ErrNodeNotRunning
	}
	if node.txCancel != nil {
		node.txCancel()
		txDone := node.txDone
		node.mu.Unlock()
		waitLoopDone(ctx, txDone)
		node.mu.Lock()
	}
	if node.commitCancel != nil {
		node.commitCancel()
		commitDone := node.commitDone
		node.mu.Unlock()
		waitLoopDone(ctx, commitDone)
		node.mu.Lock()
	}
	if node.evidenceCancel != nil {
		node.evidenceCancel()
		evidenceDone := node.evidenceDone
		node.mu.Unlock()
		waitLoopDone(ctx, evidenceDone)
		node.mu.Lock()
	}
	if node.scoreCancel != nil {
		node.scoreCancel()
		scoreDone := node.scoreDone
		node.mu.Unlock()
		waitLoopDone(ctx, scoreDone)
		node.mu.Lock()
	}
	if node.loopCancel != nil {
		node.loopCancel()
		loopDone := node.loopDone
		node.mu.Unlock()
		waitLoopDone(ctx, loopDone)
		node.mu.Lock()
	}
	if node.reactor != nil {
		// The consumers it waits for take node.mu themselves (to cache a proposal or to log an event),
		// so waiting for them with node.mu held would deadlock a stop that races with a message.
		reactor := node.reactor
		node.mu.Unlock()
		stopErr := reactor.Stop(ctx)
		node.mu.Lock()
		if stopErr != nil {
			return stopErr
		}
	}
	err := node.persistPeerScoresLocked()
	if closeErr := node.store.Close(); err == nil {
		err = closeErr
	}
	if node.consensusWAL != nil {
		if walErr := node.consensusWAL.Close(); err == nil {
			err = walErr
		}
	}
	node.running = false
	node.runtime = nil
	node.consensus = nil
	node.consensusWAL = nil
	node.reactor = nil
	node.localVoter = nil
	node.txCancel = nil
	node.txDone = nil
	node.commitCancel = nil
	node.commitDone = nil
	node.evidenceCancel = nil
	node.evidenceDone = nil
	node.scoreCancel = nil
	node.scoreDone = nil
	node.loopCancel = nil
	node.loopDone = nil
	node.loopWake = nil
	node.store = nil
	node.startedAt = time.Time{}
	return err
}

func (node *Node) Runtime() (*vexoruntime.Runtime, error) {
	node.mu.Lock()
	defer node.mu.Unlock()

	if !node.running {
		return nil, ErrNodeNotRunning
	}
	return node.runtime, nil
}

func (node *Node) Consensus() (*consensus.StateMachine, error) {
	node.mu.Lock()
	defer node.mu.Unlock()

	if !node.running {
		return nil, ErrNodeNotRunning
	}
	return node.consensus, nil
}

func (node *Node) ConsensusReactor() (*consensus.TransportReactor, error) {
	node.mu.Lock()
	defer node.mu.Unlock()

	if !node.running || node.reactor == nil {
		return nil, ErrNodeNotRunning
	}
	return node.reactor, nil
}

func (node *Node) Status(ctx context.Context) Status {
	node.mu.Lock()
	defer node.mu.Unlock()

	status := Status{
		ChainID:    node.cfg.Chain.ChainID,
		EVMChainID: node.cfg.Chain.Execution.EVMChainID,
		Running:    node.running,
		StartedAt:  node.startedAt,
		DataDir:    node.cfg.DataDir,
	}
	if !node.running || node.runtime == nil {
		return status
	}
	commit, err := node.runtime.App.Commit()
	if err == nil {
		status.LatestHeight = commit.Height
		status.LatestAppHash = commit.AppHash
	}
	if node.runtime.P2PScore != nil {
		peers, err := node.runtime.P2PScore.Snapshot(ctx)
		if err == nil {
			status.Peers = peers
			status.ScoredPeerCount = len(peers)
			for _, peer := range peers {
				if peer.Banned {
					status.BannedPeers++
				}
			}
		}
	}
	status.ConfiguredPeerCount, status.ActivePeerCount = node.peerCountsLocked()
	status.PeerCount = status.ActivePeerCount
	if status.PeerCount == 0 {
		status.PeerCount = status.ScoredPeerCount
	}
	if node.consensus != nil {
		decisions := node.consensus.CommitDecisions()
		if len(decisions) > 0 {
			latest := decisions[len(decisions)-1]
			status.LatestFinalizedHeight = latest.CommittedHeight
			status.LatestFinalizedHash = latest.CommittedBlockHash
		}
	}
	if status.LatestFinalizedHeight == 0 {
		if proofStore, ok := node.runtime.Store.(store.FinalityProofStore); ok && proofStore != nil {
			if proof, err := proofStore.LatestFinalityProof(ctx); err == nil {
				loaded := finalityProofFromRecord(proof)
				if loaded.HasCommitProof() {
					status.LatestFinalizedHeight = loaded.Header.Height
					status.LatestFinalizedHash = loaded.BlockHash
				}
			}
		}
	}
	return status
}

func (node *Node) peerCountsLocked() (configured int, active int) {
	if node.wire == nil {
		return 0, 0
	}
	if stats, ok := node.wire.(transport.PeerStatsTransport); ok {
		return len(stats.ConfiguredPeerIDs()), len(stats.ActivePeerIDs())
	}
	if exchange, ok := node.wire.(transport.PeerExchangeTransport); ok {
		return len(exchange.KnownPeers()), 0
	}
	return 0, 0
}
