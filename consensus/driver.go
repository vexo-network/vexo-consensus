package consensus

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var ErrNoValidators = errors.New("no validators")

type StepDriver struct {
	machine    *StateMachine
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

func NewStepDriver(machine *StateMachine) (*StepDriver, error) {
	validators := machine.validatorSet.List()
	if len(validators) == 0 {
		return nil, ErrNoValidators
	}
	sort.Slice(validators, func(left, right int) bool {
		return validators[left].ID < validators[right].ID
	})
	return &StepDriver{
		machine:    machine,
		validators: validators,
	}, nil
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
		err := driver.machine.OnVote(ctx, Vote{
			Height:      status.Height,
			Round:       status.Round,
			BlockHash:   blockHash,
			ValidatorID: validatorInfo.ID,
		})
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
		timeoutCert, err := driver.machine.OnTimeoutVote(ctx, TimeoutVote{
			Height:      status.Height,
			Round:       status.Round,
			ValidatorID: validatorInfo.ID,
			HighQC:      highQC,
		})
		if err == nil {
			return timeoutCert, nil
		}
		if !errors.Is(err, ErrNoQuorum) {
			return finality.TimeoutCert{}, err
		}
	}
	return finality.TimeoutCert{}, ErrNoQuorum
}
