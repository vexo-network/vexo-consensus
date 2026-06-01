package consensus

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var ErrUnexpectedQuorum = errors.New("unexpected quorum")

type AdversarialRunner struct {
	machine    *StateMachine
	validators []validator.Validator
}

func NewAdversarialRunner(machine *StateMachine) (*AdversarialRunner, error) {
	driver, err := NewStepDriver(machine)
	if err != nil {
		return nil, err
	}
	return &AdversarialRunner{
		machine:    machine,
		validators: driver.validators,
	}, nil
}

func (runner *AdversarialRunner) Propose(ctx context.Context, block types.Block, proposer types.ValidatorID) (ProposeResult, error) {
	driver := StepDriver{machine: runner.machine, validators: runner.validators}
	return driver.Propose(ctx, block, proposer)
}

func (runner *AdversarialRunner) VoteWith(ctx context.Context, blockHash types.Hash, voterIDs ...types.ValidatorID) (finality.QuorumCert, error) {
	status := runner.machine.Status(ctx)
	for _, voterID := range voterIDs {
		if err := runner.machine.OnVote(ctx, Vote{
			Height:      status.Height,
			Round:       status.Round,
			BlockHash:   blockHash,
			ValidatorID: voterID,
		}); err != nil {
			return finality.QuorumCert{}, err
		}
	}
	return runner.machine.BuildQuorumCert(status.Height, status.Round, blockHash)
}

func (runner *AdversarialRunner) VoteConflict(ctx context.Context, validatorID types.ValidatorID, firstHash types.Hash, secondHash types.Hash) error {
	status := runner.machine.Status(ctx)
	if err := runner.machine.OnVote(ctx, Vote{
		Height:      status.Height,
		Round:       status.Round,
		BlockHash:   firstHash,
		ValidatorID: validatorID,
	}); err != nil {
		return err
	}
	return runner.machine.OnVote(ctx, Vote{
		Height:      status.Height,
		Round:       status.Round,
		BlockHash:   secondHash,
		ValidatorID: validatorID,
	})
}

func (runner *AdversarialRunner) TimeoutWith(ctx context.Context, highQC finality.QuorumCert, voterIDs ...types.ValidatorID) (finality.TimeoutCert, error) {
	status := runner.machine.Status(ctx)
	for _, voterID := range voterIDs {
		timeoutCert, err := runner.machine.OnTimeoutVote(ctx, TimeoutVote{
			Height:      status.Height,
			Round:       status.Round,
			ValidatorID: voterID,
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

func (runner *AdversarialRunner) TimeoutEquivocation(ctx context.Context, validatorID types.ValidatorID, firstQC finality.QuorumCert, secondQC finality.QuorumCert) error {
	status := runner.machine.Status(ctx)
	if _, err := runner.machine.OnTimeoutVote(ctx, TimeoutVote{
		Height:      status.Height,
		Round:       status.Round,
		ValidatorID: validatorID,
		HighQC:      firstQC,
	}); err != nil && !errors.Is(err, ErrNoQuorum) {
		return err
	}
	_, err := runner.machine.OnTimeoutVote(ctx, TimeoutVote{
		Height:      status.Height,
		Round:       status.Round,
		ValidatorID: validatorID,
		HighQC:      secondQC,
	})
	return err
}

func (runner *AdversarialRunner) Evidence() []slashing.Evidence {
	return runner.machine.Evidence()
}
