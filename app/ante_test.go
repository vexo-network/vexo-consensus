package app

import (
	"context"
	"encoding/binary"
	"errors"
	"math/big"
	"strings"
	"testing"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/kvbatch"
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

func TestAnteKeeperChecksBaseFeeAndAmountUnits(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{BaseFee: 2, MaxGas: 100})
	ctx := Context{}
	if err := keeper.CheckTx(ctx, types.Tx("bank:send:fee=10avxo:gas=5")); err != nil {
		t.Fatal(err)
	}
	if err := keeper.CheckTx(ctx, types.Tx("bank:send:fee=1gvxo:gas_limit=5")); err != nil {
		t.Fatal(err)
	}
	if err := keeper.CheckTx(ctx, types.Tx("bank:send:fee=9avxo:gas=5")); !errors.Is(err, ErrInsufficientFee) {
		t.Fatalf("expected insufficient base fee, got %v", err)
	}
	if err := keeper.CheckTx(ctx, types.Tx("bank:send:fee=10avxo")); !errors.Is(err, ErrInvalidGas) {
		t.Fatalf("expected missing gas rejection with base fee, got %v", err)
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

func TestAnteKeeperUsesConfiguredSignatureVerifier(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	signedTx, err := SignTx("vexo-test", types.Tx("bank:send:alice:bob:1"), signer)
	if err != nil {
		t.Fatal(err)
	}
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true, SignatureVerifier: rejectingTxVerifier{}})
	if err := keeper.CheckTx(Context{ChainID: "vexo-test"}, signedTx); !errors.Is(err, ErrInvalidTxSignature) {
		t.Fatalf("expected custom verifier rejection, got %v", err)
	}
}

type rejectingTxVerifier struct{}

func (rejectingTxVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	return false
}

type failingAnteBatchStore struct {
	store.Store
	err error
}

func (storage failingAnteBatchStore) SetBatch(ctx context.Context, writes []kvbatch.KVWrite) error {
	return storage.err
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

func TestAnteKeeperTracksEthereumNonceFromZero(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true})
	ctx := Context{Store: storage}
	signer := types.Address("0x000000000000000000000000000000000000aaaa")
	if err := setBankBalance(context.Background(), storage, signer, 10); err != nil {
		t.Fatal(err)
	}
	first := types.Tx("evm:call:fee=1:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := keeper.CheckTx(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := keeper.AfterTx(ctx, first); err != nil {
		t.Fatal(err)
	}
	balance, err := bankBalance(context.Background(), storage, signer)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10 {
		t.Fatalf("Ethereum raw tx fee must be handled by the EVM state transition, got balance %d", balance)
	}
	if err := keeper.CheckTx(ctx, first); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected replay nonce rejection, got %v", err)
	}
	second := types.Tx("evm:call:fee=1:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=1:eth_raw=0x02:eth_hash=0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err := keeper.CheckTx(ctx, second); err != nil {
		t.Fatalf("expected Ethereum nonce 1 to pass after nonce 0, got %v", err)
	}
}

func TestAnteKeeperLetsEthereumRawTxOwnBalanceAccounting(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{BaseFee: 100, MinFee: 100, RequireNonce: true})
	tx := types.Tx("evm:call:fee=1:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := keeper.CheckTx(Context{}, tx); err != nil {
		t.Fatalf("expected raw Ethereum tx to skip Vexo fee accounting: %v", err)
	}
	if keeper.FeePaid(tx) != 1 {
		t.Fatalf("expected raw Ethereum fee metadata to be reported for block metrics, got %d", keeper.FeePaid(tx))
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

func TestAnteKeeperReportsDetailedNonceMismatch(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if err := keeperAccountSequenceForTest(storage, "alice", 1); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true})
	err = keeper.CheckTx(Context{Store: storage}, []byte("bank:send:fee=1:gas=10:signer=alice:nonce=3"))
	if !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected invalid nonce, got %v", err)
	}
	if !strings.Contains(err.Error(), "signer=alice") || !strings.Contains(err.Error(), "nonce=3") {
		t.Fatalf("expected detailed nonce mismatch, got %v", err)
	}
}

func TestParseInvalidNonceLog(t *testing.T) {
	nonce, expected, ok := ParseInvalidNonceLog("invalid transaction nonce: signer=alice nonce=7 expected=6")
	if !ok || nonce != 7 || expected != 6 {
		t.Fatalf("unexpected parsed nonce mismatch: nonce=%d expected=%d ok=%t", nonce, expected, ok)
	}
	if _, _, ok := ParseInvalidNonceLog("invalid transaction gas"); ok {
		t.Fatal("non-nonce error was parsed as a nonce mismatch")
	}
}

func keeperAccountSequenceForTest(storage StateStore, signer string, sequence uint64) error {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], sequence)
	return storage.Set(context.Background(), "auth", []byte("nonce/"+signer), encoded[:])
}

func TestAnteKeeperFeeBatchFailureDoesNotMutateBalances(t *testing.T) {
	base, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	storage := failingAnteBatchStore{Store: base, err: errors.New("batch failed")}
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, "treasury", 1); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{MinFee: 1, FeeCollector: "treasury"})
	tx := types.Tx("bank:send:signer=alice:nonce=1:fee=7:gas=99")
	if err := keeper.AfterTx(Context{Store: storage}, tx); err == nil {
		t.Fatal("expected fee batch failure")
	}
	alice, err := bankBalance(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	treasury, err := bankBalance(context.Background(), storage, "treasury")
	if err != nil {
		t.Fatal(err)
	}
	if alice != 100 || treasury != 1 {
		t.Fatalf("expected unchanged balances after failed fee batch, alice=%d treasury=%d", alice, treasury)
	}
}

func TestAnteKeeperCollects256BitNativeFee(t *testing.T) {
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	hugeBalance := new(big.Int).Lsh(big.NewInt(1), 90)
	hugeFee := new(big.Int).Lsh(big.NewInt(1), 80)
	if err := setBankBalanceBig(context.Background(), storage, "alice", hugeBalance); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{BaseFee: 1, FeeCollector: "treasury"})
	tx := types.Tx("bank:send:signer=alice:nonce=1:fee=" + hugeFee.String() + ":gas=1")
	if err := keeper.CheckTx(Context{Store: storage}, tx); err != nil {
		t.Fatal(err)
	}
	if err := keeper.AfterTx(Context{Store: storage}, tx); err != nil {
		t.Fatal(err)
	}
	alice, err := bankBalanceBig(context.Background(), storage, "alice")
	if err != nil {
		t.Fatal(err)
	}
	treasury, err := bankBalanceBig(context.Background(), storage, "treasury")
	if err != nil {
		t.Fatal(err)
	}
	if alice.Cmp(new(big.Int).Sub(hugeBalance, hugeFee)) != 0 || treasury.Cmp(hugeFee) != 0 {
		t.Fatalf("unexpected 256-bit fee balances: alice=%s treasury=%s", alice, treasury)
	}
	if keeper.FeePaid(tx) != ^uint64(0) {
		t.Fatalf("expected saturated FeePaid compatibility value, got %d", keeper.FeePaid(tx))
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
