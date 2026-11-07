package bank

import (
	"context"
	"math/big"
	"testing"

	"github.com/vexo-network/vexo-consensus/address"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/fairordering"
	"github.com/vexo-network/vexo-consensus/types"
)

// Regression tests for the bank audit findings. Each one replays the exploit
// and requires it to fail while the legitimate form of the same operation
// still works.

func seedBalance(t *testing.T, storage vexoapp.StateStore, account types.Address, amount int64) {
	t.Helper()
	if err := MintBig(context.Background(), storage, account, big.NewInt(amount)); err != nil {
		t.Fatal(err)
	}
}

// send used to move funds out of ANY account named in the transaction: the
// signer tag was never compared with the sender.
func TestBankSendRequiresTheSignerToOwnTheSource(t *testing.T) {
	storage := newBankStore(t)
	module := newMintingModule()
	ctx := vexoapp.Context{Store: storage}
	seedBalance(t, storage, "victim", 100)

	for _, tx := range []string{
		"bank:send:victim:attacker:100:signer=attacker",
		"bank:send:victim:attacker:100",
		"bank:send:victim:attacker:100:signer=",
		"bank:send:victim:attacker:100:signer=victim:signer=attacker",
		"bank:send:victim:attacker:100:signer=governance",
	} {
		if result := module.DeliverTx(ctx, types.Tx(tx)); result.Code == 0 {
			t.Fatalf("expected %q to be rejected", tx)
		}
		if err := module.ValidateTx(ctx, types.Tx(tx)); err == nil {
			t.Fatalf("expected admission to reject %q", tx)
		}
	}
	assertBalance(t, storage, "victim", 100)
	assertBalance(t, storage, "attacker", 0)

	if err := module.ValidateTx(ctx, types.Tx("bank:send:victim:attacker:100:signer=victim")); err != nil {
		t.Fatalf("expected the owner's send to be admitted: %v", err)
	}
	if result := module.DeliverTx(ctx, types.Tx("bank:send:victim:attacker:100:signer=victim")); result.Code != 0 {
		t.Fatalf("expected the owner's send to succeed: %+v", result)
	}
	assertBalance(t, storage, "victim", 0)
	assertBalance(t, storage, "attacker", 100)
}

// A transfer to oneself read both balances before writing, so the credit
// overwrote the debit and minted the amount.
func TestBankSelfTransferDoesNotMint(t *testing.T) {
	storage := newBankStore(t)
	module := newMintingModule()
	ctx := vexoapp.Context{Store: storage}
	seedBalance(t, storage, "alice", 100)

	for i := 0; i < 3; i++ {
		if result := module.DeliverTx(ctx, types.Tx("bank:send:alice:alice:100:signer=alice")); result.Code != 0 {
			t.Fatalf("expected the self transfer to succeed without effect: %+v", result)
		}
	}
	assertBalance(t, storage, "alice", 100)

	if result := module.DeliverTx(ctx, types.Tx("bank:send:alice:alice:101:signer=alice")); result.Code == 0 || result.Log != ErrInsufficientFunds.Error() {
		t.Fatalf("a self transfer must still be covered by the balance, got %+v", result)
	}
	assertBalance(t, storage, "alice", 100)
	if err := TransferBig(context.Background(), storage, "alice", "alice", big.NewInt(100)); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, storage, "alice", 100)
}

// The same Ethereum account written in two letter cases is one account.
func TestBankSelfTransferAcrossAddressSpellingsDoesNotMint(t *testing.T) {
	storage := newBankStore(t)
	module := newMintingModule()
	ctx := vexoapp.Context{Store: storage}
	lower := types.Address("0x000000000000000000000000000000000000bbbb")
	checksum := types.Address("0x000000000000000000000000000000000000BbBB")
	short := types.Address("0xbbbb")
	seedBalance(t, storage, lower, 100)

	for _, tx := range []string{
		"bank:send:" + string(checksum) + ":" + string(lower) + ":100:signer=" + string(lower),
		"bank:send:" + string(lower) + ":" + string(checksum) + ":100:signer=" + string(checksum),
		"bank:send:" + string(short) + ":" + string(lower) + ":100:signer=" + string(lower),
	} {
		if result := module.DeliverTx(ctx, types.Tx(tx)); result.Code != 0 {
			t.Fatalf("%s: %+v", tx, result)
		}
	}
	assertBalance(t, storage, lower, 100)
}

