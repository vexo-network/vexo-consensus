package slashing

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	vexoslashing "github.com/vexo-network/vexo-consensus/slashing"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	testChainID  = "vexo-slashing-test"
	genesisPower = 100
)

// testPenalties makes the arithmetic of a slash obvious: equivocating in a vote costs a quarter of
// the stake and a ten block jail, a faulty timeout vote everything, the two proposal faults a tenth
// and a fifth, and a finality conflict half of the stake and a twenty block jail.
func testPenalties() vexoslashing.PenaltyPolicy {
	return vexoslashing.PenaltyPolicy{
		vexoslashing.EvidenceConflictingVote:        {SlashBPS: 2500, JailDuration: 10},
		vexoslashing.EvidenceConflictingTimeoutVote: {SlashBPS: vexoslashing.BasisPoints},
		vexoslashing.EvidenceInvalidProposal:        {SlashBPS: 1000},
		vexoslashing.EvidenceUnavailableData:        {SlashBPS: 2000},
		vexoslashing.EvidenceFinalityConflict:       {SlashBPS: 5000, JailDuration: 20},
	}
}

type fixtureSettings struct {
	policy    Policy
	setHeight types.Height
	unstaked  []types.ValidatorID
}

type fixtureOption func(*fixtureSettings)

func withPolicy(policy Policy) fixtureOption {
	return func(settings *fixtureSettings) { settings.policy = policy }
}

// withSetFrom persists the first validator-set snapshot at height instead of 1.
func withSetFrom(height types.Height) fixtureOption {
	return func(settings *fixtureSettings) { settings.setHeight = height }
}

// withUnstaked adds validators to the validator set that the staking module does not know.
func withUnstaked(ids ...types.ValidatorID) fixtureOption {
	return func(settings *fixtureSettings) { settings.unstaked = ids }
}

// fixture is a chain of the validators a, b, c and d with equal power, running the staking and the
// slashing module in a runtime over a real store. The runtime has no ante handler unless a test
// installs one.
type fixture struct {
	t        *testing.T
	module   *Module
	runtime  *vexoapp.Runtime
	store    *vexostore.LevelDBStore
	registry *validator.StoreRegistry
	signers  consensus.SimulationSigners
	keys     map[types.ValidatorID]types.PublicKey
}

func deterministicSuite(t *testing.T) vexocrypto.RuntimeSuite {
	t.Helper()
	suite, err := vexocrypto.NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendDeterministic})
	if err != nil {
		t.Fatal(err)
	}
	return suite
}

func newFixture(t *testing.T, options ...fixtureOption) *fixture {
	t.Helper()
	settings := fixtureSettings{policy: Policy{Penalties: testPenalties()}, setHeight: 1}
	for _, option := range options {
		option(&settings)
	}

	storage, err := vexostore.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	ids := append([]types.ValidatorID{"a", "b", "c", "d"}, settings.unstaked...)
	members := make([]validator.Validator, 0, len(ids))
	for _, id := range ids {
		members = append(members, validator.Validator{ID: id, Address: types.Address(id), VotingPower: genesisPower, Stake: genesisPower})
	}
	keyed, signers, err := consensus.NewSimulationValidators(members)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := validator.NewStoreRegistry(context.Background(), storage, nil, settings.setHeight, keyed)
	if err != nil {
		t.Fatal(err)
	}

	module, err := NewModuleWithPolicy(settings.policy, deterministicSuite(t))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := vexoapp.NewRuntime(testChainID, []vexoapp.Module{staking.NewModule(), module}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithStore(storage)

	unstaked := make(map[types.ValidatorID]bool, len(settings.unstaked))
	for _, id := range settings.unstaked {
		unstaked[id] = true
	}
	keys := make(map[types.ValidatorID]types.PublicKey, len(keyed))
	genesis := make([]types.ValidatorUpdate, 0, len(keyed))
	for _, member := range keyed {
		keys[member.ID] = member.PublicKey
		if unstaked[member.ID] {
			continue
		}
		genesis = append(genesis, types.ValidatorUpdate{ID: member.ID, Address: member.Address, PublicKey: member.PublicKey, VotingPower: member.VotingPower, Stake: member.Stake})
	}
	if _, err := runtime.InitChain(vexoapp.InitChainRequest{ChainID: testChainID, Validators: genesis}); err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, module: module, runtime: runtime, store: storage, registry: registry, signers: signers, keys: keys}
}

// forTest returns the fixture for use inside a subtest, which must not call the fatal helpers of the
// test that built the fixture. State is shared.
func (f *fixture) forTest(t *testing.T) *fixture {
	clone := *f
	clone.t = t
	return &clone
}

func blockOf(height types.Height, txs ...types.Tx) types.Block {
	return types.Block{Header: types.Header{ChainID: testChainID, Height: height}, Txs: txs}
}

// finalize executes a block of txs at height and returns the response; every tx has a result.
func (f *fixture) finalize(height types.Height, txs ...types.Tx) vexoapp.FinalizeBlockResponse {
	f.t.Helper()
	response, err := f.runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(height, txs...)})
	if err != nil {
		f.t.Fatalf("finalize block %d: %v", height, err)
	}
	if len(response.Results) != len(txs) {
		f.t.Fatalf("block %d: expected %d results, got %d", height, len(txs), len(response.Results))
	}
	return response
}

func (f *fixture) sign(signer vexocrypto.Signer, domain vexocrypto.Domain, message []byte) types.Signature {
	f.t.Helper()
	signature, err := vexocrypto.SignWithDomain(signer, domain, message)
	if err != nil {
		f.t.Fatal(err)
	}
	return signature
}

// need returns evidence a constructor built, or fails the test.
func (f *fixture) need(evidence vexoslashing.Evidence, err error) vexoslashing.Evidence {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	return evidence
}

// handmade assembles evidence around any proof without the checks of the constructors, as an
// attacker would.
func (f *fixture) handmade(evidenceType vexoslashing.EvidenceType, validatorID types.ValidatorID, height types.Height, round types.Round, proof any) vexoslashing.Evidence {
	f.t.Helper()
	encoded, err := json.Marshal(proof)
	if err != nil {
		f.t.Fatal(err)
	}
	return vexoslashing.Evidence{Type: evidenceType, Validator: validatorID, Height: height, Round: round, Proof: encoded}
}

// voteBy is a vote in the name of voter that signer signed, whoever signer is.
func (f *fixture) voteBy(signer vexocrypto.Signer, voter types.ValidatorID, height types.Height, round types.Round, block types.Hash) consensus.Vote {
	f.t.Helper()
	vote := consensus.Vote{Height: height, Round: round, BlockHash: block, ValidatorID: voter}
	vote.Signature = f.sign(signer, vexocrypto.DomainConsensusVote, consensus.VoteSignBytes(vote))
	return vote
}

func (f *fixture) vote(voter types.ValidatorID, height types.Height, round types.Round, block types.Hash) consensus.Vote {
	f.t.Helper()
	return f.voteBy(f.signers[voter], voter, height, round, block)
}

// equivocationBy is evidence that voter signed two blocks in one view, with votes signer signed.
func (f *fixture) equivocationBy(signer vexocrypto.Signer, voter types.ValidatorID, height types.Height, round types.Round) vexoslashing.Evidence {
	f.t.Helper()
	return f.need(consensus.NewConflictingVoteEvidence(
		f.voteBy(signer, voter, height, round, types.Hash{1}),
		f.voteBy(signer, voter, height, round, types.Hash{2}),
	))
}

func (f *fixture) equivocation(voter types.ValidatorID, height types.Height, round types.Round) vexoslashing.Evidence {
	f.t.Helper()
	return f.equivocationBy(f.signers[voter], voter, height, round)
}

func (f *fixture) timeoutVote(voter types.ValidatorID, height types.Height, round types.Round, highQC finality.QuorumCert) consensus.TimeoutVote {
	f.t.Helper()
	vote := consensus.TimeoutVote{Height: height, Round: round, ValidatorID: voter, HighQC: highQC}
	vote.Signature = f.sign(f.signers[voter], vexocrypto.DomainConsensusTimeoutVote, consensus.TimeoutVoteSignBytes(vote))
	return vote
}

