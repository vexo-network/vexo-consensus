package app

import (
	"context"
	"encoding/binary"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// checkTx is the admission a single transaction goes through in CheckTx.
func checkTx(keeper *AnteKeeper, ctx Context, tx types.Tx) error {
	adm, err := keeper.admit(ctx, tx)
	if err != nil {
		return err
	}
	return keeper.newSimulation(ctx).apply(adm)
}

// checkBlock admits txs as one block: each one after the ones before it.
func checkBlock(keeper *AnteKeeper, ctx Context, txs []types.Tx) error {
	sim := keeper.newSimulation(ctx)
	for _, tx := range txs {
		adm, err := keeper.admit(ctx, tx)
		if err != nil {
			return err
		}
		if err := sim.apply(adm); err != nil {
			return err
		}
	}
	return nil
}

// executeTx charges tx the way a block does before and after its module runs.
func executeTx(keeper *AnteKeeper, ctx Context, tx types.Tx) error {
	adm, err := keeper.admit(ctx, tx)
	if err != nil {
		return err
	}
	if err := keeper.begin(ctx, adm); err != nil {
		return err
	}
	return keeper.finish(ctx, adm)
}

func openAnteStore(t *testing.T) *store.LevelDBStore {
	t.Helper()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { storage.Close() })
	return storage
}

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
		{name: "exhausted nonce", tx: []byte("bank:send:fee=10:gas=5:signer=alice:nonce=18446744073709551615"), err: ErrInvalidNonce},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := checkTx(keeper, ctx, testCase.tx); !errors.Is(err, testCase.err) {
				t.Fatalf("expected %v, got %v", testCase.err, err)
			}
		})
	}
}

func TestAnteKeeperRejectsFeeWithoutSigner(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{})
	if err := checkTx(keeper, Context{}, types.Tx("bank:send:alice:bob:1:fee=5:gas=5")); !errors.Is(err, ErrMissingSigner) {
		t.Fatalf("expected a fee without a payer to be rejected, got %v", err)
	}
}

func TestAnteKeeperChecksBaseFeeAndAmountUnits(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{MaxGas: 100})
	ctx := Context{BaseFee: 2}
	if err := checkTx(keeper, ctx, types.Tx("bank:send:fee=10avxo:gas=5:signer=alice")); err != nil {
		t.Fatal(err)
	}
	if err := checkTx(keeper, ctx, types.Tx("bank:send:fee=1gvxo:gas_limit=5:signer=alice")); err != nil {
		t.Fatal(err)
	}
	if err := checkTx(keeper, ctx, types.Tx("bank:send:fee=9avxo:gas=5:signer=alice")); !errors.Is(err, ErrInsufficientFee) {
		t.Fatalf("expected insufficient base fee, got %v", err)
	}
	if err := checkTx(keeper, ctx, types.Tx("bank:send:fee=10avxo:signer=alice")); !errors.Is(err, ErrInvalidGas) {
		t.Fatalf("expected missing gas rejection with base fee, got %v", err)
	}
}

// The base fee a transaction is priced against is the one of the context's
// state, never a value cached inside the ante handler.
func TestAnteKeeperPricesAgainstTheContextBaseFee(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{BaseFee: 1})
	tx := types.Tx("bank:send:signer=alice:fee=10:gas=5")
	if err := checkTx(keeper, Context{BaseFee: 2}, tx); err != nil {
		t.Fatalf("expected fee 10 to cover gas 5 at base fee 2, got %v", err)
	}
	if err := checkTx(keeper, Context{BaseFee: 3}, tx); !errors.Is(err, ErrInsufficientFee) {
		t.Fatalf("expected the raised base fee to reject the same transaction, got %v", err)
	}
	if err := checkTx(keeper, Context{}, tx); err != nil {
		t.Fatalf("expected a zero base fee to impose no price, got %v", err)
	}
}

