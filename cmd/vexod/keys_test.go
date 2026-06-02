package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
