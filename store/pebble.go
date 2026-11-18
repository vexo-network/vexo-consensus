package store

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/cockroachdb/pebble/v2"
	"sync"

	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

var (
	blockHeightPrefix       = []byte("block:height:")
	blockHashPrefix         = []byte("block:hash:")
	blockIndexKey           = []byte("block:index")
	evidencePrefix          = []byte("evidence:")
	finalityPrefix          = []byte("finality:height:")
	finalityLatestKey       = []byte("finality:latest")
	kvPrefix                = []byte("kv:")
	kvHistoryPrefix         = []byte("kvh:")
	schemaStateKey          = []byte("schema:state")
	stateLatestKey          = []byte("state:latest")
	stateHeightPrefix       = []byte("state:height:")
	stateRootPrefix         = []byte("state:root:")
	upgradePlanHeightPrefix = []byte("upgrade:plan:height:")
	upgradePlanNamePrefix   = []byte("upgrade:plan:name:")
)

type PebbleStore struct {
	db           *pebble.DB
	writeOptions *pebble.WriteOptions
	// writeMu orders the writes that maintain state history: block commits,
	// direct writes, imports and pruning each read the history they extend.
	writeMu   sync.Mutex
	closeOnce sync.Once
	// treeCache holds the nodes of the current state trees; see Tree node cache.
	treeCache *treeNodeCache
	// computed holds the last root computation of each namespace; see Computed trees.
	computed *computedTrees
}

type PebbleOptions struct {
	SyncWrites bool
}

type kvHistoryRecord struct {
	Height    types.Height
	Namespace string
	Key       []byte
	Value     []byte
	Deleted   bool
}

func OpenPebble(path string) (*PebbleStore, error) {
	return OpenPebbleWithOptions(path, PebbleOptions{SyncWrites: true})
}

// pebbleMemTableSize is the size of a memtable. A block commit is one batch holding the whole
// block, so a memtable must hold several: a small memtable flushes on nearly every block, stalling
// writes behind level-0 compaction.
const pebbleMemTableSize = 64 << 20

// pebbleCacheSize is the block cache. Executing a block walks the state trees from their roots to
// every key it writes, so the upper levels of the trees are read by every block.
const pebbleCacheSize = 256 << 20

func OpenPebbleWithOptions(path string, options PebbleOptions) (*PebbleStore, error) {
	cache := pebble.NewCache(pebbleCacheSize)
	defer cache.Unref()
	db, err := pebble.Open(path, &pebble.Options{
		Cache:        cache,
		MemTableSize: pebbleMemTableSize,
		Logger:       quietLogger{},
	})
	if err != nil {
		return nil, err
	}
	writeOptions := pebble.NoSync
	if options.SyncWrites {
		writeOptions = pebble.Sync
	}
	return &PebbleStore{db: db, writeOptions: writeOptions, treeCache: newTreeNodeCache(), computed: newComputedTrees()}, nil
}

// writeBatch commits batch. Every write of the store goes through it, so the tree node cache sees
// every change of a tree node.
func (store *PebbleStore) writeBatch(batch *pebble.Batch) error {
	return store.treeCache.commit(batch, store.writeOptions)
}

func (store *PebbleStore) put(key []byte, value []byte) error {
	batch := store.db.NewBatch()
	defer batch.Close()
	batch.Set(key, value, nil)
	return store.writeBatch(batch)
}

func (store *PebbleStore) delete(key []byte) error {
	batch := store.db.NewBatch()
	defer batch.Close()
	batch.Delete(key, nil)
	return store.writeBatch(batch)
}

// snapshot opens a snapshot of the store with the view of the tree node cache that matches it.
func (store *PebbleStore) snapshot() (*pebble.Snapshot, treeCacheView) {
	generation, stable := store.treeCache.stable()
	snapshot := store.db.NewSnapshot()
	if after, _ := store.treeCache.stable(); !stable || after != generation {
		return snapshot, treeCacheView{}
	}
	return snapshot, treeCacheView{cache: store.treeCache, generation: generation}
}

// liveTreeView is the view of the tree node cache for reads of the database itself by a writer
// that holds writeMu, so no batch that changes tree nodes commits while it reads.
func (store *PebbleStore) liveTreeView() treeCacheView {
	generation, stable := store.treeCache.stable()
	if !stable {
		return treeCacheView{}
	}
	return treeCacheView{cache: store.treeCache, generation: generation}
}

