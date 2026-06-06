package stateproof

import (
	"errors"
	"testing"
)

func TestMembershipProofSortsLeavesAndRejectsTampering(t *testing.T) {
	pairs := []Pair{
		{Key: []byte("carol"), Value: []byte("300")},
		{Key: []byte("alice"), Value: []byte("100")},
		{Key: []byte("bob"), Value: []byte("200")},
	}
	root, value, exists, path, err := BuildMembership("bank", pairs, []byte("bob"))
	if err != nil {
		t.Fatal(err)
	}
	if !exists || string(value) != "200" {
		t.Fatalf("unexpected membership exists=%t value=%q", exists, value)
	}
	if !VerifyMembership("bank", []byte("bob"), []byte("200"), path, root) {
		t.Fatalf("expected membership proof to verify")
	}
	if VerifyMembership("bank", []byte("bob"), []byte("201"), path, root) {
		t.Fatalf("tampered value verified")
	}
	if len(path) > 0 {
		path[0].Side = "invalid"
		if VerifyMembership("bank", []byte("bob"), []byte("200"), path, root) {
			t.Fatalf("tampered path verified")
		}
	}
}

func TestCompactNonMembershipProofBoundaries(t *testing.T) {
	pairs := []Pair{
		{Key: []byte("alice"), Value: []byte("100")},
		{Key: []byte("bob"), Value: []byte("200")},
		{Key: []byte("carol"), Value: []byte("300")},
	}
	root, left, right, exists, err := BuildNonMembership("bank", pairs, []byte("brian"))
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("unexpected existing key")
	}
	if left == nil || string(left.Key) != "bob" || right == nil || string(right.Key) != "carol" {
		t.Fatalf("unexpected neighbors left=%+v right=%+v", left, right)
	}
	if !VerifyNonMembershipCompact("bank", []byte("brian"), root, left, right) {
		t.Fatalf("expected compact non-membership proof to verify")
	}
	tamperedRight := *right
	tamperedRight.Index++
	if VerifyNonMembershipCompact("bank", []byte("brian"), root, left, &tamperedRight) {
		t.Fatalf("tampered neighbor index verified")
	}
}

func TestCompactNonMembershipProofAtEdgesAndEmptyNamespace(t *testing.T) {
	pairs := []Pair{{Key: []byte("bob"), Value: []byte("200")}}
	root, left, right, exists, err := BuildNonMembership("bank", pairs, []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if exists || left != nil || right == nil {
		t.Fatalf("unexpected left-edge proof left=%+v right=%+v exists=%t", left, right, exists)
	}
	if !VerifyNonMembershipCompact("bank", []byte("alice"), root, left, right) {
		t.Fatalf("expected left-edge non-membership proof to verify")
	}
	root, left, right, exists, err = BuildNonMembership("bank", pairs, []byte("carol"))
	if err != nil {
		t.Fatal(err)
	}
	if exists || left == nil || right != nil {
		t.Fatalf("unexpected right-edge proof left=%+v right=%+v exists=%t", left, right, exists)
	}
	if !VerifyNonMembershipCompact("bank", []byte("carol"), root, left, right) {
		t.Fatalf("expected right-edge non-membership proof to verify")
	}
	emptyRoot, left, right, exists, err := BuildNonMembership("bank", nil, []byte("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if exists || left != nil || right != nil {
		t.Fatalf("unexpected empty proof left=%+v right=%+v exists=%t", left, right, exists)
	}
	if !VerifyNonMembershipCompact("bank", []byte("alice"), emptyRoot, left, right) {
		t.Fatalf("expected empty namespace proof to verify")
	}
}

func TestDuplicateKeysAreRejected(t *testing.T) {
	_, err := Root("bank", []Pair{
		{Key: []byte("alice"), Value: []byte("100")},
		{Key: []byte("alice"), Value: []byte("200")},
	})
	if !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("expected duplicate key error, got %v", err)
	}
}
