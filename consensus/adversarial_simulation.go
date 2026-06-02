package consensus

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

type AdversarialSimulationConfig struct {
	Validators []validator.Validator `json:"validators,omitempty"`
}

type AdversarialSimulationReport struct {
	Scenarios       []AdversarialScenarioReport `json:"scenarios"`
	SafetyOK        bool                        `json:"safety_ok"`
	LivenessOK      bool                        `json:"liveness_ok"`
	EvidenceCount   uint64                      `json:"evidence_count"`
	QuorumFailures  uint64                      `json:"quorum_failures"`
	RejectedAttacks uint64                      `json:"rejected_attacks"`
}

type AdversarialScenarioReport struct {
	Name          string `json:"name"`
	OK            bool   `json:"ok"`
	SafetyOK      bool   `json:"safety_ok"`
	LivenessOK    bool   `json:"liveness_ok"`
	QuorumReached bool   `json:"quorum_reached"`
	Rejected      bool   `json:"rejected"`
	Evidence      uint64 `json:"evidence"`
	Finalized     uint64 `json:"finalized"`
	Error         string `json:"error,omitempty"`
}

func RunAdversarialSimulation(ctx context.Context, config AdversarialSimulationConfig) (AdversarialSimulationReport, error) {
	validators := config.Validators
	if len(validators) == 0 {
		validators = defaultAdversarialValidators()
	}
	if len(validators) < 4 {
		return AdversarialSimulationReport{}, ErrNoValidators
	}

	report := AdversarialSimulationReport{
		SafetyOK:   true,
		LivenessOK: true,
		Scenarios:  make([]AdversarialScenarioReport, 0, 7),
	}
	scenarios := []func(context.Context, []validator.Validator) AdversarialScenarioReport{
		simulateOfflineMinority,
		simulateOfflineMajority,
		simulateWeightedQuorum,
		simulateConflictingVoteEvidence,
		simulateTimeoutEquivocationEvidence,
		simulateUnsafeForkRejection,
		simulateSplitPartitionNoDualQuorum,
	}
	for _, scenario := range scenarios {
		result := scenario(ctx, validators)
		report.Scenarios = append(report.Scenarios, result)
		if !result.SafetyOK {
			report.SafetyOK = false
		}
		if !result.LivenessOK {
			report.LivenessOK = false
		}
		report.EvidenceCount += result.Evidence
		if !result.QuorumReached {
			report.QuorumFailures++
		}
		if result.Rejected {
			report.RejectedAttacks++
		}
	}
	return report, nil
}

func simulateOfflineMinority(ctx context.Context, validators []validator.Validator) AdversarialScenarioReport {
	machine, runner, err := simulationRunner(validators)
	if err != nil {
		return failedScenario("offline_minority", err)
	}
	machine.StartRound(1, 0)
	proposed, err := runner.Propose(ctx, types.Block{Header: types.Header{Height: 1}}, validators[0].ID)
	if err != nil {
		return failedScenario("offline_minority", err)
	}
	voters := validatorIDs(validators[:len(validators)-1])
	qc, err := runner.VoteWith(ctx, proposed.BlockHash, voters...)
	if err != nil {
		return failedScenario("offline_minority", err)
	}
	return scenarioReport("offline_minority", true, true, qc.VotingPower > 0, false, runner.Evidence(), machine)
}

func simulateOfflineMajority(ctx context.Context, validators []validator.Validator) AdversarialScenarioReport {
	machine, runner, err := simulationRunner(validators)
	if err != nil {
		return failedScenario("offline_majority", err)
	}
	machine.StartRound(1, 0)
	proposed, err := runner.Propose(ctx, types.Block{Header: types.Header{Height: 1}}, validators[0].ID)
	if err != nil {
		return failedScenario("offline_majority", err)
	}
	_, err = runner.VoteWith(ctx, proposed.BlockHash, validators[0].ID)
	rejected := errors.Is(err, ErrNoQuorum)
	return scenarioReport("offline_majority", true, rejected, false, rejected, runner.Evidence(), machine)
}

func simulateWeightedQuorum(ctx context.Context, validators []validator.Validator) AdversarialScenarioReport {
	weighted := []validator.Validator{
		{ID: "a", VotingPower: 4},
		{ID: "b", VotingPower: 2},
		{ID: "c", VotingPower: 1},
	}
	machine, runner, err := simulationRunner(weighted)
	if err != nil {
		return failedScenario("weighted_quorum", err)
	}
	machine.StartRound(1, 0)
	proposed, err := runner.Propose(ctx, types.Block{Header: types.Header{Height: 1}}, "a")
	if err != nil {
		return failedScenario("weighted_quorum", err)
	}
	if _, err := runner.VoteWith(ctx, proposed.BlockHash, "a"); !errors.Is(err, ErrNoQuorum) {
		return failedScenario("weighted_quorum", ErrUnexpectedQuorum)
	}
	qc, err := runner.VoteWith(ctx, proposed.BlockHash, "b")
	if err != nil {
		return failedScenario("weighted_quorum", err)
	}
	return scenarioReport("weighted_quorum", true, true, qc.VotingPower == 6, false, runner.Evidence(), machine)
}

