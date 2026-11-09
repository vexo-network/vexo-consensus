package store

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	leveldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
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

type LevelDBStore struct {
	db           *leveldb.DB
	writeOptions *opt.WriteOptions
	// writeMu orders the writes that maintain state history: block commits,
	// direct writes, imports and pruning each read the history they extend.
	writeMu sync.Mutex
}

type LevelDBOptions struct {
	SyncWrites bool
}

type kvHistoryRecord struct {
	Height    types.Height `json:"height"`
	Namespace string       `json:"namespace"`
	Key       []byte       `json:"key"`
	Value     []byte       `json:"value,omitempty"`
	Deleted   bool         `json:"deleted,omitempty"`
}

func OpenLevelDB(path string) (*LevelDBStore, error) {
	return OpenLevelDBWithOptions(path, LevelDBOptions{SyncWrites: true})
}

// levelDBWriteBuffer is the size of the memtable. A block commit is one batch holding the whole
// block, so the memtable must hold several: a batch larger than it bypasses the memtable and is
// written as level-0 tables in the commit itself, and a small memtable flushes on nearly every
// block, stalling writes behind level-0 compaction.
const levelDBWriteBuffer = 64 << 20

func OpenLevelDBWithOptions(path string, options LevelDBOptions) (*LevelDBStore, error) {
	db, err := leveldb.OpenFile(path, &opt.Options{
		WriteBuffer:         levelDBWriteBuffer,
		CompactionTableSize: 8 << 20,
	})
	if err != nil {
		return nil, err
	}
	return &LevelDBStore{
		db:           db,
		writeOptions: &opt.WriteOptions{Sync: options.SyncWrites},
	}, nil
}

func (store *LevelDBStore) writeBatch(batch *leveldb.Batch) error {
	return store.db.Write(batch, store.writeOptions)
}

func (store *LevelDBStore) put(key []byte, value []byte) error {
	return store.db.Put(key, value, store.writeOptions)
}

func (store *LevelDBStore) delete(key []byte) error {
	return store.db.Delete(key, store.writeOptions)
}

func (store *LevelDBStore) IsNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

