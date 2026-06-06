package node

import (
	"context"
	"errors"
	"testing"
)

func TestNormalizeConsensusLoopConfigDefaultsExecutionCommitMode(t *testing.T) {
	cfg := normalizeConsensusLoopConfig(ConsensusLoopConfig{})
	if cfg.ExecutionCommitMode != ExecutionCommitModeQC {
		t.Fatalf("expected qc execution commit mode, got %q", cfg.ExecutionCommitMode)
	}
}

func TestStepConsensusRejectsUnknownExecutionCommitMode(t *testing.T) {
	node := &Node{}
	_, err := node.StepConsensusWithConfig(context.Background(), ConsensusLoopConfig{ExecutionCommitMode: "unknown"})
	if !errors.Is(err, ErrInvalidLoopConfig) {
		t.Fatalf("expected invalid loop config, got %v", err)
	}
}
