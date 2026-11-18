package evm

import (
	"encoding/binary"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/types"
)

// An Ethereum transaction is checked against the raw transaction it carries (its signature, the
// sender the signature recovers to and the fee caps) by every admission of it: CheckTx, the
// proposal that selects it, the validation of the block that carries it and the execution of that
// block each authenticate it, validate it and, on execution, deliver it. The check is a pure
// function of the transaction bytes and of the options below, so the module remembers the
// transactions that passed it, and so does the ante handler through AuthenticationKey. The
// recovery of the sender is done once per transaction and set of options instead of once per
// stage.

// rawTxKind is the kind of verification the module remembers.
const rawTxKind = "evm/raw-tx"

// validateRawTx checks an Ethereum transaction against the raw transaction it carries under
// options. It is a variable so that a test can count how often a signature is recovered.
var validateRawTx = ethcompat.ValidateCanonicalTxForExecution

var _ vexoapp.CacheableAuthenticator = Module{}

// authenticationInputs mirrors ethcompat.DecodeOptions field for field. Converting options to it
// stops the module from compiling when an option is added, until this type, and so the encoding
// below, is brought along.
type authenticationInputs struct {
	ChainID                uint64
	BaseFee                uint64
	BlobBaseFee            uint64
	VM                     string
	AllowUnprotectedLegacy bool
}

// encode returns an encoding of every input. Two sets of inputs share an encoding only if they
// are equal.
func (inputs authenticationInputs) encode() []byte {
	encoded := make([]byte, 0, 3*8+binary.MaxVarintLen64+len(inputs.VM)+1)
	encoded = binary.BigEndian.AppendUint64(encoded, inputs.ChainID)
	encoded = binary.BigEndian.AppendUint64(encoded, inputs.BaseFee)
	encoded = binary.BigEndian.AppendUint64(encoded, inputs.BlobBaseFee)
	encoded = binary.AppendUvarint(encoded, uint64(len(inputs.VM)))
	encoded = append(encoded, inputs.VM...)
	if inputs.AllowUnprotectedLegacy {
		return append(encoded, 1)
	}
	return append(encoded, 0)
}

// rawTxDigest names the verification of tx under options: the digest the module remembers a
// passed transaction by.
func rawTxDigest(options ethcompat.DecodeOptions, tx types.Tx) vexoapp.VerifyDigest {
	return vexoapp.NewVerifyDigest(rawTxKind, authenticationInputs(options).encode(), tx)
}

// AuthenticationKey makes the module a vexoapp.CacheableAuthenticator: it names every input of
// AuthenticateTx besides the transaction bytes and the chain id, namely the options the raw
// transaction is decoded under. They include the base fee and the blob base fee, which the fee
// caps of the raw transaction are checked against, so a transaction authenticated before the base
// fee rose is authenticated afresh and rejected if its cap no longer covers it.
//
// AuthenticateTx reads nothing else of the context (no store, height or header), and the module
// is immutable once built, so the verdict is a function of the transaction bytes and this key.
// Whoever makes AuthenticateTx read more must put it in the key.
func (module Module) AuthenticationKey(ctx vexoapp.Context, _ types.Tx) ([]byte, bool) {
	if module.initErr != nil {
		return nil, false
	}
	return authenticationInputs(module.decodeOptions(ctx)).encode(), true
}