// timeoutEquivocation is evidence that voter gave up on one view with two different high QCs.
func (f *fixture) timeoutEquivocation(voter types.ValidatorID, height types.Height, round types.Round) vexoslashing.Evidence {
	f.t.Helper()
	return f.need(consensus.NewConflictingTimeoutVoteEvidence(
		f.timeoutVote(voter, height, round, finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{1}}),
		f.timeoutVote(voter, height, round, finality.QuorumCert{Height: 1, Round: 0, BlockHash: types.Hash{2}}),
	))
}

// qc is a quorum certificate of the voters for the block in a view, signed with their real keys.
func (f *fixture) qc(height types.Height, round types.Round, blockHash types.Hash, voters ...types.ValidatorID) finality.QuorumCert {
	f.t.Helper()
	sorted := append([]types.ValidatorID(nil), voters...)
	sort.Slice(sorted, func(first, second int) bool { return sorted[first] < sorted[second] })
	signatures := make([]types.Signature, 0, len(sorted))
	var power types.VotingPower
	for _, voter := range sorted {
		signatures = append(signatures, f.vote(voter, height, round, blockHash).Signature)
		power += genesisPower
	}
	aggregate, err := vexocrypto.DeterministicAggregateSigner{}.Aggregate(signatures)
	if err != nil {
		f.t.Fatal(err)
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

// finalityProof is a complete 2-chain proof that the voters finalized the block of the height with
// the application hash: its QC formed in view round and the QC of its child in view round+1.
func (f *fixture) finalityProof(height types.Height, appHash types.Hash, round types.Round, voters ...types.ValidatorID) finality.Proof {
	f.t.Helper()
	setHash := f.validatorSetAt(1).Hash()
	header := types.Header{ChainID: testChainID, Height: height, Round: round, AppHash: appHash, ValidatorSetHeight: finality.FirstValidatorSetHeight, ValidatorSetHash: setHash, NextValidatorSetHash: setHash}
	proof := finality.NewProof(header, f.qc(height, round, finality.HeaderHash(header), voters...))
	child := types.Header{ChainID: testChainID, Height: height + 1, Round: round + 1, JustifyRound: round, PreviousBlockHash: proof.BlockHash, ValidatorSetHeight: finality.ChildValidatorSetHeight(header), StateHeight: finality.ChildStateHeight(header), ValidatorSetHash: setHash, NextValidatorSetHash: setHash}
	return proof.WithChild(child, f.qc(child.Height, round+1, finality.HeaderHash(child), voters...))
}

// proposal is the proposal of block that proposer signed.
func (f *fixture) proposal(proposer types.ValidatorID, block types.Block) consensus.Proposal {
	f.t.Helper()
	proposal := consensus.Proposal{Block: block, Proposer: proposer}
	proposal.Signature = f.sign(f.signers[proposer], vexocrypto.DomainConsensusProposal, consensus.ProposalSignBytes(proposal))
	return proposal
}

// uncommittedBlock carries a transaction but no data availability commitment, which anyone can tell
// from the block alone.
func uncommittedBlock(height types.Height) types.Block {
	return types.Block{Header: types.Header{ChainID: testChainID, Height: height}, Txs: []types.Tx{[]byte("tx")}}
}

// missingData is invalid_proposal evidence against a proposer of the uncommitted block of height.
func (f *fixture) missingData(proposer types.ValidatorID, height types.Height) vexoslashing.Evidence {
	f.t.Helper()
	return f.need(consensus.NewDataAvailabilityInvalidProposalEvidence(f.proposal(proposer, uncommittedBlock(height)), consensus.InvalidProposalReasonMissingData))
}

// unavailableData is unavailable_data evidence against the proposer of the same block.
func (f *fixture) unavailableData(proposer types.ValidatorID, height types.Height) vexoslashing.Evidence {
	f.t.Helper()
	return f.need(consensus.NewUnavailableDataEvidence(f.proposal(proposer, uncommittedBlock(height)), "missing data availability commitment"))
}

// tx is the transaction that carries evidence.
func (f *fixture) tx(evidence vexoslashing.Evidence) types.Tx {
	f.t.Helper()
	tx, err := EncodeEvidenceTx(evidence)
	if err != nil {
		f.t.Fatal(err)
	}
	return tx
}

// rawTx carries evidence exactly as given. EncodeEvidenceTx normalizes the legacy double_sign label,
// so a transaction that still carries it has to be assembled by hand.
func (f *fixture) rawTx(evidence vexoslashing.Evidence) types.Tx {
	f.t.Helper()
	encoded, err := json.Marshal(evidence)
	if err != nil {
		f.t.Fatal(err)
	}
	return payloadTx(base64.RawURLEncoding.EncodeToString(encoded))
}

func payloadTx(payload string) types.Tx {
	return types.Tx(ModuleName + ":" + ActionEvidence + ":" + payload)
}

func (f *fixture) context(height types.Height) vexoapp.Context {
	return vexoapp.Context{Ctx: context.Background(), ChainID: testChainID, Height: height, Store: f.store}
}

// check is the judgement admission makes of evidence at height: the module reads state and writes
// none.
func (f *fixture) check(height types.Height, evidence vexoslashing.Evidence) error {
	f.t.Helper()
	_, err := f.module.CheckEvidence(context.Background(), f.store, height, evidence)
	return err
}

// standing is what the staking module holds about one validator.
type standing struct {
	Power       uint64
	SelfStake   uint64
	Jailed      bool
	JailedUntil types.Height
	Tombstoned  bool
}

func (f *fixture) standing(id types.ValidatorID) standing {
	f.t.Helper()
	ctx := context.Background()
	power, err := staking.ValidatorPower(ctx, f.store, id)
	if err != nil {
		f.t.Fatal(err)
	}
	stake, err := staking.Stake(ctx, f.store, types.Address(id), id)
	if err != nil {
		f.t.Fatal(err)
	}
	until, jailed, err := staking.JailedUntil(ctx, f.store, id)
	if err != nil {
		f.t.Fatal(err)
	}
	tombstoned, err := staking.Tombstoned(ctx, f.store, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return standing{Power: power, SelfStake: stake, Jailed: jailed, JailedUntil: until, Tombstoned: tombstoned}
}

var healthy = standing{Power: genesisPower, SelfStake: genesisPower}

// requireStanding fails unless every validator holds the standing the table names.
func (f *fixture) requireStanding(want map[types.ValidatorID]standing) {
	f.t.Helper()
	for id, expected := range want {
		if got := f.standing(id); got != expected {
			f.t.Fatalf("validator %s: expected standing %+v, got %+v", id, expected, got)
		}
	}
}

type stateSnapshot map[string]string

// snapshot captures every namespace the slashing module and the staking module write, and the
// validator-set snapshots evidence is judged against.
func (f *fixture) snapshot() stateSnapshot {
	f.t.Helper()
	snapshot := make(stateSnapshot)
	for _, namespace := range []string{staking.ModuleName, ModuleName, validator.RegistryNamespace} {
		pairs, err := f.store.ExportNamespace(context.Background(), namespace)
		if err != nil {
			f.t.Fatal(err)
		}
		for _, pair := range pairs {
			snapshot[namespace+"/"+string(pair.Key)] = string(pair.Value)
		}
	}
	return snapshot
}

func (f *fixture) requireUnchanged(before stateSnapshot, what string) {
	f.t.Helper()
	after := f.snapshot()
	if reflect.DeepEqual(before, after) {
		return
	}
	var changed []string
	for key, value := range after {
		if previous, found := before[key]; !found || previous != value {
			changed = append(changed, key)
		}
	}
	for key := range before {
		if _, found := after[key]; !found {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	f.t.Fatalf("%s changed state: %s", what, strings.Join(changed, ", "))
}

// stageValidatorSet persists the validator set at height the way a node does after a block: the
// registry applies the block's validator updates to the previous set.
func (f *fixture) stageValidatorSet(height types.Height, updates []types.ValidatorUpdate) {
	f.t.Helper()
	_, writes, err := f.registry.StageValidatorUpdatesAt(context.Background(), height, updates)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.store.SetBatch(context.Background(), writes); err != nil {
		f.t.Fatal(err)
	}
	f.registry.CommitStagedValidatorUpdates(height)
}

func (f *fixture) validatorSetAt(height types.Height) validator.Set {
	f.t.Helper()
	set, err := validator.LoadValidatorSet(context.Background(), f.store, height)
	if err != nil {
		f.t.Fatal(err)
	}
	return set
}

func updateFor(updates []types.ValidatorUpdate, id types.ValidatorID) (types.ValidatorUpdate, bool) {
	for _, update := range updates {
		if update.ID == id {
			return update, true
		}
	}
	return types.ValidatorUpdate{}, false
}

// requireRejected fails unless a transaction failed with a log that names want.
func requireRejected(t *testing.T, result types.Result, want error) {
	t.Helper()
	if result.Code == 0 {
		t.Fatalf("expected the evidence to be rejected with %v, but it was applied", want)
	}
	if want != nil && !strings.Contains(result.Log, want.Error()) {
		t.Fatalf("expected the evidence to be rejected with %q, got code %d: %q", want, result.Code, result.Log)
	}
}

func requireApplied(t *testing.T, result types.Result, evidence vexoslashing.Evidence) {
	t.Helper()
	if result.Code != 0 {
		t.Fatalf("expected the evidence to be applied, got code %d: %q", result.Code, result.Log)
	}
	offence, err := vexoslashing.OffenceKey(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Data) != offence {
		t.Fatalf("expected the result to name offence %q, got %q", offence, result.Data)
	}
}

func TestFinalizeBlockPunishesProvenEvidence(t *testing.T) {
	f := newFixture(t)
	evidence := f.equivocation("b", 1, 0)

	response := f.finalize(3, f.tx(evidence))

	requireApplied(t, response.Results[0], evidence)
	// A quarter of b's power is slashed and b is jailed for ten blocks from the block that punished it.
	f.requireStanding(map[types.ValidatorID]standing{
		"a": healthy, "c": healthy, "d": healthy,
		"b": {Power: 75, SelfStake: 75, Jailed: true, JailedUntil: 13},
	})
	// The block reports b's standing so the validator set follows. A jailed validator has no voting
	// power but keeps its identity, so it can come back once its jail ends.
	if len(response.ValidatorUpdates) != 1 {
		t.Fatalf("expected one validator update, got %+v", response.ValidatorUpdates)
	}
	update := response.ValidatorUpdates[0]
	if update.ID != "b" || update.VotingPower != 0 || update.Stake != 75 || !bytes.Equal(update.PublicKey, f.keys["b"]) {
		t.Fatalf("unexpected validator update %+v", update)
	}
}

func TestFinalizeBlockReportsAValidatorThatStaysInTheSet(t *testing.T) {
	f := newFixture(t)
	// An invalid proposal costs a tenth of the stake and no jail.
	evidence := f.missingData("c", 1)

	response := f.finalize(3, f.tx(evidence))

	requireApplied(t, response.Results[0], evidence)
	f.requireStanding(map[types.ValidatorID]standing{
		"a": healthy, "b": healthy, "d": healthy,
		"c": {Power: 90, SelfStake: 90},
	})
	update, found := updateFor(response.ValidatorUpdates, "c")
	if !found || len(response.ValidatorUpdates) != 1 || update.VotingPower != 90 || update.Stake != 90 {
		t.Fatalf("expected c to stay in the set with 90 power, got %+v", response.ValidatorUpdates)
	}
}

func TestFullSlashTombstonesTheValidator(t *testing.T) {
	f := newFixture(t)
	evidence := f.timeoutEquivocation("d", 1, 0)

	response := f.finalize(3, f.tx(evidence))

	requireApplied(t, response.Results[0], evidence)
	f.requireStanding(map[types.ValidatorID]standing{
		"a": healthy, "b": healthy, "c": healthy,
		"d": {Tombstoned: true},
	})
	update, found := updateFor(response.ValidatorUpdates, "d")
	if !found || update.VotingPower != 0 || update.Stake != 0 {
		t.Fatalf("expected d to leave the set with nothing staked, got %+v", response.ValidatorUpdates)
	}
}

func TestReplayedEvidenceIsPunishedOnce(t *testing.T) {
	f := newFixture(t)
	evidence := f.equivocation("b", 1, 0)
	tx := f.tx(evidence)
	punished := standing{Power: 75, SelfStake: 75, Jailed: true, JailedUntil: 13}

	// Two copies in one block: the first punishes, the second is a failed transaction.
	response := f.finalize(3, tx, tx)
	requireApplied(t, response.Results[0], evidence)
	requireRejected(t, response.Results[1], ErrEvidenceAlreadyApplied)
	f.requireStanding(map[types.ValidatorID]standing{"b": punished})
	if len(response.ValidatorUpdates) != 1 {
		t.Fatalf("expected one validator update, got %+v", response.ValidatorUpdates)
	}

	// Neither a later block nor a late one punishes b again or prolongs its jail.
	before := f.snapshot()
	for _, height := range []types.Height{4, 5, 30} {
		later := f.finalize(height, tx)
		requireRejected(t, later.Results[0], ErrEvidenceAlreadyApplied)
		if len(later.ValidatorUpdates) != 0 {
			t.Fatalf("block %d: a replay must not update the validator set, got %+v", height, later.ValidatorUpdates)
		}
	}
	f.requireUnchanged(before, "replayed evidence")
	f.requireStanding(map[types.ValidatorID]standing{"b": punished})
}

// legacyLabelTx carries evidence under the legacy double_sign label, which EncodeEvidenceTx would
// have normalized.
func legacyLabelTx(f *fixture, evidence vexoslashing.Evidence) types.Tx {
	f.t.Helper()
	evidence.Type = vexoslashing.EvidenceDoubleSign
	tx := f.rawTx(evidence)
	payload := strings.TrimPrefix(string(tx), ModuleName+":"+ActionEvidence+":")
	document, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || !bytes.Contains(document, []byte(`"double_sign"`)) {
		f.t.Fatalf("expected the transaction to carry the legacy label, got %q (%v)", document, err)
	}
	return tx
}

func TestTheSameOffenceUnderAnotherLabelIsPunishedOnce(t *testing.T) {
	cases := []struct {
		name   string
		first  func(f *fixture) types.Tx
		second func(f *fixture) types.Tx
		// secondErr is why the second transaction fails. The legacy double_sign label is not a
		// canonical spelling of evidence, so a transaction carrying it never reaches the offence
		// check at all.
		secondErr error
		punished  standing
	}{
		{
			"double_sign after conflicting_vote",
			func(f *fixture) types.Tx { return f.tx(f.equivocation("b", 1, 0)) },
			func(f *fixture) types.Tx { return legacyLabelTx(f, f.equivocation("b", 1, 0)) },
			ErrInvalidEvidenceTx,
			standing{Power: 75, SelfStake: 75, Jailed: true, JailedUntil: 13},
		},
		{
			"unavailable_data after invalid_proposal",
			func(f *fixture) types.Tx { return f.tx(f.missingData("b", 1)) },
			func(f *fixture) types.Tx { return f.tx(f.unavailableData("b", 1)) },
			ErrEvidenceAlreadyApplied,
			standing{Power: 90, SelfStake: 90},
		},
		{
			"invalid_proposal after unavailable_data",
			func(f *fixture) types.Tx { return f.tx(f.unavailableData("b", 1)) },
			func(f *fixture) types.Tx { return f.tx(f.missingData("b", 1)) },
			ErrEvidenceAlreadyApplied,
			standing{Power: 80, SelfStake: 80},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture(t)
			first, second := testCase.first(f), testCase.second(f)

			// The second label in the same block, then in the blocks after it.
			response := f.finalize(3, first, second)
			if response.Results[0].Code != 0 {
				t.Fatalf("expected the first evidence to be applied, got %+v", response.Results[0])
			}
			requireRejected(t, response.Results[1], testCase.secondErr)
			f.requireStanding(map[types.ValidatorID]standing{"b": testCase.punished})

			before := f.snapshot()
			later := f.finalize(4, second)
			requireRejected(t, later.Results[0], testCase.secondErr)
			if len(later.ValidatorUpdates) != 0 {
				t.Fatalf("expected no validator update, got %+v", later.ValidatorUpdates)
			}
			f.requireUnchanged(before, "the same offence under another label")
			f.requireStanding(map[types.ValidatorID]standing{"b": testCase.punished, "a": healthy, "c": healthy, "d": healthy})
		})
	}
}

func TestCheckEvidenceNormalizesTheLegacyLabel(t *testing.T) {
	f := newFixture(t)
	legacy := f.equivocation("b", 1, 0)
	legacy.Type = vexoslashing.EvidenceDoubleSign

	checked, err := f.module.CheckEvidence(context.Background(), f.store, 3, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if checked.Type != vexoslashing.EvidenceConflictingVote {
		t.Fatalf("expected the legacy label to be normalized, got %q", checked.Type)
	}
}

func TestDistinctOffencesArePunishedSeparately(t *testing.T) {
	f := newFixture(t)

	// b signs two blocks in view 0 and two in view 1. Each view is an offence of its own, and the
	// block reports b once, with its standing after both: 100 -> 75 -> 56, as a slash rounds up.
	first, second := f.equivocation("b", 1, 0), f.equivocation("b", 1, 1)
	response := f.finalize(3, f.tx(first), f.tx(second))
	requireApplied(t, response.Results[0], first)
	requireApplied(t, response.Results[1], second)
	f.requireStanding(map[types.ValidatorID]standing{"b": {Power: 56, SelfStake: 56, Jailed: true, JailedUntil: 13}})
	if len(response.ValidatorUpdates) != 1 || response.ValidatorUpdates[0].Stake != 56 {
		t.Fatalf("expected one update with the standing after both slashes, got %+v", response.ValidatorUpdates)
	}

	// Another height is another offence, and so is another kind of fault in a view b already
	// equivocated in. The jail only ever grows.
	third, fourth := f.equivocation("b", 2, 0), f.timeoutEquivocation("b", 1, 0)
	next := f.finalize(4, f.tx(third), f.tx(fourth))
	requireApplied(t, next.Results[0], third)
	requireApplied(t, next.Results[1], fourth)
	f.requireStanding(map[types.ValidatorID]standing{"b": {Jailed: true, JailedUntil: 14, Tombstoned: true}})
}

func TestEvidenceAgainstSeveralValidatorsInOneBlock(t *testing.T) {
	f := newFixture(t)
	evidenceD, evidenceB, evidenceC := f.equivocation("d", 1, 0), f.equivocation("b", 1, 0), f.equivocation("c", 1, 0)

	response := f.finalize(3, f.tx(evidenceD), f.tx(evidenceB), f.tx(evidenceC))

	for index, evidence := range []vexoslashing.Evidence{evidenceD, evidenceB, evidenceC} {
		requireApplied(t, response.Results[index], evidence)
	}
	punished := standing{Power: 75, SelfStake: 75, Jailed: true, JailedUntil: 13}
	f.requireStanding(map[types.ValidatorID]standing{"a": healthy, "b": punished, "c": punished, "d": punished})
	// Every node must report the updates in the same order.
	var reported []types.ValidatorID
	for _, update := range response.ValidatorUpdates {
		reported = append(reported, update.ID)
	}
	if !reflect.DeepEqual(reported, []types.ValidatorID{"b", "c", "d"}) {
		t.Fatalf("expected updates for b, c and d in that order, got %v", reported)
	}
}

func TestABadTransactionDoesNotStopTheBlock(t *testing.T) {
	f := newFixture(t)
	forged := f.need(consensus.NewConflictingVoteEvidence(
		f.vote("a", 1, 0, types.Hash{1}),
		f.voteBy(f.signers["b"], "a", 1, 0, types.Hash{2}),
	))
	genuine := f.equivocation("c", 1, 0)

	response := f.finalize(3, f.tx(forged), payloadTx("!not-base64!"), f.tx(genuine))

	requireRejected(t, response.Results[0], consensus.ErrInvalidEvidenceVoteSignature)
	requireRejected(t, response.Results[1], ErrInvalidEvidenceTx)
	requireApplied(t, response.Results[2], genuine)
	f.requireStanding(map[types.ValidatorID]standing{
		"a": healthy, "b": healthy, "d": healthy,
		"c": {Power: 75, SelfStake: 75, Jailed: true, JailedUntil: 13},
	})
}

func TestEvidenceThatDoesNotVerifyChangesNothing(t *testing.T) {
	f := newFixture(t)
	genuine := f.equivocation("b", 1, 0)

	unsigned := f.need(consensus.NewConflictingVoteEvidence(
		consensus.Vote{Height: 1, Round: 0, BlockHash: types.Hash{1}, ValidatorID: "b"},
		consensus.Vote{Height: 1, Round: 0, BlockHash: types.Hash{2}, ValidatorID: "b"},
	))
	// c signs a vote in b's name to frame b for equivocating.
	framed := f.need(consensus.NewConflictingVoteEvidence(
		f.vote("b", 1, 0, types.Hash{1}),
		f.voteBy(f.signers["c"], "b", 1, 0, types.Hash{2}),
	))
	repeated := f.vote("b", 1, 0, types.Hash{1})
	notConflicting := f.handmade(vexoslashing.EvidenceConflictingVote, "b", 1, 0, consensus.ConflictingVoteProof{First: repeated, Second: repeated})
	mallory, err := vexocrypto.NewDeterministicSigner([]byte("mallory"))
	if err != nil {
		t.Fatal(err)
	}
	outsider := f.equivocationBy(mallory, "mallory", 1, 0)

	// Genuine evidence with one field changed would punish another offence with the proof of this one.
	otherValidator := genuine
	otherValidator.Validator = "c"
	otherHeight := genuine
	otherHeight.Height = 2
	otherRound := genuine
	otherRound.Round = 1
	votesAsTimeoutVotes := genuine
	votesAsTimeoutVotes.Type = vexoslashing.EvidenceConflictingTimeoutVote
	proposalAsVotes := f.missingData("b", 1)
	proposalAsVotes.Type = vexoslashing.EvidenceConflictingVote

	timestampProposal := f.proposal("b", types.Block{Header: types.Header{ChainID: testChainID, Height: 1, TimeUnixNano: 5}})
	// A timestamp fault is no evidence: nodes could judge it differently.
	needsNodeData := f.handmade(vexoslashing.EvidenceInvalidProposal, "b", 1, timestampProposal.Round, consensus.InvalidProposalProof{Proposal: timestampProposal, Reason: "timestamp", ExpectedHash: types.Hash{9}, ActualHash: types.Hash{5}})

	noProof := genuine
	noProof.Proof = nil
	noAccused := genuine
	noAccused.Validator = ""
	noHeight := genuine
	noHeight.Height = 0
	unknownType := genuine
	unknownType.Type = "made_up"
	notAProof := vexoslashing.Evidence{Type: vexoslashing.EvidenceConflictingVote, Validator: "b", Height: 1, Proof: []byte("not a proof")}
	oversized := vexoslashing.Evidence{Type: vexoslashing.EvidenceConflictingVote, Validator: "b", Height: 1, Proof: bytes.Repeat([]byte{1}, MaxProofBytes+1)}

	cases := []struct {
		name     string
		evidence vexoslashing.Evidence
		want     error // nil: only that the evidence is rejected
	}{
		{"votes without signatures", unsigned, consensus.ErrInvalidEvidenceVoteSignature},
		{"a vote the accused did not sign", framed, consensus.ErrInvalidEvidenceVoteSignature},
		{"votes that do not conflict", notConflicting, consensus.ErrVotesDoNotConflict},
		{"a validator outside the set", outsider, consensus.ErrEvidenceValidatorNotFound},
		{"another validator than the proof names", otherValidator, consensus.ErrVotePairMismatch},
		{"another height than the proof names", otherHeight, consensus.ErrVotePairMismatch},
		{"another round than the proof names", otherRound, consensus.ErrVotePairMismatch},
		{"votes presented as timeout votes", votesAsTimeoutVotes, nil},
		{"a proposal fault presented as votes", proposalAsVotes, nil},
		{"a fault nodes could judge differently", needsNodeData, consensus.ErrUnsupportedProposalReason},
		{"a proof that is no proof", notAProof, nil},
		{"no proof", noProof, vexoslashing.ErrEmptyProof},
		{"no accused validator", noAccused, vexoslashing.ErrMissingValidator},
		{"no height", noHeight, vexoslashing.ErrMissingHeight},
		{"an unknown type", unknownType, vexoslashing.ErrUnknownEvidenceType},
		{"a proof beyond the size limit", oversized, ErrEvidenceTooLarge},
		{"a height the chain has not reached", f.equivocation("b", 50, 0), ErrEvidenceFromFuture},
	}
	before := f.snapshot()
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			f := f.forTest(t)
			tx := f.tx(testCase.evidence)
			ctx := f.context(3)

			// Admission judges evidence the way execution does, so rejected evidence neither enters a
			// block nor a pool.
			err := f.check(3, testCase.evidence)
			if err == nil {
				t.Fatal("expected admission to reject the evidence")
			}
			if testCase.want != nil && !errors.Is(err, testCase.want) {
				t.Fatalf("expected admission to reject with %v, got %v", testCase.want, err)
			}
			if err := f.module.ValidateTx(ctx, tx); err == nil {
				t.Fatal("expected ValidateTx to reject the evidence")
			}
			if err := f.module.AuthenticateTx(ctx, tx); err == nil {
				t.Fatal("expected AuthenticateTx to reject the evidence")
			}

			// A proposer that includes it anyway gets a failed transaction and no state change.
			response := f.finalize(3, tx)
			requireRejected(t, response.Results[0], testCase.want)
			if len(response.ValidatorUpdates) != 0 {
				t.Fatalf("expected no validator update, got %+v", response.ValidatorUpdates)
			}
			f.requireUnchanged(before, "rejected evidence")
		})
	}
	f.requireStanding(map[types.ValidatorID]standing{"a": healthy, "b": healthy, "c": healthy, "d": healthy})

	// None of the attempts used up the offence: the genuine evidence still punishes b, once.
	requireApplied(t, f.finalize(4, f.tx(genuine)).Results[0], genuine)
	f.requireStanding(map[types.ValidatorID]standing{
		"a": healthy, "c": healthy, "d": healthy,
		"b": {Power: 75, SelfStake: 75, Jailed: true, JailedUntil: 14},
	})
}

func TestAdmissionReadsStateAndNeverWritesIt(t *testing.T) {
	f := newFixture(t)
	evidence := f.equivocation("b", 1, 0)
	tx := f.tx(evidence)
	ctx := f.context(3)
	before := f.snapshot()

	if err := f.module.ValidateTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := f.module.AuthenticateTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	checked, err := f.module.CheckEvidence(context.Background(), f.store, 3, evidence)
	if err != nil || !reflect.DeepEqual(checked, evidence) {
		t.Fatalf("expected CheckEvidence to return the evidence, got %+v (%v)", checked, err)
	}
	if gas, err := f.module.EstimateGas(ctx, tx); err != nil || gas == 0 {
		t.Fatalf("expected a gas estimate, got %d (%v)", gas, err)
	}
	if settled, err := f.module.Settled(context.Background(), f.store, 3, evidence); err != nil || settled {
		t.Fatalf("expected unpunished evidence to need handling, got settled=%v (%v)", settled, err)
	}
	f.requireUnchanged(before, "admitting evidence")

	requireApplied(t, f.finalize(3, tx).Results[0], evidence)

	// Once punished, admission turns the same evidence away.
	if err := f.module.ValidateTx(ctx, tx); !errors.Is(err, ErrEvidenceAlreadyApplied) {
		t.Fatalf("expected ValidateTx to report the offence as punished, got %v", err)
	}
	if err := f.module.AuthenticateTx(ctx, tx); !errors.Is(err, ErrEvidenceAlreadyApplied) {
		t.Fatalf("expected AuthenticateTx to report the offence as punished, got %v", err)
	}
	if err := f.check(3, evidence); !errors.Is(err, ErrEvidenceAlreadyApplied) {
		t.Fatalf("expected CheckEvidence to report the offence as punished, got %v", err)
	}
	if settled, err := f.module.Settled(context.Background(), f.store, 3, evidence); err != nil || !settled {
		t.Fatalf("expected punished evidence to be settled, got settled=%v (%v)", settled, err)
	}
}

func TestSettledReportsEvidenceThatNeedsNoFurtherHandling(t *testing.T) {
	f := newFixture(t, withPolicy(Policy{Penalties: testPenalties(), Lifecycle: vexoslashing.LifecyclePolicy{EvidenceMaxAge: 5}}))
	punished := f.equivocation("b", 2, 0)
	requireApplied(t, f.finalize(3, f.tx(punished)).Results[0], punished)
	legacy := punished
	legacy.Type = vexoslashing.EvidenceDoubleSign
	fresh := f.equivocation("c", 2, 0)

	cases := []struct {
		name     string
		height   types.Height
		evidence vexoslashing.Evidence
		want     bool
	}{
		{"unpunished evidence", 3, fresh, false},
		{"evidence at the last height it can be punished", 7, fresh, false},
		{"evidence that outlived its age", 8, fresh, true},
		{"punished evidence", 3, punished, true},
		{"punished evidence under the legacy label", 3, legacy, true},
		{"evidence that names no validator", 3, vexoslashing.Evidence{Type: vexoslashing.EvidenceConflictingVote, Height: 2, Proof: []byte("x")}, true},
	}
	for _, testCase := range cases {
		settled, err := f.module.Settled(context.Background(), f.store, testCase.height, testCase.evidence)
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if settled != testCase.want {
			t.Fatalf("%s: expected settled=%v, got %v", testCase.name, testCase.want, settled)
		}
	}
}

func TestEvidenceIsPunishableFromItsHeightForALimitedTime(t *testing.T) {
	f := newFixture(t, withPolicy(Policy{Penalties: testPenalties(), Lifecycle: vexoslashing.LifecyclePolicy{EvidenceMaxAge: 5}}))

	// A height the chain has not reached is judged against a validator set that does not exist yet.
	// The proof is genuine: the height is all that is wrong with it.
	future := f.equivocation("b", 50, 0)
	requireRejected(t, f.finalize(49, f.tx(future)).Results[0], ErrEvidenceFromFuture)
	f.requireStanding(map[types.ValidatorID]standing{"b": healthy})
	requireApplied(t, f.finalize(50, f.tx(future)).Results[0], future)

	// Evidence stays punishable for EvidenceMaxAge blocks and not a block longer.
	lastChance, expired := f.equivocation("c", 1, 0), f.equivocation("d", 1, 0)
	requireApplied(t, f.finalize(6, f.tx(lastChance)).Results[0], lastChance)
	requireRejected(t, f.finalize(7, f.tx(expired)).Results[0], ErrEvidenceExpired)
	f.requireStanding(map[types.ValidatorID]standing{"d": healthy})
}

func TestEvidenceIsJudgedAgainstTheValidatorSetOfItsHeight(t *testing.T) {
	f := newFixture(t)

	// c is punished at height 4 and thereby leaves the validator set at height 5, where d also
	// rotates its consensus key.
	first := f.equivocation("c", 1, 0)
	punishment := f.finalize(4, f.tx(first))
	requireApplied(t, punishment.Results[0], first)
	rotated, err := vexocrypto.NewDeterministicSigner([]byte("d-rotated"))
	if err != nil {
		t.Fatal(err)
	}
	updates := append([]types.ValidatorUpdate(nil), punishment.ValidatorUpdates...)
	updates = append(updates, types.ValidatorUpdate{ID: "d", Address: "d", PublicKey: rotated.PublicKey(), VotingPower: genesisPower, Stake: genesisPower})
	f.stageValidatorSet(5, updates)
	if _, found := f.validatorSetAt(5).Get("c"); found {
		t.Fatal("expected c to have left the validator set at height 5")
	}
	if d, _ := f.validatorSetAt(5).Get("d"); !bytes.Equal(d.PublicKey, rotated.PublicKey()) {
		t.Fatal("expected d to hold its rotated key at height 5")
	}

	// What a validator signed in the past is judged against the key it had and the set it was in.
	before := f.snapshot()
	rejected := []struct {
		name     string
		evidence vexoslashing.Evidence
		want     error
	}{
		{"a vote signed with a key d only holds from height 5", f.equivocationBy(rotated, "d", 3, 0), consensus.ErrInvalidEvidenceVoteSignature},
		{"a vote signed with a key d no longer holds", f.equivocation("d", 6, 0), consensus.ErrInvalidEvidenceVoteSignature},
		{"a validator that is not in the set of the height", f.equivocation("c", 6, 0), consensus.ErrEvidenceValidatorNotFound},
	}
	for _, testCase := range rejected {
		if err := f.check(8, testCase.evidence); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: expected admission to reject with %v, got %v", testCase.name, testCase.want, err)
		}
		requireRejected(t, f.finalize(8, f.tx(testCase.evidence)).Results[0], testCase.want)
	}
	f.requireUnchanged(before, "evidence of the wrong set")

	// Leaving the set or rotating a key does not shield a validator from what it did before.
	accepted := []struct {
		name      string
		evidence  vexoslashing.Evidence
		validator types.ValidatorID
	}{
		{"a validator that has left the set since", f.equivocation("c", 3, 0), "c"},
		{"a vote signed with the key d had then", f.equivocation("d", 3, 0), "d"},
		{"a vote signed with the key d holds now", f.equivocationBy(rotated, "d", 6, 0), "d"},
	}
	for _, testCase := range accepted {
		result := f.finalize(8, f.tx(testCase.evidence)).Results[0]
		if result.Code != 0 {
			t.Fatalf("%s: expected the evidence to be applied, got %+v", testCase.name, result)
		}
	}
	f.requireStanding(map[types.ValidatorID]standing{
		"a": healthy, "b": healthy,
		"c": {Power: 56, SelfStake: 56, Jailed: true, JailedUntil: 18},
		"d": {Power: 56, SelfStake: 56, Jailed: true, JailedUntil: 18},
	})
}

func TestEvidenceOlderThanTheValidatorSetHistoryIsRejected(t *testing.T) {
	// This node holds validator sets from height 5 on.
	f := newFixture(t, withSetFrom(5))
	old := f.equivocation("b", 3, 0)
	before := f.snapshot()

	if err := f.check(8, old); !errors.Is(err, ErrValidatorSetMissing) {
		t.Fatalf("expected admission to report the missing validator set, got %v", err)
	}
	requireRejected(t, f.finalize(8, f.tx(old)).Results[0], ErrValidatorSetMissing)
	f.requireUnchanged(before, "evidence older than the validator set history")

	// The first height with a validator set is judged and punished.
	known := f.equivocation("b", 5, 0)
	requireApplied(t, f.finalize(8, f.tx(known)).Results[0], known)
}

func TestEvidenceTheStakingModuleCannotApplyLeavesNoTrace(t *testing.T) {
	// e is in the validator set but staking does not know it, so the proof verifies and the
	// punishment cannot be applied.
	f := newFixture(t, withUnstaked("e"))
	evidence := f.equivocation("e", 1, 0)
	if err := f.check(3, evidence); err != nil {
		t.Fatalf("expected the proof to verify, got %v", err)
	}
	before := f.snapshot()

	// The module runs on the store itself here, with no transaction layer to throw its writes away.
	result := f.module.DeliverTx(f.context(3), f.tx(evidence))

	requireRejected(t, result, staking.ErrValidatorNotFound)
	f.requireUnchanged(before, "evidence staking could not apply")
	if settled, err := f.module.Settled(context.Background(), f.store, 3, evidence); err != nil || settled {
		t.Fatalf("expected the offence to stay unpunished, got settled=%v (%v)", settled, err)
	}
}

func TestEvidenceExecutionIsMeteredByTheSizeOfTheEvidence(t *testing.T) {
	f := newFixture(t)
	evidence := f.equivocation("b", 1, 0)
	tx := f.tx(evidence)
	estimate, err := f.module.EstimateGas(f.context(3), tx)
	if err != nil || estimate == 0 {
		t.Fatalf("expected a gas estimate, got %d (%v)", estimate, err)
	}
	bloated, err := f.module.EstimateGas(f.context(3), payloadTx(strings.Repeat("A", 64*1024)))
	if err != nil || bloated <= estimate {
		t.Fatalf("expected a larger payload to cost more than %d, got %d (%v)", estimate, bloated, err)
	}
	before := f.snapshot()

	starved := f.context(3).WithGasMeter(vexoapp.NewGasMeter(estimate - 1))
	requireRejected(t, f.module.DeliverTx(starved, tx), vexoapp.ErrOutOfGas)
	f.requireUnchanged(before, "evidence that ran out of gas")

	funded := f.context(3).WithGasMeter(vexoapp.NewGasMeter(estimate))
	requireApplied(t, f.module.DeliverTx(funded, tx), evidence)
	if funded.GasUsed() != estimate {
		t.Fatalf("expected execution to use exactly the estimate %d, used %d", estimate, funded.GasUsed())
	}
}

func TestModuleRefusesToWorkWithoutAStore(t *testing.T) {
	f := newFixture(t)
	evidence := f.equivocation("b", 1, 0)
	tx := f.tx(evidence)

	requireRejected(t, f.module.DeliverTx(vexoapp.Context{Height: 3}, tx), ErrSlashingStoreRequired)
	if err := f.module.ValidateTx(vexoapp.Context{Height: 3}, tx); !errors.Is(err, ErrSlashingStoreRequired) {
		t.Fatalf("expected ValidateTx to need a store, got %v", err)
	}
	if _, err := f.module.CheckEvidence(context.Background(), nil, 3, evidence); !errors.Is(err, ErrSlashingStoreRequired) {
		t.Fatalf("expected CheckEvidence to need a store, got %v", err)
	}
	if _, err := f.module.Settled(context.Background(), nil, 3, evidence); !errors.Is(err, ErrSlashingStoreRequired) {
		t.Fatalf("expected Settled to need a store, got %v", err)
	}
}

func TestFinalityConflictEvidencePunishesTheValidatorsThatSignedBoth(t *testing.T) {
	f := newFixture(t)
	set := f.validatorSetAt(1)
	// a, b and c finalized one block at height 1 in view 0, and a, b and d another in view 2. Only
	// a and b voted for both, so only they are accountable for the conflict.
	first := f.finalityProof(1, types.Hash{1}, 0, "a", "b", "c")
	second := f.finalityProof(1, types.Hash{2}, 2, "a", "b", "d")
	forA := f.need(finality.NewConflictEvidence(set, first, second, "a"))
	forB := f.need(finality.NewConflictEvidence(set, first, second, "b"))
	forC := f.handmade(vexoslashing.EvidenceFinalityConflict, "c", 1, 0, finality.ConflictProof{First: first, Second: second})

	requireRejected(t, f.finalize(3, f.tx(forC)).Results[0], finality.ErrValidatorNotInFinalityConflict)
	f.requireStanding(map[types.ValidatorID]standing{"c": healthy})

	response := f.finalize(3, f.tx(forA), f.tx(forB))
	requireApplied(t, response.Results[0], forA)
	requireApplied(t, response.Results[1], forB)
	punished := standing{Power: 50, SelfStake: 50, Jailed: true, JailedUntil: 23}
	f.requireStanding(map[types.ValidatorID]standing{"a": punished, "b": punished, "c": healthy, "d": healthy})

	// The offence is keyed by validator and height, however many conflicting pairs prove it: the
	// same conflict presented the other way round, and another block finalized in a later view.
	third := f.finalityProof(1, types.Hash{3}, 4, "a", "b", "c", "d")
	before := f.snapshot()
	for _, pair := range [][2]finality.Proof{{second, first}, {first, third}, {second, third}} {
		evidence := f.need(finality.NewConflictEvidence(set, pair[0], pair[1], "a"))
		requireRejected(t, f.finalize(4, f.tx(evidence)).Results[0], ErrEvidenceAlreadyApplied)
	}
	f.requireUnchanged(before, "another proof of a punished finality conflict")
}

func TestEvidenceTransactionRoundTrip(t *testing.T) {
	f := newFixture(t)
	legacy := f.equivocation("b", 1, 0)
	legacy.Type = vexoslashing.EvidenceDoubleSign
	cases := []vexoslashing.Evidence{
		f.equivocation("b", 1, 0),
		f.timeoutEquivocation("c", 2, 1),
		f.missingData("d", 1),
		f.unavailableData("a", 1),
		legacy,
	}
	for _, evidence := range cases {
		tx, err := EncodeEvidenceTx(evidence)
		if err != nil {
			t.Fatal(err)
		}
		// The payload is unsigned and fee-less, and neither ':' nor '=' occurs in it, which the
		// canonical transaction grammar reserves for arguments and tags.
		canonical, err := txcodec.ParseCanonical(tx)
		if err != nil {
			t.Fatalf("%s: %v", evidence.Type, err)
		}
		if canonical.Module != ModuleName || canonical.Action != ActionEvidence || len(canonical.Args) != 1 || len(canonical.Tags) != 0 {
			t.Fatalf("%s: unexpected transaction %+v", evidence.Type, canonical)
		}
		if strings.Count(string(tx), ":") != 2 || strings.Contains(string(tx), "=") {
			t.Fatalf("%s: expected a payload without ':' and '=', got %q", evidence.Type, tx)
		}
		decoded, err := DecodeEvidenceTx(tx)
		if err != nil {
			t.Fatalf("%s: %v", evidence.Type, err)
		}
		if !reflect.DeepEqual(decoded, evidence.Normalize()) {
			t.Fatalf("%s: expected the normalized evidence back, got %+v", evidence.Type, decoded)
		}
	}
}

func TestDecodeEvidenceTxIsStrict(t *testing.T) {
	f := newFixture(t)
	evidence := f.equivocation("b", 1, 0)
	document, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	encode := func(document []byte) string { return base64.RawURLEncoding.EncodeToString(document) }
	payload := encode(document)
	// Standard base64 with padding, a variant of the same document that is not canonical.
	padded := append([]byte(nil), document...)
	for len(padded)%3 == 0 {
		padded = append(padded, ' ')
	}
	// The same evidence under a spelling with bits set past the end of the data. Two spellings of
	// one evidence would be two different transactions.
	spelled := append([]byte(nil), document...)
	for len(spelled)%3 != 1 {
		spelled = append(spelled, ' ')
	}
	canonicalSpelling := encode(spelled)
	const urlAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	lastIndex := strings.IndexByte(urlAlphabet, canonicalSpelling[len(canonicalSpelling)-1])
	nonCanonicalSpelling := canonicalSpelling[:len(canonicalSpelling)-1] + string(urlAlphabet[lastIndex|1])

	cases := []struct {
		name string
		tx   types.Tx
		want error
	}{
		{"another module", types.Tx("bank:" + ActionEvidence + ":" + payload), ErrInvalidEvidenceTx},
		{"another action", types.Tx(ModuleName + ":slash:" + payload), ErrInvalidEvidenceTx},
		{"no payload", types.Tx(ModuleName + ":" + ActionEvidence), ErrInvalidEvidenceTx},
		{"an empty payload", payloadTx(""), ErrInvalidEvidenceTx},
		{"two payloads", payloadTx(payload + ":" + payload), ErrInvalidEvidenceTx},
		{"a payload that is not base64", payloadTx("!not-base64!"), ErrInvalidEvidenceTx},
		{"padded base64", payloadTx(base64.URLEncoding.EncodeToString(padded)), ErrInvalidEvidenceTx},
		{"base64 with bits set past the end of the data", payloadTx(nonCanonicalSpelling), ErrInvalidEvidenceTx},
		// JSON whitespace re-spells the same evidence as another transaction.
		{"the document with trailing whitespace", payloadTx(canonicalSpelling), ErrInvalidEvidenceTx},
		{"a document that is not JSON", payloadTx(encode([]byte("evidence"))), ErrInvalidEvidenceTx},
		{"a document with an unknown field", payloadTx(encode([]byte(`{"Type":"conflicting_vote","Validator":"b","Height":1,"Round":0,"Proof":"AA==","Extra":true}`))), ErrInvalidEvidenceTx},
		{"a document followed by data", payloadTx(encode(append(append([]byte(nil), document...), []byte(` {}`)...))), ErrInvalidEvidenceTx},
		{"a transaction beyond the size limit", payloadTx(strings.Repeat("A", 4*MaxProofBytes)), ErrEvidenceTooLarge},
	}
	before := f.snapshot()
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			f := f.forTest(t)
			if _, err := DecodeEvidenceTx(testCase.tx); !errors.Is(err, testCase.want) {
				t.Fatalf("expected %v, got %v", testCase.want, err)
			}
			// Whatever the runtime does with it, it is a failed transaction and no state change.
			if result := f.finalize(3, testCase.tx).Results[0]; result.Code == 0 {
				t.Fatalf("expected a failed transaction, got %+v", result)
			}
			f.requireUnchanged(before, "a malformed evidence transaction")
		})
	}

	// Only the one canonical spelling decodes.
	decoded, err := DecodeEvidenceTx(payloadTx(payload))
	if err != nil || !reflect.DeepEqual(decoded, evidence) {
		t.Fatalf("expected the canonical encoding to decode, got %+v (%v)", decoded, err)
	}
}

