package consensus

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRunAdversarialSimulationReportsExpectedScenarios(t *testing.T) {
	report, err := RunAdversarialSimulation(context.Background(), AdversarialSimulationConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.SafetyOK {
		t.Fatalf("expected simulation safety, got %+v", report)
	}
	if report.LivenessOK {
		t.Fatalf("expected aggregate liveness to flag split-partition halt, got %+v", report)
	}
	if report.EvidenceCount < 2 {
		t.Fatalf("expected vote and timeout evidence, got %+v", report)
	}
	if report.RejectedAttacks < 4 {
		t.Fatalf("expected rejected attacks, got %+v", report)
	}

	expected := map[string]func(AdversarialScenarioReport) bool{
		"offline_minority": func(result AdversarialScenarioReport) bool { return result.OK && result.QuorumReached },
		"offline_majority": func(result AdversarialScenarioReport) bool {
			return result.OK && result.Rejected && !result.QuorumReached
		},
		"weighted_quorum": func(result AdversarialScenarioReport) bool { return result.OK && result.QuorumReached },
		"conflicting_vote": func(result AdversarialScenarioReport) bool {
			return result.OK && result.Rejected && result.Evidence == 1
		},
		"timeout_equivocation": func(result AdversarialScenarioReport) bool {
			return result.OK && result.Rejected && result.Evidence == 1
		},
		"unsafe_fork":                    func(result AdversarialScenarioReport) bool { return result.OK && result.Rejected },
		"split_partition_no_dual_quorum": func(result AdversarialScenarioReport) bool { return result.OK && result.Rejected && !result.LivenessOK },
	}
	for _, scenario := range report.Scenarios {
		check, found := expected[scenario.Name]
		if !found {
			t.Fatalf("unexpected scenario %q in %+v", scenario.Name, report.Scenarios)
		}
		if !check(scenario) {
			t.Fatalf("scenario %q failed expectations: %+v", scenario.Name, scenario)
		}
		delete(expected, scenario.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("missing scenarios: %+v", expected)
	}
}

func TestRunAdversarialSimulationRejectsTooFewValidators(t *testing.T) {
	_, err := RunAdversarialSimulation(context.Background(), AdversarialSimulationConfig{
		Validators: []validator.Validator{
			{ID: "a", VotingPower: 1},
			{ID: "b", VotingPower: 1},
			{ID: "c", VotingPower: 1},
		},
	})
	if err == nil {
		t.Fatal("expected too few validators error")
	}
}

func TestRunAdversarialSimulationAcceptsCustomValidators(t *testing.T) {
	report, err := RunAdversarialSimulation(context.Background(), AdversarialSimulationConfig{
		Validators: []validator.Validator{
			{ID: "a", VotingPower: 1},
			{ID: "b", VotingPower: 1},
			{ID: "c", VotingPower: 1},
			{ID: "d", VotingPower: 1},
			{ID: "e", VotingPower: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.SafetyOK || len(report.Scenarios) == 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}
