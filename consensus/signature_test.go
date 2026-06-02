package consensus

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStateMachineVerifiesProposalAndVoteDomains(t *testing.T) {
	firstSigner, err := vexocrypto.NewDeterministicSigner([]byte("first-validator-key"))
	if err != nil {
		t.Fatalf("new first signer: %v", err)
	}
	secondSigner, err := vexocrypto.NewDeterministicSigner([]byte("second-validator-key"))
	if err != nil {
		t.Fatalf("new second signer: %v", err)
	}
	thirdSigner, err := vexocrypto.NewDeterministicSigner([]byte("third-validator-key"))
	if err != nil {
		t.Fatalf("new third signer: %v", err)
	}

	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", PublicKey: firstSigner.PublicKey(), VotingPower: 1},
		{ID: "b", PublicKey: secondSigner.PublicKey(), VotingPower: 1},
		{ID: "c", PublicKey: thirdSigner.PublicKey(), VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Signatures:   vexocrypto.DeterministicSigner{},
		Aggregator:   vexocrypto.DeterministicAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}

	block := dataavailability.AttachCommitment(types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1, ValidatorSetHash: set.Hash()},
		Txs:    []types.Tx{[]byte("tx")},
	})
	proposal, err := machine.CreateProposal(block, 0, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	if err := machine.OnProposal(context.Background(), proposal); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected unsigned proposal rejection, got %v", err)
	}
	if err := signProposalForTest(firstSigner, &proposal); err != nil {
		t.Fatalf("sign proposal: %v", err)
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatalf("expected signed proposal acceptance: %v", err)
	}

	blockHash := HashBlock(proposal.Block)
	firstVote := Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "a"}
	if err := machine.OnVote(context.Background(), firstVote); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected unsigned vote rejection, got %v", err)
	}
	if err := signVoteForTest(firstSigner, &firstVote, vexocrypto.DomainConsensusProposal); err != nil {
		t.Fatalf("sign wrong domain vote: %v", err)
	}
	if err := machine.OnVote(context.Background(), firstVote); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected wrong-domain vote rejection, got %v", err)
	}

	firstVote.Signature = nil
	secondVote := Vote{Height: 1, Round: 0, BlockHash: blockHash, ValidatorID: "b"}
	if err := signVoteForTest(firstSigner, &firstVote, vexocrypto.DomainConsensusVote); err != nil {
		t.Fatalf("sign first vote: %v", err)
	}
	if err := signVoteForTest(secondSigner, &secondVote, vexocrypto.DomainConsensusVote); err != nil {
		t.Fatalf("sign second vote: %v", err)
	}
	if err := machine.OnVote(context.Background(), firstVote); err != nil {
		t.Fatalf("first vote: %v", err)
	}
	if err := machine.OnVote(context.Background(), secondVote); err != nil {
		t.Fatalf("second vote: %v", err)
	}

	quorumCert, err := machine.BuildQuorumCert(1, 0, blockHash)
	if err != nil {
		t.Fatalf("build qc: %v", err)
	}
	if string(quorumCert.Signature) == "placeholder-aggregate-signature" {
		t.Fatal("expected real aggregate signature")
	}
	voteAggregate, err := vexocrypto.NewDomainAggregateSigner(vexocrypto.DeterministicAggregateSigner{}, vexocrypto.DomainConsensusVote)
	if err != nil {
		t.Fatalf("new aggregate verifier: %v", err)
	}
	if !voteAggregate.VerifyAggregate([]types.PublicKey{firstSigner.PublicKey(), secondSigner.PublicKey()}, VoteSignBytes(firstVote), quorumCert.Signature) {
		t.Fatal("expected aggregate vote signature to verify")
	}
}

func TestStateMachineVerifiesTimeoutVoteDomain(t *testing.T) {
	firstSigner, err := vexocrypto.NewDeterministicSigner([]byte("first-validator-key"))
	if err != nil {
		t.Fatalf("new first signer: %v", err)
	}
	secondSigner, err := vexocrypto.NewDeterministicSigner([]byte("second-validator-key"))
	if err != nil {
		t.Fatalf("new second signer: %v", err)
	}
	thirdSigner, err := vexocrypto.NewDeterministicSigner([]byte("third-validator-key"))
	if err != nil {
		t.Fatalf("new third signer: %v", err)
	}

	set := newTestValidatorSet([]validator.Validator{
		{ID: "a", PublicKey: firstSigner.PublicKey(), VotingPower: 1},
		{ID: "b", PublicKey: secondSigner.PublicKey(), VotingPower: 1},
		{ID: "c", PublicKey: thirdSigner.PublicKey(), VotingPower: 1},
	})
	machine, err := NewStateMachine(StateMachineConfig{
		ChainID:      "vexo-test",
		ValidatorSet: set,
		Signatures:   vexocrypto.DeterministicSigner{},
		Aggregator:   vexocrypto.DeterministicAggregateSigner{},
	})
	if err != nil {
		t.Fatal(err)
	}
	machine.StartRound(1, 0)

	firstVote := TimeoutVote{Height: 1, Round: 0, ValidatorID: "a"}
	if _, err := machine.OnTimeoutVote(context.Background(), firstVote); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected unsigned timeout vote rejection, got %v", err)
	}
	if err := signTimeoutVoteForTest(firstSigner, &firstVote, vexocrypto.DomainConsensusTimeoutVote); err != nil {
		t.Fatalf("sign first timeout vote: %v", err)
	}
	if _, err := machine.OnTimeoutVote(context.Background(), firstVote); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum after first timeout vote, got %v", err)
	}

	secondVote := TimeoutVote{Height: 1, Round: 0, ValidatorID: "b"}
	if err := signTimeoutVoteForTest(secondSigner, &secondVote, vexocrypto.DomainConsensusTimeoutVote); err != nil {
		t.Fatalf("sign second timeout vote: %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(context.Background(), secondVote)
	if err != nil {
		t.Fatalf("second timeout vote: %v", err)
	}
	if string(timeoutCert.Signature) == "placeholder-timeout-signature" {
		t.Fatal("expected real timeout aggregate signature")
	}
}

func signProposalForTest(signer vexocrypto.Signer, proposal *Proposal) error {
	signature, err := vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusProposal, ProposalSignBytes(*proposal))
	if err != nil {
		return err
	}
	proposal.Signature = signature
	return nil
}

func signVoteForTest(signer vexocrypto.Signer, vote *Vote, domain vexocrypto.Domain) error {
	signature, err := vexocrypto.SignWithDomain(signer, domain, VoteSignBytes(*vote))
	if err != nil {
		return err
	}
	vote.Signature = signature
	return nil
}

func signTimeoutVoteForTest(signer vexocrypto.Signer, vote *TimeoutVote, domain vexocrypto.Domain) error {
	signature, err := vexocrypto.SignWithDomain(signer, domain, TimeoutVoteSignBytes(*vote))
	if err != nil {
		return err
	}
	vote.Signature = signature
	return nil
}
