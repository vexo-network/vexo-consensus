package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// resign signs proposal again after a test changed its header.
func (cluster *registryCluster) resign(proposal Proposal) Proposal {
	cluster.t.Helper()
	if err := cluster.signers.signProposal(&proposal); err != nil {
		cluster.t.Fatal(err)
	}
	return proposal
}

// registryVerifier judges evidence against the cluster's replicated validator sets.
func (cluster *registryCluster) registryVerifier() EvidenceVerifierBundle {
	verifier := NewEvidenceVerifier(vexocrypto.DeterministicSigner{}, vexocrypto.DeterministicAggregateSigner{})
	verifier.FinalitySets = cluster.registry
	return verifier
}

func (cluster *registryCluster) verifyEvidence(evidence slashing.Evidence) error {
	set, err := cluster.registry.ValidatorSet(context.Background(), evidence.Height)
	if err != nil {
		cluster.t.Fatal(err)
	}
	return VerifyEvidence(evidence, set, cluster.registryVerifier())
}

// onlyEvidence returns the one evidence the machine recorded.
func onlyEvidence(t *testing.T, machine *StateMachine) slashing.Evidence {
	t.Helper()
	evidence := machine.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(evidence))
	}
	return evidence[0]
}

// A signed header names its parent by hash, so every rule the parent header fixes for it is
// decidable from the two headers: a proposer that breaks one is punished on a proof that carries
// the parent header, and a proposer that keeps them cannot be framed with another parent.
func TestParentLinkageFaultsAreEvidence(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		mutate func(header *types.Header, parent types.Header)
	}{
		{"a time not after the parent's", func(header *types.Header, parent types.Header) { header.TimeUnixNano = parent.TimeUnixNano }},
		{"another state height", func(header *types.Header, _ types.Header) { header.StateHeight++ }},
		{"another validator set height", func(header *types.Header, _ types.Header) { header.ValidatorSetHeight++ }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			cluster := newRegistryCluster(t, "a", "b", "c")
			machine := cluster.newMachine()
			first := cluster.propose(machine, 0, "a")
			cluster.certify(machine, first, "a", "b", "c")
			second := cluster.propose(machine, 1, "b")
			secondQC := cluster.certify(machine, second, "a", "b", "c")
			cluster.executed(machine, 2)

			honest := cluster.proposal("c", cluster.block(3, HashBlock(second.Block)), 2, secondQC, finality.TimeoutCert{})
			faulty := honest
			testCase.mutate(&faulty.Block.Header, second.Block.Header)
			faulty = cluster.resign(faulty)
			if err := machine.OnProposal(ctx, faulty); !errors.Is(err, ErrInvalidProposal) {
				t.Fatalf("expected the faulty proposal to be refused, got %v", err)
			}
			if err := machine.OnProposal(ctx, faulty); !errors.Is(err, ErrInvalidProposal) {
				t.Fatalf("expected the retransmitted proposal to be refused, got %v", err)
			}
			evidence := onlyEvidence(t, machine)
			if evidence.Validator != "c" || evidence.Height != 3 {
				t.Fatalf("expected evidence against c at height 3, got %+v", evidence)
			}
			if err := cluster.verifyEvidence(evidence); err != nil {
				t.Fatalf("expected the evidence to verify, got %v", err)
			}
			proof, err := DecodeInvalidProposalProof(evidence.Proof)
			if err != nil {
				t.Fatal(err)
			}
			if proof.Reason != InvalidProposalReasonParentLinkage || proof.Parent == nil || *proof.Parent != second.Block.Header {
				t.Fatalf("expected parent_linkage evidence carrying the parent header, got %+v", proof)
			}

			// The honest proposal keeps every rule: no parent makes it a fault.
			if _, err := NewParentLinkageEvidence(honest, &second.Block.Header); !errors.Is(err, ErrInvalidProposal) {
				t.Fatalf("expected no evidence against the honest proposal, got %v", err)
			}
			framedParent := first.Block.Header
			if _, err := NewParentLinkageEvidence(honest, &framedParent); !errors.Is(err, ErrParentNotNamed) {
				t.Fatalf("expected a parent the header does not name to be refused, got %v", err)
			}
			proof.Proposal = honest
			proof.Parent = &framedParent
			handmade := evidence
			handmade.Proof = mustJSON(t, proof)
			if err := cluster.verifyEvidence(handmade); !errors.Is(err, ErrParentNotNamed) {
				t.Fatalf("expected handmade evidence with another parent to be refused, got %v", err)
			}
		})
	}
}

