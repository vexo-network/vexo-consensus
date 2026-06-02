package app

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestAnteKeeperRejectsFeeGasAndNonceFailures(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{MinFee: 10, MinGas: 5, MaxGas: 100, RequireNonce: true})
	ctx := Context{}

	cases := []struct {
		name string
		tx   types.Tx
		err  error
	}{
		{name: "fee", tx: []byte("bank:send:fee=9:gas=5:signer=alice:nonce=1"), err: ErrInsufficientFee},
		{name: "low gas", tx: []byte("bank:send:fee=10:gas=4:signer=alice:nonce=1"), err: ErrInvalidGas},
		{name: "high gas", tx: []byte("bank:send:fee=10:gas=101:signer=alice:nonce=1"), err: ErrInvalidGas},
		{name: "missing nonce", tx: []byte("bank:send:fee=10:gas=5"), err: ErrMissingNonce},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := keeper.CheckTx(ctx, testCase.tx); !errors.Is(err, testCase.err) {
				t.Fatalf("expected %v, got %v", testCase.err, err)
			}
		})
	}
}

func TestAnteKeeperTracksNonceAcrossCommittedTxs(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true})
	ctx := Context{Store: storage}
	first := types.Tx("bank:send:signer=alice:nonce=1")
	if err := keeper.CheckTx(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := keeper.AfterTx(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := keeper.CheckTx(ctx, first); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected replay nonce rejection, got %v", err)
	}
	if err := keeper.CheckTx(ctx, []byte("bank:send:signer=alice:nonce=2")); err != nil {
		t.Fatalf("expected next nonce to pass, got %v", err)
	}
}

func TestAnteKeeperChecksBlockNonceSequence(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true})
	ctx := Context{Store: storage}
	err = keeper.CheckBlock(ctx, []types.Tx{
		[]byte("bank:send:signer=alice:nonce=1"),
		[]byte("bank:send:signer=alice:nonce=2"),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = keeper.CheckBlock(ctx, []types.Tx{
		[]byte("bank:send:signer=alice:nonce=1"),
		[]byte("bank:send:signer=alice:nonce=1"),
	})
	if !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected duplicate nonce rejection, got %v", err)
	}
}
