package consensus

import (
	"context"
	"crypto/sha256"
	"sort"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const testChainID = "vexo-test"

// testCluster is a validator set whose members all hold deterministic signing keys. The
// helpers build correctly signed proposals, votes, timeout votes and certificates, so a
// test can start from valid traffic and break exactly one thing.
type testCluster struct {
	t       *testing.T
	set     validator.Set
	signers SimulationSigners
	ids     []types.ValidatorID
	// headers holds the header of every block the cluster sealed, so a child derives the
	// validator set height its parent fixes.
	headers map[types.Hash]types.Header
}

// testBlock is a block together with the view it was proposed in and the quorum certificate
// that certifies it.
type testBlock struct {
	block types.Block
	hash  types.Hash
	round types.Round
	qc    finality.QuorumCert
}

func newTestCluster(t *testing.T, validators ...validator.Validator) *testCluster {
	t.Helper()
	keyed, signers, err := NewSimulationValidators(validators)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]types.ValidatorID, 0, len(keyed))
	for _, validatorInfo := range keyed {
		ids = append(ids, validatorInfo.ID)
	}
	sort.Slice(ids, func(first, second int) bool { return ids[first] < ids[second] })
	return &testCluster{t: t, set: newTestValidatorSet(keyed), signers: signers, ids: ids, headers: make(map[types.Hash]types.Header)}
}

// newEqualTestCluster builds a cluster in which every listed validator has voting power 1.
func newEqualTestCluster(t *testing.T, ids ...types.ValidatorID) *testCluster {
	t.Helper()
	validators := make([]validator.Validator, 0, len(ids))
	for _, id := range ids {
		validators = append(validators, validator.Validator{ID: id, VotingPower: 1})
	}
	return newTestCluster(t, validators...)
}

func newThreeValidatorCluster(t *testing.T) *testCluster {
	t.Helper()
	return newEqualTestCluster(t, "a", "b", "c")
}

func (cluster *testCluster) newMachine() *StateMachine {
	cluster.t.Helper()
	return cluster.newMachineWith(func(*StateMachineConfig) {})
}

func (cluster *testCluster) newMachineWith(mutate func(*StateMachineConfig)) *StateMachine {
	cluster.t.Helper()
	config := StateMachineConfig{
		ChainID:           testChainID,
		ValidatorSet:      cluster.set,
		Signatures:        vexocrypto.DeterministicSigner{},
		Aggregator:        vexocrypto.DeterministicAggregateSigner{},
		AggregateVerifier: vexocrypto.DeterministicAggregateSigner{},
	}
	mutate(&config)
	machine, err := NewStateMachine(config)
	if err != nil {
		cluster.t.Fatal(err)
	}
	return machine
}

// blockAt builds a well-formed block sealed for a proposal in round.
func (cluster *testCluster) blockAt(height types.Height, parent types.Hash, round types.Round, txs ...string) types.Block {
	cluster.t.Helper()
	block := cluster.block(height, parent, txs...)
	return cluster.seal(block, round, block.Header.JustifyRound)
}

// block builds a well-formed block: ordered transactions, the transaction root, and the
// data availability commitment.
func (cluster *testCluster) block(height types.Height, parent types.Hash, txs ...string) types.Block {
	block := types.Block{Header: types.Header{
		ChainID:           testChainID,
		Height:            height,
		PreviousBlockHash: parent,
		ValidatorSetHash:  cluster.set.Hash(),
		// Block time only has to increase along a chain.
		TimeUnixNano: int64(height),
	}}
	for _, tx := range txs {
		block.Txs = append(block.Txs, types.Tx(tx))
	}
	block.Txs = fairordering.SortTxsWithSalt(block.Txs, fairordering.HeightSalt(testChainID, height))
	block.Header.TxRoot = TxRoot(block.Txs)
	if len(block.Txs) > 0 {
		block = dataavailability.AttachCommitment(block)
	}
	// The block is sealed for the view directly after its parent's, the common case; a proposal
	// in another view reseals it.
	var round, justifyRound types.Round
	if parentHeader, found := cluster.headers[parent]; found && height > 1 {
		justifyRound = parentHeader.Round
		round = justifyRound + 1
	}
	return cluster.seal(block, round, justifyRound)
}

