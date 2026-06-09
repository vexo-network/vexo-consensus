package types

import (
	"errors"
	"testing"
)

func TestAddVotingPowerRejectsOverflow(t *testing.T) {
	if _, err := AddVotingPower(^VotingPower(0), 1); !errors.Is(err, ErrVotingPowerOverflow) {
		t.Fatalf("expected voting power overflow, got %v", err)
	}
}

func TestAddUint64RejectsOverflow(t *testing.T) {
	if _, err := AddUint64(^uint64(0), 1); !errors.Is(err, ErrVotingPowerOverflow) {
		t.Fatalf("expected uint64 overflow, got %v", err)
	}
	if got := MustAddUint64Saturating(^uint64(0), 1); got != ^uint64(0) {
		t.Fatalf("expected saturating add to clamp, got %d", got)
	}
}

func TestHasTwoThirdsQuorumAvoidsMultiplicationOverflow(t *testing.T) {
	total := ^VotingPower(0)
	if !HasTwoThirdsQuorum(TwoThirdsQuorumThreshold(total), total) {
		t.Fatal("expected exact threshold to satisfy quorum")
	}
	if HasTwoThirdsQuorum(TwoThirdsQuorumThreshold(total)-1, total) {
		t.Fatal("expected below-threshold power to fail quorum")
	}
}
