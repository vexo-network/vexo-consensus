package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/queryproof"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func runProof(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("proof subcommand is required")
	}
	switch args[0] {
	case "query":
		return runProofQuery(writer, args[1:])
	case "verify":
		return runProofVerify(writer, args[1:])
	case "verify-ibc":
		return runProofVerifyIBC(writer, args[1:])
	default:
		return fmt.Errorf("unknown proof subcommand %q", args[0])
	}
}

func runProofQuery(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof query", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	namespace := flags.String("namespace", "", "state namespace")
	key := flags.String("key", "", "state key")
	height := flags.Uint64("height", 0, "proof height; defaults to latest state height")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *namespace == "" || *key == "" {
		return errors.New("namespace and key are required")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, ""))
	if err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	proofHeight := types.Height(*height)
	state, err := storage.LatestState(context.Background())
	if err != nil {
		return err
	}
	if proofHeight == 0 {
		proofHeight = state.Height
	}
	if proofHeight != state.Height {
		return vexoruntime.ErrHistoricalQueryProofUnsupported
	}
	proof, err := queryproof.Build(context.Background(), storage, cfg.Chain.ChainID, proofHeight, *namespace, []byte(*key))
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(proof)
}

func runProofVerify(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "proof JSON path")
	chainID := flags.String("chain-id", "", "expected chain id")
	height := flags.Uint64("height", 0, "expected height")
	rootHex := flags.String("root", "", "expected state root hex")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("proof input path is required")
	}
	data, err := readProofFile(*inputPath)
	if err != nil {
		return err
	}
	proof, err := queryproof.Decode(data)
	if err != nil {
		return err
	}
	root, err := parseOptionalHash(*rootHex)
	if err != nil {
		return err
	}
	if err := queryproof.Verify(proof, *chainID, types.Height(*height), root); err != nil {
		return err
	}
	fmt.Fprintf(writer, "query proof verified\n")
	fmt.Fprintf(writer, "chain_id: %s\n", proof.ChainID)
	fmt.Fprintf(writer, "height: %d\n", proof.Height)
	fmt.Fprintf(writer, "namespace: %s\n", proof.Namespace)
	fmt.Fprintf(writer, "key: %s\n", proof.Key)
	fmt.Fprintf(writer, "exists: %t\n", proof.Exists)
	return nil
}

func runProofVerifyIBC(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof verify-ibc", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	inputPath := flags.String("input", "", "proof JSON path")
	clientID := flags.String("client-id", "", "IBC client id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" || *clientID == "" {
		return errors.New("proof input path and client id are required")
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, ""))
	if err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		return err
	}
	defer storage.Close()
	data, err := readProofFile(*inputPath)
	if err != nil {
		return err
	}
	proof, err := queryproof.Decode(data)
	if err != nil {
		return err
	}
	if err := ibckeeper.NewKeeper(storage).VerifyClientProof(context.Background(), *clientID, proof); err != nil {
		return err
	}
	fmt.Fprintf(writer, "IBC proof verified\n")
	fmt.Fprintf(writer, "client_id: %s\n", *clientID)
	fmt.Fprintf(writer, "chain_id: %s\n", proof.ChainID)
	fmt.Fprintf(writer, "height: %d\n", proof.Height)
	fmt.Fprintf(writer, "namespace: %s\n", proof.Namespace)
	fmt.Fprintf(writer, "key: %s\n", proof.Key)
	fmt.Fprintf(writer, "exists: %t\n", proof.Exists)
	return nil
}

func parseOptionalHash(value string) (types.Hash, error) {
	if value == "" {
		return types.Hash{}, nil
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return types.Hash{}, err
	}
	if len(decoded) != 32 {
		return types.Hash{}, fmt.Errorf("expected 32-byte root, got %s bytes", strconv.Itoa(len(decoded)))
	}
	var hash types.Hash
	copy(hash[:], decoded)
	return hash, nil
}

func readProofFile(path string) ([]byte, error) {
	if path == "-" {
		return nil, errors.New("stdin proof input is not supported by this command runner; provide --input <path>")
	}
	return os.ReadFile(path)
}
