package app

import (
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

// ErrUnauthorizedActor is returned when a transaction names an actor (sender,
// delegator, voter, caller, ...) that is not the transaction signer.
var ErrUnauthorizedActor = errors.New("transaction signer is not authorized for actor")

// NormalizeActor returns the canonical identity used to compare a transaction
// signer with an actor named in transaction arguments. 0x-prefixed hex
// addresses of at most 20 bytes are left-padded and lowercased; every other
// identity compares verbatim.
func NormalizeActor(address types.Address) string {
	return string(bankBalanceKey(address))
}

// AuthorizeActor requires that tx carries a signer tag identifying actor.
// The ante handler authenticates the signer tag (signed envelope or verified
// Ethereum sender), so modules MUST call this before acting on behalf of an
// account named in positional arguments.
func AuthorizeActor(tx types.Tx, actor types.Address) error {
	if actor == "" {
		return ErrUnauthorizedActor
	}
	signer, found := TxTag(tx, "signer")
	if !found || signer == "" {
		return ErrUnauthorizedActor
	}
	if NormalizeActor(types.Address(signer)) != NormalizeActor(actor) {
		return ErrUnauthorizedActor
	}
	return nil
}
