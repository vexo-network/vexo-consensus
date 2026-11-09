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
	if len(proof.Path.Siblings) != 0 || proof.Path.Neighbor != nil {
		t.Fatalf("a single-leaf proof needs no siblings: %+v", proof.Path)
	}
	root, err := storage.Root(ctx, "bank")
	if err != nil || proof.StateRoot != root {
		t.Fatalf("expected the proof against the stored root %x, got %x (%v)", root, proof.StateRoot, err)
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
	decoded.Value = []byte("200")
	if err := Verify(decoded, "vexo-test", 3, proof.StateRoot); err != ErrInvalidProof {
		t.Fatalf("expected tampered proof rejection, got %v", err)
	}
}

func TestBuildAndVerifyNonMembershipQueryProof(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx := context.Background()
	if err := storage.Set(ctx, "bank", []byte("alice"), []byte("100")); err != nil {
		t.Fatal(err)
	}
	if err := storage.Set(ctx, "bank", []byte("bob"), []byte("25")); err != nil {
		t.Fatal(err)
	}
	proof, err := Build(ctx, storage, "vexo-test", 3, "bank", []byte("carol"))
	if err != nil {
		t.Fatal(err)
	}
	if proof.Exists {
		t.Fatalf("expected a non-membership proof, got %+v", proof)
	}
	if err := Verify(proof, "vexo-test", 3, proof.StateRoot); err != nil {
		t.Fatal(err)
	}
	claimed := proof
	claimed.Exists = true
	if err := Verify(claimed, "vexo-test", 3, proof.StateRoot); err != ErrInvalidProof {
		t.Fatalf("expected an absence proof to prove no value, got %v", err)
	}

	// The absence proof of a present key cannot be assembled from the proofs of other keys.
	present, err := Build(ctx, storage, "vexo-test", 3, "bank", []byte("bob"))
	if err != nil {
		t.Fatal(err)
	}
	forged := present
	forged.Exists = false
	forged.Value = nil
	forged.Path = proof.Path
	if err := Verify(forged, "vexo-test", 3, proof.StateRoot); err != ErrInvalidProof {
		t.Fatalf("expected a forged absence of bob to be rejected, got %v", err)
	}
	other, err := Build(ctx, storage, "vexo-test", 3, "bank", []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	neighbor := other.Path
	forged.Path = neighbor
	if err := Verify(forged, "vexo-test", 3, proof.StateRoot); err != ErrInvalidProof {
		t.Fatalf("expected the path of alice not to prove bob absent, got %v", err)
	}
}

// A proof built from the pairs of a past height matches the proof the store's tree serves.
func TestProofsFromPairsMatchTheStoredTree(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	ctx := context.Background()
	for index, account := range []string{"alice", "bob", "carol", "dave", "erin"} {
		if err := storage.Set(ctx, "bank", []byte(account), []byte{byte(index)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := storage.Delete(ctx, "bank", []byte("carol")); err != nil {
		t.Fatal(err)
	}
	pairs, err := storage.ExportNamespace(ctx, "bank")
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"alice", "carol", "erin", "frank"} {
		stored, err := Build(ctx, storage, "vexo-test", 3, "bank", []byte(account))
		if err != nil {
			t.Fatal(err)
		}
		rebuilt, err := BuildFromKVPairs("vexo-test", 3, "bank", []byte(account), pairs, stored.StateRoot)
		if err != nil {
			t.Fatal(err)
		}
		storedJSON, _ := Encode(stored)
		rebuiltJSON, _ := Encode(rebuilt)
		if string(storedJSON) != string(rebuiltJSON) {
			t.Fatalf("%s: stored proof %s, rebuilt %s", account, storedJSON, rebuiltJSON)
		}
		if err := Verify(stored, "vexo-test", 3, stored.StateRoot); err != nil {
			t.Fatalf("%s: %v", account, err)
		}
	}
}