func (store *PebbleStore) IsNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

func (store *PebbleStore) SaveBlock(ctx context.Context, record BlockRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if record.Block.Header.Height == 0 || record.Hash == (types.Hash{}) {
		return ErrInvalidBlockRecord
	}

	encoded := encodeBlockRecord(record)
	batch := store.db.NewBatch()
	defer batch.Close()
	batch.Set(blockHeightKey(record.Block.Header.Height), encoded, nil)
	batch.Set(blockHashKey(record.Hash), blockHashValue(record.Block.Header.Height), nil)
	index, err := store.nextBlockIndex(record.Block.Header.Height)
	if err != nil {
		return err
	}
	encodedIndex := encodeBlockIndex(index)
	batch.Set(blockIndexKey, encodedIndex, nil)
	return store.writeBatch(batch)
}

func (store *PebbleStore) BlockByHeight(ctx context.Context, height types.Height) (BlockRecord, error) {
	select {
	case <-ctx.Done():
		return BlockRecord{}, ctx.Err()
	default:
	}
	record, err := store.getBlock(blockHeightKey(height))
	if err != nil {
		return BlockRecord{}, err
	}
	if record.Block.Header.Height != height {
		return BlockRecord{}, ErrInvalidBlockRecord
	}
	return record, nil
}

func (store *PebbleStore) BlockByHash(ctx context.Context, hash types.Hash) (BlockRecord, error) {
	select {
	case <-ctx.Done():
		return BlockRecord{}, ctx.Err()
	default:
	}
	encodedHeight, err := getValue(store.db, blockHashKey(hash))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return BlockRecord{}, ErrBlockNotFound
		}
		return BlockRecord{}, err
	}
	if len(encodedHeight) != 8 {
		return BlockRecord{}, ErrInvalidBlockRecord
	}
	record, err := store.getBlock(blockHeightKey(types.Height(binary.BigEndian.Uint64(encodedHeight))))
	if err != nil {
		return BlockRecord{}, err
	}
	if record.Hash != hash {
		return BlockRecord{}, ErrInvalidBlockRecord
	}
	return record, nil
}

func (store *PebbleStore) BlockIndex(ctx context.Context) (BlockIndex, error) {
	select {
	case <-ctx.Done():
		return BlockIndex{}, ctx.Err()
	default:
	}

	encoded, err := getValue(store.db, blockIndexKey)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return BlockIndex{}, ErrBlockIndexNotFound
		}
		return BlockIndex{}, err
	}

	index, err := decodeBlockIndex(encoded)
	if err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

func (store *PebbleStore) PruneBelow(ctx context.Context, retainFrom types.Height) (PruneResult, error) {
	select {
	case <-ctx.Done():
		return PruneResult{}, ctx.Err()
	default:
	}
	if retainFrom == 0 {
		return PruneResult{}, ErrInvalidPruneHeight
	}
	store.writeMu.Lock()
	defer store.writeMu.Unlock()

	index, err := store.BlockIndex(ctx)
	if err != nil {
		return PruneResult{}, err
	}
	result := PruneResult{RetainFromHeight: retainFrom}
	batch := store.db.NewBatch()
	defer batch.Close()

	for height := index.EarliestHeight; height < retainFrom && height <= index.LatestHeight; height++ {
		record, err := store.getBlock(blockHeightKey(height))
		if errors.Is(err, ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return PruneResult{}, err
		}
		batch.Delete(blockHeightKey(height), nil)
		batch.Delete(blockHashKey(record.Hash), nil)
		result.PrunedBlocks++
	}

	prunedRoots, err := store.pruneStateRootsBelow(ctx, batch, retainFrom)
	if err != nil {
		return PruneResult{}, err
	}
	prunedStates, err := store.pruneStatesBelow(ctx, batch, retainFrom)
	if err != nil {
		return PruneResult{}, err
	}
	result.PrunedStates = prunedStates
	result.PrunedStateRoots = prunedRoots
	prunedVersions, prunedTreeVersions, err := store.pruneHistoryBelow(ctx, batch, retainFrom)
	if err != nil {
		return PruneResult{}, err
	}
	result.PrunedStateVersions = prunedVersions
	result.PrunedStateTreeVersions = prunedTreeVersions

	newIndex, err := store.indexAfterPrune(ctx, index, retainFrom, result.PrunedBlocks)
	if err != nil {
		return PruneResult{}, err
	}
	if newIndex.TotalBlocks == 0 {
		batch.Delete(blockIndexKey, nil)
	} else {
		encodedIndex := encodeBlockIndex(newIndex)
		batch.Set(blockIndexKey, encodedIndex, nil)
	}
	if err := store.writeBatch(batch); err != nil {
		return PruneResult{}, err
	}
	return result, nil
}

