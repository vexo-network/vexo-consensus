package app

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/store"
)

func TestStagedStoreSetBatchOverlaysWrites(t *testing.T) {
	base, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()

	staged := NewStagedStore(base)
	err = staged.SetBatch(context.Background(), []kvbatch.KVWrite{
		{Namespace: "bank", Key: []byte("alice"), Value: []byte("10")},
		{Namespace: "bank", Key: []byte("bob"), Value: []byte("20")},
		{Namespace: "bank", Key: []byte("bob"), Delete: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := staged.Get(context.Background(), "bank", []byte("alice"))
	if err != nil || string(value) != "10" {
		t.Fatalf("expected staged alice balance, value=%q err=%v", value, err)
	}
	if _, err := staged.Get(context.Background(), "bank", []byte("bob")); err != store.ErrKeyNotFound {
		t.Fatalf("expected staged bob delete, got %v", err)
	}
	if writes := staged.Writes(); len(writes) != 3 || writes[0].Namespace != "bank" || string(writes[1].Key) != "bob" || !writes[2].Delete {
		t.Fatalf("unexpected staged writes: %+v", writes)
	}
}
