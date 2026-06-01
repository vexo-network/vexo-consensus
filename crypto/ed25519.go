package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInvalidEd25519PrivateKey = errors.New("invalid ed25519 private key")
	ErrInvalidEd25519PublicKey  = errors.New("invalid ed25519 public key")
)

type Ed25519Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

func GenerateEd25519Signer() (Ed25519Signer, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Ed25519Signer{}, err
	}
	return NewEd25519Signer(privateKey, publicKey)
}

func NewEd25519Signer(privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey) (Ed25519Signer, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Ed25519Signer{}, ErrInvalidEd25519PrivateKey
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return Ed25519Signer{}, ErrInvalidEd25519PublicKey
	}
	derivedPublicKey := privateKey.Public().(ed25519.PublicKey)
	if string(derivedPublicKey) != string(publicKey) {
		return Ed25519Signer{}, ErrInvalidEd25519PublicKey
	}
	return Ed25519Signer{
		privateKey: append(ed25519.PrivateKey(nil), privateKey...),
		publicKey:  append(ed25519.PublicKey(nil), publicKey...),
	}, nil
}

func (signer Ed25519Signer) PublicKey() types.PublicKey {
	return append(types.PublicKey(nil), signer.publicKey...)
}

func (signer Ed25519Signer) Sign(message []byte) (types.Signature, error) {
	if len(signer.privateKey) != ed25519.PrivateKeySize {
		return nil, ErrInvalidEd25519PrivateKey
	}
	return types.Signature(ed25519.Sign(signer.privateKey, message)), nil
}

func (signer Ed25519Signer) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}

type Ed25519MultiVerifier struct{}

func (Ed25519MultiVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	if len(publicKeys) == 0 {
		return false
	}
	expectedLength := len(publicKeys) * ed25519.SignatureSize
	if len(signature) != expectedLength {
		return false
	}
	for index, publicKey := range publicKeys {
		start := index * ed25519.SignatureSize
		end := start + ed25519.SignatureSize
		if !ed25519.Verify(ed25519.PublicKey(publicKey), message, signature[start:end]) {
			return false
		}
	}
	return true
}

func CombineEd25519Signatures(signatures []types.Signature) (types.AggregateSignature, error) {
	if len(signatures) == 0 {
		return nil, ErrEmptySignature
	}
	combined := make(types.AggregateSignature, 0, len(signatures)*ed25519.SignatureSize)
	for _, signature := range signatures {
		if len(signature) != ed25519.SignatureSize {
			return nil, ErrEmptySignature
		}
		combined = append(combined, signature...)
	}
	return combined, nil
}
