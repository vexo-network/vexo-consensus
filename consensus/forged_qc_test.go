package consensus

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

// forgery is a quorum certificate for the block at height 1, view 0 that must not verify.
type forgery struct {
	name   string
	mutate func(cluster *testCluster, qc *finality.QuorumCert)
}

func quorumCertForgeries() []forgery {
	return []forgery{
		{"garbage aggregate signature", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.Signature = types.AggregateSignature("garbage-signature")
		}},
		{"missing aggregate signature", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.Signature = nil
		}},
		{"no signers", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.Signers = nil
		}},
		{"signer outside the validator set", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.Signers = types.Bitmap("a,b,mallory")
		}},
		{"duplicated signer", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.Signers = types.Bitmap("a,a,b")
		}},
		{"sub-quorum signers with their genuine signature", func(cluster *testCluster, qc *finality.QuorumCert) {
			*qc = cluster.qc(qc.Height, qc.Round, qc.BlockHash, "a")
		}},
		{"genuine signatures over another block", func(cluster *testCluster, qc *finality.QuorumCert) {
			*qc = cluster.qc(qc.Height, qc.Round, types.Hash{0xEE})
			qc.BlockHash = types.Hash{0xAA}
		}},
		{"genuine signatures over another view", func(cluster *testCluster, qc *finality.QuorumCert) {
			*qc = cluster.qc(qc.Height, 7, qc.BlockHash)
			qc.Round = 0
		}},
		{"genuine signatures over another height", func(cluster *testCluster, qc *finality.QuorumCert) {
			*qc = cluster.qc(9, qc.Round, qc.BlockHash)
			qc.Height = 1
		}},
		{"inflated voting power claim", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.Signers = types.Bitmap("a,b")
			qc.VotingPower = 1_000_000
		}},
		{"signer list names a validator who did not sign", func(cluster *testCluster, qc *finality.QuorumCert) {
			signed := cluster.qc(qc.Height, qc.Round, qc.BlockHash, "a", "b")
			qc.Signature = signed.Signature
			qc.Signers = types.Bitmap("a,b,c")
			qc.VotingPower = 3
		}},
		{"signatures made in another domain", func(cluster *testCluster, qc *finality.QuorumCert) {
			signatures := make([]types.Signature, 0, len(cluster.ids))
			for _, id := range cluster.ids {
				vote := Vote{Height: qc.Height, Round: qc.Round, BlockHash: qc.BlockHash, ValidatorID: id}
				if err := signVoteForTest(cluster.signers[id], &vote, vexocrypto.DomainConsensusProposal); err != nil {
					cluster.t.Fatal(err)
				}
				signatures = append(signatures, vote.Signature)
			}
			aggregate, err := vexocrypto.DeterministicAggregateSigner{}.Aggregate(signatures)
			if err != nil {
				cluster.t.Fatal(err)
			}
			qc.Signature = aggregate
		}},
		{"missing block hash", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.BlockHash = types.Hash{}
		}},
		{"missing height", func(_ *testCluster, qc *finality.QuorumCert) {
			qc.Height = 0
		}},
	}
}

// assertUntouched fails the test when a rejected message left any trace in the machine.
func assertUntouched(t *testing.T, machine *StateMachine, before Status, trees int) {
	t.Helper()
	ctx := context.Background()
	if highQC := machine.HighQC(ctx); highQC.Height != 0 {
		t.Fatalf("a rejected certificate must not become the high qc, got %+v", highQC)
	}
	if decisions := machine.CommitDecisions(); len(decisions) != 0 {
		t.Fatalf("a rejected certificate must not finalize anything, got %+v", decisions)
	}
	if after := machine.Status(ctx); after.Height != before.Height || after.Round != before.Round || after.LastFinalized != before.LastFinalized {
		t.Fatalf("a rejected certificate must not move the machine: before %+v after %+v", before, after)
	}
	if machine.blockTree.Len() != trees {
		t.Fatalf("a rejected certificate must not add blocks: expected %d, got %d", trees, machine.blockTree.Len())
	}
	if state := machine.SafetyState(); state.LockedQC.Height != 0 {
		t.Fatalf("a rejected certificate must not lock the machine, got %+v", state)
	}
}

