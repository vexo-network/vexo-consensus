package crypto

import (
	"bytes"
	"crypto/sha256"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrEmptyPrivateKey = errors.New("private key is empty")
	ErrEmptyPublicKey  = errors.New("public key is empty")
	ErrEmptySignature  = errors.New("signature is empty")
)

type DeterministicSigner struct {
	privateKey []byte
	publicKey  types.PublicKey
}

func NewDeterministicSigner(privateKey []byte) (DeterministicSigner, error) {
	if len(privateKey) == 0 {
		return DeterministicSigner{}, ErrEmptyPrivateKey
	}
	publicKey := derivePublicKey(privateKey)
	return DeterministicSigner{
		privateKey: append([]byte(nil), privateKey...),
		publicKey:  publicKey,
	}, nil
}

func (signer DeterministicSigner) PublicKey() types.PublicKey {
	return append(types.PublicKey(nil), signer.publicKey...)
}

func (signer DeterministicSigner) Sign(message []byte) (types.Signature, error) {
	if len(signer.privateKey) == 0 {
		return nil, ErrEmptyPrivateKey
	}
	return signWithPublicKey(signer.publicKey, message), nil
}

func (signer DeterministicSigner) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	if len(publicKey) == 0 || len(signature) == 0 {
		return false
	}
	expected := signWithPublicKey(publicKey, message)
	return bytes.Equal(expected, signature)
}

type DeterministicAggregateSigner struct{}

func (DeterministicAggregateSigner) Aggregate(signatures []types.Signature) (types.AggregateSignature, error) {
	if len(signatures) == 0 {
		return nil, ErrEmptySignature
	}

	hasher := sha256.New()
	for _, signature := range signatures {
		if len(signature) == 0 {
			return nil, ErrEmptySignature
		}
		hasher.Write(signature)
	}
	return types.AggregateSignature(hasher.Sum(nil)), nil
}

func (signer DeterministicAggregateSigner) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	if len(publicKeys) == 0 || len(signature) == 0 {
		return false
	}

	signatures := make([]types.Signature, 0, len(publicKeys))
	for _, publicKey := range publicKeys {
		if len(publicKey) == 0 {
			return false
		}
		signatures = append(signatures, signWithPublicKey(publicKey, message))
	}
	expected, err := signer.Aggregate(signatures)
	if err != nil {
		return false
	}
	return bytes.Equal(expected, signature)
}

func derivePublicKey(privateKey []byte) types.PublicKey {
	sum := sha256.Sum256(privateKey)
	return append(types.PublicKey(nil), sum[:]...)
}

func signWithPublicKey(publicKey types.PublicKey, message []byte) types.Signature {
	hasher := sha256.New()
	hasher.Write(publicKey)
	hasher.Write(message)
	return types.Signature(hasher.Sum(nil))
}