// The first block names no parent: its own header is the whole proof.
func TestParentLinkageOfTheFirstBlock(t *testing.T) {
	cluster := newRegistryCluster(t, "a", "b", "c")
	honest := cluster.proposal("a", cluster.block(1, types.Hash{}), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	if _, err := NewParentLinkageEvidence(honest, nil); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected no evidence against the honest first block, got %v", err)
	}
	timeless := honest
	timeless.Block.Header.TimeUnixNano = 0
	timeless = cluster.resign(timeless)
	evidence, err := NewParentLinkageEvidence(timeless, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cluster.verifyEvidence(evidence); err != nil {
		t.Fatalf("expected a first block without a time to be evidence, got %v", err)
	}
	if _, err := NewParentLinkageEvidence(timeless, &honest.Block.Header); !errors.Is(err, ErrParentNotNamed) {
		t.Fatalf("expected a first block with a parent to be refused, got %v", err)
	}
}

// A header names the hashes of the sets of two registry heights; both sets are replicated state.
// A proposer naming another hash is punished, and an honest proposer cannot be framed with the set
// of another height, the set in effect at the evidence height included.
func TestValidatorSetHashEvidenceIsJudgedAtTheHeightsTheHeaderNames(t *testing.T) {
	ctx := context.Background()
	cluster := newRegistryCluster(t, "a", "b", "c")
	machine := cluster.newMachine()
	first := cluster.propose(machine, 0, "a")
	cluster.certify(machine, first, "a", "b", "c")
	second := cluster.propose(machine, 1, "b")
	secondQC := cluster.certify(machine, second, "a", "b", "c")
	// Executing the first block removes "a": the registry holds another set from height 2.
	if err := cluster.registry.ApplyLeaveAt(ctx, 2, "a"); err != nil {
		t.Fatal(err)
	}
	cluster.executed(machine, 2)
	oldSet, newSet := mustSet(t, cluster.registry, 1), mustSet(t, cluster.registry, 2)

	// Block 3 commits the state after block 1: the old set certifies it, the new one is next.
	build := func() Proposal {
		block := cluster.block(3, HashBlock(second.Block))
		block.Header.ValidatorSetHash = oldSet.Hash()
		block.Header.NextValidatorSetHash = newSet.Hash()
		return cluster.proposal("c", block, 2, secondQC, finality.TimeoutCert{})
	}
	honest := build()
	if honest.Block.Header.ValidatorSetHeight != 1 || honest.Block.Header.StateHeight != 1 {
		t.Fatalf("setup: block 3 must name set 1 and state 1, got %+v", honest.Block.Header)
	}

	staleNext := build()
	staleNext.Block.Header.NextValidatorSetHash = oldSet.Hash()
	staleNext = cluster.resign(staleNext)
	if err := machine.OnProposal(ctx, staleNext); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a proposal naming the old set as next to be refused, got %v", err)
	}
	evidence := onlyEvidence(t, machine)
	if err := cluster.verifyEvidence(evidence); err != nil {
		t.Fatalf("expected next_validator_set_hash evidence to verify, got %v", err)
	}

	// Judged against the set in effect at the evidence height, the honest block's own set would
	// look wrong: it is the set of the height its header names that counts.
	framed, err := NewInvalidProposalHashEvidence(honest, InvalidProposalReasonValidatorSetHash, newSet.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if err := cluster.verifyEvidence(framed); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected the honest proposer not to be framed with the set of the evidence height, got %v", err)
	}
	framedNext, err := NewInvalidProposalHashEvidence(honest, InvalidProposalReasonNextValidatorSetHash, oldSet.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if err := cluster.verifyEvidence(framedNext); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected the honest proposer not to be framed with another next set, got %v", err)
	}
	if err := machine.OnProposal(ctx, honest); err != nil {
		t.Fatalf("expected the honest proposal to be accepted, got %v", err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func mustSet(t *testing.T, registry *validator.InMemoryRegistry, height types.Height) validator.Set {
	t.Helper()
	set, err := registry.ValidatorSet(context.Background(), height)
	if err != nil {
		t.Fatal(err)
	}
	return set
}
