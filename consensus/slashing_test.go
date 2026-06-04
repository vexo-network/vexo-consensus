package consensus

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestSubmitEvidenceForSlashingReducesValidatorPower(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected slash result: %+v", result)
	}
	if keeper.EvidenceCount() != 1 || keeper.PenaltyCount() != 1 {
		t.Fatalf("expected keeper records, got evidence=%d penalty=%d", keeper.EvidenceCount(), keeper.PenaltyCount())
	}
	set, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	validatorInfo, found := set.Get("a")
	if !found {
		t.Fatal("expected validator")
	}
	if validatorInfo.VotingPower != 75 {
		t.Fatalf("expected updated voting power 75, got %d", validatorInfo.VotingPower)
	}
}

func TestSubmitEvidenceForSlashingRejectsTamperedEvidence(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Validator = "b"

	_, err = SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if !errors.Is(err, ErrEvidenceValidatorNotFound) {
		t.Fatalf("expected validator lookup rejection, got %v", err)
	}
	if keeper.EvidenceCount() != 0 {
		t.Fatal("tampered evidence must not be submitted")
	}
}

func TestSubmitEvidenceForSlashingRejectsUnknownValidator(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	unknownSigner := testEvidenceSigner(t, "b")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, unknownSigner, "b", 1, 0, types.Hash{1}),
		signedTestVote(t, unknownSigner, "b", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if !errors.Is(err, ErrEvidenceValidatorNotFound) {
		t.Fatalf("expected unknown evidence validator, got %v", err)
	}
}

func TestSubmitEvidenceForSlashingRejectsDuplicateEvidence(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence); err != nil {
		t.Fatal(err)
	}
	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if err != nil {
		t.Fatalf("expected duplicate evidence to resume idempotently, got %v", err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 95 {
		t.Fatalf("expected existing receipt, got %+v", result)
	}
}

func TestSubmitTimeoutEvidenceForSlashingReducesValidatorPower(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceConflictingTimeoutVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	evidence, err := NewConflictingTimeoutVoteEvidence(
		signedTestTimeoutVote(t, signer, TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}}),
		signedTestTimeoutVote(t, signer, TimeoutVote{Height: 1, Round: 0, ValidatorID: "a", HighQC: finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}}}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected slash result: %+v", result)
	}
}

