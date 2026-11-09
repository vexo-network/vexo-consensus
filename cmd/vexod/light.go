package main

import (
	"context"
	"encoding/hex"
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

	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// lightBlockMaxBytes bounds one light block response: a proof and two validator sets.
const lightBlockMaxBytes = 32 << 20

func runLight(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("light subcommand is required")
	}
	switch args[0] {
	case "verify":
		return runLightVerify(writer, args[1:])
	default:
		return fmt.Errorf("unknown light subcommand %q", args[0])
	}
}

// lightVerifyResult is the block a light client verification ends on.
type lightVerifyResult struct {
	ChainID          string `json:"chain_id"`
	Height           uint64 `json:"height"`
	BlockHash        string `json:"block_hash"`
	AppHash          string `json:"app_hash"`
	Time             string `json:"time"`
	ValidatorSetHash string `json:"validator_set_hash"`
	Fetched          int    `json:"fetched"`
}

// runLightVerify verifies, against an untrusted RPC node, that the block at --height is final, from a
// block the operator trusts. The trusted block's own proof is fetched from the node and must hash to
// --trust-hash; its validator set must be the one its header names.
func runLightVerify(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("light verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rpcURL := flags.String("rpc", "", "http or https RPC URL of the untrusted full node")
	trustHeight, trustHash := snapshotTrustFlags(flags)
	height := flags.Uint64("height", 0, "height of the block to verify")
	trustingPeriod := flags.Duration("trusting-period", 0, "how long the trusted block's validators are assumed honest; must be shorter than the unbonding period")
	maxClockDrift := flags.Duration("max-clock-drift", 10*time.Second, "how far ahead of the local clock a block time may be")
	timeout := flags.Duration("timeout", 30*time.Second, "timeout of each RPC request")
	backend := flags.String("crypto-backend", string(config.CryptoBackendBLS), "the chain's crypto backend: bls, ed25519 or deterministic")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *rpcURL == "" || *height == 0 {
		return errors.New("--rpc and --height are required")
	}
	if *trustingPeriod <= 0 {
		return errors.New("--trusting-period is required: the time the trusted block's validators stay slashable after they leave the set bounds it")
	}
	trust, err := parseSnapshotTrust(*trustHeight, *trustHash)
	if err != nil {
		return err
	}
	suite, err := vexocrypto.NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackend(*backend)})
	if err != nil {
		return err
	}
	source, err := newHTTPLightSource(*rpcURL, *timeout)
	if err != nil {
		return err
	}
	ctx := context.Background()
	trusted, err := trustedLightBlock(ctx, source, trust)
	if err != nil {
		return err
	}
	client, err := finality.NewLightClient(trusted, source, finality.LightClientOptions{TrustingPeriod: *trustingPeriod, MaxClockDrift: *maxClockDrift}, suite)
	if err != nil {
		return err
	}
	fetched := source.fetched
	verified, err := client.VerifyTo(ctx, types.Height(*height))
	if err != nil {
		return err
	}
	header := verified.Header
	blockHash := finality.HeaderHash(header)
	return writeIndentedJSON(writer, lightVerifyResult{
		ChainID:          header.ChainID,
		Height:           uint64(header.Height),
		BlockHash:        hex.EncodeToString(blockHash[:]),
		AppHash:          hex.EncodeToString(header.AppHash[:]),
		Time:             time.Unix(0, header.TimeUnixNano).UTC().Format(time.RFC3339Nano),
		ValidatorSetHash: hex.EncodeToString(header.ValidatorSetHash[:]),
		Fetched:          source.fetched - fetched,
	})
}

// trustedLightBlock loads the trusted block from source: its header must hash to the trusted hash,
// and its next validator set must be the set the header names.
func trustedLightBlock(ctx context.Context, source finality.LightSource, trust snapshotTrust) (finality.TrustedBlock, error) {
	block, err := source.LightBlock(ctx, trust.Height)
	if err != nil {
		return finality.TrustedBlock{}, fmt.Errorf("trusted block %d: %w", trust.Height, err)
	}
	header := block.Proof.Header
	if header.Height != trust.Height || finality.HeaderHash(header) != trust.Hash {
		return finality.TrustedBlock{}, fmt.Errorf("%w: the node serves block %d %x, trusted block is %d %x", ErrUntrustedSnapshot, header.Height, finality.HeaderHash(header), trust.Height, trust.Hash)
	}
	return finality.NewTrustedBlock(header, block.NextValidatorSet)
}

// httpLightSource fetches light blocks from a full node's RPC.
type httpLightSource struct {
	base    *url.URL
	client  http.Client
	fetched int
}

func newHTTPLightSource(rawURL string, timeout time.Duration) (*httpLightSource, error) {
	base, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("RPC URL %q: %w", rawURL, err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, fmt.Errorf("RPC URL %q must be an http or https URL", rawURL)
	}
	return &httpLightSource{base: base, client: http.Client{Timeout: timeout}}, nil
}

func (source *httpLightSource) LightBlock(ctx context.Context, height types.Height) (finality.LightBlock, error) {
	target := *source.base
	target.Path = strings.TrimSuffix(target.Path, "/") + "/v1/light/" + strconv.FormatUint(uint64(height), 10)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return finality.LightBlock{}, err
	}
	response, err := source.client.Do(request)
	if err != nil {
		return finality.LightBlock{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, lightBlockMaxBytes+1))
	if err != nil {
		return finality.LightBlock{}, err
	}
	if len(data) > lightBlockMaxBytes {
		return finality.LightBlock{}, fmt.Errorf("light block %d exceeds %d bytes", height, lightBlockMaxBytes)
	}
	if response.StatusCode != http.StatusOK {
		return finality.LightBlock{}, fmt.Errorf("light block %d: status %d: %s", height, response.StatusCode, strings.TrimSpace(string(data)))
	}
	var decoded vexorpc.LightBlockResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return finality.LightBlock{}, fmt.Errorf("light block %d: %w", height, err)
	}
	set, err := lightValidatorSet(decoded.ValidatorSet)
	if err != nil {
		return finality.LightBlock{}, err
	}
	nextSet, err := lightValidatorSet(decoded.NextValidatorSet)
	if err != nil {
		return finality.LightBlock{}, err
	}
	source.fetched++
	return finality.LightBlock{Proof: decoded.Proof, ValidatorSet: set, NextValidatorSet: nextSet}, nil
}

func lightValidatorSet(validators []validator.Validator) (validator.Set, error) {
	registry, err := validator.NewInMemoryRegistry(nil, validators)
	if err != nil {
		return nil, err
	}
	return registry.ValidatorSet(context.Background(), 1)
}
