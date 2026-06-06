package crypto

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEd25519KeyDocumentRoundTrip(t *testing.T) {
	document, err := GenerateEd25519KeyDocument()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := document.Ed25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !signer.Verify(signer.PublicKey(), message, signature) {
		t.Fatal("expected loaded signer to verify")
	}
	if document.SchemaVersion != KeyDocumentVersionV1 || document.Type != KeyTypeEd25519 {
		t.Fatalf("unexpected key document: %+v", document)
	}
}

func TestSaveAndLoadKeyDocument(t *testing.T) {
	document, err := GenerateEd25519KeyDocument()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "validator.key.json")
	if err := SaveKeyDocument(path, document); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadKeyDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PublicKey != document.PublicKey || loaded.PrivateKey != document.PrivateKey {
		t.Fatalf("unexpected loaded key: %+v", loaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 key permissions, got %v", info.Mode().Perm())
	}
	if err := SaveKeyDocument(path, document); err == nil {
		t.Fatal("expected save to reject existing key")
	}
}

func TestEncryptedKeyDocumentRoundTrip(t *testing.T) {
	document, err := GenerateEd25519KeyDocument()
	if err != nil {
		t.Fatal(err)
	}
	document.Metadata = KeyMetadata{ID: "validator-key", ActiveFrom: 10, ActiveUntil: 20}
	encrypted, err := document.Encrypted("secret")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted.PrivateKey != "" || encrypted.Encryption == nil {
		t.Fatalf("expected encrypted document without plaintext key: %+v", encrypted)
	}
	if encrypted.Encryption.KDF != KeyKDFArgon2ID ||
		encrypted.Encryption.Iterations < 2 ||
		encrypted.Encryption.MemoryKiB < 19*1024 ||
		encrypted.Encryption.Parallelism < 1 {
		t.Fatalf("expected hardened key encryption params: %+v", encrypted.Encryption)
	}
	salt, err := base64.StdEncoding.DecodeString(encrypted.Encryption.Salt)
	if err != nil {
		t.Fatal(err)
	}
	if len(salt) != 32 {
		t.Fatalf("expected 32-byte salt, got %d", len(salt))
	}
	if _, err := encrypted.Ed25519Signer(); !errors.Is(err, ErrEncryptedKey) {
		t.Fatalf("expected encrypted key error, got %v", err)
	}
	signer, err := encrypted.Ed25519SignerWithPassphrase("secret")
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("vote")
	signature, err := signer.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	if !signer.Verify(signer.PublicKey(), message, signature) {
		t.Fatal("expected decrypted signer to verify")
	}
	if _, err := encrypted.Ed25519SignerWithPassphrase("wrong"); err == nil {
		t.Fatal("expected wrong passphrase to fail")
	}
	record, err := encrypted.KeyRecordWithPassphrase("secret")
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "validator-key" || record.ActiveFrom != 10 || record.ActiveUntil != 20 {
		t.Fatalf("unexpected key record: %+v", record)
	}
	decrypted, err := encrypted.Decrypted("secret")
	if err != nil {
		t.Fatal(err)
	}
	if decrypted.PrivateKey == "" || decrypted.Encryption != nil {
		t.Fatalf("expected decrypted document with plaintext key: %+v", decrypted)
	}
}

func TestEncryptedKeyDocumentRejectsMissingPassphrase(t *testing.T) {
	document, err := GenerateEd25519KeyDocument()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Encrypted(""); !errors.Is(err, ErrMissingPassphrase) {
		t.Fatalf("expected missing passphrase, got %v", err)
	}
	encrypted, err := document.Encrypted("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encrypted.Ed25519SignerWithPassphrase(""); !errors.Is(err, ErrMissingPassphrase) {
		t.Fatalf("expected missing passphrase, got %v", err)
	}
}

func TestRemoteKeyDocumentBuildsRemoteSigner(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	document := KeyDocument{
		SchemaVersion: KeyDocumentVersionV1,
		Type:          KeyTypeRemote,
		PublicKey:     base64.StdEncoding.EncodeToString(signer.PublicKey()),
		Metadata:      KeyMetadata{ID: "remote-1", RemoteURL: "http://127.0.0.1:9000/sign", ActiveFrom: 5},
	}
	record, err := document.KeyRecordWithPassphrase("")
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "remote-1" || record.ActiveFrom != 5 {
		t.Fatalf("unexpected remote key record: %+v", record)
	}
	if string(record.Signer.PublicKey()) != string(signer.PublicKey()) {
		t.Fatal("expected remote signer public key")
	}
}

func TestRemoteKeyDocumentAppliesPolicyAndGuardMetadata(t *testing.T) {
	signer, err := GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	guardPath := filepath.Join(t.TempDir(), "remote_guard.json")
	document := KeyDocument{
		SchemaVersion: KeyDocumentVersionV1,
		Type:          KeyTypeRemote,
		PublicKey:     base64.StdEncoding.EncodeToString(signer.PublicKey()),
		Metadata: KeyMetadata{
			ID:            "remote-guarded",
			RemoteURL:     "http://127.0.0.1:9000/sign",
			RequirePolicy: true,
			GuardPath:     guardPath,
		},
	}
	remoteSigner, err := document.RemoteSigner(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := remoteSigner.Sign([]byte("raw-message")); !errors.Is(err, ErrMissingSignPolicy) {
		t.Fatalf("expected raw signing to require policy, got %v", err)
	}
	if remoteSigner.guard == nil || remoteSigner.guard.snapshotPath != guardPath {
		t.Fatalf("expected file-backed guard from metadata, got %+v", remoteSigner.guard)
	}
}

func TestKeyDocumentRejectsInvalidData(t *testing.T) {
	document, err := GenerateEd25519KeyDocument()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		document KeyDocument
		expected error
	}{
		{name: "schema", document: withKeySchema(document, "v0"), expected: ErrUnsupportedKeyVersion},
		{name: "type", document: withKeyType(document, "unknown"), expected: ErrUnsupportedKeyType},
		{name: "public key", document: withPublicKey(document, "not-base64")},
		{name: "private key", document: withPrivateKey(document, "not-base64")},
		{name: "mismatched key", document: withPublicKey(document, base64.StdEncoding.EncodeToString([]byte("short")))},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := testCase.document.Ed25519Signer()
			if testCase.expected != nil && !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
			if testCase.expected == nil && err == nil {
				t.Fatal("expected invalid key document error")
			}
		})
	}
}

func withKeySchema(document KeyDocument, schema string) KeyDocument {
	document.SchemaVersion = schema
	return document
}

func withKeyType(document KeyDocument, keyType string) KeyDocument {
	document.Type = keyType
	return document
}

func withPublicKey(document KeyDocument, publicKey string) KeyDocument {
	document.PublicKey = publicKey
	return document
}

func withPrivateKey(document KeyDocument, privateKey string) KeyDocument {
	document.PrivateKey = privateKey
	return document
}
