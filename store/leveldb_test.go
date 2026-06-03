package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
)

func TestLevelDBStoreSavesAndLoadsBlockByHeightAndHash(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	record := BlockRecord{
		Block: types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: 1},
			Txs:    []types.Tx{[]byte("tx")},
		},
		Hash:       types.Hash{1},
		AppHash:    types.Hash{2},
		StateRoots: []StateRootRecord{{Height: 1, Namespace: "bank", Root: types.Hash{3}}},
	}

	if err := store.SaveBlock(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	byHeight, err := store.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if byHeight.Hash != record.Hash || byHeight.AppHash != record.AppHash {
		t.Fatalf("unexpected block by height: %+v", byHeight)
	}
	if len(byHeight.StateRoots) != 1 || byHeight.StateRoots[0].Namespace != "bank" {
		t.Fatalf("unexpected state roots by height: %+v", byHeight.StateRoots)
	}

	byHash, err := store.BlockByHash(context.Background(), record.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if byHash.Block.Header.Height != 1 {
		t.Fatalf("expected height 1, got %d", byHash.Block.Header.Height)
	}
	if len(byHash.StateRoots) != 1 || byHash.StateRoots[0].Root != (types.Hash{3}) {
		t.Fatalf("unexpected state roots by hash: %+v", byHash.StateRoots)
	}
	index, err := store.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 1 || index.LatestHeight != 1 || index.TotalBlocks != 1 {
		t.Fatalf("unexpected block index: %+v", index)
	}
}

func TestLevelDBStoreCommitBlockStatePersistsBlockStateAndRootsAtomically(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	block := BlockRecord{
		Block:   types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}},
		Hash:    types.Hash{1},
		AppHash: types.Hash{2},
	}
	state := StateRecord{
		Height:           1,
		AppHash:          types.Hash{2},
		LastBlockHash:    types.Hash{1},
		ValidatorSetHash: types.Hash{3},
	}
	roots := []StateRootRecord{{Height: 1, Namespace: "bank", Root: types.Hash{4}}}

	if err := store.CommitBlockState(context.Background(), block, state, roots); err != nil {
		t.Fatal(err)
	}
	savedBlock, err := store.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if savedBlock.Hash != block.Hash || savedBlock.AppHash != block.AppHash || len(savedBlock.StateRoots) != 1 {
		t.Fatalf("unexpected block record: %+v", savedBlock)
	}
	savedState, err := store.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if savedState.AppHash != state.AppHash || savedState.ValidatorSetHash != state.ValidatorSetHash {
		t.Fatalf("unexpected state record: %+v", savedState)
	}
	savedRoot, err := store.StateRoot(context.Background(), 1, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if savedRoot.Root != roots[0].Root {
		t.Fatalf("unexpected state root: %+v", savedRoot)
	}
}

func TestLevelDBStorePersistsSchemaState(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	state := upgrade.State{
		Height:              10,
		BinaryVersion:       "v1.2.3",
		ConfigSchemaVersion: 2,
		StoreSchemaVersion:  3,
		AppStateVersion:     4,
	}
	if err := store.SaveSchemaState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.SchemaState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded != state {
		t.Fatalf("unexpected schema state: %+v", loaded)
	}
}

func TestLevelDBStoreBlockIndexTracksRangeAndUniqueBlocks(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	records := []BlockRecord{
		{Block: types.Block{Header: types.Header{Height: 3}}, Hash: types.Hash{3}},
		{Block: types.Block{Header: types.Header{Height: 1}}, Hash: types.Hash{1}},
		{Block: types.Block{Header: types.Header{Height: 3}}, Hash: types.Hash{9}},
	}
	for _, record := range records {
		if err := store.SaveBlock(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}

	index, err := store.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 1 || index.LatestHeight != 3 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected block index: %+v", index)
	}
}

