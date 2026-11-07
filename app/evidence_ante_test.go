package app

import (
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

type evidenceAuthenticatorStub struct {
	err   error
	gas   uint64
	calls int
}

func (stub *evidenceAuthenticatorStub) AuthenticateTx(ctx Context, tx types.Tx) error {
	stub.calls++
	return stub.err
}

func (stub *evidenceAuthenticatorStub) EstimateGas(ctx Context, tx types.Tx) (uint64, error) {
	return stub.gas, nil
}

// Evidence is signed by the offender, so a network that requires signed transactions still
// admits a fee-less evidence transaction, but only one the slashing module authenticates.
func TestAnteAdmitsAuthenticatedUnsignedEvidenceWithoutFee(t *testing.T) {
	storage := openAnteStore(t)
	authenticator := &evidenceAuthenticatorStub{gas: 50}
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true, RequireNonce: true, MinFee: 10, BaseFee: 1, EvidenceAuthenticator: authenticator})
	ctx := Context{ChainID: "vexo-test", Store: storage, BaseFee: 1}
	evidence := types.Tx("slashing:evidence:abc")
	adm, err := keeper.admit(ctx, evidence)
	if err != nil {
		t.Fatalf("expected authenticated evidence to be admitted, got %v", err)
	}
	if adm.payer != "" || adm.gasLimit != 50 {
		t.Fatalf("evidence must pay nothing and use its execution gas, got payer=%q gas=%d", adm.payer, adm.gasLimit)
	}
	if err := keeper.newSimulation(ctx).apply(adm); err != nil {
		t.Fatalf("expected fee-less evidence to pass simulation, got %v", err)
	}
	if authenticator.calls != 1 {
		t.Fatalf("expected the slashing module to authenticate the evidence once, got %d", authenticator.calls)
	}
}

func TestAnteRejectsEvidenceTheModuleDoesNotAuthenticate(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true, EvidenceAuthenticator: &evidenceAuthenticatorStub{err: errors.New("bad proof"), gas: 50}})
	if err := checkTx(keeper, Context{ChainID: "vexo-test"}, types.Tx("slashing:evidence:abc")); !errors.Is(err, ErrInvalidTxSignature) {
		t.Fatalf("expected forged evidence to be rejected, got %v", err)
	}
	withoutAuthenticator := NewAnteKeeper(AnteConfig{RequireSigned: true})
	if err := checkTx(withoutAuthenticator, Context{ChainID: "vexo-test"}, types.Tx("slashing:evidence:abc")); !errors.Is(err, ErrInvalidSignedTx) {
		t.Fatalf("expected unsigned evidence without an authenticator to need a signature, got %v", err)
	}
}

// The exemption covers exactly slashing:evidence without tags: tags would name a signer, fee or
// nonce the exemption never checks, and no other module may ride on it.
func TestAnteEvidenceExemptionDoesNotCoverOtherTxs(t *testing.T) {
	keeper := NewAnteKeeper(AnteConfig{RequireSigned: true, EvidenceAuthenticator: &evidenceAuthenticatorStub{gas: 50}})
	for _, tx := range []string{
		"slashing:evidence:abc:signer=alice",
		"bank:mint:attacker:100",
		"slashing:other:abc",
	} {
		if err := checkTx(keeper, Context{ChainID: "vexo-test"}, types.Tx(tx)); !errors.Is(err, ErrInvalidSignedTx) {
			t.Fatalf("%s: expected the signature requirement to apply, got %v", tx, err)
		}
	}
}