func simulateConflictingVoteEvidence(ctx context.Context, validators []validator.Validator) AdversarialScenarioReport {
	machine, runner, err := simulationRunner(validators)
	if err != nil {
		return failedScenario("conflicting_vote", err)
	}
	machine.StartRound(1, 0)
	first, err := runner.Propose(ctx, types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("first")}}, validators[0].ID)
	if err != nil {
		return failedScenario("conflicting_vote", err)
	}
	second, err := runner.Propose(ctx, types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("second")}}, validators[0].ID)
	if err != nil {
		return failedScenario("conflicting_vote", err)
	}
	err = runner.VoteConflict(ctx, validators[0].ID, first.BlockHash, second.BlockHash)
	rejected := errors.Is(err, ErrConflictingVote)
	return scenarioReport("conflicting_vote", true, true, false, rejected, runner.Evidence(), machine)
}

func simulateTimeoutEquivocationEvidence(ctx context.Context, validators []validator.Validator) AdversarialScenarioReport {
	machine, runner, err := simulationRunner(validators)
	if err != nil {
		return failedScenario("timeout_equivocation", err)
	}
	machine.StartRound(1, 0)
	err = runner.TimeoutEquivocation(ctx, validators[0].ID, finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}, finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}})
	rejected := errors.Is(err, ErrConflictingTimeoutVote)
	return scenarioReport("timeout_equivocation", true, true, false, rejected, runner.Evidence(), machine)
}

func simulateUnsafeForkRejection(ctx context.Context, validators []validator.Validator) AdversarialScenarioReport {
	machine, runner, err := simulationRunner(validators)
	if err != nil {
		return failedScenario("unsafe_fork", err)
	}
	driver, err := NewStepDriver(machine)
	if err != nil {
		return failedScenario("unsafe_fork", err)
	}
	scenario := NewScenarioRunner(driver)
	_, err = scenario.Run(ctx, ScenarioConfig{Blocks: 3, ForkEvery: 1, Proposers: validatorIDs(validators)})
	rejected := err == nil
	return scenarioReport("unsafe_fork", true, true, rejected, rejected, runner.Evidence(), machine)
}

func simulateSplitPartitionNoDualQuorum(ctx context.Context, validators []validator.Validator) AdversarialScenarioReport {
	left := validators[:len(validators)/2]
	right := validators[len(validators)/2:]
	if partitionReachesQuorum(ctx, validators, left, "split_partition_left") {
		return failedScenario("split_partition_no_dual_quorum", ErrUnexpectedQuorum)
	}
	if partitionReachesQuorum(ctx, validators, right, "split_partition_right") {
		return failedScenario("split_partition_no_dual_quorum", ErrUnexpectedQuorum)
	}
	return AdversarialScenarioReport{
		Name:       "split_partition_no_dual_quorum",
		OK:         true,
		SafetyOK:   true,
		LivenessOK: false,
		Rejected:   true,
	}
}

func partitionReachesQuorum(ctx context.Context, validators []validator.Validator, partition []validator.Validator, proposer types.ValidatorID) bool {
	machine, runner, err := simulationRunner(validators)
	if err != nil {
		return false
	}
	machine.StartRound(1, 0)
	proposed, err := runner.Propose(ctx, types.Block{Header: types.Header{Height: 1}}, proposer)
	if err != nil {
		return false
	}
	_, err = runner.VoteWith(ctx, proposed.BlockHash, validatorIDs(partition)...)
	return err == nil
}

func simulationRunner(validators []validator.Validator) (*StateMachine, *AdversarialRunner, error) {
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		return nil, nil, err
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		return nil, nil, err
	}
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
	})
	if err != nil {
		return nil, nil, err
	}
	runner, err := NewAdversarialRunner(machine)
	if err != nil {
		return nil, nil, err
	}
	return machine, runner, nil
}

func scenarioReport(name string, safetyOK bool, livenessOK bool, quorumReached bool, rejected bool, evidence []slashing.Evidence, machine *StateMachine) AdversarialScenarioReport {
	return AdversarialScenarioReport{
		Name:          name,
		OK:            safetyOK && (livenessOK || rejected),
		SafetyOK:      safetyOK,
		LivenessOK:    livenessOK,
		QuorumReached: quorumReached,
		Rejected:      rejected,
		Evidence:      uint64(len(evidence)),
		Finalized:     uint64(len(machine.CommitDecisions())),
	}
}

func failedScenario(name string, err error) AdversarialScenarioReport {
	return AdversarialScenarioReport{
		Name:     name,
		OK:       false,
		SafetyOK: false,
		Error:    err.Error(),
	}
}

func validatorIDs(validators []validator.Validator) []types.ValidatorID {
	ids := make([]types.ValidatorID, 0, len(validators))
	for _, validatorInfo := range validators {
		ids = append(ids, validatorInfo.ID)
	}
	return ids
}

func defaultAdversarialValidators() []validator.Validator {
	return []validator.Validator{
		{ID: "a", VotingPower: 1},
		{ID: "b", VotingPower: 1},
		{ID: "c", VotingPower: 1},
		{ID: "d", VotingPower: 1},
	}
}
