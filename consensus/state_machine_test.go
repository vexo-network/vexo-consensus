package consensus

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestStateMachineBuildsQuorumCert(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	block := cluster.block(1, types.Hash{}, "tx")
	blockHash := HashBlock(block)
	if err := machine.OnProposal(ctx, cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}

	if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if err := machine.OnVote(ctx, cluster.vote("b", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}

	qc, err := machine.BuildQuorumCert(1, 0, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	if qc.VotingPower != 2 {
		t.Fatalf("expected voting power 2, got %d", qc.VotingPower)
	}
	if err := machine.VerifyQuorumCert(qc); err != nil {
		t.Fatalf("built quorum cert must verify: %v", err)
	}
	if machine.Status(ctx).Phase != PhaseCommit {
		t.Fatalf("expected commit phase")
	}
}

func TestStateMachineBuildQuorumCertRejectsVotingPowerOverflow(t *testing.T) {
	cluster := newTestCluster(t,
		validator.Validator{ID: "a", VotingPower: ^types.VotingPower(0)},
		validator.Validator{ID: "b", VotingPower: 1},
	)
	machine := cluster.newMachine()
	ctx := context.Background()

	block := cluster.block(1, types.Hash{})
	blockHash := HashBlock(block)
	if err := machine.OnProposal(ctx, cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	for _, voter := range []types.ValidatorID{"b", "a"} {
		if err := machine.OnVote(ctx, cluster.vote(voter, 1, 0, blockHash)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := machine.BuildQuorumCert(1, 0, blockHash); !errors.Is(err, types.ErrVotingPowerOverflow) {
		t.Fatalf("expected voting power overflow, got %v", err)
	}
}

func TestStateMachineStoresVoteQuorumCertInBlockTree(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	block := cluster.block(1, types.Hash{})
	blockHash := HashBlock(block)
	proposal := cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})
	if err := machine.OnProposal(ctx, proposal); err != nil {
		t.Fatal(err)
	}

	if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}
	node, found := machine.blockTree.Get(blockHash)
	if !found {
		t.Fatal("expected proposed block in tree")
	}
	if node.QuorumCert.Height != 0 {
		t.Fatal("expected no quorum cert before quorum")
	}

	if err := machine.OnVote(ctx, cluster.vote("b", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}
	node, _ = machine.blockTree.Get(blockHash)
	if node.QuorumCert.BlockHash != blockHash || node.QuorumCert.VotingPower != 2 {
		t.Fatalf("expected stored quorum cert, got %+v", node.QuorumCert)
	}
	if highQC := machine.HighQC(ctx); highQC.BlockHash != blockHash || highQC.Height != 1 {
		t.Fatalf("expected highQC to track vote quorum cert, got %+v", highQC)
	}
	if err := machine.OnProposal(ctx, proposal); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected duplicate certified proposal to be idempotent, got %v", err)
	}
}

func TestStateMachineWeightedQuorum(t *testing.T) {
	cluster := newTestCluster(t,
		validator.Validator{ID: "a", VotingPower: 4},
		validator.Validator{ID: "b", VotingPower: 2},
		validator.Validator{ID: "c", VotingPower: 1},
	)
	machine := cluster.newMachine()
	ctx := context.Background()

	block := cluster.block(1, types.Hash{})
	blockHash := HashBlock(block)
	if err := machine.OnProposal(ctx, cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}

	if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum with 4/7 power, got %v", err)
	}
	if err := machine.OnVote(ctx, cluster.vote("b", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.BuildQuorumCert(1, 0, blockHash); err != nil {
		t.Fatalf("expected quorum with 6/7 power, got %v", err)
	}
}

func TestStateMachineRequiresVerifiersAndSigners(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	cases := map[string]func(*StateMachineConfig){
		"signature verifier": func(config *StateMachineConfig) { config.Signatures = nil },
		"aggregate signer":   func(config *StateMachineConfig) { config.Aggregator = nil },
		"aggregate verifier": func(config *StateMachineConfig) { config.AggregateVerifier = nil },
		"validator set":      func(config *StateMachineConfig) { config.ValidatorSet = nil },
		"chain id":           func(config *StateMachineConfig) { config.ChainID = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			config := StateMachineConfig{
				ChainID:           testChainID,
				ValidatorSet:      cluster.set,
				Signatures:        vexocrypto.DeterministicSigner{},
				Aggregator:        vexocrypto.DeterministicAggregateSigner{},
				AggregateVerifier: vexocrypto.DeterministicAggregateSigner{},
			}
			mutate(&config)
			if _, err := NewStateMachine(config); err == nil {
				t.Fatalf("a state machine without a %s must not start: there is no unsigned mode", name)
			}
		})
	}
}

func TestStateMachineCreateProposalUsesHighQC(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 1)

	proposal, err := machine.CreateProposal(types.Block{}, 1, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.JustifyQC.BlockHash != blocks[0].hash {
		t.Fatalf("expected high qc as justify qc, got %+v", proposal.JustifyQC)
	}
	if proposal.Block.Header.PreviousBlockHash != blocks[0].hash || proposal.Block.Header.Height != 2 {
		t.Fatalf("expected proposal to extend the high qc block, got %+v", proposal.Block.Header)
	}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(ctx, proposal); err != nil {
		t.Fatalf("proposal built from the high qc must be accepted: %v", err)
	}
}

func TestStateMachineCreateProposalRequiresJustifyQCAboveGenesis(t *testing.T) {
	machine := newThreeValidatorCluster(t).newMachine()

	if _, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 5}}, 0, "a", finality.QuorumCert{}); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a proposal above height 1 without any qc to be refused, got %v", err)
	}
}

