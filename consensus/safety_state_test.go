package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
)

func TestSafetyStateReportsLockHighQCAndLastVote(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	if state := machine.SafetyState(); state.LockedQC.Height != 0 || state.HighQC.Height != 0 || state.LastVotedHeight != 0 || state.LastVotedRound != 0 {
		t.Fatalf("a new machine has no safety state, got %+v", state)
	}
	blocks := cluster.extendChain(machine, 3)

	state := machine.SafetyState()
	if state.LastVotedHeight != 3 || state.LastVotedRound != 2 {
		t.Fatalf("expected the last vote at height 3, view 2, got %+v", state)
	}
	if state.HighQC.BlockHash != blocks[2].hash {
		t.Fatalf("expected the high qc to be the third block's, got %+v", state.HighQC)
	}
	if state.LockedQC.BlockHash != blocks[1].hash || state.LockedQC.Round != 1 {
		t.Fatalf("expected the lock on the second block's qc, got %+v", state.LockedQC)
	}
}

// TestSafetyStateSurvivesRestart is the regression for the lock and the last voted view
// living in memory only: a restarted validator voted again in a view it had already voted
// in and accepted proposals below its lock.
func TestSafetyStateSurvivesRestart(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 3)

	path := t.TempDir() + "/consensus.wal"
	wal := openTestWAL(t, path)
	if err := wal.RecordSafetyState(machine.SafetyState()); err != nil {
		t.Fatal(err)
	}
	if err := wal.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openTestWAL(t, path)
	defer reopened.Close()
	persisted, found, err := reopened.SafetyState()
	if err != nil || !found {
		t.Fatalf("safety state lost: found=%t err=%v", found, err)
	}
	restarted := cluster.newMachine()
	if err := restarted.RestoreSafetyState(persisted); err != nil {
		t.Fatal(err)
	}
	if got := restarted.SafetyState(); got.LastVotedRound != 2 || got.LockedQC.Round != 1 || got.HighQC.Round != 2 {
		t.Fatalf("unexpected restored state %+v", got)
	}
	if status := restarted.Status(ctx); status.Round < 2 || status.Height < 3 {
		t.Fatalf("the restored machine must not sit below its last vote, got %+v", status)
	}

	// The certified blocks reach the restarted machine again; it holds them but votes for none.
	for index, entry := range blocks[:2] {
		var justify finality.QuorumCert
		if index > 0 {
			justify = blocks[index-1].qc
		}
		if err := restarted.OnProposal(ctx, cluster.proposal("a", entry.block, entry.round, justify, finality.TimeoutCert{})); !errors.Is(err, ErrStaleProposal) {
			t.Fatalf("expected the replayed proposal for height %d to be stale, got %v", entry.block.Header.Height, err)
		}
	}

	// The restarted machine must not vote again in view 2, for any block.
	rival := cluster.block(3, blocks[1].hash, "rival")
	if err := restarted.OnProposal(ctx, cluster.proposal("c", rival, 2, blocks[1].qc, finality.TimeoutCert{})); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected the restarted machine to refuse a second vote in view 2, got %v", err)
	}

	// A proposal justified by a certificate older than the lock must stay unsafe.
	old := cluster.block(2, blocks[0].hash, "old")
	timeout := cluster.timeoutCert(2, 4, blocks[0].qc, "a", "b", "c")
	stale := cluster.proposal("a", old, 5, blocks[0].qc, timeout)
	fresh := cluster.newMachine()
	cluster.feed(fresh, blocks[:1])
	if err := fresh.OnProposal(ctx, stale); err != nil {
		t.Fatalf("control: a machine without the safety state accepts it, got %v", err)
	}
	if err := restarted.OnProposal(ctx, stale); !errors.Is(err, ErrUnsafeProposal) {
		t.Fatalf("expected the restored lock to reject it, got %v", err)
	}
}

func TestRestoreSafetyStateOnlyRaisesTheMachine(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.chain(4, 0)

	high := SafetyState{LockedQC: blocks[2].qc, HighQC: blocks[3].qc, LastVotedHeight: 4, LastVotedRound: 3}
	if err := machine.RestoreSafetyState(high); err != nil {
		t.Fatal(err)
	}
	low := SafetyState{LockedQC: blocks[0].qc, HighQC: blocks[1].qc, LastVotedHeight: 2, LastVotedRound: 1}
	if err := machine.RestoreSafetyState(low); err != nil {
		t.Fatal(err)
	}
	state := machine.SafetyState()
	if state.LockedQC.Round != 2 || state.HighQC.Round != 3 || state.LastVotedHeight != 4 || state.LastVotedRound != 3 {
		t.Fatalf("an older state must not lower the machine, got %+v", state)
	}
	if status := machine.Status(ctx); status.Height < 5 || status.Round < 4 {
		t.Fatalf("the status must follow the restored certificates, got %+v", status)
	}
}

func TestRestoreSafetyStateKeepsTheHighQCAboveTheLock(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	blocks := cluster.chain(3, 0)

	if err := machine.RestoreSafetyState(SafetyState{LockedQC: blocks[2].qc, HighQC: blocks[0].qc, LastVotedHeight: 3, LastVotedRound: 2}); err != nil {
		t.Fatal(err)
	}
	state := machine.SafetyState()
	if state.HighQC.Round < state.LockedQC.Round {
		t.Fatalf("the highest qc may never sit below the lock, got %+v", state)
	}
}

func TestRestoreSafetyStateRejectsInconsistentState(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	if err := machine.RestoreSafetyState(SafetyState{LastVotedRound: 4}); !errors.Is(err, ErrInvalidSafetyState) {
		t.Fatalf("expected a last voted view without a height to be rejected, got %v", err)
	}
	malformed := SafetyState{LockedQC: finality.QuorumCert{Round: 3}}
	if err := machine.RestoreSafetyState(malformed); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("expected a malformed empty certificate to be rejected, got %v", err)
	}
	if err := machine.RestoreSafetyState(SafetyState{}); err != nil {
		t.Fatalf("an empty state is valid, got %v", err)
	}
	if state := machine.SafetyState(); state.LastVotedHeight != 0 || state.LockedQC.Height != 0 {
		t.Fatalf("unexpected state %+v", state)
	}
}

func TestSafetyStateReadFromTheWALRestores(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	cluster.extendChain(machine, 2)
	state := machine.SafetyState()

	wal := openTestWAL(t, t.TempDir()+"/consensus.wal")
	defer wal.Close()
	if err := wal.RecordSafetyState(state); err != nil {
		t.Fatal(err)
	}
	restored, found, err := wal.SafetyState()
	if err != nil || !found {
		t.Fatalf("lost: found=%t err=%v", found, err)
	}
	other := cluster.newMachine()
	if err := other.RestoreSafetyState(restored); err != nil {
		t.Fatalf("a safety state read from the log must restore: %v", err)
	}
	if got := other.SafetyState(); got.HighQC.BlockHash != state.HighQC.BlockHash || got.LastVotedRound != state.LastVotedRound {
		t.Fatalf("restored state differs: %+v vs %+v", got, state)
	}
}