func TestNewModuleWithPolicyRejectsUnusablePolicies(t *testing.T) {
	suite := deterministicSuite(t)
	cases := []struct {
		name   string
		policy Policy
		suite  vexocrypto.RuntimeSuite
	}{
		// Evidence is always normalized, so a penalty under the legacy label would never be consulted.
		{"a penalty under the legacy label", Policy{Penalties: vexoslashing.PenaltyPolicy{vexoslashing.EvidenceDoubleSign: {SlashBPS: 100}}}, suite},
		{"a slash beyond the whole stake", Policy{Penalties: vexoslashing.PenaltyPolicy{vexoslashing.EvidenceConflictingVote: {SlashBPS: vexoslashing.BasisPoints + 1}}}, suite},
		{"a penalty for an unknown type", Policy{Penalties: vexoslashing.PenaltyPolicy{"made_up": {SlashBPS: 100}}}, suite},
		{"no verifiers", Policy{}, vexocrypto.RuntimeSuite{}},
	}
	for _, testCase := range cases {
		if _, err := NewModuleWithPolicy(testCase.policy, testCase.suite); !errors.Is(err, ErrSlashingPolicyInvalid) {
			t.Fatalf("%s: expected an invalid policy, got %v", testCase.name, err)
		}
	}
}

func TestEvidenceWithoutAConfiguredPenaltyIsNotPunished(t *testing.T) {
	onlyVotes := Policy{Penalties: vexoslashing.PenaltyPolicy{vexoslashing.EvidenceConflictingVote: {SlashBPS: 2500}}}
	f := newFixture(t, withPolicy(onlyVotes))
	// The proof is genuine: the missing penalty is all that is wrong with it.
	evidence := f.timeoutEquivocation("b", 1, 0)
	before := f.snapshot()

	if err := f.check(3, evidence); !errors.Is(err, vexoslashing.ErrPenaltyNotConfigured) {
		t.Fatalf("expected admission to find no penalty, got %v", err)
	}
	requireRejected(t, f.finalize(3, f.tx(evidence)).Results[0], vexoslashing.ErrPenaltyNotConfigured)
	f.requireUnchanged(before, "evidence without a penalty")
}