func TestSubmitEvidenceForSlashingSupportsStoreBackedState(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper, err := slashing.NewStoreKeeper(storage, slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected store-backed slash result: %+v", result)
	}
	status, found, err := keeper.EvidenceLifecycle(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !found || status != slashing.EvidenceStatusApplied {
		t.Fatalf("expected applied evidence, got %s found=%t", status, found)
	}
	reopenedRegistry, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, err := reopenedRegistry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	validatorInfo, found := set.Get("a")
	if !found || validatorInfo.VotingPower != 75 {
		t.Fatalf("expected persisted voting power 75, got %+v found=%t", validatorInfo, found)
	}
}

func TestSubmitEvidenceForSlashingAppliesAtExplicitHeight(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper, err := slashing.NewStoreKeeper(storage, slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 10, evidence); err != nil {
		t.Fatal(err)
	}
	setAtEvidenceHeight, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	original, found := setAtEvidenceHeight.Get("a")
	if !found || original.VotingPower != 100 {
		t.Fatalf("expected height 1 power preserved, got %+v found=%t", original, found)
	}
	setAtApplyHeight, err := registry.ValidatorSet(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	slashed, found := setAtApplyHeight.Get("a")
	if !found || slashed.VotingPower != 75 {
		t.Fatalf("expected height 10 power slashed, got %+v found=%t", slashed, found)
	}
}

func TestSubmitEvidenceForSlashingUsesApplyHeightCurrentPower(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.UpdateVotingPowerAt(context.Background(), 5, "a", 200); err != nil {
		t.Fatal(err)
	}
	keeper, err := slashing.NewStoreKeeper(storage, slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 10, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 200 || result.RemainingPower != 150 {
		t.Fatalf("expected slash from apply-height current power, got %+v", result)
	}
	setAtEvidenceHeight, err := registry.ValidatorSet(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	original, found := setAtEvidenceHeight.Get("a")
	if !found || original.VotingPower != 100 {
		t.Fatalf("expected evidence-height power preserved, got %+v found=%t", original, found)
	}
	setAtApplyHeight, err := registry.ValidatorSet(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	slashed, found := setAtApplyHeight.Get("a")
	if !found || slashed.VotingPower != 150 {
		t.Fatalf("expected apply-height power 150, got %+v found=%t", slashed, found)
	}
}

func TestSubmitEvidenceForSlashingResumesSubmittedOnlyEvidence(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper, err := slashing.NewStoreKeeper(storage, slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.SubmitEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 10, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("expected resumed slash result, got %+v", result)
	}
	setAtApplyHeight, err := registry.ValidatorSet(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	slashed, found := setAtApplyHeight.Get("a")
	if !found || slashed.VotingPower != 75 {
		t.Fatalf("expected registry update after resume, got %+v found=%t", slashed, found)
	}
}

func TestSubmitEvidenceForSlashingResumesPenaltyOnlyEvidence(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.UpdateVotingPowerAt(context.Background(), 5, "a", 200); err != nil {
		t.Fatal(err)
	}
	keeper, err := slashing.NewStoreKeeper(storage, slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.SubmitEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.ApplyPenaltyWithStake(context.Background(), evidence, 200); err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 10, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 200 || result.RemainingPower != 150 {
		t.Fatalf("expected existing penalty receipt to be reused, got %+v", result)
	}
	setAtApplyHeight, err := registry.ValidatorSet(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	slashed, found := setAtApplyHeight.Get("a")
	if !found || slashed.VotingPower != 150 {
		t.Fatalf("expected registry update from existing receipt, got %+v found=%t", slashed, found)
	}
}

func TestSubmitEvidenceForSlashingAccountsForInactiveValidator(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	registry, err := validator.NewStoreRegistry(context.Background(), storage, nil, 1, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
		{ID: "b", Address: "b", VotingPower: 1, Stake: 1, PublicKey: []byte("b-pub")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.ApplyLeaveAt(context.Background(), 5, "a"); err != nil {
		t.Fatal(err)
	}
	keeper, err := slashing.NewStoreKeeper(storage, slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "0.25", JailDuration: 30},
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 10, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("expected inactive slash accounting from evidence power, got %+v", result)
	}
	if receipt, found, err := keeper.PenaltyReceipt(context.Background(), evidence); err != nil || !found || receipt.RemainingPower != 75 {
		t.Fatalf("expected durable inactive penalty receipt, found=%t receipt=%+v err=%v", found, receipt, err)
	}
	setAtApplyHeight, err := registry.ValidatorSet(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := setAtApplyHeight.Get("a"); found {
		t.Fatal("inactive validator must not be reintroduced into active set")
	}
}

func TestSubmitEvidenceForSlashingFullSlashRemovesValidator(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
		{ID: "b", Address: "b", VotingPower: 1, Stake: 1, PublicKey: []byte("b-pub")},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceConflictingVote: {SlashFraction: "1.0", JailDuration: 30},
	})
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 2, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.RemainingPower != 0 {
		t.Fatalf("expected full slash to zero power, got %+v", result)
	}
	setAtApplyHeight, err := registry.ValidatorSet(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := setAtApplyHeight.Get("a"); found {
		t.Fatal("fully slashed validator must leave active set")
	}
}

func TestSubmitDoubleSignEvidenceForSlashingUsesConflictingVoteProof(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceDoubleSign: {SlashFraction: "0.25", JailDuration: 30},
	})
	evidence, err := NewConflictingVoteEvidence(
		signedTestVote(t, signer, "a", 1, 0, types.Hash{1}),
		signedTestVote(t, signer, "a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Type = slashing.EvidenceDoubleSign

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected double-sign slash result: %+v", result)
	}
}

func TestSubmitInvalidProposalEvidenceForSlashing(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceInvalidProposal: {SlashFraction: "0.25", JailDuration: 30},
	})
	proposal := signedTestProposal(t, signer, Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1, ConsensusHash: dataavailability.Commitment([]types.Tx{[]byte("other")})}, Txs: []types.Tx{[]byte("tx")}},
		Round:    0,
		Proposer: "a",
	})
	evidence, err := NewInvalidProposalEvidence(proposal, string(InvalidProposalReasonDAMismatch))
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected invalid proposal slash result: %+v", result)
	}
}

func TestSubmitUnavailableDataEvidenceForSlashing(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(slashing.PenaltyPolicy{
		slashing.EvidenceUnavailableData: {SlashFraction: "0.25", JailDuration: 30},
	})
	proposal := signedTestProposal(t, signer, Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}, Txs: []types.Tx{[]byte("tx")}},
		Round:    0,
		Proposer: "a",
	})
	evidence, err := NewUnavailableDataEvidence(proposal, "missing data availability commitment")
	if err != nil {
		t.Fatal(err)
	}

	result, err := SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreviousPower != 100 || result.RemainingPower != 75 {
		t.Fatalf("unexpected unavailable data slash result: %+v", result)
	}
}