// With no mint authority configured the module used to let ANYONE mint.
func TestBankMintIsDisabledWithoutAnAuthority(t *testing.T) {
	storage := newBankStore(t)
	module := NewModule()
	ctx := vexoapp.Context{Store: storage}

	for _, tx := range []string{
		"bank:mint:attacker:1000000000",
		"bank:mint:attacker:1000000000:signer=attacker",
		"bank:mint:attacker:1000000000:signer=governance",
		"bank:mint:attacker:1000000000:signer=",
	} {
		result := module.DeliverTx(ctx, types.Tx(tx))
		if result.Code == 0 || result.Log != ErrUnauthorizedMint.Error() {
			t.Fatalf("%s: expected minting to be disabled, got %+v", tx, result)
		}
		if err := module.ValidateTx(ctx, types.Tx(tx)); err == nil {
			t.Fatalf("%s: expected admission to reject the mint", tx)
		}
	}
	assertBalance(t, storage, "attacker", 0)
}

func TestBankMintRequiresTheMintAuthoritySigner(t *testing.T) {
	storage := newBankStore(t)
	module := newMintingModule()
	ctx := vexoapp.Context{Store: storage}

	for _, tx := range []string{
		"bank:mint:attacker:1000000000:signer=attacker",
		"bank:mint:attacker:1000000000",
		"bank:mint:attacker:1000000000:signer=governance:signer=attacker",
		"bank:mint:attacker:1000000000:signer=Governance",
	} {
		if result := module.DeliverTx(ctx, types.Tx(tx)); result.Code == 0 {
			t.Fatalf("%s: expected the mint to be rejected", tx)
		}
	}
	assertBalance(t, storage, "attacker", 0)

	if result := module.DeliverTx(ctx, types.Tx("bank:mint:attacker:7:signer=governance")); result.Code != 0 {
		t.Fatalf("expected the mint authority to mint: %+v", result)
	}
	assertBalance(t, storage, "attacker", 7)
}

// The runtime-level exploit: an unsigned transaction carrying eth_raw/eth_hash
// tags skipped signature verification and the fee, and minted to the attacker.
func TestBankRuntimeRejectsUnsignedEthereumTagMint(t *testing.T) {
	storage := newBankStore(t)
	runtime := newSignedRuntime(t, storage, nil, vexoapp.AnteConfig{})
	tx := types.Tx("bank:mint:attacker:1000000000:gas=10:signer=governance:nonce=0:eth_raw=00:eth_hash=00")

	if response := runtime.CheckTx(tx); response.Result.Code == 0 {
		t.Fatal("expected CheckTx to reject the unsigned Ethereum-tagged mint")
	}
	if response := runtime.ProcessProposal(vexoapp.ProcessProposalRequest{Block: blockOf(1, tx)}); response.Accepted {
		t.Fatal("expected ProcessProposal to reject the unsigned Ethereum-tagged mint")
	}
	prepared, err := runtime.PrepareProposal(vexoapp.PrepareProposalRequest{Height: 1, Txs: []types.Tx{tx}})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Txs) != 0 {
		t.Fatalf("expected PrepareProposal to drop the mint, got %d txs", len(prepared.Txs))
	}
	response, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(1, tx)})
	if err != nil {
		t.Fatalf("a bad transaction must not abort the block: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].Code == 0 {
		t.Fatalf("expected a failed result, got %+v", response.Results)
	}
	assertBalance(t, storage, "attacker", 0)
}

// An Ethereum authenticator that accepts everything still cannot authenticate
// a transaction that does not route to the evm module.
func TestBankRuntimeEthereumAuthenticatorCannotAuthenticateBankTx(t *testing.T) {
	storage := newBankStore(t)
	runtime := newSignedRuntime(t, storage, acceptAllAuthenticator{}, vexoapp.AnteConfig{})
	tx := types.Tx("bank:mint:attacker:1000000000:gas=10:signer=governance:nonce=0:eth_raw=00:eth_hash=00")

	if response := runtime.CheckTx(tx); response.Result.Code == 0 {
		t.Fatal("expected CheckTx to reject the Ethereum-tagged bank transaction")
	}
	if _, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(1, tx)}); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, storage, "attacker", 0)
}