func TestZeroPolicySelectsTheDefaultPenalties(t *testing.T) {
	f := newFixture(t, withPolicy(Policy{}))
	evidence := f.equivocation("b", 1, 0)

	requireApplied(t, f.finalize(3, f.tx(evidence)).Results[0], evidence)

	penalty := vexoslashing.DefaultPenaltyPolicy()[vexoslashing.EvidenceConflictingVote]
	f.requireStanding(map[types.ValidatorID]standing{"b": {
		Power:       genesisPower - genesisPower*penalty.SlashBPS/vexoslashing.BasisPoints,
		SelfStake:   genesisPower - genesisPower*penalty.SlashBPS/vexoslashing.BasisPoints,
		Jailed:      true,
		JailedUntil: types.Height(3 + penalty.JailDuration),
	}})
}

func TestClonedModuleAppliesTheSamePolicy(t *testing.T) {
	f := newFixture(t)
	evidence := f.equivocation("b", 1, 0)

	// Replaying a block runs it on clones of the modules, which must punish like the original.
	replay, err := f.runtime.NewReplayApp(f.store)
	if err != nil {
		t.Fatal(err)
	}
	response, err := replay.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(3, f.tx(evidence))})
	if err != nil {
		t.Fatal(err)
	}
	requireApplied(t, response.Results[0], evidence)
	f.requireStanding(map[types.ValidatorID]standing{"b": {Power: 75, SelfStake: 75, Jailed: true, JailedUntil: 13}})
}

