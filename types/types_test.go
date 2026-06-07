package types

import "testing"

func TestCoreValueTypesAreStable(t *testing.T) {
	hash := Hash{1, 2, 3}
	if len(hash) != 32 {
		t.Fatalf("hash length changed: %d", len(hash))
	}
	if Address("alice") != "alice" ||
		ValidatorID("validator-1") != "validator-1" ||
		Height(7) != 7 ||
		Round(2) != 2 ||
		VotingPower(100) != 100 {
		t.Fatal("core alias semantics changed")
	}
}

func TestValidatorUpdateCarriesMetadata(t *testing.T) {
	update := ValidatorUpdate{
		ID:          "validator-1",
		Address:     "vexo1abc",
		PublicKey:   PublicKey{1, 2, 3},
		VotingPower: 10,
		Stake:       20,
		Metadata:    map[string]string{"role": "validator"},
	}
	if update.ID == "" || update.Address == "" || len(update.PublicKey) == 0 || update.Metadata["role"] != "validator" {
		t.Fatalf("unexpected validator update: %+v", update)
	}
}