// TestForgedQuorumCertCannotFinalizeBlock replays the audit exploit: a single validator
// chains blocks whose justify certificates carry a garbage signature. Before the fix the
// state machine never verified Proposal.JustifyQC, so the forged chain finalized a block.
func TestForgedQuorumCertCannotFinalizeBlock(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	blocks := cluster.chain(5, 0)
	for index := range blocks {
		blocks[index].qc = cluster.garbageQC(blocks[index].block.Header.Height, blocks[index].round, blocks[index].hash)
	}
	for index, entry := range blocks {
		var justify finality.QuorumCert
		if index > 0 {
			justify = blocks[index-1].qc
		}
		err := machine.OnProposal(ctx, cluster.proposal("a", entry.block, entry.round, justify, finality.TimeoutCert{}))
		if index == 0 {
			if err != nil {
				t.Fatalf("the first block carries no certificate and is acceptable: %v", err)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidQuorumCert) {
			t.Fatalf("block %d: expected the forged justify qc to be rejected, got %v", index+1, err)
		}
	}

	if decisions := machine.CommitDecisions(); len(decisions) != 0 {
		t.Fatalf("forged certificates finalized %d blocks", len(decisions))
	}
	if machine.Status(ctx).LastFinalized != (types.Hash{}) {
		t.Fatal("forged certificates moved the last finalized block")
	}
	if highQC := machine.HighQC(ctx); highQC.Height != 0 {
		t.Fatalf("forged certificate became the high qc: %+v", highQC)
	}
	if machine.blockTree.Len() != 1 {
		t.Fatalf("only the first block may be stored, got %d", machine.blockTree.Len())
	}
}

func TestQuorumCertForgeriesAreRejectedAsProposalJustification(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	first := cluster.block(1, types.Hash{})
	firstHash := HashBlock(first)
	second := cluster.block(2, firstHash)
	genuine := cluster.qc(1, 0, firstHash)

	control := cluster.newMachine()
	if err := control.OnProposal(context.Background(), cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	if err := control.OnProposal(context.Background(), cluster.proposal("b", second, 1, genuine, finality.TimeoutCert{})); err != nil {
		t.Fatalf("the genuine certificate must be accepted by the same harness: %v", err)
	}

	for _, forged := range quorumCertForgeries() {
		t.Run(forged.name, func(t *testing.T) {
			machine := cluster.newMachine()
			if err := machine.OnProposal(context.Background(), cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
				t.Fatal(err)
			}
			before := machine.Status(context.Background())

			qc := genuine
			forged.mutate(cluster, &qc)
			err := machine.OnProposal(context.Background(), cluster.proposal("b", second, 1, qc, finality.TimeoutCert{}))
			if err == nil {
				t.Fatal("the forged certificate justified a proposal")
			}
			if errors.Is(err, ErrStaleProposal) || errors.Is(err, ErrUnsafeProposal) {
				t.Fatalf("an unverified proposal must not reach the stale or lock checks, got %v", err)
			}
			assertUntouched(t, machine, before, 1)
		})
	}
}

func TestQuorumCertForgeriesFailVerification(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	hash := types.Hash{7}
	genuine := cluster.qc(1, 0, hash)
	if err := machine.VerifyQuorumCert(genuine); err != nil {
		t.Fatalf("the control certificate must verify: %v", err)
	}

	for _, forged := range quorumCertForgeries() {
		t.Run(forged.name, func(t *testing.T) {
			qc := genuine
			forged.mutate(cluster, &qc)
			if err := machine.VerifyQuorumCert(qc); !errors.Is(err, ErrInvalidQuorumCert) {
				t.Fatalf("expected an invalid quorum certificate, got %v", err)
			}
		})
	}
}

func TestVerifiedQuorumCertCacheDoesNotVouchForTamperedCopies(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	genuine := cluster.qc(1, 0, types.Hash{7})
	if err := machine.VerifyQuorumCert(genuine); err != nil {
		t.Fatal(err)
	}

	tampered := genuine
	tampered.Signature = types.AggregateSignature("garbage-signature")
	if err := machine.VerifyQuorumCert(tampered); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("a tampered copy of a verified certificate must still fail, got %v", err)
	}
	retargeted := genuine
	retargeted.BlockHash = types.Hash{8}
	if err := machine.VerifyQuorumCert(retargeted); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("a retargeted copy of a verified certificate must still fail, got %v", err)
	}
	if err := machine.VerifyQuorumCert(genuine); err != nil {
		t.Fatalf("the genuine certificate must still verify: %v", err)
	}
}

