package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	goruntime "runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

// countingVerifier verifies with Ed25519 and counts the verifications it was asked for.
type countingVerifier struct {
	calls atomic.Int64
}

func (verifier *countingVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	verifier.calls.Add(1)
	return vexocrypto.Ed25519Signer{}.Verify(publicKey, message, signature)
}

// rendezvousVerifier holds a verification until another is in flight beside it, which only a
// pass that verifies on several goroutines brings about. It stops waiting after a while, so a pass
// that verifies one transaction after the other fails the test instead of hanging it.
type rendezvousVerifier struct {
	countingVerifier
	inFlight atomic.Int32
	met      chan struct{}
	once     sync.Once
	expired  atomic.Bool
}

func newRendezvousVerifier() *rendezvousVerifier {
	return &rendezvousVerifier{met: make(chan struct{})}
}

func (verifier *rendezvousVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	if verifier.inFlight.Add(1) >= 2 {
		verifier.once.Do(func() { close(verifier.met) })
	}
	defer verifier.inFlight.Add(-1)
	if !verifier.expired.Load() {
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-verifier.met:
		case <-timer.C:
			verifier.expired.Store(true)
		}
	}
	return verifier.countingVerifier.Verify(publicKey, message, signature)
}

// overlapped reports whether two verifications were ever in flight at once.
func (verifier *rendezvousVerifier) overlapped() bool {
	select {
	case <-verifier.met:
		return true
	default:
		return false
	}
}

// cancelingVerifier cancels a context the moment it is asked to verify.
type cancelingVerifier struct {
	countingVerifier
	cancel context.CancelFunc
}

func (verifier *cancelingVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	verifier.cancel()
	return verifier.countingVerifier.Verify(publicKey, message, signature)
}

// panickingVerifier panics on a payload that names "boom".
type panickingVerifier struct{ countingVerifier }

func (verifier *panickingVerifier) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	if bytes.Contains(message, []byte("boom")) {
		panic("verifier exploded")
	}
	return verifier.countingVerifier.Verify(publicKey, message, signature)
}

// cacheableAuthenticatorStub is an Ethereum authenticator that names the base fee as the one input
// of its verdict besides the transaction. It rejects a transaction whose raw tag is 0xbad, and one
// whose raw tag is 0xcheap once the base fee is positive, the way an Ethereum fee cap below the
// base fee is rejected.
type cacheableAuthenticatorStub struct {
	calls    atomic.Int64
	sawStore atomic.Bool
	decline  bool
}

func (stub *cacheableAuthenticatorStub) AuthenticateTx(ctx Context, tx types.Tx) error {
	stub.calls.Add(1)
	if ctx.Store != nil {
		stub.sawStore.Store(true)
	}
	switch {
	case bytes.Contains(tx, []byte("eth_raw=0xbad")):
		return errors.New("bad raw transaction")
	case ctx.BaseFee > 0 && bytes.Contains(tx, []byte("eth_raw=0xcheap")):
		return errors.New("fee cap below the base fee")
	}
	return nil
}

func (stub *cacheableAuthenticatorStub) AuthenticationKey(ctx Context, tx types.Tx) ([]byte, bool) {
	if stub.decline {
		return nil, false
	}
	return binary.BigEndian.AppendUint64(nil, ctx.BaseFee), true
}

// ethereumTx is an unsigned Ethereum-tagged transaction of sender whose raw transaction is raw.
func ethereumTx(action string, sender string, nonce uint64, raw string) types.Tx {
	return types.Tx(fmt.Sprintf("evm:%s:fee=1:gas=21000:signer=%s:nonce=%d:eth_raw=0x%s:eth_hash=0xaa", action, sender, nonce, raw))
}

// testAccount is an ed25519 key and the account that names it.
type testAccount struct {
	signer  vexocrypto.Ed25519Signer
	account types.Address
}

