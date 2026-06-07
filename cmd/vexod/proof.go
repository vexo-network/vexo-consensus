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
	"strings"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/finality"
	ibckeeper "github.com/vexo-network/vexo-consensus/ibc"
	"github.com/vexo-network/vexo-consensus/queryproof"
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
	case "da-export":
		return runProofDAExport(writer, args[1:])
	case "da-proof":
		return runProofDAProof(writer, args[1:])
	case "da-verify":
		return runProofDAVerify(writer, args[1:])
	case "da-sample":
		return runProofDASample(writer, args[1:])
	case "da-recover":
		return runProofDARecover(writer, args[1:])
	default:
		return fmt.Errorf("unknown proof subcommand %q", args[0])
	}
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

type uint64ListFlag []uint64

func (values *uint64ListFlag) String() string {
	if values == nil {
		return ""
	}
	parts := make([]string, 0, len(*values))
	for _, value := range *values {
		parts = append(parts, strconv.FormatUint(value, 10))
	}
	return strings.Join(parts, ",")
}

func (values *uint64ListFlag) Set(value string) error {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid uint64 value %q: %w", value, err)
	}
	*values = append(*values, parsed)
	return nil
}

type dataAvailabilityBundle struct {
	Proof  dataavailability.Proof   `json:"proof"`
	Chunks []dataavailability.Chunk `json:"chunks"`
	Parity []dataavailability.Chunk `json:"parity"`
}

type dataAvailabilityRecoverResult struct {
	TxsHex []string `json:"txs_hex"`
}

type dataAvailabilitySampleResult struct {
	Request dataavailability.SampleRequest `json:"request"`
	Proofs  []dataavailability.ChunkProof  `json:"proofs"`
	Report  dataavailability.SampleReport  `json:"report"`
}

func runProofDAExport(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof da-export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var txHexValues stringListFlag
	flags.Var(&txHexValues, "tx-hex", "hex-encoded transaction payload; repeat for multiple transactions")
	chunkSize := flags.Uint64("chunk-size", dataavailability.DefaultChunkSize, "data availability chunk size")
	dataShards := flags.Uint64("data-shards", dataavailability.DefaultDataShards, "data chunks covered by each parity chunk")
	parityShards := flags.Uint64("parity-shards", dataavailability.DefaultParityShards, "Reed-Solomon parity chunks per data shard group")
	if err := flags.Parse(args); err != nil {
		return err
	}
	txs, err := txsFromHexFlags(txHexValues)
	if err != nil {
		return err
	}
	proof, err := dataavailability.BuildProofWithErasureOptions(txs, *chunkSize, *dataShards, *parityShards)
	if err != nil {
		return err
	}
	chunks, err := dataavailability.BuildChunks(txs, *chunkSize)
	if err != nil {
		return err
	}
	parity, err := dataavailability.BuildParityChunksWithOptions(txs, *chunkSize, *dataShards, *parityShards)
	if err != nil {
		return err
	}
	return writeIndentedJSON(writer, dataAvailabilityBundle{Proof: proof, Chunks: chunks, Parity: parity})
}

func runProofDAProof(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof da-proof", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var txHexValues stringListFlag
	flags.Var(&txHexValues, "tx-hex", "hex-encoded transaction payload; repeat for multiple transactions")
	chunkSize := flags.Uint64("chunk-size", dataavailability.DefaultChunkSize, "data availability chunk size")
	index := flags.Uint64("index", 0, "chunk index to prove")
	if err := flags.Parse(args); err != nil {
		return err
	}
	txs, err := txsFromHexFlags(txHexValues)
	if err != nil {
		return err
	}
	proof, err := dataavailability.BuildChunkProof(txs, *chunkSize, *index)
	if err != nil {
		return err
	}
	return writeIndentedJSON(writer, proof)
}

func runProofDAVerify(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof da-verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "data availability chunk proof JSON path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("data availability proof input path is required")
	}
	data, err := readProofFile(*inputPath)
	if err != nil {
		return err
	}
	var proof dataavailability.ChunkProof
	if err := json.Unmarshal(data, &proof); err != nil {
		return err
	}
	if err := dataavailability.VerifyChunkProof(proof); err != nil {
		return err
	}
	fmt.Fprintf(writer, "data availability chunk proof verified\n")
	fmt.Fprintf(writer, "commitment: %s\n", hex.EncodeToString(proof.Commitment[:]))
	fmt.Fprintf(writer, "chunk_index: %d\n", proof.Index)
	fmt.Fprintf(writer, "chunk_count: %d\n", proof.ChunkCount)
	fmt.Fprintf(writer, "chunk_size: %d\n", proof.ChunkSize)
	return nil
}