func TestStateMachineCreateProposalKeepsExplicitQC(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	blocks := cluster.extendChain(machine, 2)

	// The explicit certificate is not the highest one the machine holds.
	explicitQC := blocks[0].qc
	proposal, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 2}}, 1, "a", explicitQC)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.JustifyQC.BlockHash != explicitQC.BlockHash {
		t.Fatalf("expected explicit qc, got %+v", proposal.JustifyQC)
	}
	if proposal.Block.Header.PreviousBlockHash != explicitQC.BlockHash {
		t.Fatal("expected proposal parent to follow explicit qc")
	}
	if _, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 3}}, 1, "a", explicitQC); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a block that does not extend the qc to be refused, got %v", err)
	}
	if _, err := machine.CreateProposal(types.Block{Header: types.Header{Height: 2}}, 0, "a", explicitQC); !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected a view that does not follow the qc view to be refused, got %v", err)
	}
}

func TestStateMachineCreateProposalAttachesTimeoutCertForSkippedView(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 1)

	// View 1 times out; the next proposal is for view 2 and must carry the certificate.
	for _, voter := range []types.ValidatorID{"a", "b"} {
		_, _ = machine.OnTimeoutVote(ctx, cluster.timeoutVote(voter, 2, 1, blocks[0].qc))
	}
	proposal, err := machine.CreateProposal(types.Block{}, 2, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.TimeoutCert.Round != 1 || proposal.JustifyQC.BlockHash != blocks[0].hash {
		t.Fatalf("expected the timeout certificate of view 1 and the high qc, got %+v", proposal)
	}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(ctx, proposal); err != nil {
		t.Fatalf("proposal carrying its timeout certificate must be accepted: %v", err)
	}

	if _, err := machine.CreateProposal(types.Block{}, 3, "a", finality.QuorumCert{}); !errors.Is(err, ErrInvalidTimeoutCert) {
		t.Fatalf("expected a view without a timeout certificate for its predecessor to be refused, got %v", err)
	}
}

