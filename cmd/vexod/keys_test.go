package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
)

func TestRunKeysGenAndShow(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	if !strings.Contains(output, "generated validator key") || !strings.Contains(output, "public_key:") {
		t.Fatalf("unexpected keys gen output:\n%s", output)
	}
	if strings.Contains(output, "private_key") {
		t.Fatalf("key generation output leaked private key:\n%s", output)
	}
	keyPath := filepath.Join(home, keyFileName)
	document, err := vexocrypto.LoadKeyDocument(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Ed25519Signer(); err != nil {
		t.Fatal(err)
	}

	buffer.Reset()
	if err := runKeys(&buffer, []string{"show", "--home", home}); err != nil {
		t.Fatal(err)
	}
	showOutput := buffer.String()
	if !strings.Contains(showOutput, "validator key") || !strings.Contains(showOutput, document.PublicKey) {
		t.Fatalf("unexpected keys show output:\n%s", showOutput)
	}
	if strings.Contains(showOutput, document.PrivateKey) || strings.Contains(showOutput, "private_key") {
		t.Fatalf("key show output leaked private key:\n%s", showOutput)
	}
}

func TestRunKeysShowJSON(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"show", "--home", home, "--json"}); err != nil {
		t.Fatal(err)
	}
	var info keyInfoDocument
	if err := json.Unmarshal(buffer.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.SchemaVersion != vexocrypto.KeyDocumentVersionV1 || info.Type != vexocrypto.KeyTypeEd25519 || info.PublicKey == "" {
		t.Fatalf("unexpected key info: %+v", info)
	}
	if strings.Contains(buffer.String(), "private_key") {
		t.Fatalf("key json output leaked private key:\n%s", buffer.String())
	}
}

func TestRunKeysGenEncryptedAndShow(t *testing.T) {
	home := t.TempDir()
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"gen", "--home", home, "--encrypt", "--passphrase", "secret", "--id", "key-1", "--active-from", "10", "--active-until", "20"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "encrypted: true") {
		t.Fatalf("expected encrypted output, got:\n%s", buffer.String())
	}
	document, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if document.PrivateKey != "" || document.Encryption == nil {
		t.Fatalf("expected encrypted key on disk: %+v", document)
	}
	buffer.Reset()
	if err := runKeys(&buffer, []string{"show", "--home", home, "--passphrase", "secret", "--json"}); err != nil {
		t.Fatal(err)
	}
	var info keyInfoDocument
	if err := json.Unmarshal(buffer.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if !info.Encrypted || info.KeyID != "key-1" || info.ActiveFrom != 10 || info.ActiveUntil != 20 {
		t.Fatalf("unexpected encrypted key info: %+v", info)
	}
	if strings.Contains(buffer.String(), "private_key") {
		t.Fatalf("key json output leaked private key:\n%s", buffer.String())
	}
}

func TestRunKeysRemoteAndShow(t *testing.T) {
	home := t.TempDir()
	signer, err := vexocrypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatal(err)
	}
	encodedPublicKey := base64.StdEncoding.EncodeToString(signer.PublicKey())

	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"remote", "--home", home, "--public-key", encodedPublicKey, "--url", "http://127.0.0.1:9000/sign", "--id", "remote-1", "--active-from", "10"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "registered remote validator key") {
		t.Fatalf("unexpected remote key output:\n%s", buffer.String())
	}
	document, err := vexocrypto.LoadKeyDocument(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if document.Type != vexocrypto.KeyTypeRemote || document.Metadata.RemoteURL == "" || document.Metadata.ActiveFrom != 10 {
		t.Fatalf("unexpected remote key document: %+v", document)
	}

	buffer.Reset()
	if err := runKeys(&buffer, []string{"show", "--home", home, "--json"}); err != nil {
		t.Fatal(err)
	}
	var info keyInfoDocument
	if err := json.Unmarshal(buffer.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Type != vexocrypto.KeyTypeRemote || info.RemoteURL != "http://127.0.0.1:9000/sign" || info.KeyID != "remote-1" {
		t.Fatalf("unexpected remote key info: %+v", info)
	}
}

func TestRunKeysShowEncryptedRejectsWrongPassphrase(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--encrypt", "--passphrase", "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"show", "--home", home, "--passphrase", "wrong"}); err == nil {
		t.Fatal("expected wrong passphrase error")
	}
}

func TestRunKeysGenRejectsExistingUnlessOverwrite(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err == nil {
		t.Fatal("expected keys gen to reject existing key")
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home, "--overwrite"}); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(home, keyFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("expected overwrite to generate a new key")
	}
}

func TestRunKeysSupportsExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.key.json")
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--path", path}); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := runKeys(&buffer, []string{"show", "--path", path}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), path) {
		t.Fatalf("expected explicit path in output:\n%s", buffer.String())
	}
}

func TestRunKeysSignTx(t *testing.T) {
	home := t.TempDir()
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runKeys(&output, []string{"sign-tx", "--home", home, "--chain-id", "vexo-test", "--tx", "bank:send:alice:bob:1:fee=1:gas=1:signer=alice:nonce=1"}); err != nil {
		t.Fatal(err)
	}
	signedTx := strings.TrimSpace(strings.TrimPrefix(output.String(), "tx: "))
	if !vexoapp.IsSignedTx([]byte(signedTx)) {
		t.Fatalf("expected signed tx output, got %s", output.String())
	}
	if err := vexoapp.VerifySignedTx("vexo-test", []byte(signedTx), vexocrypto.Ed25519Signer{}); err != nil {
		t.Fatal(err)
	}
}

func TestRunKeysRejectsInvalidCommandsAndKeys(t *testing.T) {
	if err := runKeys(&bytes.Buffer{}, nil); err == nil {
		t.Fatal("expected missing keys subcommand error")
	}
	if err := runKeys(&bytes.Buffer{}, []string{"unknown"}); err == nil {
		t.Fatal("expected unknown keys subcommand error")
	}
	if err := runKeys(&bytes.Buffer{}, []string{"show", "--home", t.TempDir()}); err == nil {
		t.Fatal("expected missing key error")
	}
	path := filepath.Join(t.TempDir(), keyFileName)
	if err := os.WriteFile(path, []byte(`{"schema_version":"v1","type":"ed25519","public_key":"bad","private_key":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"show", "--path", path}); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestResolveKeyPath(t *testing.T) {
	if got := resolveKeyPath("", "custom.key"); got != "custom.key" {
		t.Fatalf("expected explicit key path, got %q", got)
	}
	if got := resolveKeyPath("/tmp/vexo", ""); got != filepath.Join("/tmp/vexo", keyFileName) {
		t.Fatalf("unexpected key path: %q", got)
	}
}