func runProofDASample(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof da-sample", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "data availability bundle JSON path")
	chainID := flags.String("chain-id", "", "chain id used to derive deterministic sample seed")
	height := flags.Uint64("height", 0, "block height used to derive deterministic sample seed")
	samples := flags.Uint64("samples", 0, "number of chunk samples to verify")
	minSamples := flags.Uint64("min-samples", 0, "minimum acceptable samples")
	entropyHex := flags.String("entropy-hex", "", "optional hex entropy mixed into sample seed")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" || *chainID == "" || *height == 0 {
		return errors.New("data availability bundle input, chain id, and height are required")
	}
	entropy, err := parseOptionalBytesHex(*entropyHex)
	if err != nil {
		return err
	}
	data, err := readProofFile(*inputPath)
	if err != nil {
		return err
	}
	var bundle dataAvailabilityBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return err
	}
	request, err := dataavailability.PlanSamples(*chainID, types.Height(*height), bundle.Proof, dataavailability.SamplePolicy{Samples: *samples, MinSamples: *minSamples}, entropy)
	if err != nil {
		return err
	}
	proofs := make([]dataavailability.ChunkProof, 0, len(request.Indices))
	for _, index := range request.Indices {
		proof, err := dataavailability.BuildChunkProofFromChunks(bundle.Chunks, bundle.Proof.ChunkSize, index)
		if err != nil {
			return err
		}
		proofs = append(proofs, proof)
	}
	report, err := dataavailability.VerifySamples(request, proofs)
	if err != nil {
		return err
	}
	return writeIndentedJSON(writer, dataAvailabilitySampleResult{
		Request: request,
		Proofs:  proofs,
		Report:  report,
	})
}

func runProofDARecover(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("proof da-recover", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "data availability bundle JSON path")
	var dropIndexes uint64ListFlag
	flags.Var(&dropIndexes, "drop", "optional chunk index to drop before recovery; repeat to simulate multiple missing chunks")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *inputPath == "" {
		return errors.New("data availability bundle input path is required")
	}
	data, err := readProofFile(*inputPath)
	if err != nil {
		return err
	}
	var bundle dataAvailabilityBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return err
	}
	chunks := bundle.Chunks
	if len(dropIndexes) > 0 {
		dropped := make(map[uint64]struct{}, len(dropIndexes))
		for _, index := range dropIndexes {
			dropped[index] = struct{}{}
		}
		filtered := make([]dataavailability.Chunk, 0, len(chunks))
		for _, chunk := range chunks {
			if _, ok := dropped[chunk.Index]; !ok {
				filtered = append(filtered, chunk)
			}
		}
		chunks = filtered
	}
	txs, err := dataavailability.RecoverTransactions(bundle.Proof, chunks, bundle.Parity)
	if err != nil {
		return err
	}
	result := dataAvailabilityRecoverResult{TxsHex: make([]string, 0, len(txs))}
	for _, tx := range txs {
		result.TxsHex = append(result.TxsHex, hex.EncodeToString(tx))
	}
	return writeIndentedJSON(writer, result)
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
	var proof queryproof.Proof
	if proofHeight == state.Height {
		proof, err = queryproof.Build(context.Background(), storage, cfg.Chain.ChainID, proofHeight, *namespace, []byte(*key))
	} else {
		root, rootErr := storage.StateRoot(context.Background(), proofHeight, *namespace)
		if rootErr != nil {
			return rootErr
		}
		pairs, pairsErr := storage.ExportNamespaceAt(context.Background(), proofHeight, *namespace)
		if pairsErr != nil {
			return pairsErr
		}
		proof, err = queryproof.BuildFromKVPairs(cfg.Chain.ChainID, proofHeight, *namespace, []byte(*key), pairs, root.Root)
	}
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

func txsFromHexFlags(values []string) ([]types.Tx, error) {
	if len(values) == 0 {
		return nil, errors.New("at least one --tx-hex value is required")
	}
	txs := make([]types.Tx, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimPrefix(strings.TrimSpace(value), "0x")
		decoded, err := hex.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("decode --tx-hex: %w", err)
		}
		txs = append(txs, types.Tx(decoded))
	}
	return txs, nil
}

func writeIndentedJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
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

func parseOptionalBytesHex(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(value), "0x"))
	if err != nil {
		return nil, err
	}
	return decoded, nil
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
