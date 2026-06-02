package app

import (
	"context"
	"errors"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
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

func TestAnteKeeperRequiresAndVerifiesSignedTx(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true})
	raw := types.Tx("bank:send:alice:bob:1")
	if err := keeper.CheckTx(Context{ChainID: "vexo-test"}, raw); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("expected unsigned tx rejection, got %v", err)
	}
	signedTx, err := SignTx("vexo-test", raw, signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := keeper.CheckTx(Context{ChainID: "vexo-test"}, signedTx); err != nil {
		t.Fatal(err)
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

func TestAnteKeeperCollectsFeeAndReportsGas(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{MinFee: 1, FeeCollector: "treasury"})
	tx := types.Tx("bank:send:signer=alice:nonce=1:fee=7:gas=99")
	if err := keeper.AfterTx(Context{Store: storage}, tx); err != nil {
		t.Fatal(err)
	}
	alice, err := bankBalance(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	treasury, err := bankBalance(context.Background(), storage, "treasury")
	if err != nil {
		t.Fatal(err)
	}
	if alice != 93 || treasury != 7 {
		t.Fatalf("unexpected fee balances: alice=%d treasury=%d", alice, treasury)
	}
	if keeper.GasUsed(tx) != 99 || keeper.FeePaid(tx) != 7 {
		t.Fatalf("unexpected gas/fee metadata: gas=%d fee=%d", keeper.GasUsed(tx), keeper.FeePaid(tx))
	}
}

func TestAnteKeeperRejectsFeeCollectionWithoutBalance(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	keeper := NewAnteKeeper(AnteConfig{MinFee: 1})
	err = keeper.AfterTx(Context{Store: storage}, []byte("bank:send:signer=alice:nonce=1:fee=7"))
	if !errors.Is(err, ErrInsufficientFeeBalance) {
		t.Fatalf("expected insufficient fee balance, got %v", err)
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
