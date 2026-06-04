package queryproof

import (
	"context"
	"testing"

	"github.com/vexo-network/vexo-consensus/store"
)

func TestBuildAndVerifyQueryProof(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx := context.Background()
	if err := storage.Set(ctx, "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	proof, err := Build(ctx, storage, "vexo-test", 3, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if !proof.Exists || !EqualValue(proof, []byte("100")) {
		t.Fatalf("unexpected proof: %+v", proof)
	}
	if err := Verify(proof, "vexo-test", 3, proof.StateRoot); err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(proof)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(decoded, "vexo-test", 3, proof.StateRoot); err != nil {
		t.Fatal(err)
	}
}
