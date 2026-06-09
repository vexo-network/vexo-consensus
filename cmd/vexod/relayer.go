package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/queryproof"
	"github.com/vexo-network/vexo-consensus/types"
)

func runRelayer(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("relayer subcommand is required")
	}
	client := http.Client{Timeout: 10 * time.Second}
	switch args[0] {
	case "client-update":
		return runRelayerClientUpdate(writer, args[1:], client)
	case "packet-ack":
		return runRelayerPacketAck(writer, args[1:], client)
	case "packet-timeout":
		return runRelayerPacketTimeout(writer, args[1:], client)
	case "packet-proof":
		return runRelayerPacketProof(writer, args[1:], client)
	case "discover":
		return runRelayerDiscover(writer, args[1:], client)
	case "loop":
		return runRelayerLoop(writer, args[1:], client)
	case "run":
		return runRelayerRun(writer, args[1:], client)
	default:
		return fmt.Errorf("unknown relayer subcommand %q", args[0])
	}
}

func runRelayerClientUpdate(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer client-update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rpcAddress := flags.String("rpc", "", "destination RPC base URL used when --submit is set")
	sourceRPC := flags.String("source-rpc", "", "counterparty RPC base URL used to fetch /v1/state/latest")
	clientID := flags.String("client-id", "", "IBC client id")
	height := flags.Uint64("height", 0, "counterparty latest height")
	validatorSetHash := flags.String("validator-set-hash", "", "counterparty validator set hash hex")
	stateRoot := flags.String("state-root", "", "counterparty state root hex")
	submit := flags.Bool("submit", false, "submit the built transaction to --rpc")
	tags := relayerTxTagFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *sourceRPC != "" && (*height == 0 || *validatorSetHash == "" || *stateRoot == "") {
		state, err := fetchRelayerLatestState(context.Background(), client, *sourceRPC)
		if err != nil {
			return err
		}
		if *height == 0 {
			*height = state.Height
		}
		if *validatorSetHash == "" {
			*validatorSetHash = state.ValidatorSetHash
		}
		if *stateRoot == "" {
			*stateRoot = state.AppHash
		}
		fmt.Fprintf(writer, "source_height: %d\n", state.Height)
		fmt.Fprintf(writer, "source_validator_set_hash: %s\n", state.ValidatorSetHash)
		fmt.Fprintf(writer, "source_state_root: %s\n", state.AppHash)
	}
	if *clientID == "" || *height == 0 || *validatorSetHash == "" || *stateRoot == "" {
		return errors.New("client-id and either source-rpc or explicit height, validator-set-hash, and state-root are required")
	}
	if err := validateRelayerHexHash(*validatorSetHash); err != nil {
		return fmt.Errorf("validator-set-hash: %w", err)
	}
	if err := validateRelayerHexHash(*stateRoot); err != nil {
		return fmt.Errorf("state-root: %w", err)
	}
	tx, err := buildRelayerTx("client-update", []string{*clientID, strconv.FormatUint(*height, 10), *validatorSetHash, *stateRoot}, tags)
	if err != nil {
		return err
	}
	return writeOrSubmitRelayerTx(context.Background(), writer, client, *rpcAddress, tx, *submit)
}

func runRelayerPacketAck(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer packet-ack", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := packetRelayerFlags(flags)
	ack := flags.String("ack", "", "acknowledgement bytes as plain text")
	proofRPC := flags.String("proof-rpc", "", "optional source RPC base URL to fetch packet proof before building tx")
	submit := flags.Bool("submit", false, "submit the built transaction to --rpc")
	tags := relayerTxTagFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	packetArgs, err := options.packetArgs()
	if err != nil {
		return err
	}
	if *ack == "" {
		return errors.New("ack is required")
	}
	if *proofRPC != "" {
		proof, err := fetchRelayerPacketProof(context.Background(), client, *proofRPC, packetArgs)
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "proof_height: %d\n", proof.Height)
		fmt.Fprintf(writer, "proof_namespace: %s\n", proof.Namespace)
	}
	txArgs := append(packetArgs, base64.RawStdEncoding.EncodeToString([]byte(*ack)))
	tx, err := buildRelayerTx("packet-ack", txArgs, tags)
	if err != nil {
		return err
	}
	return writeOrSubmitRelayerTx(context.Background(), writer, client, options.rpcAddress, tx, *submit)
}