func newTestAccount(t *testing.T) testAccount {
	t.Helper()
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	account, err := address.AccountFromPublicKey(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	return testAccount{signer: signer, account: account}
}

func (account testAccount) payload(action string, nonce uint64) types.Tx {
	return types.Tx(fmt.Sprintf("bank:%s:fee=5:gas=20:signer=%s:nonce=%d", action, account.account, nonce))
}

func (account testAccount) signed(t *testing.T, chainID string, action string, nonce uint64) types.Tx {
	t.Helper()
	tx, err := SignTx(chainID, account.payload(action, nonce), account.signer)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

// forgingSigner signs with its key, but over another message than the one it is given: the
// transaction it makes is well formed and names its key, and no signature of it verifies.
type forgingSigner struct{ vexocrypto.Ed25519Signer }

func (signer forgingSigner) Sign(message []byte) (types.Signature, error) {
	return signer.Ed25519Signer.Sign(append([]byte("forged:"), message...))
}

func (account testAccount) forged(t *testing.T, chainID string, action string, nonce uint64) types.Tx {
	t.Helper()
	tx, err := SignTx(chainID, account.payload(action, nonce), forgingSigner{account.signer})
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

// withMaxProcs runs the rest of the test with procs processors, so that a pass that fans out does
// so wherever the tests run.
func withMaxProcs(t *testing.T, procs int) {
	t.Helper()
	previous := goruntime.GOMAXPROCS(procs)
	t.Cleanup(func() { goruntime.GOMAXPROCS(previous) })
}

func mustPanic(t *testing.T, name string, run func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: expected a panic", name)
		}
	}()
	run()
}

// signedBlock returns n signed transactions of distinct funded accounts, each the first of its
// account, and funds the accounts in storage.
func signedBlock(t *testing.T, storage StateStore, n int) []types.Tx {
	t.Helper()
	txs := make([]types.Tx, 0, n)
	for index := range n {
		account := newTestAccount(t)
		fundAccount(t, storage, account.account, 1000)
		txs = append(txs, account.signed(t, "vexo-test", fmt.Sprintf("ok%d", index), 1))
	}
	return txs
}

// A transaction is checked on arrival, selected into a proposal, validated as part of a block and
// executed. Its signature is verified when it first arrives and not again by any of the later
// stages.
func TestSignedTransactionIsVerifiedOncePerNode(t *testing.T) {
	storage := openAnteStore(t)
	account := newTestAccount(t)
	fundAccount(t, storage, account.account, 1000)
	verifier := &countingVerifier{}
	rt := newFeeRuntime(t, storage, &scriptedModule{name: "bank"}, AnteConfig{MinFee: 1, RequireSigned: true, SignatureVerifier: verifier})

	tx := account.signed(t, "vexo-test", "ok", 1)
	if response := rt.CheckTx(tx); response.Result.Code != 0 {
		t.Fatalf("CheckTx rejected a genuine transaction: %+v", response.Result)
	}
	if got := verifier.calls.Load(); got != 1 {
		t.Fatalf("CheckTx verifies the signature once, got %d verifications", got)
	}
	prepared, err := rt.PrepareProposal(PrepareProposalRequest{Height: 1, Txs: []types.Tx{tx}})
	if err != nil || len(prepared.Txs) != 1 {
		t.Fatalf("PrepareProposal must select the checked transaction, got %d (%v)", len(prepared.Txs), err)
	}
	block := blockAt(1, tx)
	if response := rt.ProcessProposal(ProcessProposalRequest{Block: block}); !response.Accepted {
		t.Fatalf("ProcessProposal rejected the block: %s", response.Reason)
	}
	response, err := rt.FinalizeBlock(FinalizeBlockRequest{Block: block})
	if err != nil || len(response.Results) != 1 || response.Results[0].Code != 0 {
		t.Fatalf("FinalizeBlock must execute the transaction, got %+v (%v)", response.Results, err)
	}
	if got := verifier.calls.Load(); got != 1 {
		t.Fatalf("the signature must be verified once for the transaction's whole life on a node, got %d verifications", got)
	}
}

// A block whose transactions this node never checked, as when it is caught up or replayed, is
// verified on several goroutines by the stage that meets it first and by none after.
func TestBlockOfUncheckedTransactionsIsVerifiedInParallelOnce(t *testing.T) {
	withMaxProcs(t, 4)
	for name, stage := range map[string]func(t *testing.T, rt *Runtime, block types.Block){
		"validation": func(t *testing.T, rt *Runtime, block types.Block) {
			if response := rt.ProcessProposal(ProcessProposalRequest{Block: block}); !response.Accepted {
				t.Fatalf("ProcessProposal rejected the block: %s", response.Reason)
			}
		},
		"execution": func(t *testing.T, rt *Runtime, block types.Block) {
			response, err := rt.FinalizeBlock(FinalizeBlockRequest{Block: block})
			if err != nil {
				t.Fatal(err)
			}
			for index, result := range response.Results {
				if result.Code != 0 {
					t.Fatalf("transaction %d failed: %+v", index, result)
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			storage := openAnteStore(t)
			verifier := newRendezvousVerifier()
			rt := newFeeRuntime(t, storage, &scriptedModule{name: "bank"}, AnteConfig{MinFee: 1, RequireSigned: true, SignatureVerifier: verifier})
			txs := signedBlock(t, storage, 8)
			block := blockAt(1, txs...)

			stage(t, rt, block)
			if !verifier.overlapped() {
				t.Fatal("the signatures of a block must be verified on several goroutines")
			}
			if got := verifier.calls.Load(); got != int64(len(txs)) {
				t.Fatalf("each signature must be verified once, got %d verifications of %d transactions", got, len(txs))
			}
		})
	}
}

// A block that was validated is executed without verifying its signatures again, with the pass in
// front of the execution finding every one of them remembered.
func TestValidatedBlockIsExecutedWithoutVerifyingAgain(t *testing.T) {
	withMaxProcs(t, 4)
	storage := openAnteStore(t)
	verifier := &countingVerifier{}
	rt := newFeeRuntime(t, storage, &scriptedModule{name: "bank"}, AnteConfig{MinFee: 1, RequireSigned: true, SignatureVerifier: verifier})
	txs := signedBlock(t, storage, 8)
	block := blockAt(1, txs...)

	if response := rt.ProcessProposal(ProcessProposalRequest{Block: block}); !response.Accepted {
		t.Fatalf("ProcessProposal rejected the block: %s", response.Reason)
	}
	if got := verifier.calls.Load(); got != int64(len(txs)) {
		t.Fatalf("validation must verify each signature once, got %d verifications", got)
	}
	if _, err := rt.FinalizeBlock(FinalizeBlockRequest{Block: block}); err != nil {
		t.Fatal(err)
	}
	if got := verifier.calls.Load(); got != int64(len(txs)) {
		t.Fatalf("execution must find every signature remembered, got %d verifications", got)
	}
}

// A rejection is never remembered: the transaction is verified, and rejected, every time it is
// presented, however often that is.
func TestRejectedSignaturesAreNeverRemembered(t *testing.T) {
	withMaxProcs(t, 4)
	account := newTestAccount(t)
	forged := account.forged(t, "vexo-test", "ok", 1)
	verifier := &countingVerifier{}
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true, SignatureVerifier: verifier})

	for attempt := int64(1); attempt <= 3; attempt++ {
		if _, err := keeper.admit(Context{ChainID: "vexo-test"}, forged); !errors.Is(err, ErrInvalidTxSignature) {
			t.Fatalf("attempt %d: expected the forged signature to be rejected, got %v", attempt, err)
		}
		if got := verifier.calls.Load(); got != attempt {
			t.Fatalf("attempt %d: a rejected transaction must be verified again, got %d verifications", attempt, got)
		}
	}
	keeper.preverify(Context{ChainID: "vexo-test"}, []types.Tx{forged, forged, forged, forged})
	if keeper.verified.Len() != 0 {
		t.Fatalf("the parallel pass must not remember a rejection, remembers %d", keeper.verified.Len())
	}

	// Through the runtime the same transaction fails the block's validation and execution, each
	// time, and a genuine transaction beside it is not affected.
	storage := openAnteStore(t)
	other := newTestAccount(t)
	fundAccount(t, storage, account.account, 1000)
	fundAccount(t, storage, other.account, 1000)
	rt := newFeeRuntime(t, storage, &scriptedModule{name: "bank"}, AnteConfig{MinFee: 1, RequireSigned: true, SignatureVerifier: &countingVerifier{}})
	genuine := other.signed(t, "vexo-test", "ok", 1)
	block := blockAt(1, forged, genuine)
	for attempt := 1; attempt <= 3; attempt++ {
		if response := rt.ProcessProposal(ProcessProposalRequest{Block: block}); response.Accepted {
			t.Fatalf("attempt %d: a block with a forged signature must be rejected", attempt)
		}
		if response := rt.CheckTx(forged); response.Result.Code == 0 {
			t.Fatalf("attempt %d: CheckTx must reject the forged signature", attempt)
		}
	}
	response, err := rt.FinalizeBlock(FinalizeBlockRequest{Block: block})
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]uint32{}
	for index, tx := range block.Txs {
		codes[string(tx)] = response.Results[index].Code
	}
	if codes[string(forged)] == 0 || codes[string(genuine)] != 0 {
		t.Fatalf("the forged transaction must fail and the genuine one execute, got %v", codes)
	}
}