func TestForgedHighQCIsRejectedInTimeoutVotes(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(1, 0)
	before := machine.Status(ctx)

	for _, forged := range quorumCertForgeries() {
		if forged.name == "missing height" || forged.name == "missing block hash" {
			continue
		}
		t.Run(forged.name, func(t *testing.T) {
			qc := cluster.qc(1, 0, types.Hash{7})
			forged.mutate(cluster, &qc)
			vote := cluster.timeoutVote("a", 2, 1, qc)
			if _, err := machine.OnTimeoutVote(ctx, vote); !errors.Is(err, ErrInvalidQuorumCert) {
				t.Fatalf("expected the forged high qc to be rejected, got %v", err)
			}
		})
	}

	assertUntouched(t, machine, before, 0)
	if len(machine.timeouts.votes) != 0 {
		t.Fatal("a timeout vote carrying a forged high qc must not be stored")
	}
}

func TestForgedHighQCIsRejectedInTimeoutCertificates(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	genuine := cluster.qc(1, 0, types.Hash{7})
	forged := genuine
	forged.Signature = types.AggregateSignature("garbage-signature")

	// The votes are honestly signed; only the certificate they quote is forged.
	timeoutCert := cluster.timeoutCert(2, 1, forged, "a", "b", "c")
	if err := machine.VerifyTimeoutCert(timeoutCert); !errors.Is(err, ErrInvalidTimeoutCert) {
		t.Fatalf("expected a timeout certificate quoting a forged qc to be rejected, got %v", err)
	}
	if err := machine.VerifyTimeoutCert(cluster.timeoutCert(2, 1, genuine, "a", "b", "c")); err != nil {
		t.Fatalf("the genuine timeout certificate must verify: %v", err)
	}
}

func TestForgedQuorumCertIsRejectedForCommittedBlocks(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	block := cluster.block(1, types.Hash{})
	hash := HashBlock(block)
	before := machine.Status(context.Background())

	for _, forged := range quorumCertForgeries() {
		if forged.name == "missing height" || forged.name == "missing block hash" {
			continue
		}
		t.Run(forged.name, func(t *testing.T) {
			qc := cluster.qc(1, 0, hash)
			forged.mutate(cluster, &qc)
			if err := machine.ObserveCommittedBlock(block, qc); err == nil {
				t.Fatal("a forged certificate recorded a committed block")
			}
		})
	}
	assertUntouched(t, machine, before, 0)
	if machine.floorHeight != 0 {
		t.Fatalf("a forged certificate raised the finalized height to %d", machine.floorHeight)
	}

	if err := machine.ObserveCommittedBlock(block, cluster.qc(1, 0, hash)); err != nil {
		t.Fatalf("the genuine certificate must be accepted: %v", err)
	}
	if machine.floorHeight != 1 {
		t.Fatalf("expected the committed block to become the finalized root, got %d", machine.floorHeight)
	}
}

func TestForgedQuorumCertIsRejectedInSafetyState(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	forged := cluster.garbageQC(5, 4, types.Hash{9})

	if err := machine.RestoreSafetyState(SafetyState{LockedQC: forged, LastVotedHeight: 6, LastVotedRound: 5}); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("expected a forged locked qc to be rejected, got %v", err)
	}
	if err := machine.RestoreSafetyState(SafetyState{HighQC: forged}); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("expected a forged high qc to be rejected, got %v", err)
	}
	if state := machine.SafetyState(); state.LockedQC.Height != 0 || state.HighQC.Height != 0 || state.LastVotedHeight != 0 || state.LastVotedRound != 0 {
		t.Fatalf("a rejected safety state must leave the machine unchanged, got %+v", state)
	}
	if status := machine.Status(context.Background()); status.Height != 0 || status.Round != 0 {
		t.Fatalf("a rejected safety state must not move the machine, got %+v", status)
	}
}