func TestStateMachineEnforcesScheduledProposer(t *testing.T) {
	cluster := newEqualTestCluster(t, "carol", "alice", "bob")
	machine := cluster.newMachineWith(func(config *StateMachineConfig) { config.EnforceProposerSelection = true })
	machine.StartRound(1, 0)

	block := types.Block{Header: types.Header{Height: 1}}
	if _, err := machine.CreateProposal(block, 0, "bob", finality.QuorumCert{}); !errors.Is(err, ErrUnexpectedProposer) {
		t.Fatalf("expected non-scheduled proposer rejection, got %v", err)
	}
	proposal, err := machine.CreateProposal(block, 0, "alice", finality.QuorumCert{})
	if err != nil {
		t.Fatalf("expected scheduled proposer to create proposal: %v", err)
	}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnProposal(context.Background(), proposal); err != nil {
		t.Fatalf("expected scheduled proposal to be accepted: %v", err)
	}

	wrong := cluster.proposal("carol", cluster.block(1, types.Hash{}, "other"), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	if err := machine.OnProposal(context.Background(), wrong); !errors.Is(err, ErrUnexpectedProposer) {
		t.Fatalf("expected a validly signed proposal from the wrong proposer to be refused, got %v", err)
	}
}

func TestStateMachineRejectsUnsafeVoteBelowLockedQC(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 1)

	// Accepting B2 locks the machine on the certificate of B1 (view 0).
	second := cluster.block(2, blocks[0].hash)
	if err := machine.OnProposal(ctx, cluster.proposal("b", second, 1, blocks[0].qc, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}

	// A fork from genesis in a later view justifies nothing newer than the lock: it is
	// recorded but never earns a vote.
	fork := cluster.blockAt(1, types.Hash{}, 3, "fork")
	forkHash := HashBlock(fork)
	skipped := cluster.timeoutCert(1, 2, finality.QuorumCert{}, "a", "b")
	if err := machine.OnProposal(ctx, cluster.proposal("c", fork, 3, finality.QuorumCert{}, skipped)); !errors.Is(err, ErrUnsafeProposal) {
		t.Fatalf("expected unsafe proposal, got %v", err)
	}
	if err := machine.OnVote(ctx, cluster.vote("a", 1, 3, forkHash)); !errors.Is(err, ErrUnsafeVote) {
		t.Fatalf("expected unsafe vote, got %v", err)
	}
}

func TestStateMachineRejectsUnknownValidatorVote(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	vote := cluster.vote("a", 1, 0, types.Hash{1})
	vote.ValidatorID = "unknown"
	if err := machine.OnVote(context.Background(), vote); !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected unknown validator, got %v", err)
	}
}

func TestStateMachineRejectsUnknownVoteTarget(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	machine.StartRound(1, 0)

	err := machine.OnVote(context.Background(), cluster.vote("a", 1, 0, types.Hash{1}))
	if !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected invalid vote, got %v", err)
	}
	if !errors.Is(err, ErrUnknownVoteBlock) {
		t.Fatalf("expected unknown vote block, got %v", err)
	}
}

func TestStateMachineRejectsInvalidVoteFields(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	machine.StartRound(1, 1)

	wrongDomain := Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "a"}
	if err := signVoteForTest(cluster.signers["a"], &wrongDomain, vexocrypto.DomainConsensusProposal); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		vote     Vote
		expected error
	}{
		{name: "missing height", vote: cluster.vote("a", 0, 1, types.Hash{1}), expected: ErrInvalidVote},
		{name: "missing block hash", vote: cluster.vote("a", 1, 1, types.Hash{}), expected: ErrInvalidVote},
		{name: "unsigned", vote: Vote{Height: 1, Round: 1, BlockHash: types.Hash{1}, ValidatorID: "a"}, expected: ErrInvalidVote},
		{name: "wrong domain", vote: wrongDomain, expected: ErrInvalidVote},
		{name: "signed by another validator", vote: withValidator(cluster.vote("b", 1, 1, types.Hash{1}), "a"), expected: ErrInvalidVote},
		{name: "unknown block", vote: cluster.vote("a", 1, 1, types.Hash{1}), expected: ErrInvalidVote},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := machine.OnVote(context.Background(), testCase.vote)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
	if len(machine.votes) != 0 || len(machine.votedVotes) != 0 {
		t.Fatal("rejected votes must not allocate vote tables")
	}
}