func runRelayerPacketTimeout(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer packet-timeout", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := packetRelayerFlags(flags)
	proofRPC := flags.String("proof-rpc", "", "optional source RPC base URL to fetch packet proof before building tx")
	submit := flags.Bool("submit", false, "submit the built transaction to --rpc")
	tags := relayerTxTagFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	packetArgs, err := options.packetArgs()
	if err != nil {
		return err
	}
	if options.timeoutHeight == 0 {
		return errors.New("timeout-height is required")
	}
	if *proofRPC != "" {
		proof, err := fetchRelayerPacketProof(context.Background(), client, *proofRPC, packetArgs)
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "proof_height: %d\n", proof.Height)
		fmt.Fprintf(writer, "proof_namespace: %s\n", proof.Namespace)
	}
	tx, err := buildRelayerTx("packet-timeout", packetArgs, tags)
	if err != nil {
		return err
	}
	return writeOrSubmitRelayerTx(context.Background(), writer, client, options.rpcAddress, tx, *submit)
}

func runRelayerPacketProof(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer packet-proof", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := packetRelayerFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	packetArgs, err := options.packetArgs()
	if err != nil {
		return err
	}
	if options.rpcAddress == "" {
		return errors.New("rpc is required")
	}
	proof, err := fetchRelayerPacketProof(context.Background(), client, options.rpcAddress, packetArgs)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(proof)
}

func runRelayerDiscover(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer discover", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rpcAddress := flags.String("rpc", "", "source RPC base URL used to query indexed packet events")
	eventKey := flags.String("event-key", "ibc_packet_event", "indexed event key")
	eventValue := flags.String("event-value", "send", "indexed event value")
	jsonOutput := flags.Bool("json", false, "write discovered packets as JSON")
	limit := flags.Uint64("limit", 0, "maximum packets to print; 0 means all")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *rpcAddress == "" {
		return errors.New("rpc is required")
	}
	packets, err := fetchRelayerDiscoveredPackets(context.Background(), client, *rpcAddress, *eventKey, *eventValue)
	if err != nil {
		return err
	}
	if *limit > 0 && uint64(len(packets)) > *limit {
		packets = packets[:*limit]
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(packets)
	}
	for _, packet := range packets {
		fmt.Fprintf(writer, "height: %d\n", packet.Height)
		fmt.Fprintf(writer, "tx_index: %d\n", packet.TxIndex)
		fmt.Fprintf(writer, "sequence: %d\n", packet.Sequence)
		fmt.Fprintf(writer, "source: %s/%s\n", packet.SourcePort, packet.SourceChannel)
		fmt.Fprintf(writer, "destination: %s/%s\n", packet.DestinationPort, packet.DestinationChannel)
		fmt.Fprintf(writer, "data: %s\n", packet.Data)
		if packet.TimeoutHeight > 0 {
			fmt.Fprintf(writer, "timeout_height: %d\n", packet.TimeoutHeight)
		}
		fmt.Fprintln(writer, "---")
	}
	return nil
}

func runRelayerLoop(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer loop", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := packetRelayerFlags(flags)
	mode := flags.String("mode", "", "packet relay mode: ack or timeout")
	ack := flags.String("ack", "", "acknowledgement bytes as plain text when --mode ack")
	proofRPC := flags.String("proof-rpc", "", "source RPC base URL used to poll packet proofs")
	interval := flags.Duration("interval", 5*time.Second, "poll interval")
	failureBackoff := flags.Duration("failure-backoff", 0, "optional wait duration after proof or submit errors; defaults to --interval")
	maxIterations := flags.Uint64("max-iterations", 0, "maximum poll iterations; 0 means run until interrupted")
	continueOnError := flags.Bool("continue-on-error", false, "continue polling after proof fetch or submit errors")
	submit := flags.Bool("submit", false, "submit the built transaction to --rpc")
	statePath := flags.String("state", "", "optional checkpoint JSON path used to avoid duplicate submissions")
	tags := relayerTxTagFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *mode != "ack" && *mode != "timeout" {
		return errors.New("mode must be ack or timeout")
	}
	if *proofRPC == "" {
		return errors.New("proof-rpc is required")
	}
	if *interval < 0 {
		return errors.New("interval must be non-negative")
	}
	if *failureBackoff < 0 {
		return errors.New("failure-backoff must be non-negative")
	}
	packetArgs, err := options.packetArgs()
	if err != nil {
		return err
	}
	if *mode == "ack" && *ack == "" {
		return errors.New("ack is required when mode is ack")
	}
	if *mode == "timeout" && options.timeoutHeight == 0 {
		return errors.New("timeout-height is required when mode is timeout")
	}
	checkpoint, err := openRelayerCheckpointStore(*statePath)
	if err != nil {
		return err
	}
	return runRelayerPollingLoop(context.Background(), writer, client, relayerLoopConfig{
		Mode:            *mode,
		RPCAddress:      options.rpcAddress,
		ProofRPC:        *proofRPC,
		PacketArgs:      packetArgs,
		Ack:             *ack,
		Tags:            tags,
		Submit:          *submit,
		Interval:        *interval,
		FailureBackoff:  *failureBackoff,
		MaxIterations:   *maxIterations,
		ContinueOnError: *continueOnError,
		Checkpoint:      checkpoint,
	})
}

