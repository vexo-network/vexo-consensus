package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestStateMachineCreateProposalAttachesDataCommitment(t *testing.T) {
	cluster := newEqualTestCluster(t, "a")
	machine := cluster.newMachine()

	proposal, err := machine.CreateProposal(types.Block{
		Header: types.Header{Height: 1},
		Txs:    []types.Tx{[]byte("tx")},
	}, 0, "a", machine.blockTree.HighQC())
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Block.Header.ConsensusHash != dataavailability.Commitment(proposal.Block.Txs) {
		t.Fatal("expected proposal to attach data availability commitment")
	}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineRejectsProposalWithMissingDataCommitment(t *testing.T) {
	cluster := newEqualTestCluster(t, "a")
	machine := cluster.newMachine()

	block := cluster.block(1, types.Hash{})
	block.Txs = []types.Tx{[]byte("tx")}
	block.Header.TxRoot = TxRoot(block.Txs)
	err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{}))
	if !errors.Is(err, ErrInvalidProposal) || !errors.Is(err, dataavailability.ErrMissingData) {
		t.Fatalf("expected invalid proposal for missing data commitment, got %v", err)
	}
}

func TestStateMachineRejectsProposalWithWrongDataCommitment(t *testing.T) {
	cluster := newEqualTestCluster(t, "a")
	machine := cluster.newMachine()

	block := cluster.block(1, types.Hash{})
	block.Txs = []types.Tx{[]byte("tx")}
	block.Header.TxRoot = TxRoot(block.Txs)
	block.Header.ConsensusHash = dataavailability.Commitment([]types.Tx{[]byte("other")})
	err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{}))
	if !errors.Is(err, ErrInvalidProposal) || !errors.Is(err, dataavailability.ErrCommitmentMismatch) {
		t.Fatalf("expected invalid proposal for wrong data commitment, got %v", err)
	}
}

func hashEvidenceProposal() Proposal {
	return Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{2},
				AppHash:          types.Hash{3},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}
}

// Hash-mismatch evidence proves the header carries another hash than the one its height requires,
// and verifies only against that hash.
func TestInvalidProposalHashEvidenceVerifiesAgainstTheHashItsHeightRequires(t *testing.T) {
	proposal := hashEvidenceProposal()
	for _, reason := range []InvalidProposalReason{InvalidProposalReasonValidatorSetHash, InvalidProposalReasonAppHash} {
		evidence, err := NewInvalidProposalHashEvidence(proposal, reason, types.Hash{1})
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyInvalidProposalEvidence(evidence, types.Hash{1}); err != nil {
			t.Fatalf("%s: %v", reason, err)
		}
		if err := VerifyInvalidProposalEvidence(evidence, types.Hash{9}); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("%s: expected another required hash to be refused, got %v", reason, err)
		}
		if err := VerifyInvalidProposalEvidence(evidence, types.Hash{}); !errors.Is(err, ErrMissingExpectedHash) {
			t.Fatalf("%s: expected verification without the required hash to be refused, got %v", reason, err)
		}
		decoded, err := DecodeInvalidProposalProof(evidence.Proof)
		if err != nil {
			t.Fatal(err)
		}
		decoded.ActualHash = types.Hash{8}
		if evidence.Proof, err = json.Marshal(decoded); err != nil {
			t.Fatal(err)
		}
		if err := VerifyInvalidProposalEvidence(evidence, types.Hash{1}); !errors.Is(err, ErrInvalidProposal) {
			t.Fatalf("%s: expected an actual hash the header does not carry to be refused, got %v", reason, err)
		}
	}
}

func TestInvalidProposalHashEvidenceRequiresAMismatch(t *testing.T) {
	proposal := hashEvidenceProposal()
	if _, err := NewInvalidProposalHashEvidence(proposal, InvalidProposalReasonAppHash, proposal.Block.Header.AppHash); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a proposal carrying the required app hash to be no evidence, got %v", err)
	}
	if _, err := NewInvalidProposalHashEvidence(proposal, InvalidProposalReasonDAMismatch, types.Hash{1}); !errors.Is(err, ErrUnsupportedProposalReason) {
		t.Fatalf("expected a data-availability reason to be refused, got %v", err)
	}
}

// Faults nodes could judge differently, such as a bad timestamp or an invalid transaction, are
// not evidence: a proof naming one is refused.
func TestInvalidProposalEvidenceRefusesReasonsNodesCouldJudgeDifferently(t *testing.T) {
	proposal := hashEvidenceProposal()
	for _, reason := range []InvalidProposalReason{"timestamp", "tx_validity", ""} {
		encoded, err := json.Marshal(InvalidProposalProof{Proposal: proposal, Reason: reason, ExpectedHash: types.Hash{1}, ActualHash: types.Hash{2}})
		if err != nil {
			t.Fatal(err)
		}
		evidence := slashing.Evidence{Type: slashing.EvidenceInvalidProposal, Validator: proposal.Proposer, Height: 7, Round: 2, Proof: encoded}
		if err := VerifyInvalidProposalEvidence(evidence, types.Hash{1}); !errors.Is(err, ErrUnsupportedProposalReason) {
			t.Fatalf("reason %q: expected ErrUnsupportedProposalReason, got %v", reason, err)
		}
	}
}

func TestDataAvailabilityEvidenceBindsCommitmentProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:       "vexo-test",
				Height:        7,
				ConsensusHash: dataavailability.Commitment([]types.Tx{[]byte("other")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}
	evidence, err := NewDataAvailabilityInvalidProposalEvidence(proposal, InvalidProposalReasonDAMismatch)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidence(evidence, types.Hash{}); err != nil {
		t.Fatalf("expected DA mismatch evidence to verify: %v", err)
	}
	decoded, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		t.Fatal(err)
	}
	decoded.ExpectedHash = types.Hash{}
	tampered, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Proof = tampered
	if err := VerifyInvalidProposalEvidence(evidence, types.Hash{}); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected tampered DA proof to fail, got %v", err)
	}
}

func TestUnavailableDataEvidenceBindsMissingCommitmentProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
			Txs:    []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}
	evidence, err := NewUnavailableDataEvidence(proposal, "missing data availability commitment")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnavailableDataEvidence(evidence); err != nil {
		t.Fatalf("expected unavailable data evidence to verify: %v", err)
	}
	decoded, err := DecodeUnavailableDataProof(evidence.Proof)
	if err != nil {
		t.Fatal(err)
	}
	decoded.TxCount++
	tampered, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Proof = tampered
	if err := VerifyUnavailableDataEvidence(evidence); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected tampered unavailable data proof to fail, got %v", err)
	}
}