// A block header names the view it is proposed in, so a vote or certificate for the block in any
// other view is invalid: one block can never be certified in two views.
func TestStateMachineBindsVotesAndCertificatesToTheViewOfTheBlock(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	block := cluster.block(1, types.Hash{})
	blockHash := HashBlock(block)
	if err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), cluster.vote("a", 1, 1, blockHash)); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected a vote in another view than the block's to be invalid, got %v", err)
	}
	if err := machine.VerifyQuorumCert(cluster.qc(1, 1, blockHash)); !errors.Is(err, ErrInvalidQuorumCert) {
		t.Fatalf("expected a certificate in another view than the block's to be invalid, got %v", err)
	}
	if err := machine.OnVote(context.Background(), cluster.vote("a", 1, 0, blockHash)); err != nil {
		t.Fatalf("expected the vote in the block's view to be accepted, got %v", err)
	}
}

func TestStateMachineRejectsStaleVote(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 3)

	// The blocks of heights 1 and 2 are final, and the machine is past view 2.
	if err := machine.OnVote(ctx, cluster.vote("c", 1, 5, blocks[0].hash)); !errors.Is(err, ErrStaleVote) {
		t.Fatalf("expected stale vote by finalized height, got %v", err)
	}
	if err := machine.OnVote(ctx, cluster.vote("c", 3, 1, blocks[2].hash)); !errors.Is(err, ErrStaleVote) {
		t.Fatalf("expected stale vote by view, got %v", err)
	}
}

func TestStateMachineRejectsVotesOutsideTheWindow(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	machine.StartRound(1, 0)

	farView := cluster.vote("a", 1, maxViewLookahead+1, types.Hash{1})
	if err := machine.OnVote(context.Background(), farView); !errors.Is(err, ErrOutOfWindow) {
		t.Fatalf("expected a far-future view to be refused, got %v", err)
	}
	farHeight := cluster.vote("a", maxHeightWindow+1, 0, types.Hash{1})
	if err := machine.OnVote(context.Background(), farHeight); !errors.Is(err, ErrOutOfWindow) {
		t.Fatalf("expected a far-future height to be refused, got %v", err)
	}
	if len(machine.votes) != 0 || len(machine.votedVotes) != 0 {
		t.Fatal("refused votes must not allocate vote tables")
	}
}

func TestStateMachineAllowsSameVoteReplay(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	block := cluster.block(1, types.Hash{})
	blockHash := HashBlock(block)
	if err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	vote := cluster.vote("a", 1, 0, blockHash)
	if err := machine.OnVote(context.Background(), vote); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(context.Background(), vote); err != nil {
		t.Fatalf("same vote replay should be idempotent, got %v", err)
	}
	if got := len(machine.votesForBlock(1, 0, blockHash)); got != 1 {
		t.Fatalf("a replayed vote must count once, got %d", got)
	}
}

func TestStateMachineRejectsConflictingVote(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	first := cluster.block(1, types.Hash{}, "first")
	second := cluster.block(1, types.Hash{}, "second")
	firstHash := HashBlock(first)
	secondHash := HashBlock(second)
	if err := machine.OnProposal(ctx, cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}

	if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, firstHash)); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, secondHash)); !errors.Is(err, ErrConflictingVote) {
		t.Fatalf("expected conflicting vote, got %v", err)
	}
	evidence := machine.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(evidence))
	}
	if err := VerifyConflictingVoteEvidence(evidence[0]); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineRequiresSignatureBeforeRecordingEvidence(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	block := cluster.block(1, types.Hash{}, "first")
	blockHash := HashBlock(block)
	if err := machine.OnProposal(ctx, cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, blockHash)); err != nil {
		t.Fatal(err)
	}

	forged := Vote{Height: 1, Round: 0, BlockHash: types.Hash{9}, ValidatorID: "a", Signature: types.Signature("forged")}
	if err := machine.OnVote(ctx, forged); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected a forged conflicting vote to be refused as invalid, got %v", err)
	}
	if len(machine.Evidence()) != 0 {
		t.Fatal("an unsigned conflicting vote must not become slashing evidence")
	}
}