func runRelayerRun(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "", "relayer config JSON path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return errors.New("config path is required")
	}
	document, err := readRelayerConfigDocument(*configPath)
	if err != nil {
		return err
	}
	return runRelayerConfig(context.Background(), writer, client, document)
}

const relayerConfigSchemaVersion = "v1"

type relayerConfigDocument struct {
	SchemaVersion string             `json:"schema_version"`
	Jobs          []relayerJobConfig `json:"jobs"`
}

type relayerJobConfig struct {
	Name            string              `json:"name"`
	Mode            string              `json:"mode"`
	RPC             string              `json:"rpc"`
	ProofRPC        string              `json:"proof_rpc"`
	Packet          relayerPacketConfig `json:"packet"`
	Ack             string              `json:"ack,omitempty"`
	Fee             string              `json:"fee,omitempty"`
	Gas             string              `json:"gas,omitempty"`
	Signer          string              `json:"signer,omitempty"`
	Nonce           string              `json:"nonce,omitempty"`
	Submit          bool                `json:"submit,omitempty"`
	StatePath       string              `json:"state_path,omitempty"`
	Interval        string              `json:"interval,omitempty"`
	MaxIterations   uint64              `json:"max_iterations,omitempty"`
	FailureBackoff  string              `json:"failure_backoff,omitempty"`
	ContinueOnError bool                `json:"continue_on_error,omitempty"`
}

type relayerPacketConfig struct {
	Sequence           uint64 `json:"sequence"`
	SourcePort         string `json:"source_port"`
	SourceChannel      string `json:"source_channel"`
	DestinationPort    string `json:"destination_port"`
	DestinationChannel string `json:"destination_channel"`
	Data               string `json:"data"`
	TimeoutHeight      uint64 `json:"timeout_height,omitempty"`
}

type relayerDiscoveredPacket struct {
	Height             uint64 `json:"height"`
	TxIndex            int    `json:"tx_index"`
	Sequence           uint64 `json:"sequence"`
	SourcePort         string `json:"source_port"`
	SourceChannel      string `json:"source_channel"`
	DestinationPort    string `json:"destination_port"`
	DestinationChannel string `json:"destination_channel"`
	Data               string `json:"data"`
	TimeoutHeight      uint64 `json:"timeout_height,omitempty"`
}

type relayerLatestState struct {
	Height           uint64 `json:"height"`
	AppHash          string `json:"app_hash"`
	ValidatorSetHash string `json:"validator_set_hash"`
}

func readRelayerConfigDocument(path string) (relayerConfigDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return relayerConfigDocument{}, err
	}
	var document relayerConfigDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return relayerConfigDocument{}, err
	}
	if document.SchemaVersion != relayerConfigSchemaVersion {
		return relayerConfigDocument{}, fmt.Errorf("unsupported relayer config schema %q", document.SchemaVersion)
	}
	if len(document.Jobs) == 0 {
		return relayerConfigDocument{}, errors.New("relayer config must include at least one job")
	}
	for index, job := range document.Jobs {
		if job.Name == "" {
			return relayerConfigDocument{}, fmt.Errorf("relayer job %d missing name", index)
		}
		if job.StatePath != "" && !filepath.IsAbs(job.StatePath) {
			document.Jobs[index].StatePath = filepath.Join(filepath.Dir(path), job.StatePath)
			job.StatePath = document.Jobs[index].StatePath
		}
		if _, err := relayerLoopConfigFromJob(job); err != nil {
			return relayerConfigDocument{}, fmt.Errorf("relayer job %q: %w", job.Name, err)
		}
	}
	return document, nil
}