// What a verification is a function of is part of what it is remembered by: a success under one
// chain id is not a success under another.
func TestRememberedAuthenticationDoesNotAuthenticateUnderAnotherChain(t *testing.T) {
	account := newTestAccount(t)
	verifier := &countingVerifier{}
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true, SignatureVerifier: verifier})
	tx := account.signed(t, "vexo-test", "ok", 1)

	if _, err := keeper.admit(Context{ChainID: "vexo-test"}, tx); err != nil {
		t.Fatal(err)
	}
	if _, err := keeper.admit(Context{ChainID: "vexo-test"}, tx); err != nil {
		t.Fatal(err)
	}
	if got := verifier.calls.Load(); got != 1 {
		t.Fatalf("the second admission must be answered from memory, got %d verifications", got)
	}
	if _, err := keeper.admit(Context{ChainID: "other-chain"}, tx); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("a signature remembered for one chain must not authenticate under another, got %v", err)
	}

	// An authenticator's success is remembered by chain id too, so the same transaction and the
	// same inputs under another chain id run the authenticator afresh.
	stub := &cacheableAuthenticatorStub{}
	keeper = NewAnteKeeper(AnteConfig{EthereumAuthenticator: stub})
	tx = ethereumTx("ok", "0x000000000000000000000000000000000000aaaa", 0, "01")
	for _, chainID := range []string{"chain-a", "chain-a", "chain-b", "chain-b", "chain-a"} {
		if _, err := keeper.admit(Context{ChainID: chainID}, tx); err != nil {
			t.Fatal(err)
		}
	}
	if got := stub.calls.Load(); got != 2 {
		t.Fatalf("the authenticator must run once per chain id, ran %d times", got)
	}
}