func TestAnteKeeperRequiresAndVerifiesSignedTx(t *testing.T) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true})
	raw := types.Tx("bank:send:alice:bob:1")
	if err := checkTx(keeper, Context{ChainID: "vexo-test"}, raw); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("expected unsigned tx rejection, got %v", err)
	}
	signedTx, err := SignTx("vexo-test", raw, signer)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkTx(keeper, Context{ChainID: "vexo-test"}, signedTx); err != nil {
		t.Fatal(err)
	}
	if err := checkTx(keeper, Context{ChainID: "other-chain"}, signedTx); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("expected a signature for another chain to be rejected, got %v", err)
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
	if err := checkTx(keeper, Context{ChainID: "vexo-test"}, signedTx); !errors.Is(err, ErrInvalidTxSignature) {
		t.Fatalf("expected custom verifier rejection, got %v", err)
	}
}

type rejectingTxVerifier struct{}

func (rejectingTxVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	return false
}

type ethereumAuthenticatorStub struct {
	err   error
	calls int
}

func (stub *ethereumAuthenticatorStub) AuthenticateTx(ctx Context, tx types.Tx) error {
	stub.calls++
	return stub.err
}

// Regression: an unsigned transaction carrying eth_raw and eth_hash tags used to
// skip the signature requirement and the fee, so anyone could mint with
// `bank:mint:attacker:...:signer=governance:eth_raw=00:eth_hash=00`.
func TestAnteKeeperEthereumTagsAreNotASignatureBypass(t *testing.T) {
	const mint = "bank:mint:attacker:1000000000:gas=10:signer=governance:nonce=0:eth_raw=00:eth_hash=00"
	accepting := &ethereumAuthenticatorStub{}
	for name, keeper := range map[string]*AnteKeeper{
		"no authenticator":        NewAnteKeeper(AnteConfig{RequireSigned: true}),
		"accepting authenticator": NewAnteKeeper(AnteConfig{RequireSigned: true, EthereumAuthenticator: accepting}),
		"not requiring signing":   NewAnteKeeper(AnteConfig{EthereumAuthenticator: accepting}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkTx(keeper, Context{ChainID: "vexo-test"}, types.Tx(mint)); !errors.Is(err, ErrInvalidTxSignature) {
				t.Fatalf("expected the unsigned Ethereum-tagged mint to be rejected, got %v", err)
			}
		})
	}
	if accepting.calls != 0 {
		t.Fatalf("the Ethereum authenticator must only see evm transactions, saw %d calls", accepting.calls)
	}
}