func (store *PebbleStore) PruneByRetention(ctx context.Context, policy RetentionPolicy) (PruneResult, error) {
	select {
	case <-ctx.Done():
		return PruneResult{}, ctx.Err()
	default:
	}
	if policy.RetainRecent == 0 {
		return PruneResult{}, ErrInvalidRetention
	}
	index, err := store.BlockIndex(ctx)
	if err != nil {
		return PruneResult{}, err
	}
	if index.TotalBlocks <= policy.RetainRecent {
		return PruneResult{RetainFromHeight: index.EarliestHeight}, nil
	}
	retainFrom := index.LatestHeight - types.Height(policy.RetainRecent) + 1
	return store.PruneBelow(ctx, retainFrom)
}

func (store *PebbleStore) SaveState(ctx context.Context, state StateRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if state.Height == 0 {
		return ErrInvalidStateRecord
	}

	encoded := encodeStateRecord(state)
	batch := store.db.NewBatch()
	defer batch.Close()
	batch.Set(stateLatestKey, encoded, nil)
	batch.Set(stateHeightKey(state.Height), encoded, nil)
	return store.writeBatch(batch)
}

func (store *PebbleStore) SaveSchemaState(ctx context.Context, state upgrade.State) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	encoded := encodeSchemaState(state)
	return store.put(schemaStateKey, encoded)
}

func (store *PebbleStore) SchemaState(ctx context.Context) (upgrade.State, error) {
	select {
	case <-ctx.Done():
		return upgrade.State{}, ctx.Err()
	default:
	}
	encoded, err := getValue(store.db, schemaStateKey)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return upgrade.State{}, ErrStateNotFound
		}
		return upgrade.State{}, err
	}
	state, err := decodeSchemaState(encoded)
	if err != nil {
		return upgrade.State{}, err
	}
	return state, nil
}

func (store *PebbleStore) SaveUpgradePlan(ctx context.Context, plan upgrade.Plan) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	currentHeight, err := store.currentHeight(ctx)
	if err != nil {
		return err
	}
	if err := upgrade.ValidateSchedule(plan, currentHeight); err != nil {
		return err
	}
	encoded := encodeUpgradePlan(plan)
	batch := store.db.NewBatch()
	defer batch.Close()
	batch.Set(upgradePlanHeightKey(plan.Height), encoded, nil)
	batch.Set(upgradePlanNameKey(plan.Name), encoded, nil)
	return store.writeBatch(batch)
}

// currentHeight is the height of the latest committed state, zero before the first block.
func (store *PebbleStore) currentHeight(ctx context.Context) (types.Height, error) {
	state, err := store.LatestState(ctx)
	if errors.Is(err, ErrStateNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return state.Height, nil
}

func (store *PebbleStore) UpgradePlanByHeight(ctx context.Context, height types.Height) (upgrade.Plan, bool, error) {
	select {
	case <-ctx.Done():
		return upgrade.Plan{}, false, ctx.Err()
	default:
	}
	if height == 0 {
		return upgrade.Plan{}, false, ErrUpgradePlanNotFound
	}
	return store.upgradePlan(upgradePlanHeightKey(height))
}

func (store *PebbleStore) UpgradePlanByName(ctx context.Context, name string) (upgrade.Plan, bool, error) {
	select {
	case <-ctx.Done():
		return upgrade.Plan{}, false, ctx.Err()
	default:
	}
	if name == "" {
		return upgrade.Plan{}, false, ErrUpgradePlanNotFound
	}
	return store.upgradePlan(upgradePlanNameKey(name))
}

// NextUpgradePlan returns the scheduled plan with the lowest height at or above height.
func (store *PebbleStore) NextUpgradePlan(ctx context.Context, height types.Height) (upgrade.Plan, bool, error) {
	select {
	case <-ctx.Done():
		return upgrade.Plan{}, false, ctx.Err()
	default:
	}
	heights := prefixRange(upgradePlanHeightPrefix)
	heights.lower = upgradePlanHeightKey(height)
	iterator := newIter(store.db, heights)
	defer iterator.Close()
	if !iterator.First() {
		return upgrade.Plan{}, false, iterator.Error()
	}
	return decodeUpgradePlan(iterator.Value())
}

func (store *PebbleStore) upgradePlan(key []byte) (upgrade.Plan, bool, error) {
	encoded, err := getValue(store.db, key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return upgrade.Plan{}, false, nil
		}
		return upgrade.Plan{}, false, err
	}
	return decodeUpgradePlan(encoded)
}