func runRelayerConfig(ctx context.Context, writer io.Writer, client http.Client, document relayerConfigDocument) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var outputMu sync.Mutex
	var waitGroup sync.WaitGroup
	errCh := make(chan error, len(document.Jobs))
	checkpoints := map[string]*relayerCheckpointStore{}
	for _, job := range document.Jobs {
		job := job
		cfg, err := relayerLoopConfigFromJob(job)
		if err != nil {
			return err
		}
		if job.StatePath != "" {
			checkpoint, ok := checkpoints[job.StatePath]
			if !ok {
				checkpoint, err = openRelayerCheckpointStore(job.StatePath)
				if err != nil {
					return err
				}
				checkpoints[job.StatePath] = checkpoint
			}
			cfg.Checkpoint = checkpoint
		}
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			jobWriter := relayerJobWriter{writer: writer, mu: &outputMu, name: job.Name}
			if err := runRelayerPollingLoop(ctx, jobWriter, client, cfg); err != nil {
				errCh <- fmt.Errorf("%s: %w", job.Name, err)
				cancel()
			}
		}()
	}
	waitGroup.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func relayerLoopConfigFromJob(job relayerJobConfig) (relayerLoopConfig, error) {
	interval := 5 * time.Second
	if job.Interval != "" {
		parsed, err := time.ParseDuration(job.Interval)
		if err != nil {
			return relayerLoopConfig{}, err
		}
		interval = parsed
	}
	if interval < 0 {
		return relayerLoopConfig{}, errors.New("interval must be non-negative")
	}
	failureBackoff := time.Duration(0)
	if job.FailureBackoff != "" {
		parsed, err := time.ParseDuration(job.FailureBackoff)
		if err != nil {
			return relayerLoopConfig{}, err
		}
		failureBackoff = parsed
	}
	if failureBackoff < 0 {
		return relayerLoopConfig{}, errors.New("failure_backoff must be non-negative")
	}
	packetArgs, err := relayerPacketOptions{
		rpcAddress:         job.RPC,
		sequence:           job.Packet.Sequence,
		sourcePort:         job.Packet.SourcePort,
		sourceChannel:      job.Packet.SourceChannel,
		destinationPort:    job.Packet.DestinationPort,
		destinationChannel: job.Packet.DestinationChannel,
		data:               job.Packet.Data,
		timeoutHeight:      job.Packet.TimeoutHeight,
	}.packetArgs()
	if err != nil {
		return relayerLoopConfig{}, err
	}
	if job.Mode != "ack" && job.Mode != "timeout" {
		return relayerLoopConfig{}, errors.New("mode must be ack or timeout")
	}
	if job.Mode == "ack" && job.Ack == "" {
		return relayerLoopConfig{}, errors.New("ack is required when mode is ack")
	}
	if job.Mode == "timeout" && job.Packet.TimeoutHeight == 0 {
		return relayerLoopConfig{}, errors.New("timeout_height is required when mode is timeout")
	}
	if job.ProofRPC == "" {
		return relayerLoopConfig{}, errors.New("proof_rpc is required")
	}
	if job.Submit && job.RPC == "" {
		return relayerLoopConfig{}, errors.New("rpc is required when submit is enabled")
	}
	return relayerLoopConfig{
		Mode:       job.Mode,
		RPCAddress: job.RPC,
		ProofRPC:   job.ProofRPC,
		PacketArgs: packetArgs,
		Ack:        job.Ack,
		Tags: &relayerTxTags{
			fee:    job.Fee,
			gas:    job.Gas,
			signer: job.Signer,
			nonce:  job.Nonce,
		},
		Submit:          job.Submit,
		Interval:        interval,
		FailureBackoff:  failureBackoff,
		MaxIterations:   job.MaxIterations,
		ContinueOnError: job.ContinueOnError,
	}, nil
}

type relayerJobWriter struct {
	writer io.Writer
	mu     *sync.Mutex
	name   string
}

