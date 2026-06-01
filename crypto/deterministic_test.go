package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestDeterministicSignerSignsAndVerifies(t *testing.T) {
	signer, err := NewDeterministicSigner([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("hello")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !signer.Verify(signer.PublicKey(), message, signature) {
		t.Fatal("expected signature to verify")
	}
}

func TestDeterministicSignerRejectsWrongMessageOrKey(t *testing.T) {
	signer, err := NewDeterministicSigner([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewDeterministicSigner([]byte("other"))
	if err != nil {
		t.Fatal(err)
	}

	signature, err := signer.Sign([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if signer.Verify(signer.PublicKey(), []byte("bye"), signature) {
		t.Fatal("wrong message verified")
	}
	if signer.Verify(other.PublicKey(), []byte("hello"), signature) {
		t.Fatal("wrong public key verified")
	}
}

func TestDeterministicSignerRejectsEmptyPrivateKey(t *testing.T) {
	_, err := NewDeterministicSigner(nil)
	if !errors.Is(err, ErrEmptyPrivateKey) {
		t.Fatalf("expected empty private key, got %v", err)
	}
}

func TestDeterministicSignerPublicKeyIsCopy(t *testing.T) {
	signer, err := NewDeterministicSigner([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	publicKey := signer.PublicKey()
	publicKey[0] ^= 0xff
	if string(publicKey) == string(signer.PublicKey()) {
		t.Fatal("expected public key copy")
	}
}

func TestDeterministicAggregateSignerAggregatesAndVerifies(t *testing.T) {
	first, err := NewDeterministicSigner([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewDeterministicSigner([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")

	firstSignature, err := first.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	secondSignature, err := second.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	aggregate := DeterministicAggregateSigner{}
	aggregateSignature, err := aggregate.Aggregate([]types.Signature{firstSignature, secondSignature})
	if err != nil {
		t.Fatal(err)
	}
	if !aggregate.VerifyAggregate([]types.PublicKey{first.PublicKey(), second.PublicKey()}, message, aggregateSignature) {
		t.Fatal("expected aggregate signature to verify")
	}
}

func TestDeterministicAggregateSignerRejectsInvalidAggregate(t *testing.T) {
	first, err := NewDeterministicSigner([]byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewDeterministicSigner([]byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")

	firstSignature, err := first.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	aggregate := DeterministicAggregateSigner{}
	aggregateSignature, err := aggregate.Aggregate([]types.Signature{firstSignature})
	if err != nil {
		t.Fatal(err)
	}
	if aggregate.VerifyAggregate([]types.PublicKey{first.PublicKey(), second.PublicKey()}, message, aggregateSignature) {
		t.Fatal("invalid aggregate verified")
	}
	if aggregate.VerifyAggregate([]types.PublicKey{first.PublicKey()}, []byte("other"), aggregateSignature) {
		t.Fatal("wrong message aggregate verified")
	}
}

func TestDeterministicAggregateSignerRejectsEmptyInputs(t *testing.T) {
	aggregate := DeterministicAggregateSigner{}

	if _, err := aggregate.Aggregate(nil); !errors.Is(err, ErrEmptySignature) {
		t.Fatalf("expected empty signature, got %v", err)
	}
	if _, err := aggregate.Aggregate([]types.Signature{nil}); !errors.Is(err, ErrEmptySignature) {
		t.Fatalf("expected empty signature element, got %v", err)
	}
	if aggregate.VerifyAggregate(nil, []byte("msg"), types.AggregateSignature("sig")) {
		t.Fatal("empty public keys verified")
	}
	if aggregate.VerifyAggregate([]types.PublicKey{nil}, []byte("msg"), types.AggregateSignature("sig")) {
		t.Fatal("empty public key verified")
	}
	if aggregate.VerifyAggregate([]types.PublicKey{[]byte("pub")}, []byte("msg"), nil) {
		t.Fatal("empty aggregate signature verified")
	}
}