// An authenticator that names the inputs of its verdict is run again when one of them changes, and
// a rejection is never remembered.
func TestCacheableAuthenticatorIsRememberedUntilItsInputsChange(t *testing.T) {
	stub := &cacheableAuthenticatorStub{}
	keeper := NewAnteKeeper(AnteConfig{EthereumAuthenticator: stub})
	cheap := ethereumTx("ok", "0x000000000000000000000000000000000000aaaa", 0, "cheap")
	admit := func(baseFee uint64) error {
		_, err := keeper.admit(Context{ChainID: "vexo-test", BaseFee: baseFee}, cheap)
		return err
	}

	if err := admit(0); err != nil {
		t.Fatalf("the transaction authenticates while the base fee is zero, got %v", err)
	}
	if err := admit(0); err != nil || stub.calls.Load() != 1 {
		t.Fatalf("the second admission must be answered from memory, got %v after %d runs", err, stub.calls.Load())
	}
	for attempt := int64(2); attempt <= 4; attempt++ {
		if err := admit(5); !errors.Is(err, ErrInvalidTxSignature) {
			t.Fatalf("the raised base fee must reject what authenticated at base fee zero, got %v", err)
		}
		if got := stub.calls.Load(); got != attempt {
			t.Fatalf("a rejection must be authenticated again, ran %d times after %d attempts", got, attempt-1)
		}
	}
	if err := admit(0); err != nil || stub.calls.Load() != 4 {
		t.Fatalf("the success at base fee zero is still remembered, got %v after %d runs", err, stub.calls.Load())
	}
}

// An authenticator that does not name its inputs, or declines to for a call, runs every time and
// is never run beside another call.
func TestUnkeyedAuthenticatorsAreNeverRememberedNorRunInParallel(t *testing.T) {
	withMaxProcs(t, 4)
	tx := ethereumTx("ok", "0x000000000000000000000000000000000000aaaa", 0, "01")
	txs := make([]types.Tx, 0, 16)
	for index := range 16 {
		txs = append(txs, ethereumTx("ok", fmt.Sprintf("0x%040x", index), 0, "01"))
	}

	plain := &ethereumAuthenticatorStub{}
	keeper := NewAnteKeeper(AnteConfig{EthereumAuthenticator: plain})
	keeper.preverify(Context{ChainID: "vexo-test"}, txs)
	if plain.calls != 0 || keeper.verified.Len() != 0 {
		t.Fatalf("an authenticator that is not a CacheableAuthenticator must be left to the sequential pass, ran %d times", plain.calls)
	}
	for range 3 {
		if _, err := keeper.admit(Context{ChainID: "vexo-test"}, tx); err != nil {
			t.Fatal(err)
		}
	}
	if plain.calls != 3 || keeper.verified.Len() != 0 {
		t.Fatalf("an authenticator that names no inputs must run every time, ran %d times", plain.calls)
	}

	declining := &cacheableAuthenticatorStub{decline: true}
	keeper = NewAnteKeeper(AnteConfig{EthereumAuthenticator: declining})
	for range 3 {
		if _, err := keeper.admit(Context{ChainID: "vexo-test"}, tx); err != nil {
			t.Fatal(err)
		}
	}
	keeper.preverify(Context{ChainID: "vexo-test"}, txs)
	if got := declining.calls.Load(); got != 3 || keeper.verified.Len() != 0 {
		t.Fatalf("a call the authenticator declines to key must run every time and not in parallel, ran %d times", got)
	}
}

// The parallel pass hands authenticators no state store and leaves every other field of the
// context as it is.
func TestPreverifyHidesTheStateStoreFromAuthenticators(t *testing.T) {
	withMaxProcs(t, 4)
	storage := openAnteStore(t)
	stub := &cacheableAuthenticatorStub{}
	keeper := NewAnteKeeper(AnteConfig{EthereumAuthenticator: stub})
	txs := make([]types.Tx, 0, 8)
	for index := range 8 {
		txs = append(txs, ethereumTx("ok", fmt.Sprintf("0x%040x", index), 0, "01"))
	}
	keeper.preverify(Context{ChainID: "vexo-test", Store: storage}, txs)
	if stub.calls.Load() != int64(len(txs)) || stub.sawStore.Load() {
		t.Fatalf("expected %d authentications without a store, got %d (store seen: %t)", len(txs), stub.calls.Load(), stub.sawStore.Load())
	}
	for _, tx := range txs {
		if _, err := keeper.admit(Context{ChainID: "vexo-test", Store: storage}, tx); err != nil {
			t.Fatal(err)
		}
	}
	if stub.calls.Load() != int64(len(txs)) {
		t.Fatalf("the admission must find what the pass remembered, got %d authentications", stub.calls.Load())
	}
}