func TestSubmitEvidenceForSlashingRejectsUnsignedProof(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence, err := NewConflictingVoteEvidence(
		testVote("a", 1, 0, types.Hash{1}),
		testVote("a", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if !errors.Is(err, ErrInvalidEvidenceVoteSignature) {
		t.Fatalf("expected invalid vote signature, got %v", err)
	}
}

func TestSubmitEvidenceForSlashingRejectsUnsupportedProofType(t *testing.T) {
	signer := testEvidenceSigner(t, "a")
	registry, err := validator.NewInMemoryRegistry(nil, []validator.Validator{
		{ID: "a", Address: "a", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	})
	if err != nil {
		t.Fatal(err)
	}
	keeper := slashing.NewInMemoryKeeper(nil)
	evidence := slashing.Evidence{Type: slashing.EvidenceType("opaque"), Validator: "a", Height: 1, Proof: []byte("opaque")}

	_, err = SubmitEvidenceForSlashing(context.Background(), keeper, registry, vexocrypto.DeterministicSigner{}, 0, evidence)
	if !errors.Is(err, ErrUnsupportedEvidenceProof) {
		t.Fatalf("expected unsupported proof, got %v", err)
	}
}

func testEvidenceSigner(t *testing.T, validatorID types.ValidatorID) vexocrypto.DeterministicSigner {
	t.Helper()
	signer, err := vexocrypto.NewDeterministicSigner([]byte(string(validatorID) + "-evidence-key"))
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func signedTestVote(t *testing.T, signer vexocrypto.Signer, validatorID types.ValidatorID, height types.Height, round types.Round, blockHash types.Hash) Vote {
	t.Helper()
	vote := testVote(validatorID, height, round, blockHash)
	signature, err := vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusVote, VoteSignBytes(vote))
	if err != nil {
		t.Fatal(err)
	}
	vote.Signature = signature
	return vote
}

func signedTestTimeoutVote(t *testing.T, signer vexocrypto.Signer, vote TimeoutVote) TimeoutVote {
	t.Helper()
	signature, err := vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusTimeoutVote, TimeoutVoteSignBytes(vote))
	if err != nil {
		t.Fatal(err)
	}
	vote.Signature = signature
	return vote
}

func signedTestProposal(t *testing.T, signer vexocrypto.Signer, proposal Proposal) Proposal {
	t.Helper()
	signature, err := vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusProposal, ProposalSignBytes(proposal))
	if err != nil {
		t.Fatal(err)
	}
	proposal.Signature = signature
	return proposal
}
