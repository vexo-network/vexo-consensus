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
		Hash:    types.Hash{1},
		AppHash: types.Hash{2},
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

	byHash, err := store.BlockByHash(context.Background(), record.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if byHash.Block.Header.Height != 1 {
		t.Fatalf("expected height 1, got %d", byHash.Block.Header.Height)
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
	if err := store.SaveState(ctx, StateRecord{Height: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected save state canceled, got %v", err)
	}
	if _, err := store.LatestState(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected latest state canceled, got %v", err)
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
