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
	if len(proof.MerklePath) != 0 {
		t.Fatalf("single-leaf proof should not need siblings: %+v", proof.MerklePath)
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
	if proof.Exists || len(proof.NamespaceLeaves) != 2 {
		t.Fatalf("expected full namespace non-membership proof, got %+v", proof)
	}
	if err := Verify(proof, "vexo-test", 3, proof.StateRoot); err != nil {
		t.Fatal(err)
	}
	proof.NamespaceLeaves = append(proof.NamespaceLeaves, proof.NamespaceLeaves[0])
	if err := Verify(proof, "vexo-test", 3, proof.StateRoot); err != ErrInvalidProof {
		t.Fatalf("expected duplicate/tampered absence proof rejection, got %v", err)
	}
}