func TestHonestNodesDoNotLetUnprovenEvidenceIntoBlocks(t *testing.T) {
	for _, withAnte := range []bool{false, true} {
		name := "without an ante handler"
		if withAnte {
			name = "with an ante handler"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			if withAnte {
				// Evidence is unsigned and pays no fee: the ante handler admits it because the slashing
				// module authenticates it.
				f.runtime.WithAnte(vexoapp.NewAnteKeeper(vexoapp.AnteConfig{RequireSigned: true, EvidenceAuthenticator: f.module}))
			}
			f.finalize(2)
			genuine := f.equivocation("b", 1, 0)
			// d signs a vote in c's name to frame c.
			forged := f.need(consensus.NewConflictingVoteEvidence(
				f.vote("c", 1, 0, types.Hash{1}),
				f.voteBy(f.signers["d"], "c", 1, 0, types.Hash{2}),
			))
			genuineTx, forgedTx := f.tx(genuine), f.tx(forged)

			if check := f.runtime.CheckTx(genuineTx); check.Result.Code != 0 {
				t.Fatalf("expected the mempool to admit genuine evidence, got %+v", check.Result)
			}
			if check := f.runtime.CheckTx(forgedTx); check.Result.Code == 0 {
				t.Fatal("expected the mempool to refuse forged evidence")
			}
			if response := f.runtime.ProcessProposal(vexoapp.ProcessProposalRequest{Block: blockOf(3, genuineTx)}); !response.Accepted {
				t.Fatalf("expected a block with genuine evidence to be accepted, got %q", response.Reason)
			}
			if response := f.runtime.ProcessProposal(vexoapp.ProcessProposalRequest{Block: blockOf(3, forgedTx)}); response.Accepted {
				t.Fatal("expected a block with forged evidence to be refused")
			}
			prepared, err := f.runtime.PrepareProposal(vexoapp.PrepareProposalRequest{Height: 3, Txs: []types.Tx{forgedTx, genuineTx}})
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Txs) != 1 || !bytes.Equal(prepared.Txs[0], genuineTx) {
				t.Fatalf("expected a proposer to keep only the genuine evidence, got %d transactions", len(prepared.Txs))
			}

			result := f.finalize(3, genuineTx).Results[0]
			requireApplied(t, result, genuine)
			if withAnte && (result.FeePaid != 0 || result.GasUsed == 0) {
				t.Fatalf("expected fee-less evidence that is metered, got %+v", result)
			}

			// Punished evidence is turned away by every gate, so replaying it costs the network nothing.
			if check := f.runtime.CheckTx(genuineTx); check.Result.Code == 0 {
				t.Fatal("expected the mempool to refuse punished evidence")
			}
			if response := f.runtime.ProcessProposal(vexoapp.ProcessProposalRequest{Block: blockOf(4, genuineTx)}); response.Accepted {
				t.Fatal("expected a block that replays punished evidence to be refused")
			}
			prepared, err = f.runtime.PrepareProposal(vexoapp.PrepareProposalRequest{Height: 4, Txs: []types.Tx{genuineTx}})
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Txs) != 0 {
				t.Fatalf("expected a proposer to drop punished evidence, kept %d transactions", len(prepared.Txs))
			}
		})
	}
}

