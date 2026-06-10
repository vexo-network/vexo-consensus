package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/queryproof"
	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStateMachineCreateProposalAttachesDataCommitment(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineRejectsProposalWithMissingDataCommitment(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()},
			Txs:    []types.Tx{[]byte("tx")},
		},
		Proposer: "a",
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

func TestStateMachineRejectsProposalWithWrongDataCommitment(t *testing.T) {
	set := newTestValidatorSet([]validator.Validator{{ID: "a", VotingPower: 1}})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Aggregator:   testAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = machine.OnProposal(context.Background(), Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           1,
				ValidatorSetHash: set.Hash(),
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("other")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Proposer: "a",
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

func TestInvalidProposalHashEvidenceVerifiesReasonSpecificProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{2},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}

	evidence, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonValidatorSetHash), types.Hash{1}, types.Hash{2})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidence(evidence); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected standalone context rejection, got %v", err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedValidatorSetHash: types.Hash{1}}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedValidatorSetHash: types.Hash{9}}); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected context mismatch rejection, got %v", err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{}); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected missing context rejection, got %v", err)
	}
}

func TestInvalidProposalHashEvidenceVerifiesStateProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{2},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}
	evidence, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonValidatorSetHash), types.Hash{1}, types.Hash{2})
	if err != nil {
		t.Fatal(err)
	}
	expectedValidatorSetHash := types.Hash{1}
	appHash := types.Hash{3}
	pairs := []stateproof.Pair{
		{Key: []byte("validator_set_hash"), Value: expectedValidatorSetHash[:]},
		{Key: []byte("app_hash"), Value: appHash[:]},
	}
	root, err := stateproof.Root("consensus", pairs)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := queryproof.BuildFromPairs("vexo-test", 7, "consensus", []byte("validator_set_hash"), pairs, root)
	if err != nil {
		t.Fatal(err)
	}
	expectedExists := true
	context := InvalidProposalVerificationContext{
		ExpectedValidatorSetHash: expectedValidatorSetHash,
		ChainID:                  "vexo-test",
		ExpectedStateRoot:        root,
		ExpectedProofNamespace:   "consensus",
		ExpectedProofKey:         []byte("validator_set_hash"),
		ExpectedProofValue:       expectedValidatorSetHash[:],
		ExpectedProofExists:      &expectedExists,
		RequireStateProof:        true,
	}
	evidence, err = BindInvalidProposalEvidenceStateProof(evidence, context, proof)
	if err != nil {
		t.Fatal(err)
	}
	context.ContextProofHash = context.ProofHash()
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, context); err != nil {
		t.Fatalf("expected state proof evidence to verify: %v", err)
	}
	tamperedContext := context
	tamperedContext.ExpectedStateRoot = types.Hash{9}
	tamperedContext.ContextProofHash = tamperedContext.ProofHash()
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, tamperedContext); !errors.Is(err, ErrInvalidProposal) && !errors.Is(err, queryproof.ErrRootMismatch) && !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected tampered state root to fail, got %v", err)
	}
}

func TestInvalidProposalEvidenceWithBoundContextRequiresContextHash(t *testing.T) {
	proposal := Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: types.Hash{2}}},
		Round:    0,
		Proposer: "a",
	}
	context := InvalidProposalVerificationContext{ExpectedValidatorSetHash: types.Hash{1}}
	evidence, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonValidatorSetHash), types.Hash{1}, types.Hash{2})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidenceWithBoundContext(evidence, context); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected unbound context error, got %v", err)
	}
	bound, err := BindInvalidProposalEvidenceContext(evidence, context)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidenceWithBoundContext(bound, context); err != nil {
		t.Fatalf("expected bound context evidence to verify, got %v", err)
	}
}

func TestInvalidProposalEvidenceWithContextBuildsReasonSpecificProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{2},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}
	context := InvalidProposalVerificationContext{ExpectedValidatorSetHash: types.Hash{1}}
	evidence, err := NewInvalidProposalEvidenceWithContext(proposal, context, InvalidProposalReasonValidatorSetHash, proposal.Block.Header.ValidatorSetHash, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, context); err != nil {
		t.Fatalf("expected contextual evidence to verify: %v", err)
	}
}