func TestLevelDBStorePruneBelow(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	for height := types.Height(1); height <= 4; height++ {
		record := BlockRecord{
			Block: types.Block{Header: types.Header{Height: height}},
			Hash:  types.Hash{byte(height)},
		}
		if err := store.SaveBlock(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveStateRoot(context.Background(), StateRootRecord{Height: height, Namespace: "bank", Root: types.Hash{byte(height)}}); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveState(context.Background(), StateRecord{Height: height, AppHash: types.Hash{byte(height)}}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := store.PruneBelow(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.PrunedBlocks != 2 || result.PrunedStates != 2 || result.PrunedStateRoots != 2 {
		t.Fatalf("unexpected prune result: %+v", result)
	}
	if _, err := store.BlockByHeight(context.Background(), 1); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected pruned block not found, got %v", err)
	}
	if _, err := store.StateRoot(context.Background(), 2, "bank"); !errors.Is(err, ErrStateRootNotFound) {
		t.Fatalf("expected pruned state root not found, got %v", err)
	}
	if _, err := store.StateByHeight(context.Background(), 2); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("expected pruned state not found, got %v", err)
	}
	if _, err := store.BlockByHeight(context.Background(), 3); err != nil {
		t.Fatalf("expected retained block, got %v", err)
	}
	if _, err := store.StateByHeight(context.Background(), 3); err != nil {
		t.Fatalf("expected retained state, got %v", err)
	}
	index, err := store.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 3 || index.LatestHeight != 4 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected index after prune: %+v", index)
	}
}

func TestLevelDBStorePruneByRetention(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	for height := types.Height(1); height <= 5; height++ {
		if err := store.SaveBlock(context.Background(), BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height)}}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := store.PruneByRetention(context.Background(), RetentionPolicy{RetainRecent: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.RetainFromHeight != 4 || result.PrunedBlocks != 3 {
		t.Fatalf("unexpected retention prune result: %+v", result)
	}
	index, err := store.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 4 || index.LatestHeight != 5 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected retained index: %+v", index)
	}
}

func TestLevelDBStorePruneAllRemovesIndex(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.SaveBlock(context.Background(), BlockRecord{Block: types.Block{Header: types.Header{Height: 1}}, Hash: types.Hash{1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PruneBelow(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BlockIndex(context.Background()); !errors.Is(err, ErrBlockIndexNotFound) {
		t.Fatalf("expected block index not found, got %v", err)
	}
}

func TestLevelDBStorePersistsAcrossReopen(t *testing.T) {
	path := t.TempDir()
	store, err := OpenLevelDB(path)
	if err != nil {
		t.Fatal(err)
	}

	record := BlockRecord{
		Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: 7}},
		Hash:  types.Hash{7},
	}
	if err := store.SaveBlock(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenLevelDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, reopened)

	loaded, err := reopened.BlockByHeight(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Hash != record.Hash {
		t.Fatalf("expected persisted hash %x, got %x", record.Hash, loaded.Hash)
	}
}

func TestLevelDBStoreSavesAndLoadsLatestState(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	state := StateRecord{
		Height:           3,
		AppHash:          types.Hash{3},
		LastBlockHash:    types.Hash{4},
		ValidatorSetHash: types.Hash{5},
	}
	if err := store.SaveState(context.Background(), state); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Height != 3 || loaded.AppHash != state.AppHash || loaded.LastBlockHash != state.LastBlockHash {
		t.Fatalf("unexpected latest state: %+v", loaded)
	}
}

func TestLevelDBStoreRejectsTamperedBlockRecordHeight(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	record := BlockRecord{
		Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}},
		Hash:  types.Hash{1},
	}
	if err := store.SaveBlock(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	record.Block.Header.Height = 2
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.Put(blockHeightKey(1), encoded, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := store.BlockByHeight(context.Background(), 1); !errors.Is(err, ErrInvalidBlockRecord) {
		t.Fatalf("expected invalid block record after tamper, got %v", err)
	}
	if _, err := store.RecoverIndexes(context.Background()); !errors.Is(err, ErrInvalidBlockRecord) {
		t.Fatalf("expected index recovery to reject tampered block, got %v", err)
	}
}

func TestLevelDBStoreRejectsTamperedBlockRecordHash(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	record := BlockRecord{
		Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}},
		Hash:  types.Hash{1},
	}
	if err := store.SaveBlock(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	record.Hash = types.Hash{2}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.Put(blockHashKey(types.Hash{1}), encoded, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := store.BlockByHash(context.Background(), types.Hash{1}); !errors.Is(err, ErrInvalidBlockRecord) {
		t.Fatalf("expected invalid block hash record after tamper, got %v", err)
	}
}

func TestLevelDBStoreRejectsTamperedStateAndRootRecords(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.SaveState(context.Background(), StateRecord{Height: 1, AppHash: types.Hash{1}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStateRoot(context.Background(), StateRootRecord{Height: 1, Namespace: "bank", Root: types.Hash{2}}); err != nil {
		t.Fatal(err)
	}
	encodedState, err := json.Marshal(StateRecord{Height: 2, AppHash: types.Hash{1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.Put(stateHeightKey(1), encodedState, nil); err != nil {
		t.Fatal(err)
	}
	encodedRoot, err := json.Marshal(StateRootRecord{Height: 1, Namespace: "staking", Root: types.Hash{2}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.Put(stateRootKey(1, "bank"), encodedRoot, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := store.StateByHeight(context.Background(), 1); !errors.Is(err, ErrInvalidStateRecord) {
		t.Fatalf("expected invalid state after tamper, got %v", err)
	}
	if _, err := store.StateRoot(context.Background(), 1, "bank"); !errors.Is(err, ErrInvalidStateRoot) {
		t.Fatalf("expected invalid state root after tamper, got %v", err)
	}
}

func TestLevelDBStoreSavesVersionedStateByHeight(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	first := StateRecord{Height: 1, AppHash: types.Hash{1}}
	second := StateRecord{Height: 2, AppHash: types.Hash{2}}
	if err := store.SaveState(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveState(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	latest, err := store.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if latest.Height != 2 {
		t.Fatalf("expected latest state height 2, got %+v", latest)
	}
	loaded, err := store.StateByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AppHash != first.AppHash {
		t.Fatalf("unexpected state by height: %+v", loaded)
	}
	if _, err := store.StateByHeight(context.Background(), 0); !errors.Is(err, ErrInvalidStateRecord) {
		t.Fatalf("expected invalid state height, got %v", err)
	}
}

func TestLevelDBStoreSavesAndLoadsStateRootByHeight(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	record := StateRootRecord{
		Height:    5,
		Namespace: "bank",
		Root:      types.Hash{5},
	}
	if err := store.SaveStateRoot(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.StateRoot(context.Background(), 5, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Height != 5 || loaded.Namespace != "bank" || loaded.Root != (types.Hash{5}) {
		t.Fatalf("unexpected state root record: %+v", loaded)
	}
}

func TestLevelDBStoreStateRootKeepsHeightsSeparate(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.SaveStateRoot(context.Background(), StateRootRecord{Height: 1, Namespace: "bank", Root: types.Hash{1}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStateRoot(context.Background(), StateRootRecord{Height: 2, Namespace: "bank", Root: types.Hash{2}}); err != nil {
		t.Fatal(err)
	}

	first, err := store.StateRoot(context.Background(), 1, "bank")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.StateRoot(context.Background(), 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if first.Root == second.Root {
		t.Fatal("expected different roots for different heights")
	}
}

func TestLevelDBStoreNotFound(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if _, err := store.BlockByHeight(context.Background(), 1); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected block not found by height, got %v", err)
	}
	if _, err := store.BlockByHash(context.Background(), types.Hash{1}); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected block not found by hash, got %v", err)
	}
	if _, err := store.LatestState(context.Background()); !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("expected state not found, got %v", err)
	}
	if _, err := store.Get(context.Background(), "bank", []byte("alice")); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected key not found, got %v", err)
	}
	if _, err := store.StateRoot(context.Background(), 1, "bank"); !errors.Is(err, ErrStateRootNotFound) {
		t.Fatalf("expected state root not found, got %v", err)
	}
	if _, err := store.BlockIndex(context.Background()); !errors.Is(err, ErrBlockIndexNotFound) {
		t.Fatalf("expected block index not found, got %v", err)
	}
	if _, err := store.PruneBelow(context.Background(), 1); !errors.Is(err, ErrBlockIndexNotFound) {
		t.Fatalf("expected prune block index not found, got %v", err)
	}
}

func TestLevelDBStoreRejectsInvalidRecords(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.SaveBlock(context.Background(), BlockRecord{}); !errors.Is(err, ErrInvalidBlockRecord) {
		t.Fatalf("expected invalid block record, got %v", err)
	}
	if err := store.SaveState(context.Background(), StateRecord{}); !errors.Is(err, ErrInvalidStateRecord) {
		t.Fatalf("expected invalid state record, got %v", err)
	}
	if err := store.SaveStateRoot(context.Background(), StateRootRecord{}); !errors.Is(err, ErrInvalidStateRoot) {
		t.Fatalf("expected invalid state root record, got %v", err)
	}
	if _, err := store.StateRoot(context.Background(), 0, "bank"); !errors.Is(err, ErrInvalidStateRoot) {
		t.Fatalf("expected invalid state root lookup, got %v", err)
	}
	if _, err := store.PruneBelow(context.Background(), 0); !errors.Is(err, ErrInvalidPruneHeight) {
		t.Fatalf("expected invalid prune height, got %v", err)
	}
	if _, err := store.PruneByRetention(context.Background(), RetentionPolicy{}); !errors.Is(err, ErrInvalidRetention) {
		t.Fatalf("expected invalid retention, got %v", err)
	}
}

func TestLevelDBStoreContextCancellation(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	record := BlockRecord{Block: types.Block{Header: types.Header{Height: 1}}, Hash: types.Hash{1}}
	if err := store.SaveBlock(ctx, record); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected save block canceled, got %v", err)
	}
	if _, err := store.BlockByHeight(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected block by height canceled, got %v", err)
	}
	if _, err := store.BlockByHash(ctx, types.Hash{1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected block by hash canceled, got %v", err)
	}
	if _, err := store.BlockIndex(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected block index canceled, got %v", err)
	}
	if _, err := store.PruneBelow(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected prune canceled, got %v", err)
	}
	if _, err := store.PruneByRetention(ctx, RetentionPolicy{RetainRecent: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected prune by retention canceled, got %v", err)
	}
	if err := store.SaveState(ctx, StateRecord{Height: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected save state canceled, got %v", err)
	}
	if _, err := store.LatestState(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected latest state canceled, got %v", err)
	}
	if _, err := store.StateByHeight(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected state by height canceled, got %v", err)
	}
	if err := store.SaveStateRoot(ctx, StateRootRecord{Height: 1, Namespace: "bank", Root: types.Hash{1}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected save state root canceled, got %v", err)
	}
	if _, err := store.StateRoot(ctx, 1, "bank"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected state root canceled, got %v", err)
	}
	if err := store.Set(ctx, "bank", []byte("alice"), []byte("1")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected set canceled, got %v", err)
	}
	if _, err := store.Get(ctx, "bank", []byte("alice")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected get canceled, got %v", err)
	}
	if err := store.Delete(ctx, "bank", []byte("alice")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected delete canceled, got %v", err)
	}
	if _, err := store.Root(ctx, "bank"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected root canceled, got %v", err)
	}
	if _, err := store.RecoverIndexes(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected recover indexes canceled, got %v", err)
	}
	if err := store.Compact(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected compact canceled, got %v", err)
	}
}

func TestLevelDBStoreKVSetGetDelete(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.Set(context.Background(), "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	value, err := store.Get(context.Background(), "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "100" {
		t.Fatalf("expected value 100, got %s", value)
	}
	value[0] = '9'
	again, err := store.Get(context.Background(), "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != "100" {
		t.Fatalf("stored value mutated through copy: %s", again)
	}
	if err := store.Delete(context.Background(), "bank", []byte("alice")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "bank", []byte("alice")); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected deleted key not found, got %v", err)
	}
}

func TestLevelDBStoreRootReflectsNamespaceState(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	emptyRoot, err := store.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	firstRoot, err := store.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if emptyRoot == firstRoot {
		t.Fatal("expected root to change after set")
	}

	if err := store.Set(context.Background(), "staking", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	afterOtherNamespace, err := store.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if firstRoot != afterOtherNamespace {
		t.Fatal("other namespace changed bank root")
	}

	if err := store.Set(context.Background(), "bank", []byte("alice"), []byte("200")); err != nil {
		t.Fatal(err)
	}
	updatedRoot, err := store.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if updatedRoot == firstRoot {
		t.Fatal("expected root to change after update")
	}

	if err := store.Delete(context.Background(), "bank", []byte("alice")); err != nil {
		t.Fatal(err)
	}
	deletedRoot, err := store.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if deletedRoot != emptyRoot {
		t.Fatal("expected deleted root to match empty root")
	}
}

func TestLevelDBStoreExportsAndImportsNamespace(t *testing.T) {
	source := openTestStore(t)
	defer closeStore(t, source)
	if err := source.Set(context.Background(), "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	if err := source.Set(context.Background(), "bank", []byte("bob"), []byte("25")); err != nil {
		t.Fatal(err)
	}
	sourceRoot, err := source.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := source.ExportNamespace(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected two exported pairs, got %+v", pairs)
	}

	target := openTestStore(t)
	defer closeStore(t, target)
	if err := target.Set(context.Background(), "bank", []byte("stale"), []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := target.ImportNamespace(context.Background(), "bank", pairs); err != nil {
		t.Fatal(err)
	}
	if _, err := target.Get(context.Background(), "bank", []byte("stale")); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected stale key removed, got %v", err)
	}
	value, err := target.Get(context.Background(), "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "100" {
		t.Fatalf("unexpected imported value %q", value)
	}
	targetRoot, err := target.Root(context.Background(), "bank")
	if err != nil {
		t.Fatal(err)
	}
	if targetRoot != sourceRoot {
		t.Fatalf("expected root %x, got %x", sourceRoot, targetRoot)
	}
}

func TestLevelDBStoreRootRejectsInvalidNamespace(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if _, err := store.Root(context.Background(), ""); !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("expected invalid namespace, got %v", err)
	}
}

func TestLevelDBStoreKVRejectsInvalidNamespaceAndKey(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.Set(context.Background(), "", []byte("key"), []byte("value")); !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("expected invalid namespace, got %v", err)
	}
	if err := store.Set(context.Background(), "bank", nil, []byte("value")); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
	if _, err := store.Get(context.Background(), "", []byte("key")); !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("expected invalid namespace, got %v", err)
	}
	if _, err := store.Get(context.Background(), "bank", nil); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
	if err := store.Delete(context.Background(), "", []byte("key")); !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("expected invalid namespace, got %v", err)
	}
	if err := store.Delete(context.Background(), "bank", nil); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
}

func TestLevelDBStoreSavesAndLoadsEvidence(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	record := EvidenceRecord{
		Evidence: slashing.Evidence{
			Type:      slashing.EvidenceConflictingVote,
			Validator: "alice",
			Height:    7,
			Round:     2,
			Proof:     []byte("proof"),
		},
		Applied:   true,
		CreatedAt: 123,
	}
	if err := store.SaveEvidence(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	index, err := store.EvidenceIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 {
		t.Fatalf("expected one evidence index entry, got %d", len(index))
	}
	loaded, err := store.EvidenceByKey(context.Background(), index[0])
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Evidence.Validator != "alice" || !loaded.Applied || loaded.CreatedAt != 123 {
		t.Fatalf("unexpected evidence record: %+v", loaded)
	}
	if err := store.SaveEvidence(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	index, err = store.EvidenceIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 1 {
		t.Fatalf("expected idempotent evidence index, got %d", len(index))
	}
}

func TestLevelDBStoreRejectsInvalidEvidenceRecord(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.SaveEvidence(context.Background(), EvidenceRecord{}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
	if _, err := store.EvidenceByKey(context.Background(), ""); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
	if _, err := store.EvidenceByKey(context.Background(), "missing"); !errors.Is(err, ErrEvidenceNotFound) {
		t.Fatalf("expected evidence not found, got %v", err)
	}
	if _, err := store.EvidenceIndex(context.Background()); !errors.Is(err, ErrEvidenceNotFound) {
		t.Fatalf("expected evidence index not found, got %v", err)
	}
}

func TestLevelDBStoreRecoverIndexesRebuildsAfterCrash(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	for height := types.Height(1); height <= 3; height++ {
		if err := store.SaveBlock(context.Background(), BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: types.Hash{byte(height)}}); err != nil {
			t.Fatal(err)
		}
	}
	evidence := EvidenceRecord{
		Evidence: slashing.Evidence{
			Type:      slashing.EvidenceDoubleSign,
			Validator: "alice",
			Height:    1,
			Proof:     []byte("proof"),
		},
		Applied: true,
	}
	if err := store.SaveEvidence(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if err := store.db.Delete(blockIndexKey, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.db.Delete(evidenceIndexKey, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BlockIndex(context.Background()); !errors.Is(err, ErrBlockIndexNotFound) {
		t.Fatalf("expected missing block index, got %v", err)
	}
	if _, err := store.EvidenceIndex(context.Background()); !errors.Is(err, ErrEvidenceNotFound) {
		t.Fatalf("expected missing evidence index, got %v", err)
	}

	result, err := store.RecoverIndexes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.BlockIndexKeys != 3 || result.EvidenceKeys != 1 || result.RecoveredIndexes != 2 {
		t.Fatalf("unexpected recover result: %+v", result)
	}
	index, err := store.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 1 || index.LatestHeight != 3 || index.TotalBlocks != 3 {
		t.Fatalf("unexpected recovered block index: %+v", index)
	}
	evidenceIndex, err := store.EvidenceIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(evidenceIndex) != 1 {
		t.Fatalf("unexpected recovered evidence index: %+v", evidenceIndex)
	}
}

func TestLevelDBStoreCompacts(t *testing.T) {
	store := openTestStore(t)
	defer closeStore(t, store)

	if err := store.SaveBlock(context.Background(), BlockRecord{Block: types.Block{Header: types.Header{Height: 1}}, Hash: types.Hash{1}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func openTestStore(t *testing.T) *LevelDBStore {
	t.Helper()
	store, err := OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func closeStore(t *testing.T, store *LevelDBStore) {
	t.Helper()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