func TestStateMachineRejectsUnknownProposalProposer(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	proposal := cluster.proposal("a", cluster.block(1, types.Hash{}), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	proposal.Proposer = "unknown"
	if err := machine.OnProposal(context.Background(), proposal); !errors.Is(err, ErrUnknownValidator) {
		t.Fatalf("expected unknown proposer, got %v", err)
	}
}

func TestStateMachineRejectsWrongChainProposal(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	block := cluster.block(1, types.Hash{})
	block.Header.ChainID = "wrong-chain"
	err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 0, finality.QuorumCert{}, finality.TimeoutCert{}))
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected chain id mismatch, got %v", err)
	}
}

func TestStateMachineAcceptsValidProposal(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	err := machine.OnProposal(context.Background(), cluster.proposal("a", cluster.block(1, types.Hash{}), 0, finality.QuorumCert{}, finality.TimeoutCert{}))
	if err != nil {
		t.Fatal(err)
	}
	if machine.Status(context.Background()).Phase != PhaseVote {
		t.Fatal("expected vote phase")
	}
}

func TestStateMachineRejectsInvalidProposalFields(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	noHeight := cluster.block(1, types.Hash{})
	noHeight.Header.Height = 0
	wrongSet := cluster.block(1, types.Hash{})
	wrongSet.Header.ValidatorSetHash = types.Hash{9}
	genesisWithQC := cluster.block(1, types.Hash{})

	unsigned := cluster.proposal("a", cluster.block(1, types.Hash{}), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	unsigned.Signature = nil
	wrongKey := Proposal{Block: cluster.block(1, types.Hash{}), Proposer: "a"}
	if err := signProposalForTest(cluster.signers["b"], &wrongKey); err != nil {
		t.Fatal(err)
	}
	tampered := cluster.proposal("a", cluster.block(1, types.Hash{}, "tx"), 0, finality.QuorumCert{}, finality.TimeoutCert{})
	tampered.Block.Txs = []types.Tx{[]byte("other")}

	cases := []struct {
		name     string
		proposal Proposal
		expected error
	}{
		{name: "missing height", proposal: cluster.proposal("a", noHeight, 0, finality.QuorumCert{}, finality.TimeoutCert{}), expected: ErrInvalidProposal},
		{name: "validator set mismatch", proposal: cluster.proposal("a", wrongSet, 0, finality.QuorumCert{}, finality.TimeoutCert{}), expected: ErrInvalidProposal},
		{name: "qc height exceeds proposal height", proposal: cluster.proposal("a", genesisWithQC, 0, finality.QuorumCert{Height: 2, Round: 0, BlockHash: types.Hash{1}}, finality.TimeoutCert{}), expected: ErrInvalidProposal},
		{name: "unsigned", proposal: unsigned, expected: ErrInvalidProposal},
		{name: "signed by another validator", proposal: wrongKey, expected: ErrInvalidProposal},
		{name: "block changed after signing", proposal: tampered, expected: ErrInvalidProposal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := machine.OnProposal(context.Background(), testCase.proposal)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
		})
	}
	if machine.blockTree.Len() != 0 {
		t.Fatal("rejected proposals must not enter the block tree")
	}
}