func decodeUpgradePlan(encoded []byte) (upgrade.Plan, bool, error) {
	plan, err := decodeUpgradePlanRecord(encoded)
	if err != nil {
		return upgrade.Plan{}, false, err
	}
	if err := upgrade.ValidatePlan(plan); err != nil {
		return upgrade.Plan{}, false, err
	}
	return plan, true, nil
}

func (store *PebbleStore) LatestState(ctx context.Context) (StateRecord, error) {
	select {
	case <-ctx.Done():
		return StateRecord{}, ctx.Err()
	default:
	}

	encoded, err := getValue(store.db, stateLatestKey)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return StateRecord{}, ErrStateNotFound
		}
		return StateRecord{}, err
	}

	state, err := decodeStateRecord(encoded)
	if err != nil {
		return StateRecord{}, err
	}
	if state.Height == 0 {
		return StateRecord{}, ErrInvalidStateRecord
	}
	return state, nil
}

func (store *PebbleStore) StateByHeight(ctx context.Context, height types.Height) (StateRecord, error) {
	select {
	case <-ctx.Done():
		return StateRecord{}, ctx.Err()
	default:
	}
	if height == 0 {
		return StateRecord{}, ErrInvalidStateRecord
	}
	encoded, err := getValue(store.db, stateHeightKey(height))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return StateRecord{}, ErrStateNotFound
		}
		return StateRecord{}, err
	}
	state, err := decodeStateRecord(encoded)
	if err != nil {
		return StateRecord{}, err
	}
	if state.Height != height {
		return StateRecord{}, ErrInvalidStateRecord
	}
	return state, nil
}

func (store *PebbleStore) SaveStateRoot(ctx context.Context, record StateRootRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if record.Height == 0 || record.Namespace == "" {
		return ErrInvalidStateRoot
	}
	encoded := encodeStateRootRecord(record)
	return store.put(stateRootKey(record.Height, record.Namespace), encoded)
}

func (store *PebbleStore) CommitBlockState(ctx context.Context, block BlockRecord, state StateRecord, roots []StateRootRecord) error {
	return store.commitBlockStateBatch(ctx, nil, block, state, roots)
}

func (store *PebbleStore) CommitBlockStateWithWrites(ctx context.Context, writes []KVWrite, block BlockRecord, state StateRecord, roots []StateRootRecord) error {
	return store.commitBlockStateBatch(ctx, writes, block, state, roots)
}

