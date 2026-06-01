package consensus

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrSafetyInvariant = errors.New("safety invariant violated")

type ScenarioConfig struct {
	Blocks       uint64
	TimeoutEvery uint64
	ForkEvery    uint64
	Proposers    []types.ValidatorID
}

type ScenarioResult struct {
	Blocks        []BlockStepResult
	Timeouts      []finality.TimeoutCert
	RejectedForks uint64
	Finalized     []CommitDecision
	LastFinalized types.Hash
	LastHighQC    finality.QuorumCert
}

type ScenarioRunner struct {
	driver *StepDriver
}

func NewScenarioRunner(driver *StepDriver) *ScenarioRunner {
	return &ScenarioRunner{driver: driver}
}

func (runner *ScenarioRunner) Run(ctx context.Context, config ScenarioConfig) (ScenarioResult, error) {
	if config.Blocks == 0 {
		return ScenarioResult{}, nil
	}
	if len(config.Proposers) == 0 {
		for _, validatorInfo := range runner.driver.validators {
			config.Proposers = append(config.Proposers, validatorInfo.ID)
		}
	}

	result := ScenarioResult{
		Blocks:   make([]BlockStepResult, 0, config.Blocks),
		Timeouts: make([]finality.TimeoutCert, 0),
	}
	runner.driver.machine.StartRound(1, 0)

	for index := uint64(0); index < config.Blocks; index++ {
		height := types.Height(index + 1)
		if config.TimeoutEvery > 0 && index > 0 && index%config.TimeoutEvery == 0 {
			runner.driver.machine.StartRound(height, 0)
			timeoutCert, err := runner.driver.TimeoutQuorum(ctx, runner.driver.machine.blockTree.HighQC())
			if err != nil {
				return ScenarioResult{}, err
			}
			result.Timeouts = append(result.Timeouts, timeoutCert)
		}

		proposer := config.Proposers[index%uint64(len(config.Proposers))]
		blockResult, err := runner.driver.StepBlock(ctx, types.Block{Header: types.Header{Height: height}}, proposer)
		if err != nil {
			return ScenarioResult{}, err
		}
		result.Blocks = append(result.Blocks, blockResult)

		if config.ForkEvery > 0 && index > 0 && index%config.ForkEvery == 0 {
			if err := runner.tryUnsafeFork(ctx, height+1, proposer); err == nil {
				return ScenarioResult{}, ErrSafetyInvariant
			}
			result.RejectedForks++
		}

		if err := runner.checkFinalityChain(); err != nil {
			return ScenarioResult{}, err
		}
	}

	result.Finalized = runner.driver.machine.CommitDecisions()
	result.LastFinalized = runner.driver.machine.Status(ctx).LastFinalized
	result.LastHighQC = runner.driver.machine.blockTree.HighQC()
	return result, nil
}

func (runner *ScenarioRunner) tryUnsafeFork(ctx context.Context, height types.Height, proposer types.ValidatorID) error {
	_, err := runner.driver.Propose(ctx, types.Block{Header: types.Header{
		Height:            height,
		PreviousBlockHash: types.Hash{255},
	}}, proposer)
	return err
}

func (runner *ScenarioRunner) checkFinalityChain() error {
	decisions := runner.driver.machine.CommitDecisions()
	for index := 1; index < len(decisions); index++ {
		previous := decisions[index-1].CommittedBlockHash
		current := decisions[index].CommittedBlockHash
		if previous == (types.Hash{}) || current == (types.Hash{}) {
			return ErrSafetyInvariant
		}
		if current == previous {
			return ErrSafetyInvariant
		}
		if !runner.driver.machine.blockTree.Extends(current, previous) {
			return ErrSafetyInvariant
		}
	}
	return nil
}