type acceptAllAuthenticator struct{}

func (acceptAllAuthenticator) AuthenticateTx(vexoapp.Context, types.Tx) error { return nil }

// A genuinely signed transaction that sends from someone else's account is
// rejected at admission, and when a block carries it anyway it fails after
// charging its own signer.
func TestBankRuntimeSignedSendFromAnotherAccountFails(t *testing.T) {
	storage := newBankStore(t)
	victim := newAccount(t)
	attacker := newAccount(t)
	runtime := newSignedRuntime(t, storage, nil, vexoapp.AnteConfig{})
	seedBalance(t, storage, victim.address, 100)
	seedBalance(t, storage, attacker.address, 50)

	theft := attacker.sign(t, "bank:send:"+string(victim.address)+":"+string(attacker.address)+":100:fee=10:gas=10:signer="+string(attacker.address)+":nonce=1")
	if response := runtime.CheckTx(theft); response.Result.Code == 0 {
		t.Fatal("expected CheckTx to reject a send from an account the signer does not own")
	}
	response, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(1, theft)})
	if err != nil {
		t.Fatalf("a failing transaction must not abort the block: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].Code == 0 {
		t.Fatalf("expected a failed result, got %+v", response.Results)
	}
	assertBalance(t, storage, victim.address, 100)
	assertBalance(t, storage, attacker.address, 40)

	own := victim.sign(t, "bank:send:"+string(victim.address)+":"+string(attacker.address)+":60:fee=10:gas=10:signer="+string(victim.address)+":nonce=1")
	if response := runtime.CheckTx(own); response.Result.Code != 0 {
		t.Fatalf("expected the owner's send to pass CheckTx: %+v", response.Result)
	}
	if _, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(2, own)}); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, storage, victim.address, 30)
	assertBalance(t, storage, attacker.address, 100)
}

// Paying a fee to oneself must not mint: the collector is the signer here.
func TestBankRuntimeSelfTransferAndSelfFeeDoNotMint(t *testing.T) {
	storage := newBankStore(t)
	alice := newAccount(t)
	runtime := newSignedRuntime(t, storage, nil, vexoapp.AnteConfig{FeeCollector: string(alice.address)})
	seedBalance(t, storage, alice.address, 100)

	transfer := alice.sign(t, "bank:send:"+string(alice.address)+":"+string(alice.address)+":100:fee=10:gas=10:signer="+string(alice.address)+":nonce=1")
	if _, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: blockOf(1, transfer)}); err != nil {
		t.Fatal(err)
	}
	assertBalance(t, storage, alice.address, 100)
}

type testAccount struct {
	signer  vexocrypto.Signer
	address types.Address
}

func newAccount(t *testing.T) testAccount {
	t.Helper()
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	account, err := address.AccountFromPublicKey(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	return testAccount{signer: signer, address: account}
}

func (account testAccount) sign(t *testing.T, payload string) types.Tx {
	t.Helper()
	tx, err := vexoapp.SignTx("vexo-test", types.Tx(payload), account.signer)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

// newSignedRuntime builds a runtime that demands signatures and nonces, with a
// bank module whose only mint authority is "governance" (an account no key can
// sign for).
func newSignedRuntime(t *testing.T, storage vexoapp.StateStore, authenticator vexoapp.TxAuthenticator, config vexoapp.AnteConfig) *vexoapp.Runtime {
	t.Helper()
	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{newMintingModule()}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	config.MinFee = 1
	config.BaseFee = 1
	config.RequireNonce = true
	config.RequireSigned = true
	config.EthereumAuthenticator = authenticator
	if config.FeeCollector == "" {
		config.FeeCollector = "fee_collector"
	}
	runtime.WithStore(storage).WithAnte(vexoapp.NewAnteKeeper(config))
	return runtime
}

func blockOf(height types.Height, txs ...types.Tx) types.Block {
	return types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: height},
		Txs:    fairordering.SortTxsWithSalt(txs, fairordering.HeightSalt("vexo-test", height)),
	}
}
