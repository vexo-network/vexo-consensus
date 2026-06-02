package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/vexo-network/vexo-consensus/store"
)

type snapshotDocument struct {
	SchemaVersion string                  `json:"schema_version"`
	State         store.StateRecord       `json:"state"`
	StateRoots    []store.StateRootRecord `json:"state_roots"`
}

func runSnapshot(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("snapshot subcommand is required")
	}
	switch args[0] {
	case "export":
		return runSnapshotExport(writer, args[1:])
	case "restore":
		return runSnapshotRestore(writer, args[1:])
	default:
		return fmt.Errorf("unknown snapshot subcommand %q", args[0])
	}
}

func runSnapshotExport(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	outputPath := flags.String("output", "", "snapshot output path; stdout when empty")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	document, err := buildSnapshotDocument(storage)
	if err != nil {
		return err
	}
	if *outputPath == "" {
		return writeSnapshotDocument(writer, document)
	}
	file, err := os.OpenFile(*outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := writeSnapshotDocument(file, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot exported\n")
	fmt.Fprintf(writer, "path: %s\n", *outputPath)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	return nil
}

func runSnapshotRestore(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("snapshot restore", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	inputPath := flags.String("input", "", "snapshot input path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("snapshot input path is required")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	document, err := readSnapshotDocument(*inputPath)
	if err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	if err := restoreSnapshotDocument(storage, document); err != nil {
		return err
	}
	fmt.Fprintf(writer, "snapshot restored\n")
	fmt.Fprintf(writer, "path: %s\n", *inputPath)
	fmt.Fprintf(writer, "height: %d\n", document.State.Height)
	return nil
}

func buildSnapshotDocument(storage store.Store) (snapshotDocument, error) {
	state, err := storage.LatestState(context.Background())
	if err != nil {
		return snapshotDocument{}, err
	}
	roots := make([]store.StateRootRecord, 0)
	for _, namespace := range []string{"bank"} {
		root, err := storage.StateRoot(context.Background(), state.Height, namespace)
		if err == nil {
			roots = append(roots, root)
		} else if !errors.Is(err, store.ErrStateRootNotFound) {
			return snapshotDocument{}, err
		}
	}
	return snapshotDocument{SchemaVersion: "v1", State: state, StateRoots: roots}, nil
}

func restoreSnapshotDocument(storage store.Store, document snapshotDocument) error {
	if document.SchemaVersion != "v1" {
		return fmt.Errorf("unsupported snapshot schema %q", document.SchemaVersion)
	}
	if err := storage.SaveState(context.Background(), document.State); err != nil {
		return err
	}
	for _, root := range document.StateRoots {
		if err := storage.SaveStateRoot(context.Background(), root); err != nil {
			return err
		}
	}
	return nil
}

func writeSnapshotDocument(writer io.Writer, document snapshotDocument) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func readSnapshotDocument(path string) (snapshotDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return snapshotDocument{}, err
	}
	defer file.Close()
	var document snapshotDocument
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return snapshotDocument{}, err
	}
	return document, nil
}
