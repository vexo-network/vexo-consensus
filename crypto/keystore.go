package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"os"
	"time"
)

const (
	KeyTypeEd25519       = "ed25519"
	KeyTypeBLS           = "bls"
	KeyTypeRemote        = "remote"
	KeyDocumentVersionV1 = "v1"
	KeyEncryptionAESGCM  = "aes-256-gcm"
	KeyKDFPBKDF2SHA256   = "pbkdf2-sha256"
	defaultKDFIterations = 100_000
)

var (
	ErrUnsupportedKeyType    = errors.New("unsupported key type")
	ErrUnsupportedKeyVersion = errors.New("unsupported key version")
	ErrEncryptedKey          = errors.New("key is encrypted")
	ErrUnencryptedKey        = errors.New("key is not encrypted")
	ErrMissingPassphrase     = errors.New("key passphrase is required")
	ErrInvalidKeyEncryption  = errors.New("invalid key encryption")
)

type KeyDocument struct {
	SchemaVersion string         `json:"schema_version"`
	Type          string         `json:"type"`
	PublicKey     string         `json:"public_key"`
	PrivateKey    string         `json:"private_key,omitempty"`
	Encryption    *KeyEncryption `json:"encryption,omitempty"`
	Metadata      KeyMetadata    `json:"metadata,omitempty"`
}