// Regression: two proofs of one offence (here an invalid proposal shown as missing data and as
// unavailable data at the same height) were both admitted into a block for free; only the first
// could take effect. A block carries at most one evidence transaction per offence.
func TestBlockCarriesOneEvidenceTransactionPerOffence(t *testing.T) {
	f := newFixture(t)
	first, second := f.tx(f.missingData("b", 1)), f.tx(f.unavailableData("b", 1))
	prepared, err := f.runtime.PrepareProposal(vexoapp.PrepareProposalRequest{Height: 3, Txs: []types.Tx{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Txs) != 1 {
		t.Fatalf("expected a proposer to keep one proof of the offence, kept %d", len(prepared.Txs))
	}
	rejectedForConflict := false
	for _, txs := range [][]types.Tx{{first, second}, {second, first}} {
		response := f.runtime.ProcessProposal(vexoapp.ProcessProposalRequest{Block: blockOf(3, txs...)})
		if response.Accepted {
			t.Fatalf("expected a block with two proofs of one offence to be rejected")
		}
		if strings.Contains(response.Reason, vexoapp.ErrExclusiveTxConflict.Error()) {
			rejectedForConflict = true
		}
	}
	if !rejectedForConflict {
		t.Fatal("expected the fairly ordered block to be rejected for the duplicate offence")
	}
	if response := f.runtime.ProcessProposal(vexoapp.ProcessProposalRequest{Block: blockOf(3, prepared.Txs...)}); !response.Accepted {
		t.Fatalf("expected the prepared block to be accepted, got %q", response.Reason)
	}
}

// certifiedBlock is a block of height whose header commits appHash for the state of stateHeight.
func certifiedBlock(height types.Height, stateHeight types.Height, appHash types.Hash, txs ...types.Tx) types.Block {
	return types.Block{Header: types.Header{ChainID: testChainID, Height: height, ValidatorSetHeight: stateHeight + 1, StateHeight: stateHeight, AppHash: appHash}, Txs: txs}
}

// appHashEvidence is invalid-proposal evidence that proposer signed a block of height naming
// claimed as the app hash of stateHeight, which expected the reporter says it is.
func (f *fixture) appHashEvidence(proposer types.ValidatorID, height types.Height, stateHeight types.Height, claimed types.Hash, expected types.Hash) vexoslashing.Evidence {
	f.t.Helper()
	proposal := f.proposal(proposer, certifiedBlock(height, stateHeight, claimed))
	return f.need(consensus.NewInvalidProposalHashEvidence(proposal, consensus.InvalidProposalReasonAppHash, expected))
}

func (f *fixture) finalizeBlock(block types.Block) vexoapp.FinalizeBlockResponse {
	f.t.Helper()
	response, err := f.runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: block})
	if err != nil {
		f.t.Fatalf("finalize block %d: %v", block.Header.Height, err)
	}
	return response
}

