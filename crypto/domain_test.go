package crypto

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

func TestDomainSignerRejectsCrossDomainReplay(t *testing.T) {
	baseSigner, err := NewDeterministicSigner([]byte("validator-key"))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	voteSigner, err := NewDomainSigner(baseSigner, DomainConsensusVote)
	if err != nil {
		t.Fatalf("new vote signer: %v", err)
	}
	proposalSigner, err := NewDomainSigner(baseSigner, DomainConsensusProposal)
	if err != nil {
		t.Fatalf("new proposal signer: %v", err)
	}

	message := []byte("block-hash")
	signature, err := voteSigner.Sign(message)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !voteSigner.Verify(baseSigner.PublicKey(), message, signature) {
		t.Fatal("expected vote signature to verify in vote domain")
	}
	if proposalSigner.Verify(baseSigner.PublicKey(), message, signature) {
		t.Fatal("expected vote signature to fail in proposal domain")
	}
}

func TestDomainMessageRejectsEmptyDomain(t *testing.T) {
	if _, err := DomainMessage("", []byte("message")); !errors.Is(err, ErrEmptyDomain) {
		t.Fatalf("expected empty domain error, got %v", err)
	}

	baseSigner, err := NewDeterministicSigner([]byte("validator-key"))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	if _, err := NewDomainSigner(baseSigner, ""); !errors.Is(err, ErrEmptyDomain) {
		t.Fatalf("expected empty domain signer error, got %v", err)
	}
}

func TestDomainAggregateSignerRejectsCrossDomainReplay(t *testing.T) {
	firstSigner, err := NewDeterministicSigner([]byte("first-validator-key"))
	if err != nil {
		t.Fatalf("new first signer: %v", err)
	}
	secondSigner, err := NewDeterministicSigner([]byte("second-validator-key"))
	if err != nil {
		t.Fatalf("new second signer: %v", err)
	}

	message := []byte("finality-proof")
	voteMessage, err := DomainMessage(DomainConsensusVote, message)
	if err != nil {
		t.Fatalf("vote message: %v", err)
	}
	firstSignature, err := firstSigner.Sign(voteMessage)
	if err != nil {
		t.Fatalf("first sign: %v", err)
	}
	secondSignature, err := secondSigner.Sign(voteMessage)
	if err != nil {
		t.Fatalf("second sign: %v", err)
	}

	voteAggregate, err := NewDomainAggregateSigner(DeterministicAggregateSigner{}, DomainConsensusVote)
	if err != nil {
		t.Fatalf("new vote aggregate: %v", err)
	}
	signature, err := voteAggregate.Aggregate([]types.Signature{firstSignature, secondSignature})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	publicKeys := []types.PublicKey{firstSigner.PublicKey(), secondSigner.PublicKey()}
	if !voteAggregate.VerifyAggregate(publicKeys, message, signature) {
		t.Fatal("expected aggregate to verify in vote domain")
	}

	proposalAggregate, err := NewDomainAggregateSigner(DeterministicAggregateSigner{}, DomainConsensusProposal)
	if err != nil {
		t.Fatalf("new proposal aggregate: %v", err)
	}
	if proposalAggregate.VerifyAggregate(publicKeys, message, signature) {
		t.Fatal("expected aggregate to fail in proposal domain")
	}
}