func TestAnteKeeperAuthenticatesEthereumTransactionsThroughTheEVMModule(t *testing.T) {
	const ethereumTx = "evm:call:vm:0x000000000000000000000000000000000000aaaa:0x000000000000000000000000000000000000bbbb:call:0x:21000:fee=1:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaa"
	authenticator := &ethereumAuthenticatorStub{}
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true, RequireNonce: true, EthereumAuthenticator: authenticator})
	adm, err := keeper.admit(Context{ChainID: "vexo-test"}, types.Tx(ethereumTx))
	if err != nil {
		t.Fatalf("expected an authenticated Ethereum transaction to pass, got %v", err)
	}
	if !adm.ethereum || authenticator.calls != 1 {
		t.Fatalf("expected one authentication of an Ethereum transaction, got ethereum=%t calls=%d", adm.ethereum, authenticator.calls)
	}

	authenticator.err = errors.New("sender mismatch")
	if _, err := keeper.admit(Context{ChainID: "vexo-test"}, types.Tx(ethereumTx)); !errors.Is(err, ErrInvalidTxSignature) {
		t.Fatalf("expected the evm module's rejection to reject the transaction, got %v", err)
	}
	authenticator.err = nil

	for name, tx := range map[string]string{
		"missing eth_hash":   strings.Replace(ethereumTx, ":eth_hash=0xaa", "", 1),
		"missing eth_raw":    strings.Replace(ethereumTx, ":eth_raw=0x01", "", 1),
		"other module":       strings.Replace(ethereumTx, "evm:call", "bank:call", 1),
		"stray ethereum tag": strings.Replace(ethereumTx, ":eth_raw=0x01:eth_hash=0xaa", ":eth_gas_price=1", 1),
	} {
		if _, err := keeper.admit(Context{ChainID: "vexo-test"}, types.Tx(tx)); !errors.Is(err, ErrInvalidTxSignature) {
			t.Fatalf("%s: expected rejection, got %v", name, err)
		}
	}

	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	account, err := address.AccountFromPublicKey(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	signedWithEthereumTags := "evm:call:fee=1:gas=21000:signer=" + string(account) + ":nonce=1:eth_raw=0x01:eth_hash=0xaa"
	signedTx, err := SignTx("vexo-test", types.Tx(signedWithEthereumTags), signer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.admit(Context{ChainID: "vexo-test"}, signedTx); !errors.Is(err, ErrInvalidTxSignature) {
		t.Fatalf("expected a signed transaction with Ethereum tags to be rejected, got %v", err)
	}
}

func TestAnteKeeperRejectsAmbiguousTags(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{})
	for name, tx := range map[string]string{
		"duplicate fee":      "bank:send:alice:bob:1:fee=1:fee=100:signer=alice",
		"duplicate signer":   "bank:send:alice:bob:1:signer=alice:signer=bob",
		"empty signer":       "bank:send:alice:bob:1:signer=",
		"conflicting gas":    "bank:send:alice:bob:1:gas=5:gas_limit=6",
		"invalid nonce":      "bank:send:alice:bob:1:signer=alice:nonce=x",
		"invalid fee":        "bank:send:alice:bob:1:signer=alice:fee=-1",
		"overlong fee":       "bank:send:alice:bob:1:signer=alice:fee=" + strings.Repeat("9", 200),
		"undecodable signed": "signed:@@@",
	} {
		if _, err := keeper.admit(Context{}, types.Tx(tx)); err == nil {
			t.Fatalf("%s: expected the ambiguous transaction to be rejected", name)
		}
	}
}

func TestAnteKeeperTracksNonceAcrossCommittedTxs(t *testing.T) {
	storage := openAnteStore(t)
	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true})
	ctx := Context{Store: storage}
	first := types.Tx("bank:send:signer=alice:nonce=1")
	if err := checkTx(keeper, ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := executeTx(keeper, ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := checkTx(keeper, ctx, first); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected replay nonce rejection, got %v", err)
	}
	if err := checkTx(keeper, ctx, []byte("bank:send:signer=alice:nonce=2")); err != nil {
		t.Fatalf("expected next nonce to pass, got %v", err)
	}
}

func TestAnteKeeperTracksEthereumNonceFromZero(t *testing.T) {
	storage := openAnteStore(t)
	authenticator := &ethereumAuthenticatorStub{}
	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true, EthereumAuthenticator: authenticator})
	ctx := Context{Store: storage}
	signer := types.Address("0x000000000000000000000000000000000000aaaa")
	if err := setBankBalance(context.Background(), storage, signer, 10); err != nil {
		t.Fatal(err)
	}
	first := types.Tx("evm:call:fee=1:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := checkTx(keeper, ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := executeTx(keeper, ctx, first); err != nil {
		t.Fatal(err)
	}
	balance, err := bankBalance(context.Background(), storage, signer)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10 {
		t.Fatalf("Ethereum raw tx fee must be handled by the EVM state transition, got balance %d", balance)
	}
	if sequence, err := NewAccountKeeper().Sequence(context.Background(), storage, signer); err != nil || sequence != 0 {
		t.Fatalf("the EVM owns the sequence of an Ethereum transaction, got %d (%v)", sequence, err)
	}
	// Within one block the next Ethereum nonce follows the first.
	second := types.Tx("evm:call:fee=1:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=1:eth_raw=0x02:eth_hash=0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err := checkBlock(keeper, ctx, []types.Tx{first, second}); err != nil {
		t.Fatalf("expected Ethereum nonce 1 to pass after nonce 0, got %v", err)
	}
	if err := checkBlock(keeper, ctx, []types.Tx{first, first}); !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected replay nonce rejection, got %v", err)
	}
}

