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
	signers    SimulationSigners
	validators []validator.Validator
}

func NewAdversarialRunner(machine *StateMachine, signers SimulationSigners) (*AdversarialRunner, error) {
	driver, err := NewStepDriver(machine, signers)
	if err != nil {
		return nil, err
	}
	return &AdversarialRunner{
		machine:    machine,
		signers:    signers,
		validators: driver.validators,
	}, nil
}

func (runner *AdversarialRunner) Propose(ctx context.Context, block types.Block, proposer types.ValidatorID) (ProposeResult, error) {
	driver := StepDriver{machine: runner.machine, signers: runner.signers, validators: runner.validators}
	return driver.Propose(ctx, block, proposer)
}

// VoteWith casts a signed vote from each listed validator for the block in the current
// view and returns the quorum certificate as soon as the votes reach quorum.
func (runner *AdversarialRunner) VoteWith(ctx context.Context, blockHash types.Hash, voterIDs ...types.ValidatorID) (finality.QuorumCert, error) {
	status := runner.machine.Status(ctx)
	for _, voterID := range voterIDs {
		vote := Vote{
			Height:      status.Height,
			Round:       status.Round,
			BlockHash:   blockHash,
			ValidatorID: voterID,
		}
		if err := runner.signers.signVote(&vote); err != nil {
			return finality.QuorumCert{}, err
		}
		if err := runner.machine.OnVote(ctx, vote); err != nil {
			return finality.QuorumCert{}, err
		}
		if qc, err := runner.machine.BuildQuorumCert(status.Height, status.Round, blockHash); err == nil {
			return qc, nil
		}
	}
	return runner.machine.BuildQuorumCert(status.Height, status.Round, blockHash)
}

// VoteConflict makes one validator vote for two different blocks in the current view.
func (runner *AdversarialRunner) VoteConflict(ctx context.Context, validatorID types.ValidatorID, firstHash types.Hash, secondHash types.Hash) error {
	status := runner.machine.Status(ctx)
	for _, blockHash := range []types.Hash{firstHash, secondHash} {
		vote := Vote{
			Height:      status.Height,
			Round:       status.Round,
			BlockHash:   blockHash,
			ValidatorID: validatorID,
		}
		if err := runner.signers.signVote(&vote); err != nil {
			return err
		}
		if err := runner.machine.OnVote(ctx, vote); err != nil {
			return err
		}
	}
	return nil
}

func (runner *AdversarialRunner) TimeoutWith(ctx context.Context, highQC finality.QuorumCert, voterIDs ...types.ValidatorID) (finality.TimeoutCert, error) {
	status := runner.machine.Status(ctx)
	for _, voterID := range voterIDs {
		vote, err := runner.signedTimeoutVote(status, voterID, highQC)
		if err != nil {
			return finality.TimeoutCert{}, err
		}
		timeoutCert, err := runner.machine.OnTimeoutVote(ctx, vote)
		if err == nil {
			return timeoutCert, nil
		}
		if !errors.Is(err, ErrNoQuorum) {
			return finality.TimeoutCert{}, err
		}
	}
	return finality.TimeoutCert{}, ErrNoQuorum
}

// TimeoutEquivocation makes one validator sign two timeout votes for the current view that
// carry different high QCs. Each certificate must verify, or be empty, for its vote to
// reach the equivocation check.
func (runner *AdversarialRunner) TimeoutEquivocation(ctx context.Context, validatorID types.ValidatorID, firstQC finality.QuorumCert, secondQC finality.QuorumCert) error {
	status := runner.machine.Status(ctx)
	first, err := runner.signedTimeoutVote(status, validatorID, firstQC)
	if err != nil {
		return err
	}
	if _, err := runner.machine.OnTimeoutVote(ctx, first); err != nil && !errors.Is(err, ErrNoQuorum) {
		return err
	}
	second, err := runner.signedTimeoutVote(status, validatorID, secondQC)
	if err != nil {
		return err
	}
	_, err = runner.machine.OnTimeoutVote(ctx, second)
	return err
}

func (runner *AdversarialRunner) signedTimeoutVote(status Status, validatorID types.ValidatorID, highQC finality.QuorumCert) (TimeoutVote, error) {
	vote := TimeoutVote{
		Height:      status.Height,
		Round:       status.Round,
		ValidatorID: validatorID,
		HighQC:      highQC,
	}
	if err := runner.signers.signTimeoutVote(&vote); err != nil {
		return TimeoutVote{}, err
	}
	return vote, nil
}

func (runner *AdversarialRunner) Evidence() []slashing.Evidence {
	return runner.machine.Evidence()
}
