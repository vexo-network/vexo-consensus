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

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/queryproof"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
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
	case "detect-finality-conflict":
		return runProofDetectFinalityConflict(writer, args[1:])
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

func runProofDetectFinalityConflict(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof detect-finality-conflict", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory used to resolve genesis when --validator-set is omitted")
	firstPath := flags.String("first", "", "first finality proof JSON path")
	secondPath := flags.String("second", "", "second finality proof JSON path")
	validatorSetPath := flags.String("validator-set", "", "validator set JSON path for the proof height; defaults to genesis validators")
	genesisPath := flags.String("genesis", "", "genesis JSON path used when --validator-set is omitted")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *firstPath == "" || *secondPath == "" {
		return errors.New("first and second finality proof paths are required")
	}
	first, err := readFinalityProof(*firstPath)
	if err != nil {
		return fmt.Errorf("first proof: %w", err)
	}
	second, err := readFinalityProof(*secondPath)
	if err != nil {
		return fmt.Errorf("second proof: %w", err)
	}
	validatorSet, err := loadProofValidatorSet(*validatorSetPath, resolveGenesisPath(*home, *genesisPath))
	if err != nil {
		return err
	}
	detector := finality.NewAttackDetector(validatorSet, vexocrypto.Ed25519MultiVerifier{})
	if _, err := detector.Observe(first); err != nil {
		return fmt.Errorf("first proof verification: %w", err)
	}
	violation, err := detector.Observe(second)
	if err != nil {
		return fmt.Errorf("second proof verification: %w", err)
	}
	if violation == nil {
		fmt.Fprintf(writer, "no finality conflict detected\n")
		return nil
	}
	fmt.Fprintf(writer, "finality conflict detected\n")
	fmt.Fprintf(writer, "chain_id: %s\n", violation.ChainID)
	fmt.Fprintf(writer, "height: %d\n", violation.Height)
	fmt.Fprintf(writer, "validator_set_height: %d\n", violation.ValidatorSetHeight)
	fmt.Fprintf(writer, "validator_set_hash: %s\n", hex.EncodeToString(violation.ValidatorSetHash[:]))
	fmt.Fprintf(writer, "first_block_hash: %s\n", hex.EncodeToString(violation.FirstBlockHash[:]))
	fmt.Fprintf(writer, "second_block_hash: %s\n", hex.EncodeToString(violation.SecondBlockHash[:]))
	fmt.Fprintf(writer, "first_round: %d\n", violation.FirstRound)
	fmt.Fprintf(writer, "second_round: %d\n", violation.SecondRound)
	fmt.Fprintf(writer, "double_signers: %v\n", violation.DoubleSigners)
	fmt.Fprintf(writer, "double_sign_power: %d\n", violation.DoubleSignVotingPower)
	fmt.Fprintf(writer, "total_power: %d\n", violation.TotalVotingPower)
	fmt.Fprintf(writer, "fault_power_threshold: %d\n", violation.FaultPowerThreshold)
	fmt.Fprintf(writer, "meets_fault_threshold: %t\n", violation.MeetsFaultThreshold())
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

func readFinalityProof(path string) (finality.Proof, error) {
	data, err := readProofFile(path)
	if err != nil {
		return finality.Proof{}, err
	}
	var proof finality.Proof
	if err := json.Unmarshal(data, &proof); err != nil {
		return finality.Proof{}, err
	}
	return proof, nil
}

func loadProofValidatorSet(validatorSetPath string, genesisPath string) (validator.Set, error) {
	var validators []validator.Validator
	if validatorSetPath != "" {
		data, err := os.ReadFile(validatorSetPath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &validators); err != nil {
			return nil, err
		}
	} else {
		genesis, err := loadGenesis(genesisPath)
		if err != nil {
			return nil, err
		}
		validators = genesis.Validators
	}
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		return nil, err
	}
	return registry.ValidatorSet(context.Background(), 1)
}