func TestAnteKeeperChargesFailedEthereumTransactionOnlyWhenAffordable(t *testing.T) {
	storage := openAnteStore(t)
	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true, FeeCollector: "treasury", EthereumAuthenticator: &ethereumAuthenticatorStub{}})
	ctx := Context{Store: storage}
	signer := types.Address("0x000000000000000000000000000000000000aaaa")
	tx := types.Tx("evm:call:fee=7:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaa")
	adm, err := keeper.admit(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}

	charged, err := keeper.fail(ctx, adm)
	if err != nil || charged != nil {
		t.Fatalf("a payer without funds must not be charged or sequenced, got charged=%v err=%v", charged, err)
	}
	if sequence, _ := NewAccountKeeper().Sequence(context.Background(), storage, signer); sequence != 0 {
		t.Fatalf("expected the nonce to stay unconsumed, got %d", sequence)
	}

	if err := setBankBalance(context.Background(), storage, signer, 10); err != nil {
		t.Fatal(err)
	}
	charged, err = keeper.fail(ctx, adm)
	if err != nil || charged == nil || charged.Int64() != 7 {
		t.Fatalf("expected the failed transaction to be charged its fee tag, got charged=%v err=%v", charged, err)
	}
	if balance, _ := bankBalance(context.Background(), storage, signer); balance != 3 {
		t.Fatalf("expected the fee tag to be collected, got balance %d", balance)
	}
	if collected, _ := bankBalance(context.Background(), storage, "treasury"); collected != 7 {
		t.Fatalf("expected the collector to receive the fee, got %d", collected)
	}
	if sequence, _ := NewAccountKeeper().Sequence(context.Background(), storage, signer); sequence != 1 {
		t.Fatalf("expected the nonce to be consumed, got %d", sequence)
	}
}

// The fee tag of an Ethereum transaction is not signed. A failed transaction pays at least its gas
// at the block's base fee, so a relayer cannot have block space consumed for free by leaving the
// tag out or setting it low; a higher tag, which the evm module bounds by what the signature
// authorizes, is paid as it is.
func TestAnteKeeperChargesFailedEthereumTransactionAtLeastTheBaseFee(t *testing.T) {
	signer := types.Address("0x000000000000000000000000000000000000aaaa")
	for name, test := range map[string]struct {
		tx   string
		want int64
	}{
		"no fee tag":              {tx: "evm:call:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaa", want: 42_000},
		"fee tag below the floor": {tx: "evm:call:fee=1:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaa", want: 42_000},
		"fee tag above the floor": {tx: "evm:call:fee=50000:gas=21000:signer=0x000000000000000000000000000000000000aaaa:nonce=0:eth_raw=0x01:eth_hash=0xaa", want: 50_000},
	} {
		t.Run(name, func(t *testing.T) {
			storage := openAnteStore(t)
			keeper := NewAnteKeeper(AnteConfig{RequireNonce: true, FeeCollector: "treasury", EthereumAuthenticator: &ethereumAuthenticatorStub{}})
			ctx := Context{Store: storage, BaseFee: 2}
			if err := setBankBalance(context.Background(), storage, signer, 100_000); err != nil {
				t.Fatal(err)
			}
			adm, err := keeper.admit(ctx, types.Tx(test.tx))
			if err != nil {
				t.Fatal(err)
			}
			charged, err := keeper.fail(ctx, adm)
			if err != nil || charged == nil || charged.Int64() != test.want {
				t.Fatalf("expected a charge of %d, got %v (%v)", test.want, charged, err)
			}
			if collected, _ := bankBalance(context.Background(), storage, "treasury"); collected != uint64(test.want) {
				t.Fatalf("expected the collector to receive %d, got %d", test.want, collected)
			}
		})
	}
}

