package slashing

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func validEvidence(evidenceType EvidenceType) Evidence {
	return Evidence{Type: evidenceType, Validator: "alice", Height: 7, Round: 2, Proof: []byte("proof")}
}

func TestNormalizeMapsLegacyDoubleSignLabel(t *testing.T) {
	if got := validEvidence(EvidenceDoubleSign).Normalize().Type; got != EvidenceConflictingVote {
		t.Fatalf("expected conflicting_vote, got %s", got)
	}
	if got := validEvidence(EvidenceInvalidProposal).Normalize().Type; got != EvidenceInvalidProposal {
		t.Fatalf("normalize must keep %s, got %s", EvidenceInvalidProposal, got)
	}
}

// Two labels of one offence used to be two dedup keys, so one double sign was slashed twice.
func TestOffenceKeyIsIdenticalForBothDoubleSignLabels(t *testing.T) {
	legacy, err := OffenceKey(validEvidence(EvidenceDoubleSign))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := OffenceKey(validEvidence(EvidenceConflictingVote))
	if err != nil {
		t.Fatal(err)
	}
	if legacy != canonical {
		t.Fatalf("one offence must have one key, got %q and %q", legacy, canonical)
	}
}

func TestOffenceKeyIgnoresProofAndSeparatesOffences(t *testing.T) {
	base := validEvidence(EvidenceConflictingVote)
	baseKey, err := OffenceKey(base)
	if err != nil {
		t.Fatal(err)
	}
	otherProof := base
	otherProof.Proof = []byte("another proof of the same offence")
	if key, _ := OffenceKey(otherProof); key != baseKey {
		t.Fatalf("a different proof of one offence must keep its key: %q != %q", key, baseKey)
	}

	distinct := map[string]Evidence{
		"validator":    {Type: base.Type, Validator: "bob", Height: base.Height, Round: base.Round},
		"height":       {Type: base.Type, Validator: base.Validator, Height: base.Height + 1, Round: base.Round},
		"round":        {Type: base.Type, Validator: base.Validator, Height: base.Height, Round: base.Round + 1},
		"timeout vote": {Type: EvidenceConflictingTimeoutVote, Validator: base.Validator, Height: base.Height, Round: base.Round},
		"proposal":     {Type: EvidenceInvalidProposal, Validator: base.Validator, Height: base.Height, Round: base.Round},
		"finality":     {Type: EvidenceFinalityConflict, Validator: base.Validator, Height: base.Height, Round: base.Round},
	}
	seen := map[string]string{baseKey: "base"}
	for name, evidence := range distinct {
		key, err := OffenceKey(evidence)
		if err != nil {
			t.Fatal(err)
		}
		if other, found := seen[key]; found {
			t.Fatalf("%s shares the key %q with %s", name, key, other)
		}
		seen[key] = name
	}
}

func TestOffenceKeyGroupsProposalOffencesAndIgnoresFinalityRound(t *testing.T) {
	invalid, _ := OffenceKey(validEvidence(EvidenceInvalidProposal))
	unavailable, _ := OffenceKey(validEvidence(EvidenceUnavailableData))
	if invalid != unavailable {
		t.Fatalf("one proposer offence at one height and round must have one key: %q != %q", invalid, unavailable)
	}
	first := validEvidence(EvidenceFinalityConflict)
	second := first
	second.Round = first.Round + 5
	firstKey, _ := OffenceKey(first)
	secondKey, _ := OffenceKey(second)
	if firstKey != secondKey {
		t.Fatalf("swapping the proofs of a finality conflict must not create a second offence: %q != %q", firstKey, secondKey)
	}
}

func TestOffenceKeyCannotBeForgedThroughValidatorIDText(t *testing.T) {
	left := Evidence{Type: EvidenceConflictingVote, Validator: "a/1", Height: 2, Round: 3}
	right := Evidence{Type: EvidenceConflictingVote, Validator: "a", Height: 1, Round: 2}
	leftKey, _ := OffenceKey(left)
	rightKey, _ := OffenceKey(right)
	if leftKey == rightKey {
		t.Fatalf("distinct offences share the key %q", leftKey)
	}
}

func TestOffenceKeyRejectsIncompleteEvidence(t *testing.T) {
	cases := []struct {
		name     string
		evidence Evidence
		expected error
	}{
		{"unknown type", Evidence{Type: "other", Validator: "alice", Height: 1}, ErrUnknownEvidenceType},
		{"missing validator", Evidence{Type: EvidenceConflictingVote, Height: 1}, ErrMissingValidator},
		{"missing height", Evidence{Type: EvidenceConflictingVote, Validator: "alice"}, ErrMissingHeight},
	}
	for _, testCase := range cases {
		if _, err := OffenceKey(testCase.evidence); !errors.Is(err, testCase.expected) {
			t.Fatalf("%s: expected %v, got %v", testCase.name, testCase.expected, err)
		}
	}
}

