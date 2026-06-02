package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
)

const keyFileName = "validator.key.json"

type keyInfoDocument struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
	PublicKey     string `json:"public_key"`
	Path          string `json:"path"`
	Encrypted     bool   `json:"encrypted"`
	KeyID         string `json:"key_id,omitempty"`
	ActiveFrom    uint64 `json:"active_from,omitempty"`
	ActiveUntil   uint64 `json:"active_until,omitempty"`
	RemoteURL     string `json:"remote_url,omitempty"`
}

func runKeys(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("keys subcommand is required")
	}
	switch args[0] {
	case "gen":
		return runKeysGen(writer, args[1:])
	case "remote":
		return runKeysRemote(writer, args[1:])
	case "show":
		return runKeysShow(writer, args[1:])
	case "sign-tx":
		return runKeysSignTx(writer, args[1:])
	default:
		return fmt.Errorf("unknown keys subcommand %q", args[0])
	}
}

func runKeysGen(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys gen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	overwrite := flags.Bool("overwrite", false, "overwrite existing key")
	encrypt := flags.Bool("encrypt", false, "encrypt private key material")
	passphrase := flags.String("passphrase", "", "key encryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	keyID := flags.String("id", "", "key id metadata")
	activeFrom := flags.Uint64("active-from", 0, "first height where key is active")
	activeUntil := flags.Uint64("active-until", 0, "last height where key is active; zero means no end")
	if err := flags.Parse(args); err != nil {
		return err
	}
	keyPath := resolveKeyPath(*home, *path)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return err
	}
	if *overwrite {
		if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	document, err := vexocrypto.GenerateEd25519KeyDocument()
	if err != nil {
		return err
	}
	document.Metadata = vexocrypto.KeyMetadata{
		ID:          *keyID,
		ActiveFrom:  *activeFrom,
		ActiveUntil: *activeUntil,
	}
	if *encrypt {
		encrypted, err := document.Encrypted(resolvePassphrase(*passphrase))
		if err != nil {
			return err
		}
		document = encrypted
	}
	if err := vexocrypto.SaveKeyDocument(keyPath, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "generated validator key\n")
	fmt.Fprintf(writer, "path: %s\n", keyPath)
	fmt.Fprintf(writer, "type: %s\n", document.Type)
	fmt.Fprintf(writer, "public_key: %s\n", document.PublicKey)
	fmt.Fprintf(writer, "encrypted: %v\n", document.Encryption != nil)
	return nil
}

func runKeysRemote(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	overwrite := flags.Bool("overwrite", false, "overwrite existing key")
	publicKey := flags.String("public-key", "", "base64 encoded remote signer public key")
	url := flags.String("url", "", "remote signer HTTP endpoint")
	keyID := flags.String("id", "", "key id metadata")
	activeFrom := flags.Uint64("active-from", 0, "first height where key is active")
	activeUntil := flags.Uint64("active-until", 0, "last height where key is active; zero means no end")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *publicKey == "" {
		return errors.New("remote public key is required")
	}
	if *url == "" {
		return errors.New("remote signer URL is required")
	}
	keyPath := resolveKeyPath(*home, *path)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return err
	}
	if *overwrite {
		if err := os.Remove(keyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	document := vexocrypto.KeyDocument{
		SchemaVersion: vexocrypto.KeyDocumentVersionV1,
		Type:          vexocrypto.KeyTypeRemote,
		PublicKey:     *publicKey,
		Metadata: vexocrypto.KeyMetadata{
			ID:          *keyID,
			ActiveFrom:  *activeFrom,
			ActiveUntil: *activeUntil,
			RemoteURL:   *url,
		},
	}
	if _, err := document.RemoteSigner(0); err != nil {
		return err
	}
	if err := vexocrypto.SaveKeyDocument(keyPath, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "registered remote validator key\n")
	fmt.Fprintf(writer, "path: %s\n", keyPath)
	fmt.Fprintf(writer, "type: %s\n", document.Type)
	fmt.Fprintf(writer, "public_key: %s\n", document.PublicKey)
	fmt.Fprintf(writer, "remote_url: %s\n", document.Metadata.RemoteURL)
	return nil
}

func runKeysShow(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys show", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	keyPath := resolveKeyPath(*home, *path)
	document, err := vexocrypto.LoadKeyDocument(keyPath)
	if err != nil {
		return err
	}
	if document.Encryption != nil {
		if _, err := document.Ed25519SignerWithPassphrase(resolvePassphrase(*passphrase)); err != nil {
			return err
		}
	} else if _, err := document.SignerWithPassphrase(resolvePassphrase(*passphrase)); err != nil {
		return err
	}
	info := keyInfoDocument{
		SchemaVersion: document.SchemaVersion,
		Type:          document.Type,
		PublicKey:     document.PublicKey,
		Path:          keyPath,
		Encrypted:     document.Encryption != nil,
		KeyID:         document.Metadata.ID,
		ActiveFrom:    document.Metadata.ActiveFrom,
		ActiveUntil:   document.Metadata.ActiveUntil,
		RemoteURL:     document.Metadata.RemoteURL,
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(info)
	}
	fmt.Fprintf(writer, "validator key\n")
	fmt.Fprintf(writer, "path: %s\n", info.Path)
	fmt.Fprintf(writer, "type: %s\n", info.Type)
	fmt.Fprintf(writer, "public_key: %s\n", info.PublicKey)
	fmt.Fprintf(writer, "encrypted: %v\n", info.Encrypted)
	if info.RemoteURL != "" {
		fmt.Fprintf(writer, "remote_url: %s\n", info.RemoteURL)
	}
	return nil
}

func runKeysSignTx(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys sign-tx", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	chainID := flags.String("chain-id", defaultChainID, "chain id for signed transaction domain")
	tx := flags.String("tx", "", "raw transaction payload to sign")
	passphrase := flags.String("passphrase", "", "key decryption passphrase; prefer VEXO_KEY_PASSPHRASE")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *tx == "" {
		return errors.New("transaction payload is required")
	}
	document, err := vexocrypto.LoadKeyDocument(resolveKeyPath(*home, *path))
	if err != nil {
		return err
	}
	signer, err := document.SignerWithPassphrase(resolvePassphrase(*passphrase))
	if err != nil {
		return err
	}
	signedTx, err := vexoapp.SignTx(*chainID, types.Tx(*tx), signer)
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "tx: %s\n", signedTx)
	return nil
}

func resolvePassphrase(passphrase string) string {
	if passphrase != "" {
		return passphrase
	}
	return os.Getenv("VEXO_KEY_PASSPHRASE")
}

func resolveKeyPath(home string, path string) string {
	if path != "" {
		return path
	}
	if home == "" {
		home = defaultHomeDir
	}
	return filepath.Join(home, keyFileName)
}