func TestAnteKeeperCollectsFeeAndAdvancesSequence(t *testing.T) {
	storage := openAnteStore(t)
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{MinFee: 1, FeeCollector: "treasury"})
	ctx := Context{Store: storage}
	tx := types.Tx("bank:send:signer=alice:nonce=1:fee=7:gas=99")
	if err := executeTx(keeper, ctx, tx); err != nil {
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
	adm, err := keeper.admit(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if adm.gasLimit != 99 || adm.meta.FeeSaturated() != 7 {
		t.Fatalf("unexpected gas/fee metadata: gas=%d fee=%d", adm.gasLimit, adm.meta.FeeSaturated())
	}
}

// Regression: a payer that is also the fee collector used to be debited and
// credited from the same balance read, so the credit overwrote the debit and
// every such transaction minted its fee.
func TestAnteKeeperFeePaidToItselfDoesNotMint(t *testing.T) {
	storage := openAnteStore(t)
	if err := setBankBalance(context.Background(), storage, "treasury", 100); err != nil {
		t.Fatal(err)
	}
	keeper := NewAnteKeeper(AnteConfig{FeeCollector: "treasury"})
	ctx := Context{Store: storage}
	if err := executeTx(keeper, ctx, types.Tx("bank:send:signer=treasury:nonce=1:fee=7:gas=9")); err != nil {
		t.Fatal(err)
	}
	if balance, _ := bankBalance(context.Background(), storage, "treasury"); balance != 100 {
		t.Fatalf("a fee paid to oneself must not change the balance, got %d", balance)
	}
	if err := executeTx(keeper, ctx, types.Tx("bank:send:signer=treasury:nonce=2:fee=101:gas=9")); !errors.Is(err, ErrInsufficientFeeBalance) {
		t.Fatalf("expected an unaffordable fee to be rejected even for the collector, got %v", err)
	}
}

func TestAnteKeeperReportsDetailedNonceMismatch(t *testing.T) {
	storage := openAnteStore(t)
	if err := setBankBalance(context.Background(), storage, "alice", 100); err != nil {
		t.Fatal(err)
	}
	if err := keeperAccountSequenceForTest(storage, "alice", 1); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true})
	err := checkTx(keeper, Context{Store: storage}, []byte("bank:send:fee=1:gas=10:signer=alice:nonce=3"))
	if !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected invalid nonce, got %v", err)
	}
	if !strings.Contains(err.Error(), "signer=alice") || !strings.Contains(err.Error(), "nonce=3") {
		t.Fatalf("expected detailed nonce mismatch, got %v", err)
	}
	if nonce, expected, ok := ParseInvalidNonceLog(err.Error()); !ok || nonce != 3 || expected != 2 {
		t.Fatalf("expected the mismatch to parse back, got nonce=%d expected=%d ok=%t", nonce, expected, ok)
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

func TestAnteKeeperRejectedChargeLeavesStateUntouched(t *testing.T) {
	storage := openAnteStore(t)
	if err := setBankBalance(context.Background(), storage, "alice", 5); err != nil {
		t.Fatal(err)
	}
	if err := setBankBalance(context.Background(), storage, "treasury", 1); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{MinFee: 1, FeeCollector: "treasury"})
	ctx := Context{Store: storage}
	if err := executeTx(keeper, ctx, types.Tx("bank:send:signer=alice:nonce=1:fee=7:gas=99")); !errors.Is(err, ErrInsufficientFeeBalance) {
		t.Fatalf("expected insufficient fee balance, got %v", err)
	}
	alice, _ := bankBalance(context.Background(), storage, "alice")
	treasury, _ := bankBalance(context.Background(), storage, "treasury")
	if alice != 5 || treasury != 1 {
		t.Fatalf("expected unchanged balances after a rejected charge, alice=%d treasury=%d", alice, treasury)
	}
	if sequence, _ := NewAccountKeeper().Sequence(context.Background(), storage, "alice"); sequence != 0 {
		t.Fatalf("a rejected charge must not consume the nonce, got %d", sequence)
	}
}