func TestPreverifyAuthenticatesOnSeveralGoroutines(t *testing.T) {
	withMaxProcs(t, 4)
	verifier := newRendezvousVerifier()
	keeper := NewAnteKeeper(AnteConfig{SignatureVerifier: verifier})
	txs := make([]types.Tx, 0, 8)
	for index := range 8 {
		txs = append(txs, newTestAccount(t).signed(t, "vexo-test", fmt.Sprintf("ok%d", index), 1))
	}

	keeper.preverify(Context{ChainID: "vexo-test"}, txs)
	if !verifier.overlapped() {
		t.Fatal("two verifications must have been in flight at once")
	}
	if got := keeper.verified.Len(); got != len(txs) {
		t.Fatalf("every transaction must be remembered, got %d of %d", got, len(txs))
	}
	if got := verifier.calls.Load(); got != int64(len(txs)) {
		t.Fatalf("each transaction must be verified once, got %d verifications", got)
	}
	keeper.preverify(Context{ChainID: "vexo-test"}, txs)
	if got := verifier.calls.Load(); got != int64(len(txs)) {
		t.Fatalf("a second pass must verify nothing, got %d verifications", got)
	}
}

func TestPreverifyStopsWhenTheContextEnds(t *testing.T) {
	withMaxProcs(t, 4)
	txs := make([]types.Tx, 0, 64)
	for index := range 64 {
		txs = append(txs, newTestAccount(t).signed(t, "vexo-test", fmt.Sprintf("ok%d", index), 1))
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	verifier := &countingVerifier{}
	NewAnteKeeper(AnteConfig{SignatureVerifier: verifier}).preverify(Context{Ctx: canceled, ChainID: "vexo-test"}, txs)
	if got := verifier.calls.Load(); got != 0 {
		t.Fatalf("a pass whose context ended must verify nothing, got %d verifications", got)
	}

	// A context that ends during the pass stops each goroutine after the verification it is in.
	midway, stop := context.WithCancel(context.Background())
	defer stop()
	interrupting := &cancelingVerifier{cancel: stop}
	keeper := NewAnteKeeper(AnteConfig{SignatureVerifier: interrupting})
	keeper.preverify(Context{Ctx: midway, ChainID: "vexo-test"}, txs)
	if got := interrupting.calls.Load(); got < 1 || got > 4 {
		t.Fatalf("expected the pass to stop after at most one verification per goroutine, got %d verifications", got)
	}
}

// A panic of a verifier is left to the sequential pass, which raises it in the caller's goroutine
// as it always did; the pass beside it neither ends the process nor stops the other transactions.
func TestPreverifyLeavesAPanicToTheSequentialPass(t *testing.T) {
	withMaxProcs(t, 4)
	verifier := &panickingVerifier{}
	keeper := NewAnteKeeper(AnteConfig{SignatureVerifier: verifier})
	txs := make([]types.Tx, 0, 7)
	for index := range 6 {
		txs = append(txs, newTestAccount(t).signed(t, "vexo-test", fmt.Sprintf("ok%d", index), 1))
	}
	boom := newTestAccount(t).signed(t, "vexo-test", "boom", 1)
	txs = append(txs, boom)

	keeper.preverify(Context{ChainID: "vexo-test"}, txs)
	if got := keeper.verified.Len(); got != 6 {
		t.Fatalf("the transactions beside the one that panicked must be remembered, got %d", got)
	}
	mustPanic(t, "the sequential pass", func() { _, _ = keeper.admit(Context{ChainID: "vexo-test"}, boom) })
}

// The cache belongs to its keeper: a proof made by one keeper, with its verifier, never answers
// another.
func TestEachAnteKeeperRemembersOnlyItsOwnAuthentications(t *testing.T) {
	tx := newTestAccount(t).signed(t, "vexo-test", "ok", 1)
	accepting := NewAnteKeeper(AnteConfig{RequireSigned: true})
	rejecting := NewAnteKeeper(AnteConfig{RequireSigned: true, SignatureVerifier: rejectingTxVerifier{}})

	if _, err := accepting.admit(Context{ChainID: "vexo-test"}, tx); err != nil {
		t.Fatal(err)
	}
	if _, err := rejecting.admit(Context{ChainID: "vexo-test"}, tx); !errors.Is(err, ErrInvalidTxSignature) {
		t.Fatalf("a keeper must not accept on the word of another, got %v", err)
	}
	if _, err := accepting.admit(Context{ChainID: "vexo-test"}, tx); err != nil {
		t.Fatalf("the first keeper still remembers its own success, got %v", err)
	}
}

// Everything a digest is made of is part of it, and no two ways to split the same bytes into its
// parts make the same digest.
func TestVerifyDigestsCommitToEveryPart(t *testing.T) {
	tx := types.Tx("tx")
	base := NewVerifyDigest("kind", []byte("scope"), tx)
	if again := NewVerifyDigest("kind", []byte("scope"), types.Tx("tx")); again != base {
		t.Fatal("equal parts must make equal digests")
	}
	for name, other := range map[string]VerifyDigest{
		"kind":               NewVerifyDigest("kinD", []byte("scope"), tx),
		"scope":              NewVerifyDigest("kind", []byte("scopf"), tx),
		"transaction":        NewVerifyDigest("kind", []byte("scope"), types.Tx("ty")),
		"kind and scope":     NewVerifyDigest("kinds", []byte("cope"), tx),
		"scope and tx":       NewVerifyDigest("kind", []byte("scop"), types.Tx("etx")),
		"no scope":           NewVerifyDigest("kind", nil, types.Tx("scopetx")),
		"kind folded in":     NewVerifyDigest("", []byte("kindscope"), tx),
		"scope after the tx": NewVerifyDigest("kind", tx, types.Tx("scope")),
	} {
		if other == base {
			t.Fatalf("%s: digests of different verifications must differ", name)
		}
	}
	if bytes.Equal(verifyScope("ab", []byte("c")), verifyScope("a", []byte("bc"))) {
		t.Fatal("a chain id and the inputs after it must not blend into one another")
	}
	if NewVerifyDigest(verifyKindSigned, verifyScope("c", nil), tx) == NewVerifyDigest(verifyKindAuthenticator, verifyScope("c", nil), tx) {
		t.Fatal("a signed envelope and an authenticator must not share a digest")
	}
}

func TestVerifyCacheStaysWithinItsCapacity(t *testing.T) {
	cache := NewVerifyCache(4)
	digests := make([]VerifyDigest, 10)
	for index := range digests {
		digests[index] = NewVerifyDigest("test", nil, types.Tx{byte(index)})
		cache.Add(digests[index])
		if got := cache.Len(); got > 4 {
			t.Fatalf("the cache holds %d verifications after %d insertions", got, index+1)
		}
	}
	for index, digest := range digests {
		if got, want := cache.Contains(digest), index >= 6; got != want {
			t.Fatalf("digest %d: remembered=%t, want %t: the oldest insertions are the ones to go", index, got, want)
		}
	}

	// Adding what is remembered neither grows the cache nor makes it any younger.
	cache.Add(digests[6])
	cache.Add(digests[9])
	if cache.Len() != 4 {
		t.Fatalf("expected the cache to stay at four, got %d", cache.Len())
	}
	cache.Add(NewVerifyDigest("test", nil, types.Tx{100}))
	if cache.Contains(digests[6]) || !cache.Contains(digests[7]) {
		t.Fatal("the oldest verification must be the one a new one replaces, whatever was added again since")
	}
}

func TestVerifyCacheHoldsAtLeastOneVerification(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		cache := NewVerifyCache(capacity)
		first, second := NewVerifyDigest("test", nil, types.Tx("a")), NewVerifyDigest("test", nil, types.Tx("b"))
		cache.Add(first)
		cache.Add(second)
		if cache.Len() != 1 || cache.Contains(first) || !cache.Contains(second) {
			t.Fatalf("capacity %d: expected the cache to hold the latest verification only, holds %d", capacity, cache.Len())
		}
	}
	var none *VerifyCache
	none.Add(NewVerifyDigest("test", nil, types.Tx("a")))
	if none.Contains(NewVerifyDigest("test", nil, types.Tx("a"))) || none.Len() != 0 {
		t.Fatal("a nil cache remembers nothing")
	}
}