func TestInvalidProposalEvidenceWithStateProofBindsProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{2},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}
	expectedValidatorSetHash := types.Hash{1}
	pairs := []stateproof.Pair{{Key: []byte("validator_set_hash"), Value: expectedValidatorSetHash[:]}}
	root, err := stateproof.Root("consensus", pairs)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := queryproof.BuildFromPairs("vexo-test", 7, "consensus", []byte("validator_set_hash"), pairs, root)
	if err != nil {
		t.Fatal(err)
	}
	expectedExists := true
	context := InvalidProposalVerificationContext{
		ExpectedValidatorSetHash: expectedValidatorSetHash,
		ChainID:                  "vexo-test",
		ExpectedStateRoot:        root,
		ExpectedProofNamespace:   "consensus",
		ExpectedProofKey:         []byte("validator_set_hash"),
		ExpectedProofValue:       expectedValidatorSetHash[:],
		ExpectedProofExists:      &expectedExists,
		RequireStateProof:        true,
	}
	evidence, err := NewInvalidProposalEvidenceWithStateProof(proposal, context, InvalidProposalReasonValidatorSetHash, proposal.Block.Header.ValidatorSetHash, proof, "")
	if err != nil {
		t.Fatal(err)
	}
	context.ContextProofHash = context.ProofHash()
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, context); err != nil {
		t.Fatalf("expected state-proof evidence to verify: %v", err)
	}
}

func TestInvalidProposalHashEvidenceRejectsActualHashNotInProposal(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:          "vexo-test",
				Height:           7,
				ValidatorSetHash: types.Hash{9},
				ConsensusHash:    dataavailability.Commitment([]types.Tx{[]byte("tx")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    2,
		Proposer: "validator-1",
	}

	_, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonValidatorSetHash), types.Hash{1}, types.Hash{2})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal proof, got %v", err)
	}
}

func TestInvalidProposalHashEvidenceRejectsMissingMismatch(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
		},
		Proposer: "validator-1",
	}

	_, err := NewInvalidProposalHashEvidence(proposal, string(InvalidProposalReasonAppHash), types.Hash{1}, types.Hash{1})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal proof, got %v", err)
	}
}

func TestInvalidProposalTxValidityEvidenceRequiresDeterministicMismatch(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
			Txs:    []types.Tx{[]byte("bad-tx")},
		},
		Proposer: "validator-1",
	}
	expectedResults := []types.Result{{Code: 1, Log: "ante rejected tx"}}
	actualResults := []types.Result{{Code: 0, Log: "accepted"}}

	if _, err := NewInvalidProposalTxExecutionEvidence(proposal, expectedResults, actualResults, 0, "ante rejected tx"); err != nil {
		t.Fatal(err)
	}
	_, err := NewInvalidProposalTxExecutionEvidence(proposal, expectedResults, expectedResults, 0, "ante rejected tx")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid matching tx execution proof, got %v", err)
	}
	_, err = NewInvalidProposalTxExecutionEvidence(proposal, expectedResults, actualResults, 0, "")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid missing verification message, got %v", err)
	}
	_, err = NewInvalidProposalTxExecutionEvidence(proposal, expectedResults, actualResults, 1, "bad index")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid tx index rejection, got %v", err)
	}
}

func TestInvalidProposalTxValidityEvidenceRequiresResultHashContext(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
			Txs:    []types.Tx{[]byte("bad-tx")},
		},
		Round:    1,
		Proposer: "validator-1",
	}
	expected := HashTxResults([]types.Result{{Code: 1, Log: "ante rejected tx"}})
	expectedResults := []types.Result{{Code: 1, Log: "ante rejected tx"}}
	actualResults := []types.Result{{Code: 0, Log: "accepted"}}
	evidence, err := NewInvalidProposalTxExecutionEvidence(proposal, expectedResults, actualResults, 0, "ante rejected tx")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidence(evidence); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected standalone context rejection, got %v", err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{}); !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected missing context rejection, got %v", err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedTxResultsHash: types.Hash{9}}); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected context mismatch rejection, got %v", err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedTxResultsHash: expected}); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		t.Fatal(err)
	}
	decoded.ExecutionProof = nil
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Proof = encoded
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedTxResultsHash: expected}); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected missing execution proof rejection, got %v", err)
	}
}