func TestEvidenceValidate(t *testing.T) {
	if err := validEvidence(EvidenceDoubleSign).Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		evidence Evidence
		expected error
	}{
		{"missing validator", Evidence{Type: EvidenceConflictingVote, Height: 1, Proof: []byte("p")}, ErrMissingValidator},
		{"missing height", Evidence{Type: EvidenceConflictingVote, Validator: "alice", Proof: []byte("p")}, ErrMissingHeight},
		{"empty proof", Evidence{Type: EvidenceConflictingVote, Validator: "alice", Height: 1}, ErrEmptyProof},
		{"unknown type", Evidence{Type: "other", Validator: "alice", Height: 1, Proof: []byte("p")}, ErrUnknownEvidenceType},
	}
	for _, testCase := range cases {
		if err := testCase.evidence.Validate(); !errors.Is(err, testCase.expected) {
			t.Fatalf("%s: expected %v, got %v", testCase.name, testCase.expected, err)
		}
	}
}

func TestEvidenceCloneSharesNoProofMemory(t *testing.T) {
	evidence := validEvidence(EvidenceConflictingVote)
	clone := evidence.Clone()
	clone.Proof[0] = 'X'
	if evidence.Proof[0] == 'X' {
		t.Fatal("clone shares the proof buffer")
	}
}

func TestApplySlash(t *testing.T) {
	cases := []struct {
		name     string
		power    types.VotingPower
		penalty  Penalty
		expected types.VotingPower
	}{
		{"five percent", 100, Penalty{SlashBPS: 500}, 95},
		{"rounds the remainder down", 99, Penalty{SlashBPS: 500}, 94},
		{"no slash", 100, Penalty{}, 100},
		{"full slash", 100, Penalty{SlashBPS: BasisPoints}, 0},
		{"zero power", 0, Penalty{SlashBPS: 500}, 0},
		{"huge power does not overflow", types.VotingPower(math.MaxUint64), Penalty{SlashBPS: 5000}, types.VotingPower(math.MaxUint64 / 2)},
	}
	for _, testCase := range cases {
		remaining, err := ApplySlash(testCase.power, testCase.penalty)
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if remaining != testCase.expected {
			t.Fatalf("%s: expected %d, got %d", testCase.name, testCase.expected, remaining)
		}
	}
}

// The slash fraction used to be a float parsed from text, which accepted "NaN": every comparison
// with NaN is false, so the range check passed and the power became garbage. A penalty is now an
// integer, and anything outside 0..10000 basis points is rejected.
func TestApplySlashRejectsOutOfRangeFractions(t *testing.T) {
	for _, bps := range []uint64{BasisPoints + 1, math.MaxUint64} {
		if _, err := ApplySlash(100, Penalty{SlashBPS: bps}); !errors.Is(err, ErrInvalidSlashFraction) {
			t.Fatalf("bps %d: expected invalid slash fraction, got %v", bps, err)
		}
	}
}

func TestPenaltyDoesNotDecodeNaNOrFractions(t *testing.T) {
	for _, document := range []string{
		`{"SlashBPS":"NaN"}`,
		`{"SlashBPS":NaN}`,
		`{"SlashBPS":0.05}`,
		`{"SlashBPS":-1}`,
		`{"SlashBPS":1e400}`,
	} {
		var penalty Penalty
		if err := json.Unmarshal([]byte(document), &penalty); err == nil {
			t.Fatalf("%s decoded to %+v", document, penalty)
		}
	}
}

func TestPenaltyPolicyLooksUpByNormalizedType(t *testing.T) {
	policy := DefaultPenaltyPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	legacy, err := policy.Penalty(EvidenceDoubleSign)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := policy.Penalty(EvidenceConflictingVote)
	if err != nil {
		t.Fatal(err)
	}
	if legacy != canonical {
		t.Fatalf("both labels must be punished alike: %+v != %+v", legacy, canonical)
	}
	if _, err := (PenaltyPolicy{}).Penalty(EvidenceConflictingVote); !errors.Is(err, ErrPenaltyNotConfigured) {
		t.Fatalf("expected penalty not configured, got %v", err)
	}
}

func TestPenaltyPolicyValidateRejectsBadEntries(t *testing.T) {
	cases := map[string]PenaltyPolicy{
		"slash above everything": {EvidenceConflictingVote: {SlashBPS: BasisPoints + 1}},
		"unknown type":           {"other": {SlashBPS: 1}},
		"legacy label":           {EvidenceDoubleSign: {SlashBPS: 1}},
	}
	for name, policy := range cases {
		if err := policy.Validate(); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}