// seal fills in the header fields a proposal in round fixes: the view, the view of the justify
// certificate, and the validator set height the parent header fixes. The parent must be a block
// the cluster sealed; the first block names the genesis set.
func (cluster *testCluster) seal(block types.Block, round types.Round, justifyRound types.Round) types.Block {
	cluster.t.Helper()
	block.Header.Round = round
	block.Header.JustifyRound = justifyRound
	block.Header.ValidatorSetHeight = finality.FirstValidatorSetHeight
	block.Header.StateHeight = 0
	if parent, found := cluster.headers[block.Header.PreviousBlockHash]; found && block.Header.Height > 1 {
		block.Header.ValidatorSetHeight = finality.ChildValidatorSetHeight(parent)
		block.Header.StateHeight = finality.ChildStateHeight(parent)
	}
	if block.Header.NextValidatorSetHash == (types.Hash{}) {
		block.Header.NextValidatorSetHash = cluster.set.Hash()
	}
	cluster.headers[HashBlock(block)] = block.Header
	return block
}

// proposal returns a proposal signed by the proposer. The block is resealed for the proposal's
// views, which changes its hash unless it was built for them (see blockAt).
func (cluster *testCluster) proposal(proposer types.ValidatorID, block types.Block, round types.Round, justifyQC finality.QuorumCert, timeoutCert finality.TimeoutCert) Proposal {
	cluster.t.Helper()
	block = cluster.seal(block, round, justifyQC.Round)
	proposal := Proposal{Block: block, Round: round, Proposer: proposer, JustifyQC: justifyQC, TimeoutCert: timeoutCert}
	if err := cluster.signers.signProposal(&proposal); err != nil {
		cluster.t.Fatal(err)
	}
	return proposal
}

func (cluster *testCluster) vote(voter types.ValidatorID, height types.Height, round types.Round, blockHash types.Hash) Vote {
	cluster.t.Helper()
	vote := Vote{Height: height, Round: round, BlockHash: blockHash, ValidatorID: voter}
	if err := cluster.signers.signVote(&vote); err != nil {
		cluster.t.Fatal(err)
	}
	return vote
}

func (cluster *testCluster) timeoutVote(voter types.ValidatorID, height types.Height, round types.Round, highQC finality.QuorumCert) TimeoutVote {
	cluster.t.Helper()
	vote := TimeoutVote{Height: height, Round: round, ValidatorID: voter, HighQC: highQC}
	if err := cluster.signers.signTimeoutVote(&vote); err != nil {
		cluster.t.Fatal(err)
	}
	return vote
}

// qc aggregates the voters' signatures into a genuine quorum certificate. Without voters
// every validator signs. The certificate is valid whether or not it reaches quorum; use
// few voters to build a sub-quorum certificate.
func (cluster *testCluster) qc(height types.Height, round types.Round, blockHash types.Hash, voters ...types.ValidatorID) finality.QuorumCert {
	cluster.t.Helper()
	if len(voters) == 0 {
		voters = cluster.ids
	}
	sorted := append([]types.ValidatorID(nil), voters...)
	sort.Slice(sorted, func(first, second int) bool { return sorted[first] < sorted[second] })
	signatures := make([]types.Signature, 0, len(sorted))
	var power types.VotingPower
	for _, voter := range sorted {
		signatures = append(signatures, cluster.vote(voter, height, round, blockHash).Signature)
		validatorInfo, found := cluster.set.Get(voter)
		if !found {
			cluster.t.Fatalf("unknown validator %s", voter)
		}
		var err error
		power, err = types.AddVotingPower(power, validatorInfo.VotingPower)
		if err != nil {
			cluster.t.Fatal(err)
		}
	}
	aggregate, err := vexocrypto.DeterministicAggregateSigner{}.Aggregate(signatures)
	if err != nil {
		cluster.t.Fatal(err)
	}
	return finality.QuorumCert{
		Height:      height,
		Round:       round,
		BlockHash:   blockHash,
		Signers:     finality.EncodeSigners(sorted),
		Signature:   aggregate,
		VotingPower: power,
	}
}