func TestInvalidProposalTxValidityEvidenceWithContextBuildsExecutionProof(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
			Txs:    []types.Tx{[]byte("bad-tx")},
		},
		Round:    1,
		Proposer: "validator-1",
	}
	expectedResults := []types.Result{{Code: 1, Log: "ante rejected tx"}}
	actualResults := []types.Result{{Code: 0, Log: "accepted"}}
	expectedHash := HashTxResults(expectedResults)

	evidence, err := NewInvalidProposalEvidenceWithContext(
		proposal,
		InvalidProposalVerificationContext{
			ExpectedTxResultsHash: expectedHash,
			ExpectedTxResults:     expectedResults,
			ActualTxResults:       actualResults,
			TxIndex:               0,
		},
		InvalidProposalReasonTxValidity,
		HashTxResults(actualResults),
		"ante rejected tx",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedTxResultsHash: expectedHash}); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ExecutionProof == nil || decoded.ExecutionProof.TxIndex != 0 {
		t.Fatalf("expected tx execution proof, got %+v", decoded.ExecutionProof)
	}

	_, err = NewInvalidProposalEvidenceWithContext(
		proposal,
		InvalidProposalVerificationContext{
			ExpectedTxResultsHash: expectedHash,
			ExpectedTxResults:     expectedResults,
			ActualTxResults:       actualResults,
			TxIndex:               0,
		},
		InvalidProposalReasonTxValidity,
		types.Hash{9},
		"ante rejected tx",
	)
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected actual tx result hash mismatch rejection, got %v", err)
	}
}

func TestInvalidProposalTxValidityEvidenceWithContextRequiresResults(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
			Txs:    []types.Tx{[]byte("bad-tx")},
		},
		Proposer: "validator-1",
	}
	_, err := NewInvalidProposalEvidenceWithContext(
		proposal,
		InvalidProposalVerificationContext{ExpectedTxResultsHash: types.Hash{1}},
		InvalidProposalReasonTxValidity,
		types.Hash{2},
		"ante rejected tx",
	)
	if !errors.Is(err, ErrInvalidProposalContext) {
		t.Fatalf("expected missing result arrays to be rejected, got %v", err)
	}
}

func TestInvalidProposalTxExecutionEvidenceBindsResultArrays(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7},
			Txs:    []types.Tx{[]byte("first"), []byte("bad-tx")},
		},
		Round:    1,
		Proposer: "validator-1",
	}
	expectedResults := []types.Result{{}, {Code: 1, Log: "ante rejected tx", GasUsed: 1}}
	actualResults := []types.Result{{}, {Code: 0, Data: []byte("accepted"), GasUsed: 9}}
	expected := HashTxResults(expectedResults)
	evidence, err := NewInvalidProposalTxExecutionEvidence(proposal, expectedResults, actualResults, 1, "deterministic tx execution mismatch")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedTxResultsHash: expected}); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeInvalidProposalProof(evidence.Proof)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ExecutionProof == nil || decoded.ExecutionProof.TxIndex != 1 || string(decoded.ExecutionProof.Tx) != "bad-tx" {
		t.Fatalf("execution proof was not bound to tx index: %+v", decoded.ExecutionProof)
	}
	decoded.ExecutionProof.ActualResults[1] = decoded.ExecutionProof.ExpectedResults[1]
	tamperedProof, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Proof = tamperedProof
	if err := VerifyInvalidProposalEvidenceWithContext(evidence, InvalidProposalVerificationContext{ExpectedTxResultsHash: expected}); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected tampered execution proof rejection, got %v", err)
	}
	_, err = NewInvalidProposalTxExecutionEvidence(proposal, expectedResults[:1], actualResults, 1, "bad lengths")
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid result length rejection, got %v", err)
	}
}

func TestSupportedInvalidProposalReasonsExposeOnlyVerifiableReasons(t *testing.T) {
	reasons := SupportedInvalidProposalReasons()
	for _, reason := range reasons {
		switch reason {
		case InvalidProposalReasonDAMismatch,
			InvalidProposalReasonMissingData,
			InvalidProposalReasonValidatorSetHash,
			InvalidProposalReasonAppHash,
			InvalidProposalReasonTxValidity,
			InvalidProposalReasonTimestamp:
		default:
			t.Fatalf("unexpected unsupported reason exposed: %s", reason)
		}
	}
}

func TestInvalidProposalTimestampEvidenceBindsActualToProposal(t *testing.T) {
	proposal := Proposal{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 7, TimeUnixNano: 200},
		},
		Proposer: "validator-1",
	}

	_, err := NewInvalidProposalTimestampEvidence(proposal, 100, 300)
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected timestamp proof to bind actual proposal time, got %v", err)
	}
	if _, err := NewInvalidProposalTimestampEvidence(proposal, 100, 200); err != nil {
		t.Fatal(err)
	}
}