func (writer relayerJobWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	lines := strings.SplitAfter(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if _, err := fmt.Fprintf(writer.writer, "job=%s %s", writer.name, line); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

type relayerLoopConfig struct {
	Mode            string
	RPCAddress      string
	ProofRPC        string
	PacketArgs      []string
	Ack             string
	Tags            *relayerTxTags
	Submit          bool
	Interval        time.Duration
	FailureBackoff  time.Duration
	MaxIterations   uint64
	ContinueOnError bool
	Checkpoint      *relayerCheckpointStore
}

func runRelayerPollingLoop(ctx context.Context, writer io.Writer, client http.Client, cfg relayerLoopConfig) error {
	checkpointKey := relayerCheckpointKey(cfg)
	metrics := relayerLoopMetrics{StartedAtUnix: time.Now().Unix()}
	defer func() {
		metrics.CompletedAtUnix = time.Now().Unix()
		fmt.Fprintf(writer, "metrics: iterations=%d proof_errors=%d submit_errors=%d submitted=%d checkpoint_skips=%d completed_at_unix=%d\n",
			metrics.Iterations,
			metrics.ProofErrors,
			metrics.SubmitErrors,
			metrics.Submitted,
			metrics.CheckpointSkips,
			metrics.CompletedAtUnix,
		)
	}()
	for iteration := uint64(1); ; iteration++ {
		metrics.Iterations = iteration
		if cfg.Checkpoint != nil {
			done, err := cfg.Checkpoint.IsCompleted(checkpointKey)
			if err != nil {
				return err
			}
			if done {
				metrics.CheckpointSkips++
				fmt.Fprintf(writer, "checkpoint_skipped: true\n")
				fmt.Fprintf(writer, "checkpoint_key: %s\n", checkpointKey)
				return nil
			}
		}
		proof, err := fetchRelayerPacketProof(ctx, client, cfg.ProofRPC, cfg.PacketArgs)
		if err != nil {
			metrics.ProofErrors++
			fmt.Fprintf(writer, "iteration: %d\n", iteration)
			fmt.Fprintf(writer, "proof_error: %v\n", err)
			if !cfg.ContinueOnError {
				return err
			}
			if cfg.MaxIterations > 0 && iteration >= cfg.MaxIterations {
				return nil
			}
			if err := waitRelayerLoopInterval(ctx, relayerFailureWait(cfg)); err != nil {
				return err
			}
			continue
		}
		fmt.Fprintf(writer, "iteration: %d\n", iteration)
		fmt.Fprintf(writer, "proof_height: %d\n", proof.Height)
		fmt.Fprintf(writer, "proof_namespace: %s\n", proof.Namespace)
		tx, err := buildRelayerLoopTx(cfg)
		if err != nil {
			return err
		}
		fmt.Fprintf(writer, "tx: %s\n", tx)
		if cfg.Submit {
			if err := submitRelayerTx(ctx, client, cfg.RPCAddress, tx); err != nil {
				metrics.SubmitErrors++
				fmt.Fprintf(writer, "submit_error: %v\n", err)
				if !cfg.ContinueOnError {
					return err
				}
				if cfg.MaxIterations > 0 && iteration >= cfg.MaxIterations {
					return nil
				}
				if err := waitRelayerLoopInterval(ctx, relayerFailureWait(cfg)); err != nil {
					return err
				}
				continue
			} else {
				metrics.Submitted++
				fmt.Fprintf(writer, "submitted: true\n")
				if cfg.Checkpoint != nil {
					if err := cfg.Checkpoint.MarkCompleted(checkpointKey, relayerCheckpointEntry{
						Mode:            cfg.Mode,
						PacketArgs:      append([]string(nil), cfg.PacketArgs...),
						ProofHeight:     uint64(proof.Height),
						ProofNamespace:  proof.Namespace,
						CompletedAtUnix: time.Now().Unix(),
					}); err != nil {
						return err
					}
					fmt.Fprintf(writer, "checkpoint_saved: true\n")
					fmt.Fprintf(writer, "checkpoint_key: %s\n", checkpointKey)
					return nil
				}
			}
		}
		if cfg.MaxIterations > 0 && iteration >= cfg.MaxIterations {
			return nil
		}
		if err := waitRelayerLoopInterval(ctx, cfg.Interval); err != nil {
			return err
		}
	}
}

type relayerLoopMetrics struct {
	StartedAtUnix   int64
	CompletedAtUnix int64
	Iterations      uint64
	ProofErrors     uint64
	SubmitErrors    uint64
	Submitted       uint64
	CheckpointSkips uint64
}

func relayerFailureWait(cfg relayerLoopConfig) time.Duration {
	if cfg.FailureBackoff > 0 {
		return cfg.FailureBackoff
	}
	return cfg.Interval
}

const relayerCheckpointSchemaVersion = "v1"

type relayerCheckpointStore struct {
	path string
	mu   sync.Mutex
}

type relayerCheckpointDocument struct {
	SchemaVersion string                            `json:"schema_version"`
	Completed     map[string]relayerCheckpointEntry `json:"completed"`
}

type relayerCheckpointEntry struct {
	Mode            string   `json:"mode"`
	PacketArgs      []string `json:"packet_args"`
	ProofHeight     uint64   `json:"proof_height"`
	ProofNamespace  string   `json:"proof_namespace"`
	CompletedAtUnix int64    `json:"completed_at_unix"`
}

func openRelayerCheckpointStore(path string) (*relayerCheckpointStore, error) {
	if path == "" {
		return nil, nil
	}
	store := &relayerCheckpointStore{path: path}
	if _, err := store.read(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *relayerCheckpointStore) IsCompleted(key string) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	document, err := store.read()
	if err != nil {
		return false, err
	}
	_, ok := document.Completed[key]
	return ok, nil
}

func (store *relayerCheckpointStore) MarkCompleted(key string, entry relayerCheckpointEntry) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	document, err := store.read()
	if err != nil {
		return err
	}
	document.Completed[key] = entry
	return store.write(document)
}

func (store *relayerCheckpointStore) read() (relayerCheckpointDocument, error) {
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return relayerCheckpointDocument{
			SchemaVersion: relayerCheckpointSchemaVersion,
			Completed:     map[string]relayerCheckpointEntry{},
		}, nil
	}
	if err != nil {
		return relayerCheckpointDocument{}, err
	}
	var document relayerCheckpointDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return relayerCheckpointDocument{}, err
	}
	if document.SchemaVersion != relayerCheckpointSchemaVersion {
		return relayerCheckpointDocument{}, fmt.Errorf("unsupported relayer checkpoint schema %q", document.SchemaVersion)
	}
	if document.Completed == nil {
		document.Completed = map[string]relayerCheckpointEntry{}
	}
	return document, nil
}

