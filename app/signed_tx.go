package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInvalidSignedTx       = txcodec.ErrInvalidSignedTx
	ErrInvalidTxSignature    = errors.New("invalid transaction signature")
	ErrSignerAddressMismatch = errors.New("signer address does not match transaction public key")
)

// SignedTxEnvelope is the JSON document a signed transaction carries.
type SignedTxEnvelope = txcodec.Envelope

type TxSignatureVerifier interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

func SignTx(chainID string, payload types.Tx, signer vexocrypto.Signer) (types.Tx, error) {
	if err := validatePayloadSignerAddress(payload, signer.PublicKey()); err != nil {
		return nil, err
	}
	signature, err := signer.Sign(SignedTxSignBytes(chainID, payload))
	if err != nil {
		return nil, err
	}
	envelope := SignedTxEnvelope{
		SchemaVersion: "v1",
		ChainID:       chainID,
		Payload:       base64.StdEncoding.EncodeToString(payload),
		PublicKey:     base64.StdEncoding.EncodeToString(signer.PublicKey()),
		Signature:     base64.StdEncoding.EncodeToString(signature),
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return types.Tx(txcodec.SignedPrefix + base64.StdEncoding.EncodeToString(encoded)), nil
}

func VerifySignedTx(chainID string, tx types.Tx, verifier TxSignatureVerifier) error {
	envelope, payload, err := DecodeSignedTx(tx)
	if err != nil {
		return err
	}
	if verifier == nil {
		return ErrInvalidTxSignature
	}
	if envelope.ChainID != chainID {
		return ErrInvalidSignedTx
	}
	publicKey, err := base64.StdEncoding.DecodeString(envelope.PublicKey)
	if err != nil {
		return ErrInvalidSignedTx
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		return ErrInvalidSignedTx
	}
	if !verifier.Verify(publicKey, SignedTxSignBytes(chainID, payload), signature) {
		return ErrInvalidTxSignature
	}
	return validatePayloadSignerAddress(payload, publicKey)
}

// DecodeSignedTx decodes a signed transaction into its envelope and payload.
// A payload that is itself a signed transaction is rejected: nesting would let
// the outer and inner signer disagree about who authorized the transaction.
func DecodeSignedTx(tx types.Tx) (SignedTxEnvelope, types.Tx, error) {
	return txcodec.DecodeEnvelope(tx)
}

func IsSignedTx(tx types.Tx) bool {
	return txcodec.IsSigned(tx)
}

// TxPayload returns the payload of a signed transaction, or tx itself when it
// is not a decodable signed transaction. Consensus paths use txcodec.Unwrap,
// which rejects an undecodable signed transaction instead of reinterpreting it.
func TxPayload(tx types.Tx) types.Tx {
	payload, err := txcodec.Unwrap(tx)
	if err != nil {
		return tx
	}
	return payload
}

func SignedTxSignBytes(chainID string, payload types.Tx) []byte {
	signBytes := make([]byte, 0, len(chainID)+len(payload)+32)
	signBytes = append(signBytes, "vexo:signed-tx:v1\n"...)
	signBytes = append(signBytes, chainID...)
	signBytes = append(signBytes, '\n')
	signBytes = append(signBytes, payload...)
	return signBytes
}

// validatePayloadSignerAddress binds the signer tag of payload to publicKey. A
// payload without a signer tag is not bound to any account; a payload whose
// tags do not parse, or whose signer tag is empty, is rejected.
func validatePayloadSignerAddress(payload types.Tx, publicKey types.PublicKey) error {
	tags, err := txcodec.Tags(payload)
	if err != nil {
		return err
	}
	signer, found := tags[txcodec.TagSigner]
	if !found {
		return nil
	}
	expected, err := address.AccountFromPublicKey(publicKey)
	if err != nil {
		return err
	}
	if signer == "" || signer != string(expected) {
		return ErrSignerAddressMismatch
	}
	return nil
}
