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
	"strconv"
	"strings"
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