func (store *PebbleStore) commitBlockStateBatch(ctx context.Context, writes []KVWrite, block BlockRecord, state StateRecord, roots []StateRootRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if block.Block.Header.Height == 0 || block.Hash == (types.Hash{}) {
		return ErrInvalidBlockRecord
	}
	if state.Height == 0 {
		return ErrInvalidStateRecord
	}
	for _, root := range roots {
		if root.Height == 0 || root.Namespace == "" {
			return ErrInvalidStateRoot
		}
	}
	for _, write := range writes {
		if write.Namespace == "" {
			return ErrInvalidNamespace
		}
		if len(write.Key) == 0 {
			return ErrInvalidKey
		}
	}

	block.StateRoots = append([]StateRootRecord(nil), roots...)
	encodedBlock := encodeBlockRecord(block)
	index, err := store.nextBlockIndex(block.Block.Header.Height)
	if err != nil {
		return err
	}
	encodedIndex := encodeBlockIndex(index)
	encodedState := encodeStateRecord(state)

	store.writeMu.Lock()
	defer store.writeMu.Unlock()
	batch := store.db.NewBatch()
	defer batch.Close()
	existing, err := store.directWritesAt(state.Height)
	if err != nil {
		return err
	}
	history := newHistoryBatch(store, batch, state.Height, existing)
	trees := newStateTrees(store.db, store.liveTreeView())
	if err := trees.applyNamespaces(store.computed, writes); err != nil {
		return err
	}
	for _, write := range writes {
		key := kvKey(write.Namespace, write.Key)
		if err := history.record(write); err != nil {
			return err
		}
		if write.Delete {
			batch.Delete(key, nil)
			continue
		}
		batch.Set(key, append([]byte(nil), write.Value...), nil)
	}
	if err := trees.persist(batch, true, state.Height, existing); err != nil {
		return err
	}
	batch.Set(blockHeightKey(block.Block.Header.Height), encodedBlock, nil)
	batch.Set(blockHashKey(block.Hash), blockHashValue(block.Block.Header.Height), nil)
	batch.Set(blockIndexKey, encodedIndex, nil)
	batch.Set(stateLatestKey, encodedState, nil)
	batch.Set(stateHeightKey(state.Height), encodedState, nil)
	for _, root := range roots {
		encodedRoot := encodeStateRootRecord(root)
		batch.Set(stateRootKey(root.Height, root.Namespace), encodedRoot, nil)
	}
	return store.writeBatch(batch)
}

func (store *PebbleStore) StateRoot(ctx context.Context, height types.Height, namespace string) (StateRootRecord, error) {
	select {
	case <-ctx.Done():
		return StateRootRecord{}, ctx.Err()
	default:
	}
	if height == 0 || namespace == "" {
		return StateRootRecord{}, ErrInvalidStateRoot
	}

	encoded, err := getValue(store.db, stateRootKey(height, namespace))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return StateRootRecord{}, ErrStateRootNotFound
		}
		return StateRootRecord{}, err
	}

	record, err := decodeStateRootRecord(encoded)
	if err != nil {
		return StateRootRecord{}, err
	}
	if record.Height != height || record.Namespace != namespace {
		return StateRootRecord{}, ErrInvalidStateRoot
	}
	return record, nil
}

func (store *PebbleStore) SaveFinalityProof(ctx context.Context, proof FinalityProofRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if proof.Header.Height == 0 ||
		proof.QuorumCert.Height == 0 ||
		proof.Header.Height != proof.QuorumCert.Height ||
		proof.BlockHash == (types.Hash{}) ||
		proof.QuorumCert.BlockHash != proof.BlockHash ||
		!finalityRecordNamesItsHeader(proof) {
		return ErrInvalidFinality
	}
	if err := validateFinalityCommitChainRecord(proof); err != nil {
		return err
	}
	encoded := encodeFinalityProofRecord(proof)
	batch := store.db.NewBatch()
	defer batch.Close()
	batch.Set(finalityHeightKey(proof.Header.Height), encoded, nil)
	batch.Set(finalityLatestKey, encoded, nil)
	return store.writeBatch(batch)
}

func (store *PebbleStore) FinalityProof(ctx context.Context, height types.Height) (FinalityProofRecord, error) {
	select {
	case <-ctx.Done():
		return FinalityProofRecord{}, ctx.Err()
	default:
	}
	if height == 0 {
		return FinalityProofRecord{}, ErrInvalidFinality
	}
	return store.getFinalityProof(finalityHeightKey(height), height)
}

func (store *PebbleStore) LatestFinalityProof(ctx context.Context) (FinalityProofRecord, error) {
	select {
	case <-ctx.Done():
		return FinalityProofRecord{}, ctx.Err()
	default:
	}
	return store.getFinalityProof(finalityLatestKey, 0)
}

func (store *PebbleStore) getFinalityProof(key []byte, expectedHeight types.Height) (FinalityProofRecord, error) {
	encoded, err := getValue(store.db, key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return FinalityProofRecord{}, ErrFinalityNotFound
		}
		return FinalityProofRecord{}, err
	}
	proof, err := decodeFinalityProofRecord(encoded)
	if err != nil {
		return FinalityProofRecord{}, err
	}
	if proof.Header.Height == 0 ||
		proof.QuorumCert.Height != proof.Header.Height ||
		proof.BlockHash == (types.Hash{}) ||
		proof.QuorumCert.BlockHash != proof.BlockHash ||
		!finalityRecordNamesItsHeader(proof) {
		return FinalityProofRecord{}, ErrInvalidFinality
	}
	if err := validateFinalityCommitChainRecord(proof); err != nil {
		return FinalityProofRecord{}, err
	}
	if expectedHeight != 0 && proof.Header.Height != expectedHeight {
		return FinalityProofRecord{}, ErrInvalidFinality
	}
	return proof, nil
}