func (store *LevelDBStore) SaveBlock(ctx context.Context, record BlockRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if record.Block.Header.Height == 0 || record.Hash == (types.Hash{}) {
		return ErrInvalidBlockRecord
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	batch.Put(blockHeightKey(record.Block.Header.Height), encoded)
	batch.Put(blockHashKey(record.Hash), blockHashValue(record.Block.Header.Height))
	index, err := store.nextBlockIndex(record.Block.Header.Height)
	if err != nil {
		return err
	}
	encodedIndex, err := json.Marshal(index)
	if err != nil {
		return err
	}
	batch.Put(blockIndexKey, encodedIndex)
	return store.writeBatch(batch)
}

func (store *LevelDBStore) BlockByHeight(ctx context.Context, height types.Height) (BlockRecord, error) {
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

func (store *LevelDBStore) BlockByHash(ctx context.Context, hash types.Hash) (BlockRecord, error) {
	select {
	case <-ctx.Done():
		return BlockRecord{}, ctx.Err()
	default:
	}
	encodedHeight, err := store.db.Get(blockHashKey(hash), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
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

func (store *LevelDBStore) BlockIndex(ctx context.Context) (BlockIndex, error) {
	select {
	case <-ctx.Done():
		return BlockIndex{}, ctx.Err()
	default:
	}

	encoded, err := store.db.Get(blockIndexKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return BlockIndex{}, ErrBlockIndexNotFound
		}
		return BlockIndex{}, err
	}

	var index BlockIndex
	if err := json.Unmarshal(encoded, &index); err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

func (store *LevelDBStore) PruneBelow(ctx context.Context, retainFrom types.Height) (PruneResult, error) {
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
	batch := new(leveldb.Batch)

	for height := index.EarliestHeight; height < retainFrom && height <= index.LatestHeight; height++ {
		record, err := store.getBlock(blockHeightKey(height))
		if errors.Is(err, ErrBlockNotFound) {
			continue
		}
		if err != nil {
			return PruneResult{}, err
		}
		batch.Delete(blockHeightKey(height))
		batch.Delete(blockHashKey(record.Hash))
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
		batch.Delete(blockIndexKey)
	} else {
		encodedIndex, err := json.Marshal(newIndex)
		if err != nil {
			return PruneResult{}, err
		}
		batch.Put(blockIndexKey, encodedIndex)
	}
	if err := store.writeBatch(batch); err != nil {
		return PruneResult{}, err
	}
	return result, nil
}

func (store *LevelDBStore) PruneByRetention(ctx context.Context, policy RetentionPolicy) (PruneResult, error) {
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

func (store *LevelDBStore) SaveState(ctx context.Context, state StateRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if state.Height == 0 {
		return ErrInvalidStateRecord
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	batch.Put(stateLatestKey, encoded)
	batch.Put(stateHeightKey(state.Height), encoded)
	return store.writeBatch(batch)
}

func (store *LevelDBStore) SaveSchemaState(ctx context.Context, state upgrade.State) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return store.put(schemaStateKey, encoded)
}

func (store *LevelDBStore) SchemaState(ctx context.Context) (upgrade.State, error) {
	select {
	case <-ctx.Done():
		return upgrade.State{}, ctx.Err()
	default:
	}
	encoded, err := store.db.Get(schemaStateKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return upgrade.State{}, ErrStateNotFound
		}
		return upgrade.State{}, err
	}
	var state upgrade.State
	if err := json.Unmarshal(encoded, &state); err != nil {
		return upgrade.State{}, err
	}
	return state, nil
}

func (store *LevelDBStore) SaveUpgradePlan(ctx context.Context, plan upgrade.Plan) error {
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
	encoded, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	batch.Put(upgradePlanHeightKey(plan.Height), encoded)
	batch.Put(upgradePlanNameKey(plan.Name), encoded)
	return store.writeBatch(batch)
}

// currentHeight is the height of the latest committed state, zero before the first block.
func (store *LevelDBStore) currentHeight(ctx context.Context) (types.Height, error) {
	state, err := store.LatestState(ctx)
	if errors.Is(err, ErrStateNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return state.Height, nil
}

func (store *LevelDBStore) UpgradePlanByHeight(ctx context.Context, height types.Height) (upgrade.Plan, bool, error) {
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

func (store *LevelDBStore) UpgradePlanByName(ctx context.Context, name string) (upgrade.Plan, bool, error) {
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
func (store *LevelDBStore) NextUpgradePlan(ctx context.Context, height types.Height) (upgrade.Plan, bool, error) {
	select {
	case <-ctx.Done():
		return upgrade.Plan{}, false, ctx.Err()
	default:
	}
	heights := util.BytesPrefix(upgradePlanHeightPrefix)
	heights.Start = upgradePlanHeightKey(height)
	iterator := store.db.NewIterator(heights, nil)
	defer iterator.Release()
	if !iterator.Next() {
		return upgrade.Plan{}, false, iterator.Error()
	}
	return decodeUpgradePlan(iterator.Value())
}

func (store *LevelDBStore) upgradePlan(key []byte) (upgrade.Plan, bool, error) {
	encoded, err := store.db.Get(key, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return upgrade.Plan{}, false, nil
		}
		return upgrade.Plan{}, false, err
	}
	return decodeUpgradePlan(encoded)
}

func decodeUpgradePlan(encoded []byte) (upgrade.Plan, bool, error) {
	var plan upgrade.Plan
	if err := json.Unmarshal(encoded, &plan); err != nil {
		return upgrade.Plan{}, false, err
	}
	if err := upgrade.ValidatePlan(plan); err != nil {
		return upgrade.Plan{}, false, err
	}
	return plan, true, nil
}

func (store *LevelDBStore) LatestState(ctx context.Context) (StateRecord, error) {
	select {
	case <-ctx.Done():
		return StateRecord{}, ctx.Err()
	default:
	}

	encoded, err := store.db.Get(stateLatestKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return StateRecord{}, ErrStateNotFound
		}
		return StateRecord{}, err
	}

	var state StateRecord
	if err := json.Unmarshal(encoded, &state); err != nil {
		return StateRecord{}, err
	}
	if state.Height == 0 {
		return StateRecord{}, ErrInvalidStateRecord
	}
	return state, nil
}

func (store *LevelDBStore) StateByHeight(ctx context.Context, height types.Height) (StateRecord, error) {
	select {
	case <-ctx.Done():
		return StateRecord{}, ctx.Err()
	default:
	}
	if height == 0 {
		return StateRecord{}, ErrInvalidStateRecord
	}
	encoded, err := store.db.Get(stateHeightKey(height), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return StateRecord{}, ErrStateNotFound
		}
		return StateRecord{}, err
	}
	var state StateRecord
	if err := json.Unmarshal(encoded, &state); err != nil {
		return StateRecord{}, err
	}
	if state.Height != height {
		return StateRecord{}, ErrInvalidStateRecord
	}
	return state, nil
}

func (store *LevelDBStore) SaveStateRoot(ctx context.Context, record StateRootRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if record.Height == 0 || record.Namespace == "" {
		return ErrInvalidStateRoot
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return store.put(stateRootKey(record.Height, record.Namespace), encoded)
}

func (store *LevelDBStore) CommitBlockState(ctx context.Context, block BlockRecord, state StateRecord, roots []StateRootRecord) error {
	return store.commitBlockStateBatch(ctx, nil, block, state, roots)
}

func (store *LevelDBStore) CommitBlockStateWithWrites(ctx context.Context, writes []KVWrite, block BlockRecord, state StateRecord, roots []StateRootRecord) error {
	return store.commitBlockStateBatch(ctx, writes, block, state, roots)
}

func (store *LevelDBStore) commitBlockStateBatch(ctx context.Context, writes []KVWrite, block BlockRecord, state StateRecord, roots []StateRootRecord) error {
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
	encodedBlock, err := json.Marshal(block)
	if err != nil {
		return err
	}
	index, err := store.nextBlockIndex(block.Block.Header.Height)
	if err != nil {
		return err
	}
	encodedIndex, err := json.Marshal(index)
	if err != nil {
		return err
	}
	encodedState, err := json.Marshal(state)
	if err != nil {
		return err
	}

	store.writeMu.Lock()
	defer store.writeMu.Unlock()
	batch := new(leveldb.Batch)
	history := newHistoryBatch(store, batch, state.Height)
	trees := newStateTrees(store.db)
	for _, write := range writes {
		key := kvKey(write.Namespace, write.Key)
		if err := history.record(write); err != nil {
			return err
		}
		if err := trees.apply(write); err != nil {
			return err
		}
		if write.Delete {
			batch.Delete(key)
			continue
		}
		batch.Put(key, append([]byte(nil), write.Value...))
	}
	if err := trees.persist(batch, true, state.Height); err != nil {
		return err
	}
	batch.Put(blockHeightKey(block.Block.Header.Height), encodedBlock)
	batch.Put(blockHashKey(block.Hash), blockHashValue(block.Block.Header.Height))
	batch.Put(blockIndexKey, encodedIndex)
	batch.Put(stateLatestKey, encodedState)
	batch.Put(stateHeightKey(state.Height), encodedState)
	for _, root := range roots {
		encodedRoot, err := json.Marshal(root)
		if err != nil {
			return err
		}
		batch.Put(stateRootKey(root.Height, root.Namespace), encodedRoot)
	}
	return store.writeBatch(batch)
}

func (store *LevelDBStore) StateRoot(ctx context.Context, height types.Height, namespace string) (StateRootRecord, error) {
	select {
	case <-ctx.Done():
		return StateRootRecord{}, ctx.Err()
	default:
	}
	if height == 0 || namespace == "" {
		return StateRootRecord{}, ErrInvalidStateRoot
	}

	encoded, err := store.db.Get(stateRootKey(height, namespace), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return StateRootRecord{}, ErrStateRootNotFound
		}
		return StateRootRecord{}, err
	}

	var record StateRootRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return StateRootRecord{}, err
	}
	if record.Height != height || record.Namespace != namespace {
		return StateRootRecord{}, ErrInvalidStateRoot
	}
	return record, nil
}

func (store *LevelDBStore) SaveFinalityProof(ctx context.Context, proof FinalityProofRecord) error {
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
	encoded, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	batch := new(leveldb.Batch)
	batch.Put(finalityHeightKey(proof.Header.Height), encoded)
	batch.Put(finalityLatestKey, encoded)
	return store.writeBatch(batch)
}

func (store *LevelDBStore) FinalityProof(ctx context.Context, height types.Height) (FinalityProofRecord, error) {
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

func (store *LevelDBStore) LatestFinalityProof(ctx context.Context) (FinalityProofRecord, error) {
	select {
	case <-ctx.Done():
		return FinalityProofRecord{}, ctx.Err()
	default:
	}
	return store.getFinalityProof(finalityLatestKey, 0)
}

func (store *LevelDBStore) getFinalityProof(key []byte, expectedHeight types.Height) (FinalityProofRecord, error) {
	encoded, err := store.db.Get(key, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return FinalityProofRecord{}, ErrFinalityNotFound
		}
		return FinalityProofRecord{}, err
	}
	var proof FinalityProofRecord
	if err := json.Unmarshal(encoded, &proof); err != nil {
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
func (store *LevelDBStore) Set(ctx context.Context, namespace string, key []byte, value []byte) error {
	return store.applyDirectWrites(ctx, []KVWrite{{Namespace: namespace, Key: key, Value: value}})
}

func (store *LevelDBStore) Get(ctx context.Context, namespace string, key []byte) ([]byte, error) {
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

	value, err := store.db.Get(kvKey(namespace, key), nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return append([]byte(nil), value...), nil
}

func (store *LevelDBStore) Delete(ctx context.Context, namespace string, key []byte) error {
	return store.applyDirectWrites(ctx, []KVWrite{{Namespace: namespace, Key: key, Delete: true}})
}

func (store *LevelDBStore) SetBatch(ctx context.Context, writes []KVWrite) error {
	return store.applyDirectWrites(ctx, writes)
}

func (store *LevelDBStore) ExportNamespace(ctx context.Context, namespace string) ([]KVPair, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	prefix := kvNamespacePrefix(namespace)
	iterator := store.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iterator.Release()

	pairs := make([]KVPair, 0)
	for iterator.Next() {
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

func (store *LevelDBStore) ExportPrefix(ctx context.Context, namespace string, keyPrefix []byte) ([]KVPair, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if namespace == "" {
		return nil, ErrInvalidNamespace
	}
	prefix := append(kvNamespacePrefix(namespace), keyPrefix...)
	iterator := store.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iterator.Release()

	namespacePrefix := kvNamespacePrefix(namespace)
	pairs := make([]KVPair, 0)
	for iterator.Next() {
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
func (store *LevelDBStore) ImportNamespace(ctx context.Context, namespace string, pairs []KVPair) error {
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
	batch := new(leveldb.Batch)
	if err := store.deleteNamespaceHistory(ctx, batch, namespace); err != nil {
		return err
	}
	if err := store.replaceStateTree(ctx, batch, namespace, pairs); err != nil {
		return err
	}
	prefix := kvNamespacePrefix(namespace)
	iterator := store.db.NewIterator(util.BytesPrefix(prefix), nil)
	for iterator.Next() {
		select {
		case <-ctx.Done():
			iterator.Release()
			return ctx.Err()
		default:
		}
		batch.Delete(append([]byte(nil), iterator.Key()...))
	}
	if err := iterator.Error(); err != nil {
		iterator.Release()
		return err
	}
	iterator.Release()
	for _, pair := range pairs {
		if pair.Namespace != "" && pair.Namespace != namespace {
			return ErrInvalidNamespace
		}
		if len(pair.Key) == 0 {
			return ErrInvalidKey
		}
		batch.Put(kvKey(namespace, pair.Key), append([]byte(nil), pair.Value...))
	}
	return store.writeBatch(batch)
}

func (store *LevelDBStore) RecoverIndexes(ctx context.Context) (RecoverResult, error) {
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
	batch := new(leveldb.Batch)
	if blockIndex.TotalBlocks == 0 {
		batch.Delete(blockIndexKey)
	} else {
		encoded, err := json.Marshal(blockIndex)
		if err != nil {
			return RecoverResult{}, err
		}
		batch.Put(blockIndexKey, encoded)
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

func (store *LevelDBStore) Compact(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return store.db.CompactRange(util.Range{})
}

func (store *LevelDBStore) Close() error {
	return store.db.Close()
}

func (store *LevelDBStore) getBlock(key []byte) (BlockRecord, error) {
	encoded, err := store.db.Get(key, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return BlockRecord{}, ErrBlockNotFound
		}
		return BlockRecord{}, err
	}

	var record BlockRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		return BlockRecord{}, err
	}
	if record.Block.Header.Height == 0 || record.Hash == (types.Hash{}) {
		return BlockRecord{}, ErrInvalidBlockRecord
	}
	return record, nil
}

// pruneStateRootsBelow drops the state roots of the heights below retainFrom; root keys start
// with their big-endian height, so only the pruned keys are visited.
func (store *LevelDBStore) pruneStateRootsBelow(ctx context.Context, batch *leveldb.Batch, retainFrom types.Height) (uint64, error) {
	return store.pruneHeightKeysBelow(ctx, batch, stateRootPrefix, retainFrom)
}

// pruneStatesBelow drops the state records of the heights below retainFrom.
func (store *LevelDBStore) pruneStatesBelow(ctx context.Context, batch *leveldb.Batch, retainFrom types.Height) (uint64, error) {
	return store.pruneHeightKeysBelow(ctx, batch, stateHeightPrefix, retainFrom)
}

// pruneHeightKeysBelow drops the keys under prefix whose big-endian height that follows the prefix
// is below retainFrom.
func (store *LevelDBStore) pruneHeightKeysBelow(ctx context.Context, batch *leveldb.Batch, prefix []byte, retainFrom types.Height) (uint64, error) {
	limit := binary.BigEndian.AppendUint64(append([]byte(nil), prefix...), uint64(retainFrom))
	iterator := store.db.NewIterator(&util.Range{Start: prefix, Limit: limit}, nil)
	defer iterator.Release()

	var pruned uint64
	for iterator.Next() {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		if len(iterator.Key()) < len(prefix)+8 {
			continue
		}
		batch.Delete(append([]byte(nil), iterator.Key()...))
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
func (store *LevelDBStore) indexAfterPrune(ctx context.Context, index BlockIndex, retainFrom types.Height, pruned uint64) (BlockIndex, error) {
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
		found, err := store.db.Has(blockHeightKey(height), nil)
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

func (store *LevelDBStore) nextBlockIndex(height types.Height) (BlockIndex, error) {
	encoded, err := store.db.Get(blockIndexKey, nil)
	if err != nil {
		if errors.Is(err, leveldberrors.ErrNotFound) {
			return BlockIndex{EarliestHeight: height, LatestHeight: height, TotalBlocks: 1}, nil
		}
		return BlockIndex{}, err
	}

	var index BlockIndex
	if err := json.Unmarshal(encoded, &index); err != nil {
		return BlockIndex{}, err
	}
	if index.EarliestHeight == 0 || height < index.EarliestHeight {
		index.EarliestHeight = height
	}
	if height > index.LatestHeight {
		index.LatestHeight = height
	}
	if _, err := store.db.Get(blockHeightKey(height), nil); errors.Is(err, leveldberrors.ErrNotFound) {
		index.TotalBlocks++
	} else if err != nil {
		return BlockIndex{}, err
	}
	return index, nil
}

func (store *LevelDBStore) rebuildBlockIndex(ctx context.Context) (BlockIndex, error) {
	iterator := store.db.NewIterator(util.BytesPrefix(blockHeightPrefix), nil)
	defer iterator.Release()
	var index BlockIndex
	for ok := iterator.First(); ok; ok = iterator.Next() {
		select {
		case <-ctx.Done():
			return BlockIndex{}, ctx.Err()
		default:
		}
		var record BlockRecord
		if err := json.Unmarshal(iterator.Value(), &record); err != nil {
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