// A proposal whose header names another app hash for a state height than the certified blocks
// committed is punished; the judgement reads only the certified app hashes the chain recorded, so
// a reporter whose own state diverged cannot frame an honest proposer.
func TestAProposalNamingAnotherAppHashThanTheCertifiedOneIsPunished(t *testing.T) {
	f := newFixture(t)
	certified := types.Hash{0xc0}
	f.finalizeBlock(certifiedBlock(1, 0, certified))
	f.finalizeBlock(certifiedBlock(2, 0, certified))

	forged := f.appHashEvidence("b", 2, 0, types.Hash{0xba}, certified)
	response := f.finalizeBlock(certifiedBlock(3, 0, certified, f.tx(forged)))
	requireApplied(t, response.Results[0], forged)

	// The honest proposer named the certified hash; a reporter claiming another one is refused.
	framed := f.appHashEvidence("c", 2, 0, certified, types.Hash{0x0d})
	if err := f.check(4, framed); !errors.Is(err, consensus.ErrInvalidProposal) {
		t.Fatalf("expected evidence against the certified app hash to be refused, got %v", err)
	}
	// No certified block named state height 5, so nothing can be judged about it.
	unknown := f.appHashEvidence("d", 3, 5, types.Hash{0xba}, certified)
	if err := f.check(4, unknown); !errors.Is(err, consensus.ErrAppHashNotCertified) {
		t.Fatalf("expected evidence about an uncertified state height to be refused, got %v", err)
	}
}

