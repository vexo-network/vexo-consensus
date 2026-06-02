package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
)

const keyFileName = "validator.key.json"

type keyInfoDocument struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
	PublicKey     string `json:"public_key"`
	Path          string `json:"path"`
}

func runKeys(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("keys subcommand is required")
	}
	switch args[0] {
	case "gen":
		return runKeysGen(writer, args[1:])
	case "show":
		return runKeysShow(writer, args[1:])
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
	if err := vexocrypto.SaveKeyDocument(keyPath, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "generated validator key\n")
	fmt.Fprintf(writer, "path: %s\n", keyPath)
	fmt.Fprintf(writer, "type: %s\n", document.Type)
	fmt.Fprintf(writer, "public_key: %s\n", document.PublicKey)
	return nil
}

func runKeysShow(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("keys show", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	path := flags.String("path", "", "key file path")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	keyPath := resolveKeyPath(*home, *path)
	document, err := vexocrypto.LoadKeyDocument(keyPath)
	if err != nil {
		return err
	}
	if _, err := document.Ed25519Signer(); err != nil {
		return err
	}
	info := keyInfoDocument{
		SchemaVersion: document.SchemaVersion,
		Type:          document.Type,
		PublicKey:     document.PublicKey,
		Path:          keyPath,
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
	return nil
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
