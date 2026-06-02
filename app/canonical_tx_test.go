package app

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestCanonicalTxBuildsStableTagOrder(t *testing.T) {
	tx, err := BuildCanonicalTx(CanonicalTx{
		Module: "bank",
		Action: "send",
		Args:   []string{"alice", "bob", "1"},
		Tags: map[string]string{
			"nonce":    "7",
			"fee":      "2",
			"signer":   "alice",
			"gas":      "100",
			"priority": "9",
			"memo":     "hello",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := "bank:send:alice:bob:1:fee=2:gas=100:signer=alice:nonce=7:priority=9:memo=hello"
	if string(tx) != expected {
		t.Fatalf("expected canonical tx %q, got %q", expected, tx)
	}
}

func TestParseCanonicalTxUnwrapsSignedPayload(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	payload := types.Tx("bank:send:alice:bob:1:fee=2:gas=100:signer=alice:nonce=7")
	signedTx, err := SignTx("vexo-test", payload, signer)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCanonicalTx(signedTx)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Module != "bank" || parsed.Action != "send" || parsed.Tags["signer"] != "alice" {
		t.Fatalf("unexpected parsed tx: %+v", parsed)
	}
	if fee, found := TxUintTag(signedTx, "fee"); !found || fee != 2 {
		t.Fatalf("expected signed tx fee tag, got %d found=%t", fee, found)
	}
}

func TestAccountKeeperSequenceRoundTrip(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	keeper := NewAccountKeeper()
	next, err := keeper.NextSequence(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if next != 1 {
		t.Fatalf("expected first sequence 1, got %d", next)
	}
	if err := keeper.SetSequence(context.Background(), storage, "alice", 7); err != nil {
		t.Fatal(err)
	}
	sequence, err := keeper.Sequence(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if sequence != 7 {
		t.Fatalf("expected stored sequence 7, got %d", sequence)
	}
	next, err = keeper.NextSequence(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if next != 8 {
		t.Fatalf("expected next sequence 8, got %d", next)
	}
}

func TestAccountKeeperRejectsMissingSigner(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if _, err := NewAccountKeeper().Sequence(context.Background(), storage, ""); !errors.Is(err, ErrInvalidAccountSequence) {
		t.Fatalf("expected invalid account sequence, got %v", err)
	}
}
