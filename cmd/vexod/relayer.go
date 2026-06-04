package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	clientID := flags.String("client-id", "", "IBC client id")
	height := flags.Uint64("height", 0, "counterparty latest height")
	validatorSetHash := flags.String("validator-set-hash", "", "counterparty validator set hash hex")
	stateRoot := flags.String("state-root", "", "counterparty state root hex")
	submit := flags.Bool("submit", false, "submit the built transaction to --rpc")
	tags := relayerTxTagFlags(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *clientID == "" || *height == 0 || *validatorSetHash == "" || *stateRoot == "" {
		return errors.New("client-id, height, validator-set-hash, and state-root are required")
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

func runRelayerLoop(writer io.Writer, args []string, client http.Client) error {
	flags := flag.NewFlagSet("relayer loop", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := packetRelayerFlags(flags)
	mode := flags.String("mode", "", "packet relay mode: ack or timeout")
	ack := flags.String("ack", "", "acknowledgement bytes as plain text when --mode ack")
	proofRPC := flags.String("proof-rpc", "", "source RPC base URL used to poll packet proofs")
	interval := flags.Duration("interval", 5*time.Second, "poll interval")
	maxIterations := flags.Uint64("max-iterations", 0, "maximum poll iterations; 0 means run until interrupted")
	continueOnError := flags.Bool("continue-on-error", false, "continue polling after proof fetch or submit errors")
	submit := flags.Bool("submit", false, "submit the built transaction to --rpc")
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
	return runRelayerPollingLoop(context.Background(), writer, client, relayerLoopConfig{
		Mode:            *mode,
		RPCAddress:      options.rpcAddress,
		ProofRPC:        *proofRPC,
		PacketArgs:      packetArgs,
		Ack:             *ack,
		Tags:            tags,
		Submit:          *submit,
		Interval:        *interval,
		MaxIterations:   *maxIterations,
		ContinueOnError: *continueOnError,
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
	Interval        string              `json:"interval,omitempty"`
	MaxIterations   uint64              `json:"max_iterations,omitempty"`
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
	for _, job := range document.Jobs {
		job := job
		cfg, err := relayerLoopConfigFromJob(job)
		if err != nil {
			return err
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
	MaxIterations   uint64
	ContinueOnError bool
}

func runRelayerPollingLoop(ctx context.Context, writer io.Writer, client http.Client, cfg relayerLoopConfig) error {
	for iteration := uint64(1); ; iteration++ {
		proof, err := fetchRelayerPacketProof(ctx, client, cfg.ProofRPC, cfg.PacketArgs)
		if err != nil {
			fmt.Fprintf(writer, "iteration: %d\n", iteration)
			fmt.Fprintf(writer, "proof_error: %v\n", err)
			if !cfg.ContinueOnError {
				return err
			}
			if cfg.MaxIterations > 0 && iteration >= cfg.MaxIterations {
				return nil
			}
			if err := waitRelayerLoopInterval(ctx, cfg.Interval); err != nil {
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
				fmt.Fprintf(writer, "submit_error: %v\n", err)
				if !cfg.ContinueOnError {
					return err
				}
			} else {
				fmt.Fprintf(writer, "submitted: true\n")
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