func TestStateMachineRejectsStaleProposal(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	blocks := cluster.extendChain(machine, 3)
	ctx := context.Background()

	// Heights 1 and 2 are final. A finalized block shown again is merely old.
	if err := machine.OnProposal(ctx, cluster.proposal("a", blocks[0].block, 0, finality.QuorumCert{}, finality.TimeoutCert{})); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected stale proposal by finalized height, got %v", err)
	}

	// A different block at the finalized height conflicts with the finalized chain.
	conflicting := cluster.block(2, blocks[0].hash, "conflicting")
	skipped := cluster.timeoutCert(2, 4, blocks[0].qc, "a", "b")
	err := machine.OnProposal(ctx, cluster.proposal("a", conflicting, 5, blocks[0].qc, skipped))
	if !errors.Is(err, ErrUnsafeProposal) {
		t.Fatalf("expected a block conflicting with a finalized one to be unsafe, got %v", err)
	}

	// Once the machine has left view 3, a proposal for it is stale.
	for _, voter := range []types.ValidatorID{"a", "b"} {
		_, _ = machine.OnTimeoutVote(ctx, cluster.timeoutVote(voter, 4, 5, blocks[2].qc))
	}
	next := cluster.block(4, blocks[2].hash)
	if err := machine.OnProposal(ctx, cluster.proposal("a", next, 3, blocks[2].qc, finality.TimeoutCert{})); !errors.Is(err, ErrStaleProposal) {
		t.Fatalf("expected stale proposal by view, got %v", err)
	}
}

func TestStateMachineRejectsProposalWithMismatchedJustifyQCParent(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()

	parent := cluster.block(1, types.Hash{})
	parentHash := HashBlock(parent)
	block := cluster.block(2, types.Hash{9})
	err := machine.OnProposal(context.Background(), cluster.proposal("a", block, 1, cluster.qc(1, 0, parentHash), finality.TimeoutCert{}))
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("expected invalid proposal, got %v", err)
	}
}

func TestStateMachineAdvancesRoundOnTimeoutQuorum(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(1, 0)

	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 1, 0, finality.QuorumCert{})); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("b", 1, 0, finality.QuorumCert{}))
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.Round != 0 {
		t.Fatalf("expected timeout cert round 0, got %d", timeoutCert.Round)
	}
	if err := machine.VerifyTimeoutCert(timeoutCert); err != nil {
		t.Fatalf("a timeout cert built by the machine must verify: %v", err)
	}
	status := machine.Status(ctx)
	if status.Height != 1 || status.Round != 1 || status.Phase != PhasePropose || status.LastTimeoutCert.Round != 0 {
		t.Fatalf("expected propose phase in view 1, got %+v", status)
	}
}

func TestStateMachineJumpsToFutureViewOnTimeoutCertificate(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(1, 0)

	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 1, 3, finality.QuorumCert{})); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum for first future round timeout vote, got %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("b", 1, 3, finality.QuorumCert{}))
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.Round != 3 {
		t.Fatalf("expected timeout cert round 3, got %d", timeoutCert.Round)
	}
	status := machine.Status(ctx)
	if status.Height != 1 || status.Round != 4 || status.Phase != PhasePropose {
		t.Fatalf("expected jump to view 4 propose, got %+v", status)
	}
}

func TestStateMachineTimeoutVotesNeverRaiseTheHeight(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(1, 0)

	// The votes name height 2, but no certified block supports it.
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 2, 1, finality.QuorumCert{})); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum for first future height timeout vote, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("b", 2, 1, finality.QuorumCert{})); err != nil {
		t.Fatal(err)
	}
	status := machine.Status(ctx)
	if status.Height != 1 || status.Round != 2 || status.Phase != PhasePropose {
		t.Fatalf("a timeout certificate must advance the view but never the height, got %+v", status)
	}
}