func TestVerifyCacheIsSafeForConcurrentUse(t *testing.T) {
	cache := NewVerifyCache(64)
	var group sync.WaitGroup
	for worker := range 8 {
		group.Go(func() {
			for index := range 2000 {
				digest := NewVerifyDigest("test", binary.BigEndian.AppendUint32(nil, uint32(worker)), binary.BigEndian.AppendUint32(nil, uint32(index)))
				cache.Add(digest)
				if got := cache.Len(); got > 64 {
					t.Errorf("the cache holds %d verifications", got)
					return
				}
			}
		})
	}
	group.Wait()
	if got := cache.Len(); got != 64 {
		t.Fatalf("expected a full cache of 64, got %d", got)
	}
}

// What a full cache forgets is verified again, with the verdict it had.
func TestEvictedAuthenticationsAreVerifiedAgain(t *testing.T) {
	verifier := &countingVerifier{}
	keeper := NewAnteKeeper(AnteConfig{SignatureVerifier: verifier})
	keeper.verified = NewVerifyCache(2)
	txs := make([]types.Tx, 0, 3)
	for index := range 3 {
		txs = append(txs, newTestAccount(t).signed(t, "vexo-test", fmt.Sprintf("ok%d", index), 1))
	}

	for _, tx := range txs {
		if err := keeper.verifySigned("vexo-test", tx); err != nil {
			t.Fatal(err)
		}
	}
	if keeper.verified.Len() != 2 || verifier.calls.Load() != 3 {
		t.Fatalf("expected two remembered after three verifications, got %d after %d", keeper.verified.Len(), verifier.calls.Load())
	}
	if err := keeper.verifySigned("vexo-test", txs[2]); err != nil || verifier.calls.Load() != 3 {
		t.Fatalf("the latest verification is still remembered, got %v after %d verifications", err, verifier.calls.Load())
	}
	if err := keeper.verifySigned("vexo-test", txs[0]); err != nil || verifier.calls.Load() != 4 {
		t.Fatalf("the evicted transaction must verify again and succeed, got %v after %d verifications", err, verifier.calls.Load())
	}
	if keeper.verified.Len() != 2 {
		t.Fatalf("the cache must stay bounded, holds %d", keeper.verified.Len())
	}
}