type KeyMetadata struct {
	ID            string `json:"id,omitempty"`
	ActiveFrom    uint64 `json:"active_from,omitempty"`
	ActiveUntil   uint64 `json:"active_until,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	RemoteURL     string `json:"remote_url,omitempty"`
	AuthTokenEnv  string `json:"auth_token_env,omitempty"`
	RequirePolicy bool   `json:"require_policy,omitempty"`
	GuardPath     string `json:"guard_path,omitempty"`
}

type KeyEncryption struct {
	Algorithm  string `json:"algorithm"`
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
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
	if document.Encryption != nil {
		return Ed25519Signer{}, ErrEncryptedKey
	}
	return document.Ed25519SignerWithPassphrase("")
}

func (document KeyDocument) Ed25519SignerWithPassphrase(passphrase string) (Ed25519Signer, error) {
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
	privateKeyString := document.PrivateKey
	if document.Encryption != nil {
		privateKeyBytes, err := decryptKeyMaterial(*document.Encryption, passphrase)
		if err != nil {
			return Ed25519Signer{}, err
		}
		return NewEd25519Signer(ed25519.PrivateKey(privateKeyBytes), ed25519.PublicKey(publicKey))
	}
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyString)
	if err != nil {
		return Ed25519Signer{}, fmt.Errorf("invalid private key: %w", err)
	}
	return NewEd25519Signer(ed25519.PrivateKey(privateKey), ed25519.PublicKey(publicKey))
}

func (document KeyDocument) Encrypted(passphrase string) (KeyDocument, error) {
	if document.Encryption != nil {
		return KeyDocument{}, ErrEncryptedKey
	}
	if passphrase == "" {
		return KeyDocument{}, ErrMissingPassphrase
	}
	privateKey, err := base64.StdEncoding.DecodeString(document.PrivateKey)
	if err != nil {
		return KeyDocument{}, fmt.Errorf("invalid private key: %w", err)
	}
	encryption, err := encryptKeyMaterial(privateKey, passphrase)
	if err != nil {
		return KeyDocument{}, err
	}
	document.PrivateKey = ""
	document.Encryption = &encryption
	return document, nil
}

func (document KeyDocument) Decrypted(passphrase string) (KeyDocument, error) {
	if document.Encryption == nil {
		return KeyDocument{}, ErrUnencryptedKey
	}
	privateKey, err := decryptKeyMaterial(*document.Encryption, passphrase)
	if err != nil {
		return KeyDocument{}, err
	}
	document.PrivateKey = base64.StdEncoding.EncodeToString(privateKey)
	document.Encryption = nil
	return document, nil
}

func (document KeyDocument) KeyRecordWithPassphrase(passphrase string) (KeyRecord, error) {
	signer, err := document.SignerWithPassphrase(passphrase)
	if err != nil {
		return KeyRecord{}, err
	}
	keyID := KeyID(document.Metadata.ID)
	if keyID == "" {
		keyID = KeyID(document.PublicKey)
	}
	return KeyRecord{
		ID:          keyID,
		Signer:      signer,
		ActiveFrom:  document.Metadata.ActiveFrom,
		ActiveUntil: document.Metadata.ActiveUntil,
	}, nil
}

func (document KeyDocument) SignerWithPassphrase(passphrase string) (Signer, error) {
	switch document.Type {
	case KeyTypeEd25519:
		if document.Encryption != nil {
			signer, err := document.Ed25519SignerWithPassphrase(passphrase)
			if err != nil {
				return nil, err
			}
			return signer, nil
		}
		signer, err := document.Ed25519Signer()
		if err != nil {
			return nil, err
		}
		return signer, nil
	case KeyTypeRemote:
		return document.RemoteSigner(5 * time.Second)
	default:
		return nil, ErrUnsupportedKeyType
	}
}

func (document KeyDocument) RemoteSigner(timeout time.Duration) (RemoteSigner, error) {
	if document.SchemaVersion != KeyDocumentVersionV1 {
		return RemoteSigner{}, ErrUnsupportedKeyVersion
	}
	if document.Type != KeyTypeRemote {
		return RemoteSigner{}, ErrUnsupportedKeyType
	}
	publicKey, err := base64.StdEncoding.DecodeString(document.PublicKey)
	if err != nil {
		return RemoteSigner{}, fmt.Errorf("invalid public key: %w", err)
	}
	authToken := ""
	if document.Metadata.AuthTokenEnv != "" {
		authToken = os.Getenv(document.Metadata.AuthTokenEnv)
	}
	return NewRemoteSignerWithAuth(document.Metadata.RemoteURL, publicKey, Ed25519Signer{}, timeout, nil, authToken)
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

func encryptKeyMaterial(privateKey []byte, passphrase string) (KeyEncryption, error) {
	if passphrase == "" {
		return KeyEncryption{}, ErrMissingPassphrase
	}
	salt := make([]byte, 16)
	nonce := make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return KeyEncryption{}, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return KeyEncryption{}, err
	}
	key := pbkdf2Key([]byte(passphrase), salt, defaultKDFIterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return KeyEncryption{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return KeyEncryption{}, err
	}
	ciphertext := aead.Seal(nil, nonce, privateKey, nil)
	return KeyEncryption{
		Algorithm:  KeyEncryptionAESGCM,
		KDF:        KeyKDFPBKDF2SHA256,
		Iterations: defaultKDFIterations,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func decryptKeyMaterial(encryption KeyEncryption, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, ErrMissingPassphrase
	}
	if encryption.Algorithm != KeyEncryptionAESGCM || encryption.KDF != KeyKDFPBKDF2SHA256 || encryption.Iterations <= 0 {
		return nil, ErrInvalidKeyEncryption
	}
	salt, err := base64.StdEncoding.DecodeString(encryption.Salt)
	if err != nil {
		return nil, ErrInvalidKeyEncryption
	}
	nonce, err := base64.StdEncoding.DecodeString(encryption.Nonce)
	if err != nil {
		return nil, ErrInvalidKeyEncryption
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encryption.Ciphertext)
	if err != nil {
		return nil, ErrInvalidKeyEncryption
	}
	key := pbkdf2Key([]byte(passphrase), salt, encryption.Iterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}

func pbkdf2Key(password []byte, salt []byte, iterations int, keyLen int, hashFunc func() hash.Hash) []byte {
	hashLength := hashFunc().Size()
	blockCount := (keyLen + hashLength - 1) / hashLength
	var derived bytes.Buffer
	for block := 1; block <= blockCount; block++ {
		mac := hmac.New(hashFunc, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		sum := mac.Sum(nil)
		output := append([]byte(nil), sum...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(hashFunc, password)
			mac.Write(sum)
			sum = mac.Sum(nil)
			for j := range output {
				output[j] ^= sum[j]
			}
		}
		derived.Write(output)
	}
	return derived.Bytes()[:keyLen]
}
