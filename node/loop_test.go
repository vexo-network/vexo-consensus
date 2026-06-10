package node

import (
	"context"
	"errors"
	"testing"
)

func TestNormalizeConsensusLoopConfigDefaultsExecutionCommitMode(t *testing.T) {
	cfg := normalizeConsensusLoopConfig(ConsensusLoopConfig{})
	if cfg.ExecutionCommitMode != ExecutionCommitModeFinalized {
		t.Fatalf("expected finalized execution commit mode, got %q", cfg.ExecutionCommitMode)
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
