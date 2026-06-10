package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/address"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
)

const signedTxPrefix = "signed:"

var (
	ErrInvalidSignedTx       = errors.New("invalid signed transaction")
	ErrInvalidTxSignature    = errors.New("invalid transaction signature")
	ErrSignerAddressMismatch = errors.New("signer address does not match transaction public key")
)

type SignedTxEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	ChainID       string `json:"chain_id"`
	Payload       string `json:"payload"`
	PublicKey     string `json:"public_key"`
	Signature     string `json:"signature"`
}

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
	return types.Tx(signedTxPrefix + base64.StdEncoding.EncodeToString(encoded)), nil
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
	if err := validatePayloadSignerAddress(payload, publicKey); err != nil {
		return err
	}
	return nil
}

func DecodeSignedTx(tx types.Tx) (SignedTxEnvelope, types.Tx, error) {
	if !IsSignedTx(tx) {
		return SignedTxEnvelope{}, nil, ErrInvalidSignedTx
	}
	encoded := strings.TrimPrefix(string(tx), signedTxPrefix)
	document, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return SignedTxEnvelope{}, nil, ErrInvalidSignedTx
	}
	var envelope SignedTxEnvelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		return SignedTxEnvelope{}, nil, ErrInvalidSignedTx
	}
	if envelope.SchemaVersion != "v1" || envelope.Payload == "" || envelope.PublicKey == "" || envelope.Signature == "" {
		return SignedTxEnvelope{}, nil, ErrInvalidSignedTx
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return SignedTxEnvelope{}, nil, ErrInvalidSignedTx
	}
	return envelope, types.Tx(payload), nil
}

func IsSignedTx(tx types.Tx) bool {
	return strings.HasPrefix(string(tx), signedTxPrefix)
}

func TxPayload(tx types.Tx) types.Tx {
	_, payload, err := DecodeSignedTx(tx)
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

func validatePayloadSignerAddress(payload types.Tx, publicKey types.PublicKey) error {
	signer, found := TxTag(payload, "signer")
	if !found || signer == "" {
		return nil
	}
	expected, err := address.AccountFromPublicKey(publicKey)
	if err != nil {
		return err
	}
	if signer != string(expected) {
		return ErrSignerAddressMismatch
	}
	return nil
}
