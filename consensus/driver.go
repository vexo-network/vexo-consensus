package consensus

import (
	"context"
	"errors"
	"fmt"
	"sort"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrNoValidators  = errors.New("no validators")
	ErrMissingSigner = errors.New("no signer for validator")
)

// SimulationSigners holds the consensus signing keys of simulated validators. Every
// message a driver produces is signed: the state machine accepts nothing unsigned.
type SimulationSigners map[types.ValidatorID]vexocrypto.Signer

// NewSimulationValidators gives every validator a deterministic signing key derived from
// its id and returns the keyed validators together with their signers.
func NewSimulationValidators(validators []validator.Validator) ([]validator.Validator, SimulationSigners, error) {
	keyed := make([]validator.Validator, 0, len(validators))
	signers := make(SimulationSigners, len(validators))
	for _, validatorInfo := range validators {
		signer, err := vexocrypto.NewDeterministicSigner([]byte("vexo-simulation-validator-" + string(validatorInfo.ID)))
		if err != nil {
			return nil, nil, err
		}
		validatorInfo.PublicKey = signer.PublicKey()
		keyed = append(keyed, validatorInfo)
		signers[validatorInfo.ID] = signer
	}
	return keyed, signers, nil
}

func (signers SimulationSigners) signer(validatorID types.ValidatorID) (vexocrypto.Signer, error) {
	signer, found := signers[validatorID]
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrMissingSigner, validatorID)
	}
	return signer, nil
}

func (signers SimulationSigners) signProposal(proposal *Proposal) error {
	signer, err := signers.signer(proposal.Proposer)
	if err != nil {
		return err
	}
	proposal.Signature, err = vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusProposal, ProposalSignBytes(*proposal))
	return err
}

func (signers SimulationSigners) signVote(vote *Vote) error {
	signer, err := signers.signer(vote.ValidatorID)
	if err != nil {
		return err
	}
	vote.Signature, err = vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusVote, VoteSignBytes(*vote))
	return err
}

func (signers SimulationSigners) signTimeoutVote(vote *TimeoutVote) error {
	signer, err := signers.signer(vote.ValidatorID)
	if err != nil {
		return err
	}
	vote.Signature, err = vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(*vote))
	return err
}

type StepDriver struct {
	machine    *StateMachine
	signers    SimulationSigners
	validators []validator.Validator
}

type ProposeResult struct {
	Proposal  Proposal
	BlockHash types.Hash
}

type BlockStepResult struct {
	Proposal  Proposal
	BlockHash types.Hash
	QuorumQC  finality.QuorumCert
}

func NewStepDriver(machine *StateMachine, signers SimulationSigners) (*StepDriver, error) {
	validators := machine.validators()
	if len(validators) == 0 {
		return nil, ErrNoValidators
	}
	sort.Slice(validators, func(left, right int) bool {
		return validators[left].ID < validators[right].ID
	})
	return &StepDriver{
		machine:    machine,
		signers:    signers,
		validators: validators,
	}, nil
}

func (machine *StateMachine) validators() []validator.Validator {
	machine.mu.Lock()
	defer machine.mu.Unlock()

	return machine.currentValidatorSet().List()
}

func (driver *StepDriver) Propose(ctx context.Context, block types.Block, proposer types.ValidatorID) (ProposeResult, error) {
	status := driver.machine.Status(ctx)
	if block.Header.Height == 0 {
		block.Header.Height = status.Height
	}

	proposal, err := driver.machine.CreateProposal(block, status.Round, proposer, finality.QuorumCert{})
	if err != nil {
		return ProposeResult{}, err
	}
	if err := driver.signers.signProposal(&proposal); err != nil {
		return ProposeResult{}, err
	}
	if err := driver.machine.OnProposal(ctx, proposal); err != nil {
		return ProposeResult{}, err
	}

	return ProposeResult{
		Proposal:  proposal,
		BlockHash: driver.machine.hashBlock(proposal.Block),
	}, nil
}

func (driver *StepDriver) VoteQuorum(ctx context.Context, blockHash types.Hash) (finality.QuorumCert, error) {
	status := driver.machine.Status(ctx)
	for _, validatorInfo := range driver.validators {
		vote := Vote{
			Height:      status.Height,
			Round:       status.Round,
			BlockHash:   blockHash,
			ValidatorID: validatorInfo.ID,
		}
		if err := driver.signers.signVote(&vote); err != nil {
			return finality.QuorumCert{}, err
		}
		err := driver.machine.OnVote(ctx, vote)
		if err != nil && !errors.Is(err, ErrConflictingVote) {
			return finality.QuorumCert{}, err
		}
		if qc, err := driver.machine.BuildQuorumCert(status.Height, status.Round, blockHash); err == nil {
			return qc, nil
		}
	}
	return finality.QuorumCert{}, ErrNoQuorum
}

func (driver *StepDriver) StepBlock(ctx context.Context, block types.Block, proposer types.ValidatorID) (BlockStepResult, error) {
	proposeResult, err := driver.Propose(ctx, block, proposer)
	if err != nil {
		return BlockStepResult{}, err
	}
	qc, err := driver.VoteQuorum(ctx, proposeResult.BlockHash)
	if err != nil {
		return BlockStepResult{}, err
	}
	return BlockStepResult{
		Proposal:  proposeResult.Proposal,
		BlockHash: proposeResult.BlockHash,
		QuorumQC:  qc,
	}, nil
}

func (driver *StepDriver) TimeoutQuorum(ctx context.Context, highQC finality.QuorumCert) (finality.TimeoutCert, error) {
	status := driver.machine.Status(ctx)
	for _, validatorInfo := range driver.validators {
		vote := TimeoutVote{
			Height:      status.Height,
			Round:       status.Round,
			ValidatorID: validatorInfo.ID,
			HighQC:      highQC,
		}
		if err := driver.signers.signTimeoutVote(&vote); err != nil {
			return finality.TimeoutCert{}, err
		}
		timeoutCert, err := driver.machine.OnTimeoutVote(ctx, vote)
		if err == nil {
			return timeoutCert, nil
		}
		if !errors.Is(err, ErrNoQuorum) {
			return finality.TimeoutCert{}, err
		}
	}
	return finality.TimeoutCert{}, ErrNoQuorum
}