// timeoutCert builds a timeout certificate from the timeout votes of the given voters, all
// carrying the same high QC.
func (cluster *testCluster) timeoutCert(height types.Height, round types.Round, highQC finality.QuorumCert, voters ...types.ValidatorID) finality.TimeoutCert {
	cluster.t.Helper()
	votes := make([]TimeoutVote, 0, len(voters))
	for _, voter := range voters {
		votes = append(votes, cluster.timeoutVote(voter, height, round, highQC))
	}
	return cluster.timeoutCertFromVotes(round, votes...)
}

func (cluster *testCluster) timeoutCertFromVotes(round types.Round, votes ...TimeoutVote) finality.TimeoutCert {
	cluster.t.Helper()
	collector := NewTimeoutCollector(cluster.set, vexocrypto.DeterministicAggregateSigner{})
	for _, vote := range votes {
		if err := collector.AddVote(vote); err != nil {
			cluster.t.Fatal(err)
		}
	}
	timeoutCert, err := collector.BuildTimeoutCert(round)
	if err != nil {
		cluster.t.Fatal(err)
	}
	return timeoutCert
}

// chain builds n consecutive blocks in consecutive views starting at startRound, each
// certified by every validator, without involving any state machine.
func (cluster *testCluster) chain(n int, startRound types.Round) []testBlock {
	cluster.t.Helper()
	blocks := make([]testBlock, 0, n)
	var parent types.Hash
	for index := 0; index < n; index++ {
		height := types.Height(index + 1)
		round := startRound + types.Round(index)
		var justifyRound types.Round
		if index > 0 {
			justifyRound = blocks[index-1].round
		}
		block := cluster.seal(cluster.block(height, parent), round, justifyRound)
		hash := HashBlock(block)
		blocks = append(blocks, testBlock{block: block, hash: hash, round: round, qc: cluster.qc(height, round, hash)})
		parent = hash
	}
	return blocks
}

// feed delivers the chain to the machine as signed proposals, each justified by the
// certificate of its parent, and fails the test on any error.
func (cluster *testCluster) feed(machine *StateMachine, blocks []testBlock) {
	cluster.t.Helper()
	for index, entry := range blocks {
		var justify finality.QuorumCert
		if index > 0 {
			justify = blocks[index-1].qc
		}
		proposal := cluster.proposal(cluster.ids[0], entry.block, entry.round, justify, finality.TimeoutCert{})
		if err := machine.OnProposal(context.Background(), proposal); err != nil {
			cluster.t.Fatalf("proposal for height %d: %v", entry.block.Header.Height, err)
		}
	}
}

// extend proposes a child of parent (a first block when parent is nil) in the given view
// through the machine and certifies it with real votes. The proposal carries timeoutCert
// when the view does not directly follow the parent's view.
func (cluster *testCluster) extend(machine *StateMachine, parent *testBlock, round types.Round, timeoutCert finality.TimeoutCert, txs ...string) testBlock {
	cluster.t.Helper()
	height := types.Height(1)
	var parentHash types.Hash
	var justify finality.QuorumCert
	if parent != nil {
		height = parent.block.Header.Height + 1
		parentHash = parent.hash
		justify = parent.qc
	}
	block := cluster.seal(cluster.block(height, parentHash, txs...), round, justify.Round)
	hash := HashBlock(block)
	proposer := cluster.ids[int(round)%len(cluster.ids)]
	if err := machine.OnProposal(context.Background(), cluster.proposal(proposer, block, round, justify, timeoutCert)); err != nil {
		cluster.t.Fatalf("proposal for height %d view %d: %v", height, round, err)
	}
	for _, voter := range cluster.ids {
		if err := machine.OnVote(context.Background(), cluster.vote(voter, height, round, hash)); err != nil {
			cluster.t.Fatalf("vote of %s for height %d view %d: %v", voter, height, round, err)
		}
		if qc, err := machine.BuildQuorumCert(height, round, hash); err == nil {
			return testBlock{block: block, hash: hash, round: round, qc: qc}
		}
	}
	cluster.t.Fatalf("no quorum for height %d view %d", height, round)
	return testBlock{}
}