func (store *relayerCheckpointStore) write(document relayerCheckpointDocument) error {
	if document.Completed == nil {
		document.Completed = map[string]relayerCheckpointEntry{}
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(store.path), ".relayer-checkpoint-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, store.path)
}

func relayerCheckpointKey(cfg relayerLoopConfig) string {
	parts := append([]string{cfg.Mode}, cfg.PacketArgs...)
	if cfg.Mode == "ack" {
		parts = append(parts, base64.RawStdEncoding.EncodeToString([]byte(cfg.Ack)))
	}
	return strings.Join(parts, ":")
}

func buildRelayerLoopTx(cfg relayerLoopConfig) (types.Tx, error) {
	switch cfg.Mode {
	case "ack":
		txArgs := append(append([]string(nil), cfg.PacketArgs...), base64.RawStdEncoding.EncodeToString([]byte(cfg.Ack)))
		return buildRelayerTx("packet-ack", txArgs, cfg.Tags)
	case "timeout":
		return buildRelayerTx("packet-timeout", cfg.PacketArgs, cfg.Tags)
	default:
		return nil, errors.New("mode must be ack or timeout")
	}
}

func waitRelayerLoopInterval(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type relayerPacketOptions struct {
	rpcAddress         string
	sequence           uint64
	sourcePort         string
	sourceChannel      string
	destinationPort    string
	destinationChannel string
	data               string
	timeoutHeight      uint64
}

func packetRelayerFlags(flags *flag.FlagSet) *relayerPacketOptions {
	options := &relayerPacketOptions{}
	flags.StringVar(&options.rpcAddress, "rpc", "", "destination RPC base URL")
	flags.Uint64Var(&options.sequence, "sequence", 0, "packet sequence")
	flags.StringVar(&options.sourcePort, "source-port", "", "source port")
	flags.StringVar(&options.sourceChannel, "source-channel", "", "source channel")
	flags.StringVar(&options.destinationPort, "destination-port", "", "destination port")
	flags.StringVar(&options.destinationChannel, "destination-channel", "", "destination channel")
	flags.StringVar(&options.data, "data", "", "packet data as plain text")
	flags.Uint64Var(&options.timeoutHeight, "timeout-height", 0, "packet timeout height")
	return options
}

func (options relayerPacketOptions) packetArgs() ([]string, error) {
	if options.sequence == 0 || options.sourcePort == "" || options.sourceChannel == "" || options.destinationPort == "" || options.destinationChannel == "" || options.data == "" {
		return nil, errors.New("sequence, source-port, source-channel, destination-port, destination-channel, and data are required")
	}
	args := []string{
		strconv.FormatUint(options.sequence, 10),
		options.sourcePort,
		options.sourceChannel,
		options.destinationPort,
		options.destinationChannel,
		base64.RawStdEncoding.EncodeToString([]byte(options.data)),
	}
	if options.timeoutHeight > 0 {
		args = append(args, strconv.FormatUint(options.timeoutHeight, 10))
	}
	return args, nil
}

type relayerTxTags struct {
	fee    string
	gas    string
	signer string
	nonce  string
}

func relayerTxTagFlags(flags *flag.FlagSet) *relayerTxTags {
	tags := &relayerTxTags{}
	flags.StringVar(&tags.fee, "fee", "", "transaction fee")
	flags.StringVar(&tags.gas, "gas", "", "transaction gas")
	flags.StringVar(&tags.signer, "signer", "", "transaction signer")
	flags.StringVar(&tags.nonce, "nonce", "", "transaction nonce")
	return tags
}

func buildRelayerTx(action string, args []string, tags *relayerTxTags) (types.Tx, error) {
	tagMap := map[string]string{}
	if tags != nil {
		if tags.fee != "" {
			tagMap["fee"] = tags.fee
		}
		if tags.gas != "" {
			tagMap["gas"] = tags.gas
		}
		if tags.signer != "" {
			tagMap["signer"] = tags.signer
		}
		if tags.nonce != "" {
			tagMap["nonce"] = tags.nonce
		}
	}
	if len(tagMap) == 0 {
		tagMap = nil
	}
	return vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: "ibc",
		Action: action,
		Args:   args,
		Tags:   tagMap,
	})
}

