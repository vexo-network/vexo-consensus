package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestScenarioRunnerFinalizesLongLinearChain(t *testing.T) {
	machine, driver := newTestDriver(t)
	result, err := NewScenarioRunner(driver).Run(context.Background(), ScenarioConfig{Blocks: 12})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Blocks) != 12 {
		t.Fatalf("expected 12 blocks, got %d", len(result.Blocks))
	}
	if len(result.Finalized) != 10 {
		t.Fatalf("expected 10 finalized blocks, got %d", len(result.Finalized))
	}
	expectedFinalized := result.Blocks[9].BlockHash
	if result.LastFinalized != expectedFinalized {
		t.Fatal("expected latest finalized block to lag chain head by two blocks")
	}
	if machine.Status(context.Background()).LastFinalized != result.LastFinalized {
		t.Fatal("expected machine status to match scenario result")
	}
}

func TestScenarioRunnerHandlesTimeoutsAndForkAttempts(t *testing.T) {
	_, driver := newTestDriver(t)
	result, err := NewScenarioRunner(driver).Run(context.Background(), ScenarioConfig{
		Blocks:       15,
		TimeoutEvery: 4,
		ForkEvery:    3,
		Proposers:    []types.ValidatorID{"a", "b", "c"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Timeouts) != 3 {
		t.Fatalf("expected 3 timeout certs, got %d", len(result.Timeouts))
	}
	if result.RejectedForks != 4 {
		t.Fatalf("expected 4 rejected forks, got %d", result.RejectedForks)
	}
	if len(result.Finalized) != 13 {
		t.Fatalf("expected 13 finalized blocks, got %d", len(result.Finalized))
	}
	for _, timeoutCert := range result.Timeouts {
		if timeoutCert.HighQC.Height == 0 {
			t.Fatal("expected timeout cert to carry high qc")
		}
	}
	if result.LastHighQC.BlockHash != result.Blocks[len(result.Blocks)-1].BlockHash {
		t.Fatal("expected high qc to track chain head")
	}
}

func TestScenarioRunnerRejectsUnsafeForkInvariantBreak(t *testing.T) {
	_, driver := newTestDriver(t)
	runner := NewScenarioRunner(driver)
	driver.machine.StartRound(1, 0)

	if _, err := driver.StepBlock(context.Background(), types.Block{Header: types.Header{Height: 1}}, "a"); err != nil {
		t.Fatal(err)
	}
	if err := runner.tryUnsafeFork(context.Background(), 2, "a"); err == nil {
		t.Fatal("expected unsafe fork to be rejected")
	}
}

func TestScenarioRunnerRejectsBrokenFinalityChain(t *testing.T) {
	machine, driver := newTestDriver(t)
	runner := NewScenarioRunner(driver)

	first := types.Hash{1}
	second := types.Hash{2}
	machine.committed = append(machine.committed,
		CommitDecision{CommittedBlockHash: first},
		CommitDecision{CommittedBlockHash: second},
	)

	err := runner.checkFinalityChain()
	if !errors.Is(err, ErrSafetyInvariant) {
		t.Fatalf("expected safety invariant error, got %v", err)
	}
}

func TestScenarioRunnerNoopsOnZeroBlocks(t *testing.T) {
	_, driver := newTestDriver(t)
	result, err := NewScenarioRunner(driver).Run(context.Background(), ScenarioConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 0 || len(result.Finalized) != 0 {
		t.Fatalf("expected empty result, got %+v", result)
	}
}