// mixedBlock is a block of every kind of transaction the ante handler tells apart, with the
// accounts that pay for them.
type mixedBlock struct {
	funded []types.Address
	txs    []types.Tx
}

func newMixedBlock(t *testing.T) mixedBlock {
	t.Helper()
	var block mixedBlock
	for index := range 12 {
		account := newTestAccount(t)
		block.funded = append(block.funded, account.account)
		block.txs = append(block.txs,
			account.signed(t, "vexo-test", fmt.Sprintf("signed%d", index), 1),
			account.signed(t, "vexo-test", fmt.Sprintf("signed%db", index), 2))
	}
	for index := range 4 {
		account := newTestAccount(t)
		block.funded = append(block.funded, account.account)
		block.txs = append(block.txs, account.forged(t, "vexo-test", fmt.Sprintf("forged%d", index), 1))
	}
	elsewhere := newTestAccount(t)
	block.funded = append(block.funded, elsewhere.account)
	block.txs = append(block.txs, elsewhere.signed(t, "other-chain", "elsewhere", 1))
	// The same signed transaction twice: the second is a replay.
	block.txs = append(block.txs, block.txs[0])
	for index := range 8 {
		sender := fmt.Sprintf("0x%040x", 0xe000+index)
		block.funded = append(block.funded, types.Address(sender))
		block.txs = append(block.txs, ethereumTx(fmt.Sprintf("eth%d", index), sender, 0, "01"))
	}
	for index := range 3 {
		sender := fmt.Sprintf("0x%040x", 0xbad0+index)
		block.funded = append(block.funded, types.Address(sender))
		block.txs = append(block.txs, ethereumTx(fmt.Sprintf("bad%d", index), sender, 0, "bad"))
	}
	for index := range 3 {
		sender := fmt.Sprintf("plain%d", index)
		block.funded = append(block.funded, types.Address(sender))
		block.txs = append(block.txs, types.Tx(fmt.Sprintf("bank:plain%d:fee=5:gas=20:signer=%s:nonce=1", index, sender)))
	}
	block.txs = append(block.txs,
		types.Tx(""),
		types.Tx(txcodec.SignedPrefix+"@@@"),
		types.Tx("nobody:ok:fee=1:gas=20:signer=x:nonce=1"))
	return block
}