// extendChain extends the machine by n certified blocks in consecutive views.
func (cluster *testCluster) extendChain(machine *StateMachine, n int) []testBlock {
	cluster.t.Helper()
	blocks := make([]testBlock, 0, n)
	var parent *testBlock
	for index := 0; index < n; index++ {
		next := cluster.extend(machine, parent, types.Round(index), finality.TimeoutCert{})
		blocks = append(blocks, next)
		parent = &blocks[len(blocks)-1]
	}
	return blocks
}

// subset returns a cluster over the listed members that signs with the same keys.
func (cluster *testCluster) subset(ids ...types.ValidatorID) *testCluster {
	cluster.t.Helper()
	sorted := append([]types.ValidatorID(nil), ids...)
	sort.Slice(sorted, func(first, second int) bool { return sorted[first] < sorted[second] })
	return &testCluster{t: cluster.t, set: validator.NewSet(cluster.validators(ids...)), signers: cluster.signers, ids: sorted, headers: cluster.headers}
}

// validators returns the cluster's keyed validators for the given ids.
func (cluster *testCluster) validators(ids ...types.ValidatorID) []validator.Validator {
	cluster.t.Helper()
	validators := make([]validator.Validator, 0, len(ids))
	for _, id := range ids {
		validatorInfo, found := cluster.set.Get(id)
		if !found {
			cluster.t.Fatalf("unknown validator %s", id)
		}
		validators = append(validators, validatorInfo)
	}
	return validators
}

func withValidator(vote Vote, validatorID types.ValidatorID) Vote {
	vote.ValidatorID = validatorID
	return vote
}

// garbageQC returns a certificate that names a quorum of real validators but carries a
// signature none of them produced.
func (cluster *testCluster) garbageQC(height types.Height, round types.Round, blockHash types.Hash) finality.QuorumCert {
	qc := cluster.qc(height, round, blockHash)
	qc.Signature = types.AggregateSignature("garbage-signature")
	return qc
}

type testValidatorSet struct {
	validators []validator.Validator
	byID       map[types.ValidatorID]validator.Validator
	totalPower types.VotingPower
}

func newTestValidatorSet(validators []validator.Validator) testValidatorSet {
	byID := make(map[types.ValidatorID]validator.Validator, len(validators))
	var totalPower types.VotingPower
	for _, validatorInfo := range validators {
		byID[validatorInfo.ID] = validatorInfo
		totalPower = types.MustAddVotingPowerSaturating(totalPower, validatorInfo.VotingPower)
	}
	return testValidatorSet{
		validators: validators,
		byID:       byID,
		totalPower: totalPower,
	}
}

func (set testValidatorSet) Hash() types.Hash {
	return types.Hash{1}
}

func (set testValidatorSet) TotalVotingPower() types.VotingPower {
	return set.totalPower
}

func (set testValidatorSet) Get(id types.ValidatorID) (validator.Validator, bool) {
	validatorInfo, found := set.byID[id]
	return validatorInfo, found
}

func (set testValidatorSet) List() []validator.Validator {
	return set.validators
}

// testAggregateSigner hashes the signatures together without checking them against any
// key. Only collector tests that never verify the result use it.
type testAggregateSigner struct{}

func (testAggregateSigner) Aggregate(signatures []types.Signature) (types.AggregateSignature, error) {
	if !allSignaturesPresent(signatures) {
		return nil, ErrMissingSignature
	}
	hasher := sha256.New()
	for _, signature := range signatures {
		hasher.Write(signature)
	}
	return types.AggregateSignature(hasher.Sum(nil)), nil
}
