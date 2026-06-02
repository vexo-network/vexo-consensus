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