// finalityRecordNamesItsHeader reports whether the record's certificate is of the view the header
// was proposed in and its validator set fields repeat the set the header names.
func finalityRecordNamesItsHeader(proof FinalityProofRecord) bool {
	return proof.QuorumCert.Round == proof.Header.Round &&
		proof.ValidatorSetHeight == proof.Header.ValidatorSetHeight &&
		proof.ValidatorSetHash == proof.Header.ValidatorSetHash
}

// validateFinalityCommitChainRecord checks the shape of the 2-chain commit link of a record. A record
// without a link is a certified block still waiting for its child; a record with a link holds
// exactly one: the child of the block, justified by the block's certificate and certified by its own
// quorum certificate in the view it was proposed in, directly after the view of the block's
// certificate. The child may name another validator set than the block: the set changes where the
// chain proves a commit, which finality.Verifier checks together with the signatures.
func validateFinalityCommitChainRecord(proof FinalityProofRecord) error {
	if len(proof.CommitChain) == 0 {
		return nil
	}
	if len(proof.CommitChain) != 1 {
		return ErrInvalidFinality
	}
	child := proof.CommitChain[0]
	if child.BlockHash == (types.Hash{}) ||
		child.QuorumCert.Height == 0 ||
		child.QuorumCert.Height != child.Header.Height ||
		child.QuorumCert.BlockHash != child.BlockHash ||
		proof.QuorumCert.Round == ^types.Round(0) ||
		child.QuorumCert.Round != proof.QuorumCert.Round+1 ||
		child.Header.ChainID != proof.Header.ChainID ||
		child.Header.Height != proof.Header.Height+1 ||
		child.Header.PreviousBlockHash != proof.BlockHash ||
		child.Header.JustifyRound != proof.Header.Round ||
		child.QuorumCert.Round != child.Header.Round {
		return ErrInvalidFinality
	}
	return nil
}

// Set, Delete and SetBatch write outside a block commit. Once the chain has a
// committed state their writes take effect at the next committed height and are
// versioned like a block's writes; before it they form the base state.
func (store *PebbleStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	return store.applyDirectWrites(ctx, []KVWrite{{Namespace: namespace, Key: key, Value: value}})
}

func (store *PebbleStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	if len(key) == 0 {
		return nil, ErrInvalidKey
	}

	value, err := getValue(store.db, kvKey(namespace, key))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return append([]byte(nil), value...), nil
}

func (store *PebbleStore) Delete(ctx context.Context, namespace string, key []byte) error {
	return store.applyDirectWrites(ctx, []KVWrite{{Namespace: namespace, Key: key, Delete: true}})
}

func (store *PebbleStore) SetBatch(ctx context.Context, writes []KVWrite) error {
	return store.applyDirectWrites(ctx, writes)
}

func (store *PebbleStore) ExportNamespace(ctx context.Context, namespace string) ([]KVPair, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	prefix := kvNamespacePrefix(namespace)
	iterator := newIter(store.db, prefixRange(prefix))
	defer iterator.Close()

	pairs := make([]KVPair, 0)
	for valid := iterator.First(); valid; valid = iterator.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		rawKey := iterator.Key()
		rawValue := iterator.Value()
		pairs = append(pairs, KVPair{
			Namespace: namespace,
			Key:       append([]byte(nil), rawKey[len(prefix):]...),
			Value:     append([]byte(nil), rawValue...),
		})
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return pairs, nil
}

func (store *PebbleStore) ExportPrefix(ctx context.Context, namespace string, keyPrefix []byte) ([]KVPair, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	prefix := append(kvNamespacePrefix(namespace), keyPrefix...)
	iterator := newIter(store.db, prefixRange(prefix))
	defer iterator.Close()

	namespacePrefix := kvNamespacePrefix(namespace)
	pairs := make([]KVPair, 0)
	for valid := iterator.First(); valid; valid = iterator.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		rawKey := iterator.Key()
		rawValue := iterator.Value()
		pairs = append(pairs, KVPair{
			Namespace: namespace,
			Key:       append([]byte(nil), rawKey[len(namespacePrefix):]...),
			Value:     append([]byte(nil), rawValue...),
		})
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return pairs, nil
}

