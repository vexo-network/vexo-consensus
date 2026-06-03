package address

import (
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestAddressFromPublicKeyIsStableAndValid(t *testing.T) {
	publicKey := types.PublicKey("alice-public-key")
	first, err := AccountFromPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AccountFromPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("expected stable address, got %s and %s", first, second)
	}
	if !strings.HasPrefix(string(first), AccountHRP+"1") {
		t.Fatalf("unexpected account prefix: %s", first)
	}
	if err := Validate(first, AccountHRP); err != nil {
		t.Fatal(err)
	}
	if err := MatchesPublicKey(first, AccountHRP, publicKey); err != nil {
		t.Fatal(err)
	}
}

func TestAddressRejectsMismatchedPublicKeyAndPrefix(t *testing.T) {
	account, err := AccountFromPublicKey(types.PublicKey("alice-public-key"))
	if err != nil {
		t.Fatal(err)
	}
	if err := MatchesPublicKey(account, AccountHRP, types.PublicKey("bob-public-key")); err != ErrAddressMismatch {
		t.Fatalf("expected address mismatch, got %v", err)
	}
	if err := Validate(account, ValidatorOperatorHRP); err != ErrInvalidPrefix {
		t.Fatalf("expected prefix mismatch, got %v", err)
	}
}

func TestValidatorAddressPrefixes(t *testing.T) {
	publicKey := types.PublicKey("validator-public-key")
	operator, err := ValidatorOperatorFromPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	consensus, err := ValidatorConsensusFromPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(operator), ValidatorOperatorHRP+"1") {
		t.Fatalf("unexpected operator address: %s", operator)
	}
	if !strings.HasPrefix(string(consensus), ValidatorConsensusHRP+"1") {
		t.Fatalf("unexpected consensus address: %s", consensus)
	}
	if operator == consensus {
		t.Fatal("expected distinct validator address namespaces")
	}
}
