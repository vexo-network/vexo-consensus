package store

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
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
	}

	result, err := store.PruneBelow(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.PrunedBlocks != 2 || result.PrunedStateRoots != 2 {
		t.Fatalf("unexpected prune result: %+v", result)
	}
	if _, err := store.BlockByHeight(context.Background(), 1); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected pruned block not found, got %v", err)
	}
	if _, err := store.StateRoot(context.Background(), 2, "bank"); !errors.Is(err, ErrStateRootNotFound) {
		t.Fatalf("expected pruned state root not found, got %v", err)
	}
	if _, err := store.BlockByHeight(context.Background(), 3); err != nil {
		t.Fatalf("expected retained block, got %v", err)
	}
	index, err := store.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 3 || index.LatestHeight != 4 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected index after prune: %+v", index)
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
	if err := store.SaveState(ctx, StateRecord{Height: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected save state canceled, got %v", err)
	}
	if _, err := store.LatestState(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected latest state canceled, got %v", err)
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