func TestStateMachineObservesTimeoutCertHighQCForNextProposal(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(2, 0)

	// The machine holds both certified blocks; a proposer can only extend a block it holds.
	high := cluster.blockAt(1, types.Hash{}, 2, "high")
	lower := cluster.blockAt(1, types.Hash{}, 1, "lower")
	machine.blockTree.Insert(high, HashBlock(high), finality.QuorumCert{})
	machine.blockTree.Insert(lower, HashBlock(lower), finality.QuorumCert{})
	highQC := cluster.qc(1, 2, HashBlock(high))
	lowerQC := cluster.qc(1, 1, HashBlock(lower))
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 2, 3, lowerQC)); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("b", 2, 3, highQC))
	if err != nil {
		t.Fatal(err)
	}
	if timeoutCert.HighQC.BlockHash != highQC.BlockHash {
		t.Fatalf("expected timeout cert high qc, got %+v", timeoutCert.HighQC)
	}

	proposal, err := machine.CreateProposal(types.Block{}, 4, "a", finality.QuorumCert{})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.JustifyQC.BlockHash != highQC.BlockHash {
		t.Fatalf("expected timeout high qc to feed next proposal, got %+v", proposal.JustifyQC)
	}
}

func TestStateMachineRecordsConflictingTimeoutVoteEvidence(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(2, 1)

	first := cluster.timeoutVote("a", 2, 1, cluster.qc(1, 0, types.Hash{1}))
	second := cluster.timeoutVote("a", 2, 1, finality.QuorumCert{})
	if _, err := machine.OnTimeoutVote(ctx, first); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(ctx, second); !errors.Is(err, ErrConflictingTimeoutVote) {
		t.Fatalf("expected conflicting timeout vote, got %v", err)
	}

	evidence := machine.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one evidence, got %d", len(evidence))
	}
	if err := VerifyConflictingTimeoutVoteEvidence(evidence[0]); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineRecordsFutureViewConflictingTimeoutEvidence(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(2, 0)

	first := cluster.timeoutVote("a", 2, 5, cluster.qc(1, 0, types.Hash{1}))
	second := cluster.timeoutVote("a", 2, 5, cluster.qc(1, 1, types.Hash{2}))
	if _, err := machine.OnTimeoutVote(ctx, first); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(ctx, second); !errors.Is(err, ErrConflictingTimeoutVote) {
		t.Fatalf("expected conflicting timeout vote, got %v", err)
	}
	evidence := machine.Evidence()
	if len(evidence) != 1 {
		t.Fatalf("expected one future timeout evidence, got %d", len(evidence))
	}
	if err := VerifyConflictingTimeoutVoteEvidence(evidence[0]); err != nil {
		t.Fatal(err)
	}
}

func TestStateMachineRejectsStaleTimeoutVote(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	machine.StartRound(2, 4)

	if _, err := machine.OnTimeoutVote(context.Background(), cluster.timeoutVote("a", 2, 3, finality.QuorumCert{})); !errors.Is(err, ErrStaleTimeoutCert) {
		t.Fatalf("expected stale timeout vote, got %v", err)
	}
	if len(machine.timeouts.votes) != 0 {
		t.Fatal("a stale timeout vote must not allocate collector state")
	}
}

func TestStateMachineRejectsTimeoutVotesOutsideTheWindow(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	machine.StartRound(1, 0)

	if _, err := machine.OnTimeoutVote(context.Background(), cluster.timeoutVote("a", 1, maxViewLookahead+1, finality.QuorumCert{})); !errors.Is(err, ErrOutOfWindow) {
		t.Fatalf("expected far-future timeout view to be refused, got %v", err)
	}
	if _, err := machine.OnTimeoutVote(context.Background(), cluster.timeoutVote("a", maxHeightWindow+2, 1, finality.QuorumCert{})); !errors.Is(err, ErrOutOfWindow) {
		t.Fatalf("expected far-future timeout height to be refused, got %v", err)
	}
	if len(machine.timeouts.votes) != 0 {
		t.Fatal("refused timeout votes must not allocate collector state")
	}
}

