package node

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNormalizeConsensusLoopConfigDefaultsExecutionCommitMode(t *testing.T) {
	cfg := normalizeConsensusLoopConfig(ConsensusLoopConfig{})
	if cfg.ExecutionCommitMode != ExecutionCommitModeFinalized {
		t.Fatalf("expected finalized execution commit mode, got %q", cfg.ExecutionCommitMode)
	}
	if !cfg.AdaptiveRoundTimeoutEnabled || !cfg.RecoveryFinalityGateEnabled {
		t.Fatalf("expected adaptive pacing and recovery gate enabled by default, got adaptive=%t recovery=%t", cfg.AdaptiveRoundTimeoutEnabled, cfg.RecoveryFinalityGateEnabled)
	}
}

func TestStepConsensusRejectsUnknownExecutionCommitMode(t *testing.T) {
	node := &Node{}
	_, err := node.StepConsensusWithConfig(context.Background(), ConsensusLoopConfig{ExecutionCommitMode: "unknown"})
	if !errors.Is(err, ErrInvalidLoopConfig) {
		t.Fatalf("expected invalid loop config, got %v", err)
	}
}

func TestStepConsensusRejectsQCExecutionCommitWithoutUnsafeOptIn(t *testing.T) {
	node := &Node{}
	_, err := node.StepConsensusWithConfig(context.Background(), ConsensusLoopConfig{ExecutionCommitMode: ExecutionCommitModeQC})
	if !errors.Is(err, ErrInvalidLoopConfig) {
		t.Fatalf("expected invalid loop config, got %v", err)
	}
}

func TestCommitReadyBlockRejectsUnsafeQCCommitAPI(t *testing.T) {
	node := &Node{}
	_, committed, err := node.CommitReadyBlock(context.Background())
	if !errors.Is(err, ErrUnsafeQCCommit) || committed {
		t.Fatalf("expected unsafe qc commit rejection, committed=%t err=%v", committed, err)
	}
}

func TestConsensusLoopWakeInterruptsCommitDelay(t *testing.T) {
	wake := make(chan struct{}, 1)
	wake <- struct{}{}

	started := time.Now()
	if !waitConsensusLoop(context.Background(), wake, time.Hour) {
		t.Fatal("expected wake signal to resume consensus loop")
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("wake signal did not interrupt commit delay: %s", elapsed)
	}
}

func TestConsensusLoopDoesNotTimeoutAfterStepFailure(t *testing.T) {
	if shouldTimeoutConsensusRound(errors.New("block execution failed"), true, false, false, time.Now().Add(-time.Hour), time.Millisecond) {
		t.Fatal("a failed consensus step must not advance the round")
	}
	if !shouldTimeoutConsensusRound(nil, true, false, false, time.Now().Add(-time.Hour), time.Millisecond) {
		t.Fatal("a healthy stalled step should still advance the round after timeout")
	}
}
