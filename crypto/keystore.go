package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const (
	KeyTypeEd25519       = "ed25519"
	KeyDocumentVersionV1 = "v1"
)

var (
	ErrUnsupportedKeyType    = errors.New("unsupported key type")
	ErrUnsupportedKeyVersion = errors.New("unsupported key version")
)

type KeyDocument struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
	PublicKey     string `json:"public_key"`
	PrivateKey    string `json:"private_key"`
}

func GenerateEd25519KeyDocument() (KeyDocument, error) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		return KeyDocument{}, err
	}
	return NewEd25519KeyDocument(signer)
}

func NewEd25519KeyDocument(signer Ed25519Signer) (KeyDocument, error) {
	privateKey := signer.privateKey
	if len(privateKey) != ed25519.PrivateKeySize {
		return KeyDocument{}, ErrInvalidEd25519PrivateKey
	}
	publicKey := signer.PublicKey()
	if len(publicKey) != ed25519.PublicKeySize {
		return KeyDocument{}, ErrInvalidEd25519PublicKey
	}
	return KeyDocument{
		SchemaVersion: KeyDocumentVersionV1,
		Type:          KeyTypeEd25519,
		PublicKey:     base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey:    base64.StdEncoding.EncodeToString(privateKey),
	}, nil
}

func (document KeyDocument) Ed25519Signer() (Ed25519Signer, error) {
	if document.SchemaVersion != KeyDocumentVersionV1 {
		return Ed25519Signer{}, ErrUnsupportedKeyVersion
	}
	if document.Type != KeyTypeEd25519 {
		return Ed25519Signer{}, ErrUnsupportedKeyType
	}
	publicKey, err := base64.StdEncoding.DecodeString(document.PublicKey)
	if err != nil {
		return Ed25519Signer{}, fmt.Errorf("invalid public key: %w", err)
	}
	privateKey, err := base64.StdEncoding.DecodeString(document.PrivateKey)
	if err != nil {
		return Ed25519Signer{}, fmt.Errorf("invalid private key: %w", err)
	}
	return NewEd25519Signer(ed25519.PrivateKey(privateKey), ed25519.PublicKey(publicKey))
}

func SaveKeyDocument(path string, document KeyDocument) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func LoadKeyDocument(path string) (KeyDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return KeyDocument{}, err
	}
	defer file.Close()
	var document KeyDocument
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return KeyDocument{}, err
	}
	return document, nil
}