// executeMixedBlock validates and executes block on a runtime of its own, with the keeper
// prepared by setup, and returns what the stages said.
func executeMixedBlock(t *testing.T, block mixedBlock, setup func(*AnteKeeper)) (ProcessProposalResponse, FinalizeBlockResponse, *AnteKeeper) {
	t.Helper()
	storage := openAnteStore(t)
	for _, account := range block.funded {
		fundAccount(t, storage, account, 1000)
	}
	keeper := NewAnteKeeper(AnteConfig{
		MinFee:                1,
		FeeCollector:          "treasury",
		RequireNonce:          true,
		SignatureVerifier:     &countingVerifier{},
		EthereumAuthenticator: &cacheableAuthenticatorStub{},
	})
	setup(keeper)
	rt, err := NewRuntime("vexo-test", []Module{&scriptedModule{name: "bank"}, &scriptedModule{name: "evm"}}, PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	rt.WithStore(storage).WithAnte(keeper)
	proposed := blockAt(1, block.txs...)
	validation := rt.ProcessProposal(ProcessProposalRequest{Block: proposed})
	execution, err := rt.FinalizeBlock(FinalizeBlockRequest{Block: proposed})
	if err != nil {
		t.Fatal(err)
	}
	return validation, execution, keeper
}

// The cache and the parallel pass change how long admission takes and nothing it says: a block of
// genuine, forged and foreign signed transactions, Ethereum transactions that authenticate and
// that do not, unsigned ones and junk is validated and executed to the same results and the same
// state with them as without, whatever the number of processors.
func TestParallelPreverificationDoesNotChangeTheOutcomeOfABlock(t *testing.T) {
	block := newMixedBlock(t)
	baselineValidation, baseline, _ := executeMixedBlock(t, block, func(keeper *AnteKeeper) { keeper.verified = nil })
	if baselineValidation.Accepted {
		t.Fatal("the block carries invalid transactions, the baseline must not accept it")
	}
	var succeeded, failed int
	for _, result := range baseline.Results {
		if result.Code == 0 {
			succeeded++
		} else {
			failed++
		}
	}
	if succeeded == 0 || failed == 0 {
		t.Fatalf("the block must mix executed and failed transactions, got %d and %d", succeeded, failed)
	}

	for _, procs := range []int{2, 4, 8} {
		t.Run(fmt.Sprintf("procs=%d", procs), func(t *testing.T) {
			withMaxProcs(t, procs)
			validation, execution, keeper := executeMixedBlock(t, block, func(*AnteKeeper) {})
			if validation != baselineValidation {
				t.Fatalf("validation differs from the sequential one: %+v, want %+v", validation, baselineValidation)
			}
			if !reflect.DeepEqual(execution, baseline) {
				t.Fatalf("execution differs from the sequential one:\n got %+v\nwant %+v", execution, baseline)
			}
			// 24 genuine signed transactions and 8 Ethereum ones passed their authentication, and
			// nothing else was remembered.
			if got := keeper.verified.Len(); got != 24+8 {
				t.Fatalf("expected the 32 successful authentications to be remembered, got %d", got)
			}
		})
	}
}

// The error a block is rejected for is the first one in the order of its transactions, as it is
// without the parallel pass.
func TestParallelPreverificationKeepsTheErrorPrecedenceOfAdmission(t *testing.T) {
	withMaxProcs(t, 4)
	storage := openAnteStore(t)
	genuine := signedBlock(t, storage, 6)
	account := newTestAccount(t)
	fundAccount(t, storage, account.account, 1000)
	forged := account.forged(t, "vexo-test", "forged", 1)
	elsewhere := newTestAccount(t).signed(t, "other-chain", "elsewhere", 1)
	txs := append(append([]types.Tx{}, genuine...), forged, elsewhere)
	block := blockAt(1, txs...)

	reasons := map[bool]string{}
	for _, cached := range []bool{false, true} {
		keeper := NewAnteKeeper(AnteConfig{MinFee: 1, RequireNonce: true, RequireSigned: true, FeeCollector: "treasury"})
		if !cached {
			keeper.verified = nil
		}
		rt, err := NewRuntime("vexo-test", []Module{&scriptedModule{name: "bank"}}, PrefixRouter{})
		if err != nil {
			t.Fatal(err)
		}
		rt.WithStore(storage).WithAnte(keeper)
		response := rt.ProcessProposal(ProcessProposalRequest{Block: block})
		if response.Accepted {
			t.Fatal("the block carries invalid transactions")
		}
		reasons[cached] = response.Reason
	}
	if reasons[true] != reasons[false] {
		t.Fatalf("the rejection reason differs: %q with the parallel pass, %q without", reasons[true], reasons[false])
	}
}

func BenchmarkVerifySigned(b *testing.B) {
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		b.Fatal(err)
	}
	account, err := address.AccountFromPublicKey(signer.PublicKey())
	if err != nil {
		b.Fatal(err)
	}
	tx, err := SignTx("vexo-test", types.Tx(fmt.Sprintf("bank:ok:fee=5:gas=20:signer=%s:nonce=1", account)), signer)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("verify", func(b *testing.B) {
		for b.Loop() {
			if err := VerifySignedTx("vexo-test", tx, vexocrypto.Ed25519Signer{}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("remembered", func(b *testing.B) {
		keeper := NewAnteKeeper(AnteConfig{})
		if err := keeper.verifySigned("vexo-test", tx); err != nil {
			b.Fatal(err)
		}
		for b.Loop() {
			if err := keeper.verifySigned("vexo-test", tx); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkVerifyBlock measures the signature work of a block that no stage has seen yet: the
// sequential pass of admission alone against the parallel pass in front of it.
func BenchmarkVerifyBlock(b *testing.B) {
	const size = 2000
	txs := make([]types.Tx, 0, size)
	for index := range size {
		signer, err := vexocrypto.GenerateEd25519Signer()
		if err != nil {
			b.Fatal(err)
		}
		account, err := address.AccountFromPublicKey(signer.PublicKey())
		if err != nil {
			b.Fatal(err)
		}
		tx, err := SignTx("vexo-test", types.Tx(fmt.Sprintf("bank:ok%d:fee=5:gas=20:signer=%s:nonce=1", index, account)), signer)
		if err != nil {
			b.Fatal(err)
		}
		txs = append(txs, tx)
	}
	ctx := Context{ChainID: "vexo-test"}
	b.Run("sequential", func(b *testing.B) {
		for b.Loop() {
			keeper := NewAnteKeeper(AnteConfig{})
			for _, tx := range txs {
				if err := keeper.verifySigned("vexo-test", tx); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		for b.Loop() {
			keeper := NewAnteKeeper(AnteConfig{})
			keeper.preverify(ctx, txs)
			for _, tx := range txs {
				if err := keeper.verifySigned("vexo-test", tx); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