// ImportNamespace replaces namespace with pairs, which become its base state:
// the namespace's versions are dropped with its old pairs.
func (store *PebbleStore) ImportNamespace(ctx context.Context, namespace string, pairs []KVPair) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if namespace == "" {
		return ErrInvalidNamespace
	}
	store.writeMu.Lock()
	defer store.writeMu.Unlock()
	batch := store.db.NewBatch()
	defer batch.Close()
	if err := store.deleteNamespaceHistory(ctx, batch, namespace); err != nil {
		return err
	}
	if err := store.replaceStateTree(ctx, batch, namespace, pairs); err != nil {
		return err
	}
	prefix := kvNamespacePrefix(namespace)
	iterator := newIter(store.db, prefixRange(prefix))
	for valid := iterator.First(); valid; valid = iterator.Next() {
		select {
		case <-ctx.Done():
			iterator.Close()
			return ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), iterator.Key()...), nil)
	}
	if err := iterator.Error(); err != nil {
		iterator.Close()
		return err
	}
	iterator.Close()
	for _, pair := range pairs {
		if pair.Namespace != "" && pair.Namespace != namespace {
			return ErrInvalidNamespace
		}
		if len(pair.Key) == 0 {
			return ErrInvalidKey
		}
		batch.Set(kvKey(namespace, pair.Key), append([]byte(nil), pair.Value...), nil)
	}
	return store.writeBatch(batch)
}

func (store *PebbleStore) RecoverIndexes(ctx context.Context) (RecoverResult, error) {
	select {
	case <-ctx.Done():
		return RecoverResult{}, ctx.Err()
	default:
	}
	var result RecoverResult
	blockIndex, err := store.rebuildBlockIndex(ctx)
	if err != nil {
		return RecoverResult{}, err
	}
	evidenceKeys, err := store.evidenceKeys(ctx)
	if err != nil {
		return RecoverResult{}, err
	}
	batch := store.db.NewBatch()
	defer batch.Close()
	if blockIndex.TotalBlocks == 0 {
		batch.Delete(blockIndexKey, nil)
	} else {
		encoded := encodeBlockIndex(blockIndex)
		batch.Set(blockIndexKey, encoded, nil)
		result.RecoveredIndexes++
	}
	if err := store.writeBatch(batch); err != nil {
		return RecoverResult{}, err
	}
	result.BlockIndexKeys = blockIndex.TotalBlocks
	result.EvidenceKeys = uint64(len(evidenceKeys))
	result.EarliestHeight = blockIndex.EarliestHeight
	result.LatestHeight = blockIndex.LatestHeight
	return result, nil
}

func (store *PebbleStore) Compact(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return store.compactAll(ctx)
}

// Close closes the database. Closing a closed store does nothing: pebble panics on a second
// close, and the runtime and the commands that open a store may both close it.
func (store *PebbleStore) Close() error {
	var err error
	store.closeOnce.Do(func() { err = store.db.Close() })
	return err
}

func (store *PebbleStore) getBlock(key []byte) (BlockRecord, error) {
	encoded, err := getValue(store.db, key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return BlockRecord{}, ErrBlockNotFound
		}
		return BlockRecord{}, err
	}

	record, err := decodeBlockRecord(encoded)
	if err != nil {
		return BlockRecord{}, err
	}
	if record.Block.Header.Height == 0 || record.Hash == (types.Hash{}) {
		return BlockRecord{}, ErrInvalidBlockRecord
	}
	return record, nil
}

// pruneStateRootsBelow drops the state roots of the heights below retainFrom; root keys start
// with their big-endian height, so only the pruned keys are visited.
func (store *PebbleStore) pruneStateRootsBelow(ctx context.Context, batch *pebble.Batch, retainFrom types.Height) (uint64, error) {
	return store.pruneHeightKeysBelow(ctx, batch, stateRootPrefix, retainFrom)
}

// pruneStatesBelow drops the state records of the heights below retainFrom.
func (store *PebbleStore) pruneStatesBelow(ctx context.Context, batch *pebble.Batch, retainFrom types.Height) (uint64, error) {
	return store.pruneHeightKeysBelow(ctx, batch, stateHeightPrefix, retainFrom)
}