func TestAnteKeeperCollects256BitNativeFee(t *testing.T) {
	storage := openAnteStore(t)
	hugeBalance := new(big.Int).Lsh(big.NewInt(1), 90)
	hugeFee := new(big.Int).Lsh(big.NewInt(1), 80)
	if err := setBankBalanceBig(context.Background(), storage, "alice", hugeBalance); err != nil {
		t.Fatal(err)
	}

	keeper := NewAnteKeeper(AnteConfig{FeeCollector: "treasury"})
	ctx := Context{Store: storage, BaseFee: 1}
	tx := types.Tx("bank:send:signer=alice:nonce=1:fee=" + hugeFee.String() + ":gas=1")
	if err := checkTx(keeper, ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := executeTx(keeper, ctx, tx); err != nil {
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
	adm, err := keeper.admit(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if adm.meta.FeeSaturated() != ^uint64(0) {
		t.Fatalf("expected saturated fee metadata, got %d", adm.meta.FeeSaturated())
	}
}

func TestAnteKeeperRejectsFeeCollectionWithoutBalance(t *testing.T) {
	storage := openAnteStore(t)
	keeper := NewAnteKeeper(AnteConfig{MinFee: 1})
	err := executeTx(keeper, Context{Store: storage}, []byte("bank:send:signer=alice:nonce=1:fee=7"))
	if !errors.Is(err, ErrInsufficientFeeBalance) {
		t.Fatalf("expected insufficient fee balance, got %v", err)
	}
}

func TestAnteKeeperChecksBlockNonceSequence(t *testing.T) {
	storage := openAnteStore(t)
	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true})
	ctx := Context{Store: storage}
	err := checkBlock(keeper, ctx, []types.Tx{
		[]byte("bank:send:signer=alice:nonce=1"),
		[]byte("bank:send:signer=alice:nonce=2"),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = checkBlock(keeper, ctx, []types.Tx{
		[]byte("bank:send:signer=alice:nonce=1"),
		[]byte("bank:send:signer=alice:nonce=1"),
	})
	if !errors.Is(err, ErrInvalidNonce) {
		t.Fatalf("expected duplicate nonce rejection, got %v", err)
	}
}

// Regression: each transaction of a block was checked against the balance as if
// it were alone, so a payer could pack more fees into a block than it could
// pay, and the surplus aborted FinalizeBlock.
func TestAnteKeeperChecksFeeSolvencyCumulatively(t *testing.T) {
	storage := openAnteStore(t)
	if err := setBankBalance(context.Background(), storage, "alice", 10); err != nil {
		t.Fatal(err)
	}
	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true, FeeCollector: "treasury"})
	ctx := Context{Store: storage}
	first := types.Tx("bank:send:signer=alice:nonce=1:fee=6:gas=9")
	second := types.Tx("bank:send:signer=alice:nonce=2:fee=6:gas=9")
	for _, tx := range []types.Tx{first, second} {
		if err := checkTx(keeper, ctx, tx); err != nil && !errors.Is(err, ErrInvalidNonce) {
			t.Fatalf("each fee is affordable on its own, got %v", err)
		}
	}
	if err := checkBlock(keeper, ctx, []types.Tx{first, second}); !errors.Is(err, ErrInsufficientFeeBalance) {
		t.Fatalf("expected the second fee to be unaffordable after the first, got %v", err)
	}
	if err := checkBlock(keeper, ctx, []types.Tx{first}); err != nil {
		t.Fatal(err)
	}
	// A simulation never writes to the store.
	if balance, _ := bankBalance(context.Background(), storage, "alice"); balance != 10 {
		t.Fatalf("simulation changed the store: balance %d", balance)
	}
}

func TestAnteKeeperSimulationCreditsTheCollectorBeforeItPays(t *testing.T) {
	storage := openAnteStore(t)
	if err := setBankBalance(context.Background(), storage, "alice", 10); err != nil {
		t.Fatal(err)
	}
	keeper := NewAnteKeeper(AnteConfig{RequireNonce: true, FeeCollector: "treasury"})
	// treasury owns nothing until alice's fee reaches it within the same block.
	err := checkBlock(keeper, Context{Store: storage}, []types.Tx{
		[]byte("bank:send:signer=alice:nonce=1:fee=6:gas=9"),
		[]byte("bank:send:signer=treasury:nonce=1:fee=5:gas=9"),
	})
	if err != nil {
		t.Fatalf("expected the collector to afford a fee out of what it collected, got %v", err)
	}
}