// A certified app hash is kept while a block that named its state height is within the evidence
// window, and dropped after it.
func TestCertifiedAppHashesAreKeptForTheEvidenceWindow(t *testing.T) {
	f := newFixture(t, withPolicy(Policy{Penalties: testPenalties(), Lifecycle: vexoslashing.LifecyclePolicy{EvidenceMaxAge: 4}}))
	window := types.Height(4)
	certified := types.Hash{0xc1}
	f.finalizeBlock(certifiedBlock(1, 0, certified))
	hashes := certifiedAppHashes{ctx: context.Background(), store: f.store}
	for height := types.Height(2); height <= window+1; height++ {
		f.finalizeBlock(certifiedBlock(height, 1, types.Hash{0xc2}))
	}
	if _, found, err := hashes.CertifiedAppHash(0); err != nil || !found {
		t.Fatalf("expected the app hash of state 0 to be kept while block 1 is in the window, found=%v err=%v", found, err)
	}
	f.finalizeBlock(certifiedBlock(window+2, 1, types.Hash{0xc2}))
	if _, found, err := hashes.CertifiedAppHash(0); err != nil || found {
		t.Fatalf("expected the app hash of state 0 to be dropped once block 1 left the window, found=%v err=%v", found, err)
	}
	if hash, found, err := hashes.CertifiedAppHash(1); err != nil || !found || hash != (types.Hash{0xc2}) {
		t.Fatalf("expected the app hash of state 1 to stay, got %x found=%v err=%v", hash, found, err)
	}
	if _, err := f.store.Get(context.Background(), ModuleName, certifiedNamingKey(1)); !errors.Is(err, vexostore.ErrKeyNotFound) {
		t.Fatalf("expected the naming record of block 1 to be dropped, got %v", err)
	}
}