func TestStateMachineRejectsUnsignedTimeoutVote(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	machine.StartRound(1, 0)

	unsigned := TimeoutVote{Height: 1, Round: 0, ValidatorID: "a"}
	if _, err := machine.OnTimeoutVote(context.Background(), unsigned); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected an unsigned timeout vote to be refused, got %v", err)
	}
}

func TestStateMachineStatusIsMonotonic(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()

	machine.StartRound(5, 7)
	machine.StartRound(2, 0)
	status := machine.Status(ctx)
	if status.Height != 5 || status.Round != 7 {
		t.Fatalf("StartRound must never lower the height or view, got %+v", status)
	}
}

// An equivocator that retransmits its conflicting vote, or a peer that relays it, commits one
// offence: the machine records it once.
func TestStateMachineRecordsAnEquivocationOnce(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	first := cluster.block(1, types.Hash{}, "first")
	second := cluster.block(1, types.Hash{}, "second")
	if err := machine.OnProposal(ctx, cluster.proposal("a", first, 0, finality.QuorumCert{}, finality.TimeoutCert{})); err != nil {
		t.Fatal(err)
	}
	if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, HashBlock(first))); err != nil {
		t.Fatal(err)
	}
	machine.StartRound(2, 1)
	timeoutFirst := cluster.timeoutVote("a", 2, 1, cluster.qc(1, 0, types.Hash{1}))
	timeoutSecond := cluster.timeoutVote("a", 2, 1, finality.QuorumCert{})
	if _, err := machine.OnTimeoutVote(ctx, timeoutFirst); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum, got %v", err)
	}
	for range 50 {
		if err := machine.OnVote(ctx, cluster.vote("a", 1, 0, HashBlock(second))); !errors.Is(err, ErrConflictingVote) {
			t.Fatalf("expected conflicting vote, got %v", err)
		}
		if _, err := machine.OnTimeoutVote(ctx, timeoutSecond); !errors.Is(err, ErrConflictingTimeoutVote) {
			t.Fatalf("expected conflicting timeout vote, got %v", err)
		}
	}
	evidence, next := machine.EvidenceSince(0)
	if len(evidence) != 2 || next != 2 {
		t.Fatalf("expected the vote and the timeout equivocation once each, got %d (next %d)", len(evidence), next)
	}
}

// The machine keeps a bounded window of evidence: a reader that falls behind it resumes at the
// oldest evidence kept.
func TestStateMachineKeepsABoundedEvidenceWindow(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	total := maxRetainedEvidence + 10
	for view := 1; view <= total; view++ {
		machine.StartRound(2, types.Round(view))
		first := cluster.timeoutVote("a", 2, types.Round(view), cluster.qc(1, 0, types.Hash{1}))
		second := cluster.timeoutVote("a", 2, types.Round(view), finality.QuorumCert{})
		_, _ = machine.OnTimeoutVote(ctx, first)
		if _, err := machine.OnTimeoutVote(ctx, second); !errors.Is(err, ErrConflictingTimeoutVote) {
			t.Fatalf("view %d: expected conflicting timeout vote, got %v", view, err)
		}
	}
	if kept := len(machine.Evidence()); kept != maxRetainedEvidence {
		t.Fatalf("expected %d evidence kept, got %d", maxRetainedEvidence, kept)
	}
	evidence, next := machine.EvidenceSince(0)
	if len(evidence) != maxRetainedEvidence || next != uint64(total) {
		t.Fatalf("expected the window from a stale cursor, got %d (next %d)", len(evidence), next)
	}
	if evidence[0].Round != 11 {
		t.Fatalf("expected the oldest kept evidence of view 11, got %d", evidence[0].Round)
	}
	if fresh, again := machine.EvidenceSince(next); len(fresh) != 0 || again != next {
		t.Fatalf("expected nothing new, got %d (next %d)", len(fresh), again)
	}
}