// pruneHeightKeysBelow drops the keys under prefix whose big-endian height that follows the prefix
// is below retainFrom.
func (store *PebbleStore) pruneHeightKeysBelow(ctx context.Context, batch *pebble.Batch, prefix []byte, retainFrom types.Height) (uint64, error) {
	limit := binary.BigEndian.AppendUint64(append([]byte(nil), prefix...), uint64(retainFrom))
	iterator := newIter(store.db, keyRange{lower: prefix, upper: limit})
	defer iterator.Close()

	var pruned uint64
	for valid := iterator.First(); valid; valid = iterator.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		if len(iterator.Key()) < len(prefix)+8 {
			continue
		}
		batch.Delete(append([]byte(nil), iterator.Key()...), nil)
		pruned++
	}
	if err := iterator.Error(); err != nil {
		return 0, err
	}
	return pruned, nil
}

// indexAfterPrune returns the block index once the blocks below retainFrom, pruned of them, are
// gone: the earliest block is the first one from retainFrom on, found by visiting only the heights
// up to it.
func (store *PebbleStore) indexAfterPrune(ctx context.Context, index BlockIndex, retainFrom types.Height, pruned uint64) (BlockIndex, error) {
	if pruned >= index.TotalBlocks {
		return BlockIndex{}, nil
	}
	next := BlockIndex{LatestHeight: index.LatestHeight, TotalBlocks: index.TotalBlocks - pruned}
	for height := max(retainFrom, index.EarliestHeight); height <= index.LatestHeight; height++ {
		select {
		case <-ctx.Done():
			return BlockIndex{}, ctx.Err()
		default:
		}
		found, err := hasKey(store.db, blockHeightKey(height))
		if err != nil {
			return BlockIndex{}, err
		}
		if found {
			next.EarliestHeight = height
			return next, nil
		}
	}
	return BlockIndex{}, nil
}

func (store *PebbleStore) nextBlockIndex(height types.Height) (BlockIndex, error) {
	encoded, err := getValue(store.db, blockIndexKey)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return BlockIndex{EarliestHeight: height, LatestHeight: height, TotalBlocks: 1}, nil
		}
		return BlockIndex{}, err
	}

	index, err := decodeBlockIndex(encoded)
	if err != nil {
		return BlockIndex{}, err
	}
	if index.EarliestHeight == 0 || height < index.EarliestHeight {
		index.EarliestHeight = height
	}
	if height > index.LatestHeight {
		index.LatestHeight = height
	}
	if _, err := getValue(store.db, blockHeightKey(height)); errors.Is(err, pebble.ErrNotFound) {
		index.TotalBlocks++
	} else if err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

func (store *PebbleStore) rebuildBlockIndex(ctx context.Context) (BlockIndex, error) {
	iterator := newIter(store.db, prefixRange(blockHeightPrefix))
	defer iterator.Close()
	var index BlockIndex
	for ok := iterator.First(); ok; ok = iterator.Next() {
		select {
		case <-ctx.Done():
			return BlockIndex{}, ctx.Err()
		default:
		}
		record, err := decodeBlockRecord(iterator.Value())
		if err != nil {
			return BlockIndex{}, err
		}
		height := record.Block.Header.Height
		if height == 0 || record.Hash == (types.Hash{}) {
			return BlockIndex{}, ErrInvalidBlockRecord
		}
		keyHeight, ok := blockHeightFromKey(iterator.Key())
		if !ok || keyHeight != height {
			return BlockIndex{}, ErrInvalidBlockRecord
		}
		if index.TotalBlocks == 0 || height < index.EarliestHeight {
			index.EarliestHeight = height
		}
		if height > index.LatestHeight {
			index.LatestHeight = height
		}
		index.TotalBlocks++
	}
	if err := iterator.Error(); err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

func stateProofPairs(pairs []KVPair) []stateproof.Pair {
	proofPairs := make([]stateproof.Pair, 0, len(pairs))
	for _, pair := range pairs {
		proofPairs = append(proofPairs, stateproof.Pair{
			Key:   append([]byte(nil), pair.Key...),
			Value: append([]byte(nil), pair.Value...),
		})
	}
	return proofPairs
}