func writeOrSubmitRelayerTx(ctx context.Context, writer io.Writer, client http.Client, rpcAddress string, tx types.Tx, submit bool) error {
	fmt.Fprintf(writer, "tx: %s\n", tx)
	if !submit {
		return nil
	}
	if rpcAddress == "" {
		return errors.New("rpc is required when submit is enabled")
	}
	if err := submitRelayerTx(ctx, client, rpcAddress, tx); err != nil {
		return err
	}
	fmt.Fprintf(writer, "submitted: true\n")
	return nil
}

func submitRelayerTx(ctx context.Context, client http.Client, rpcAddress string, tx types.Tx) error {
	endpoint, err := joinRelayerURL(rpcAddress, "/v1/tx")
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"tx": base64.StdEncoding.EncodeToString(tx)})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("relayer tx returned HTTP %d", response.StatusCode)
	}
	return nil
}

func fetchRelayerPacketProof(ctx context.Context, client http.Client, rpcAddress string, packetArgs []string) (queryproof.Proof, error) {
	if len(packetArgs) < 5 {
		return queryproof.Proof{}, errors.New("packet proof requires packet path arguments")
	}
	path := "/v1/ibc/proof/packet/" + strings.Join(packetArgs[:5], "/")
	endpoint, err := joinRelayerURL(rpcAddress, path)
	if err != nil {
		return queryproof.Proof{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return queryproof.Proof{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return queryproof.Proof{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return queryproof.Proof{}, fmt.Errorf("IBC packet proof returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Proof queryproof.Proof `json:"proof"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return queryproof.Proof{}, err
	}
	if envelope.Proof.SchemaVersion == "" {
		return queryproof.Proof{}, errors.New("IBC packet proof response is missing proof")
	}
	return envelope.Proof, nil
}

func fetchRelayerDiscoveredPackets(ctx context.Context, client http.Client, rpcAddress string, eventKey string, eventValue string) ([]relayerDiscoveredPacket, error) {
	if eventKey == "" || eventValue == "" {
		return nil, errors.New("event key and value are required")
	}
	endpoint, err := joinRelayerURL(rpcAddress, "/v1/events")
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("key", eventKey)
	query.Set("value", eventValue)
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("relayer event query returned HTTP %d", response.StatusCode)
	}
	var envelope relayerEventsEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	packets := make([]relayerDiscoveredPacket, 0, len(envelope.Records))
	for _, record := range envelope.Records {
		packet, ok := discoveredPacketFromEvent(record)
		if ok {
			packets = append(packets, packet)
		}
	}
	return packets, nil
}

func fetchRelayerLatestState(ctx context.Context, client http.Client, rpcAddress string) (relayerLatestState, error) {
	endpoint, err := joinRelayerURL(rpcAddress, "/v1/state/latest")
	if err != nil {
		return relayerLatestState{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return relayerLatestState{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return relayerLatestState{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return relayerLatestState{}, fmt.Errorf("relayer source state returned HTTP %d", response.StatusCode)
	}
	var state relayerLatestState
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		return relayerLatestState{}, err
	}
	if state.Height == 0 || state.AppHash == "" || state.ValidatorSetHash == "" {
		return relayerLatestState{}, errors.New("relayer source state response is missing height, app_hash, or validator_set_hash")
	}
	if err := validateRelayerHexHash(state.AppHash); err != nil {
		return relayerLatestState{}, fmt.Errorf("source app_hash: %w", err)
	}
	if err := validateRelayerHexHash(state.ValidatorSetHash); err != nil {
		return relayerLatestState{}, fmt.Errorf("source validator_set_hash: %w", err)
	}
	return state, nil
}

func validateRelayerHexHash(value string) error {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return err
	}
	if len(decoded) != len(types.Hash{}) {
		return fmt.Errorf("expected %d bytes, got %d", len(types.Hash{}), len(decoded))
	}
	return nil
}

type relayerEventsEnvelope struct {
	Records []relayerEventRecord `json:"records"`
}

type relayerEventRecord struct {
	Height  uint64       `json:"height"`
	TxIndex int          `json:"tx_index"`
	Event   relayerEvent `json:"event"`
}

type relayerEvent struct {
	Type       string                  `json:"type"`
	Attributes []relayerEventAttribute `json:"attributes"`
}

type relayerEventAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func discoveredPacketFromEvent(record relayerEventRecord) (relayerDiscoveredPacket, bool) {
	attributes := map[string]string{}
	for _, attribute := range record.Event.Attributes {
		if attribute.Key != "" {
			attributes[attribute.Key] = attribute.Value
		}
	}
	sequence, err := strconv.ParseUint(attributes["ibc_sequence"], 10, 64)
	if err != nil || sequence == 0 {
		return relayerDiscoveredPacket{}, false
	}
	data := attributes["ibc_data"]
	if data == "" {
		return relayerDiscoveredPacket{}, false
	}
	decodedData, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return relayerDiscoveredPacket{}, false
	}
	packet := relayerDiscoveredPacket{
		Height:             record.Height,
		TxIndex:            record.TxIndex,
		Sequence:           sequence,
		SourcePort:         attributes["ibc_source_port"],
		SourceChannel:      attributes["ibc_source_channel"],
		DestinationPort:    attributes["ibc_destination_port"],
		DestinationChannel: attributes["ibc_destination_channel"],
		Data:               string(decodedData),
	}
	if packet.SourcePort == "" || packet.SourceChannel == "" || packet.DestinationPort == "" || packet.DestinationChannel == "" {
		return relayerDiscoveredPacket{}, false
	}
	if rawTimeoutHeight := attributes["ibc_timeout_height"]; rawTimeoutHeight != "" {
		timeoutHeight, err := strconv.ParseUint(rawTimeoutHeight, 10, 64)
		if err != nil {
			return relayerDiscoveredPacket{}, false
		}
		packet.TimeoutHeight = timeoutHeight
	}
	return packet, true
}

func joinRelayerURL(baseURL string, path string) (string, error) {
	if baseURL == "" {
		return "", errors.New("rpc URL is required")
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
