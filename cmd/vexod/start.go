package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	appmodules "github.com/vexo-network/vexo-consensus/modules"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	defaultRPCAddress = "127.0.0.1:26657"
	defaultP2PAddress = "127.0.0.1:26656"
)

type startPlanDocument struct {
	ChainID          string   `json:"chain_id"`
	ValidatorID      string   `json:"validator_id,omitempty"`
	DataDir          string   `json:"data_dir"`
	ConfigPath       string   `json:"config_path"`
	GenesisPath      string   `json:"genesis_path"`
	KeyPath          string   `json:"key_path"`
	RotationKeyPaths []string `json:"rotation_key_paths,omitempty"`
	ValidatorN       int      `json:"validator_count"`
	KeyType          string   `json:"key_type,omitempty"`
	PublicKey        string   `json:"public_key,omitempty"`
	DryRun           bool     `json:"dry_run"`
}

type startInputs struct {
	Config  vexonode.Config
	Genesis vexonode.Genesis
	Signer  vexocrypto.Signer
	Plan    startPlanDocument
}

type startRuntimeConfig struct {
	RPCEnabled              bool
	RPCAddress              string
	RPCAdminToken           string
	RPCEnablePprof          bool
	RPCRequestTimeout       time.Duration
	RPCMaxRequestBytes      int64
	RPCRateLimitWindow      time.Duration
	RPCRateLimitMaxRequests int
	LogFormat               string
	LogLevel                string
	LogCommitEvents         bool
	LogPeerEvents           bool
	ConsensusLoopEnabled    bool
	ConsensusLoop           vexonode.ConsensusLoopConfig
	P2PEnabled              bool
	P2PListenAddress        string
	P2PPeers                map[p2p.PeerID]string
	P2PSeeds                map[p2p.PeerID]string
	P2PNetworkID            string
	P2PMaxMessageBytes      uint64
	P2PMaxPeers             int
	P2PAuthToken            string
	AddrBookPath            string
	AddrBookMaxFailures     int
}

type peerFlags map[p2p.PeerID]string

type stringListFlags []string

func runStart(writer io.Writer, args []string) error {
	return runStartWithContext(context.Background(), writer, args)
}

func runStartWithContext(ctx context.Context, writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("start", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	keyPath := flags.String("key", "", "key file path")
	rotationKeys := stringListFlags{}
	flags.Var(&rotationKeys, "rotation-key", "additional validator key file path with active-from/active-until metadata; may be repeated")
	dryRun := flags.Bool("dry-run", false, "validate startup inputs without running a node")
	run := flags.Bool("run", false, "start the node and block until context cancellation")
	rpcEnabled := flags.Bool("rpc", true, "run HTTP RPC server with node")
	rpcAdminToken := flags.String("rpc-admin-token", "", "admin token required for protected RPC endpoints")
	rpcEnablePprof := flags.Bool("rpc-pprof", false, "enable net/http/pprof endpoints under /debug/pprof")
	rpcRequestTimeout := flags.Duration("rpc-request-timeout", 0, "HTTP RPC request timeout")
	rpcMaxRequestBytes := flags.Int64("rpc-max-request-bytes", 0, "maximum HTTP RPC request body bytes")
	rpcRateLimitWindow := flags.Duration("rpc-rate-limit-window", 0, "HTTP RPC rate limit window")
	rpcRateLimitMaxRequests := flags.Int("rpc-rate-limit-max", 0, "maximum HTTP RPC requests per client per window")
	consensusLoopEnabled := flags.Bool("consensus-loop", true, "start local consensus loop with node")
	consensusInterval := flags.Duration("consensus-interval", 0, "local consensus loop tick interval")
	timeoutPropose := flags.Duration("timeout-propose", 0, "consensus proposal timeout")
	timeoutPrevote := flags.Duration("timeout-prevote", 0, "consensus prevote timeout")
	timeoutPrecommit := flags.Duration("timeout-precommit", 0, "consensus precommit timeout")
	timeoutCommit := flags.Duration("timeout-commit", 0, "minimum delay after each committed block")
	roundTimeout := flags.Duration("consensus-round-timeout", 0, "deprecated aggregate round timeout")
	maxBlockBytes := flags.Int64("consensus-max-block-bytes", 0, "maximum bytes to include when building a block")
	createEmptyBlocks := flags.Bool("create-empty-blocks", false, "create blocks even when the mempool is empty")
	p2pEnabled := flags.Bool("p2p", true, "run gRPC P2P transport with node")
	p2pNetworkID := flags.String("p2p-network", "", "P2P network id; defaults to chain id")
	p2pMaxMessageBytes := flags.Uint64("p2p-max-message-bytes", 0, "maximum P2P message bytes")
	p2pMaxPeers := flags.Int("p2p-max-peers", 0, "maximum configured P2P peers")
	p2pAuthToken := flags.String("p2p-auth-token", "", "shared P2P handshake auth token")
	addrBookPath := flags.String("addr-book", "", "P2P address book path; defaults to <home>/addrbook.json")
	addrBookMaxFailures := flags.Int("addr-book-max-failures", 3, "failed dial attempts before addr book bans a peer")
	logFormat := flags.String("log-format", "text", "operational log format: text or json")
	logLevel := flags.String("log-level", "info", "operational log level")
	logCommitEvents := flags.Bool("log-commit-events", true, "log committed block events")
	logPeerEvents := flags.Bool("log-peer-events", true, "log P2P peer connection events")
	peers := peerFlags{}
	flags.Var(peers, "peer", "persistent peer in id=host:port form; may be repeated")
	seeds := peerFlags{}
	flags.Var(seeds, "seed", "seed peer used for bootstrap discovery in id=host:port form; may be repeated")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	visited := visitedFlags(flags)
	inputs, err := loadStartInputs(*home, *configPath, *genesisPath, *keyPath, []string(rotationKeys), *dryRun)
	if err != nil {
		return err
	}
	runtimeConfig, err := loadStartRuntimeConfig(*home, *configPath)
	if err != nil {
		return err
	}
	applyStartFlagOverrides(&runtimeConfig, visited, startFlagValues{
		rpcEnabled:              *rpcEnabled,
		rpcAdminToken:           *rpcAdminToken,
		rpcEnablePprof:          *rpcEnablePprof,
		rpcRequestTimeout:       *rpcRequestTimeout,
		rpcMaxRequestBytes:      *rpcMaxRequestBytes,
		rpcRateLimitWindow:      *rpcRateLimitWindow,
		rpcRateLimitMaxRequests: *rpcRateLimitMaxRequests,
		logFormat:               *logFormat,
		logLevel:                *logLevel,
		logCommitEvents:         *logCommitEvents,
		logPeerEvents:           *logPeerEvents,
		consensusLoopEnabled:    *consensusLoopEnabled,
		consensusInterval:       *consensusInterval,
		timeoutPropose:          *timeoutPropose,
		timeoutPrevote:          *timeoutPrevote,
		timeoutPrecommit:        *timeoutPrecommit,
		timeoutCommit:           *timeoutCommit,
		roundTimeout:            *roundTimeout,
		maxBlockBytes:           *maxBlockBytes,
		createEmptyBlocks:       *createEmptyBlocks,
		p2pEnabled:              *p2pEnabled,
		p2pNetworkID:            *p2pNetworkID,
		p2pMaxMessageBytes:      *p2pMaxMessageBytes,
		p2pMaxPeers:             *p2pMaxPeers,
		p2pAuthToken:            *p2pAuthToken,
		addrBookPath:            resolveAddrBookPath(*home, *addrBookPath),
		addrBookMaxFailures:     *addrBookMaxFailures,
		peers:                   peers,
		seeds:                   seeds,
	})
	plan := inputs.Plan
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	if *run {
		runCtx, stopSignals := signal.NotifyContext(ctx, shutdownSignals()...)
		defer stopSignals()
		return runStartNode(runCtx, writer, inputs, runtimeConfig)
	}
	if !plan.DryRun {
		if _, err := buildStartNode(inputs); err != nil {
			return err
		}
		fmt.Fprintf(writer, "startup inputs valid\n")
		if inputs.Config.ValidatorID != "" {
			fmt.Fprintf(writer, "validator signer loaded\n")
		} else {
			fmt.Fprintf(writer, "non-validator archive node\n")
		}
		fmt.Fprintf(writer, "start execution is not enabled yet; rerun with --dry-run for readiness checks\n")
	} else {
		fmt.Fprintf(writer, "startup dry-run valid\n")
	}
	fmt.Fprintf(writer, "chain_id: %s\n", plan.ChainID)
	fmt.Fprintf(writer, "validator_id: %s\n", plan.ValidatorID)
	fmt.Fprintf(writer, "validators: %d\n", plan.ValidatorN)
	fmt.Fprintf(writer, "data_dir: %s\n", plan.DataDir)
	fmt.Fprintf(writer, "key_type: %s\n", plan.KeyType)
	fmt.Fprintf(writer, "public_key: %s\n", plan.PublicKey)
	return nil
}

type startFlagValues struct {
	rpcEnabled              bool
	rpcAdminToken           string
	rpcEnablePprof          bool
	rpcRequestTimeout       time.Duration
	rpcMaxRequestBytes      int64
	rpcRateLimitWindow      time.Duration
	rpcRateLimitMaxRequests int
	logFormat               string
	logLevel                string
	logCommitEvents         bool
	logPeerEvents           bool
	consensusLoopEnabled    bool
	consensusInterval       time.Duration
	timeoutPropose          time.Duration
	timeoutPrevote          time.Duration
	timeoutPrecommit        time.Duration
	timeoutCommit           time.Duration
	roundTimeout            time.Duration
	maxBlockBytes           int64
	createEmptyBlocks       bool
	p2pEnabled              bool
	p2pNetworkID            string
	p2pMaxMessageBytes      uint64
	p2pMaxPeers             int
	p2pAuthToken            string
	addrBookPath            string
	addrBookMaxFailures     int
	peers                   peerFlags
	seeds                   peerFlags
}

func visitedFlags(flags *flag.FlagSet) map[string]bool {
	visited := make(map[string]bool)
	flags.Visit(func(flagInfo *flag.Flag) {
		visited[flagInfo.Name] = true
	})
	return visited
}

func applyStartFlagOverrides(cfg *startRuntimeConfig, visited map[string]bool, values startFlagValues) {
	if visited["rpc"] {
		cfg.RPCEnabled = values.rpcEnabled
	}
	if visited["rpc-admin-token"] {
		cfg.RPCAdminToken = values.rpcAdminToken
	}
	if visited["rpc-pprof"] {
		cfg.RPCEnablePprof = values.rpcEnablePprof
	}
	if visited["rpc-request-timeout"] {
		cfg.RPCRequestTimeout = values.rpcRequestTimeout
	}
	if visited["rpc-max-request-bytes"] {
		cfg.RPCMaxRequestBytes = values.rpcMaxRequestBytes
	}
	if visited["rpc-rate-limit-window"] {
		cfg.RPCRateLimitWindow = values.rpcRateLimitWindow
	}
	if visited["rpc-rate-limit-max"] {
		cfg.RPCRateLimitMaxRequests = values.rpcRateLimitMaxRequests
	}
	if visited["log-format"] {
		cfg.LogFormat = values.logFormat
	}
	if visited["log-level"] {
		cfg.LogLevel = values.logLevel
	}
	if visited["log-commit-events"] {
		cfg.LogCommitEvents = values.logCommitEvents
	}
	if visited["log-peer-events"] {
		cfg.LogPeerEvents = values.logPeerEvents
	}
	if visited["consensus-loop"] {
		cfg.ConsensusLoopEnabled = values.consensusLoopEnabled
	}
	if visited["consensus-interval"] {
		cfg.ConsensusLoop.Interval = values.consensusInterval
	}
	if visited["timeout-propose"] {
		cfg.ConsensusLoop.TimeoutPropose = values.timeoutPropose
	}
	if visited["timeout-prevote"] {
		cfg.ConsensusLoop.TimeoutPrevote = values.timeoutPrevote
	}
	if visited["timeout-precommit"] {
		cfg.ConsensusLoop.TimeoutPrecommit = values.timeoutPrecommit
	}
	if visited["timeout-commit"] {
		cfg.ConsensusLoop.TimeoutCommit = values.timeoutCommit
	}
	if visited["consensus-round-timeout"] {
		cfg.ConsensusLoop.RoundTimeout = values.roundTimeout
	}
	if visited["consensus-max-block-bytes"] {
		cfg.ConsensusLoop.MaxBlockBytes = values.maxBlockBytes
	}
	if visited["create-empty-blocks"] {
		cfg.ConsensusLoop.CreateEmptyBlocks = values.createEmptyBlocks
	}
	if visited["p2p"] {
		cfg.P2PEnabled = values.p2pEnabled
	}
	if visited["p2p-network"] {
		cfg.P2PNetworkID = values.p2pNetworkID
	}
	if visited["p2p-max-message-bytes"] {
		cfg.P2PMaxMessageBytes = values.p2pMaxMessageBytes
	}
	if visited["p2p-max-peers"] {
		cfg.P2PMaxPeers = values.p2pMaxPeers
	}
	if visited["p2p-auth-token"] {
		cfg.P2PAuthToken = values.p2pAuthToken
	}
	if visited["addr-book"] {
		cfg.AddrBookPath = values.addrBookPath
	}
	if visited["addr-book-max-failures"] {
		cfg.AddrBookMaxFailures = values.addrBookMaxFailures
	}
	if visited["peer"] {
		cfg.P2PPeers = values.peers
	}
	if visited["seed"] {
		cfg.P2PSeeds = values.seeds
	}
}

func runStartNode(ctx context.Context, writer io.Writer, inputs startInputs, runtimeConfig startRuntimeConfig) error {
	runtimeConfig = applyNetworkRuntimeDefaults(inputs, runtimeConfig)
	if runtimeConfig.LogFormat != "" && runtimeConfig.LogFormat != "text" && runtimeConfig.LogFormat != "json" {
		return fmt.Errorf("unsupported log format %q", runtimeConfig.LogFormat)
	}
	if runtimeConfig.LogLevel == "" {
		runtimeConfig.LogLevel = "info"
	}
	logEvent := newOperationalLogger(writer, runtimeConfig.LogFormat, runtimeConfig.LogLevel)
	node, p2pWire, err := buildRuntimeNode(inputs, runtimeConfig)
	if err != nil {
		return err
	}
	if runtimeConfig.LogCommitEvents {
		node.WithEventLogger(logEvent)
	}
	if runtimeConfig.LogPeerEvents && p2pWire != nil {
		p2pWire.SetPeerEventHook(func(event transport.PeerEvent) {
			fields := map[string]any{
				"peer_id":   event.PeerID,
				"direction": event.Direction,
			}
			if event.Address != "" {
				fields["address"] = event.Address
			}
			if event.Reason != "" {
				fields["reason"] = event.Reason
			}
			if !event.BackoffUntil.IsZero() {
				fields["backoff_until"] = event.BackoffUntil.UTC().Format(time.RFC3339Nano)
			}
			logEvent(event.Type, fields)
		})
	}
	if err := node.Start(ctx); err != nil {
		return err
	}
	consensusLoopStarted := false
	if runtimeConfig.ConsensusLoopEnabled {
		if err := node.StartConsensusLoop(ctx, runtimeConfig.ConsensusLoop); err != nil {
			_ = node.Stop(context.Background())
			return err
		}
		consensusLoopStarted = true
		logEvent("consensus_loop_running", map[string]any{"chain_id": inputs.Plan.ChainID})
	}
	serverErr := make(chan error, 1)
	rpcShutdown := func(context.Context) error { return nil }
	if runtimeConfig.RPCEnabled {
		address, shutdown, err := startRPCServerWithConfig(node, runtimeConfig.RPCAddress, vexorpc.Config{
			AdminToken:           runtimeConfig.RPCAdminToken,
			EnablePprof:          runtimeConfig.RPCEnablePprof,
			RequestTimeout:       runtimeConfig.RPCRequestTimeout,
			MaxRequestBytes:      runtimeConfig.RPCMaxRequestBytes,
			RateLimitWindow:      runtimeConfig.RPCRateLimitWindow,
			RateLimitMaxRequests: runtimeConfig.RPCRateLimitMaxRequests,
		}, serverErr)
		if err != nil {
			_ = node.Stop(context.Background())
			return err
		}
		rpcShutdown = shutdown
		logEvent("rpc_listening", map[string]any{"rpc_address": address, "pprof": runtimeConfig.RPCEnablePprof})
	}
	if p2pWire != nil {
		logEvent("p2p_listening", map[string]any{"p2p_address": p2pWire.Address(), "p2p_peers": len(runtimeConfig.P2PPeers), "p2p_seeds": len(runtimeConfig.P2PSeeds)})
	}
	logEvent("node_running", map[string]any{"chain_id": inputs.Plan.ChainID, "validator_id": inputs.Plan.ValidatorID, "data_dir": inputs.Plan.DataDir, "pid": os.Getpid(), "version": version, "go_version": runtime.Version()})
	select {
	case <-ctx.Done():
	case err := <-serverErr:
		_ = node.Stop(context.Background())
		return err
	}
	logEvent("shutdown_requested", map[string]any{"chain_id": inputs.Plan.ChainID})
	if consensusLoopStarted {
		if err := node.StopConsensusLoop(context.Background()); err != nil && err != vexonode.ErrLoopNotRunning {
			return err
		}
	}
	if err := rpcShutdown(context.Background()); err != nil {
		return err
	}
	if err := node.Stop(context.Background()); err != nil {
		return err
	}
	logEvent("node_stopped", map[string]any{"chain_id": inputs.Plan.ChainID})
	return nil
}

func newOperationalLogger(writer io.Writer, format string, level string) vexonode.EventLogger {
	var mutex sync.Mutex
	return func(event string, fields map[string]any) {
		mutex.Lock()
		defer mutex.Unlock()
		writeOperationalLog(writer, format, level, event, fields)
	}
}

func writeOperationalLog(writer io.Writer, format string, level string, event string, fields map[string]any) {
	if level == "" {
		level = "info"
	}
	if format == "json" {
		record := map[string]any{
			"event":   event,
			"level":   level,
			"ts":      time.Now().UTC().Format(time.RFC3339Nano),
			"version": version,
		}
		for key, value := range fields {
			record[key] = value
		}
		encoded, err := json.Marshal(record)
		if err == nil {
			fmt.Fprintf(writer, "%s\n", encoded)
			return
		}
	}
	fmt.Fprintf(writer, "%s\n", strings.ReplaceAll(event, "_", " "))
	fmt.Fprintf(writer, "level: %s\n", level)
	for key, value := range fields {
		fmt.Fprintf(writer, "%s: %v\n", key, value)
	}
}

func startRPCServer(provider vexorpc.StatusProvider, address string, serverErr chan<- error) (string, func(context.Context) error, error) {
	return startRPCServerWithConfig(provider, address, vexorpc.Config{}, serverErr)
}

func startRPCServerWithConfig(provider vexorpc.StatusProvider, address string, cfg vexorpc.Config, serverErr chan<- error) (string, func(context.Context) error, error) {
	if address == "" {
		address = defaultRPCAddress
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return "", nil, err
	}
	cfg.Address = address
	server := vexorpc.NewServer(provider, cfg)
	go func() {
		serverErr <- server.Start(listener)
	}()
	return listener.Addr().String(), server.Shutdown, nil
}

func loadStartPlan(home string, configPath string, genesisPath string, keyPath string, dryRun bool) (startPlanDocument, error) {
	inputs, err := loadStartInputs(home, configPath, genesisPath, keyPath, nil, dryRun)
	if err != nil {
		return startPlanDocument{}, err
	}
	return inputs.Plan, nil
}

func loadStartInputs(home string, configPath string, genesisPath string, keyPath string, rotationKeyPaths []string, dryRun bool) (startInputs, error) {
	resolvedConfigPath := resolveConfigPath(home, configPath)
	resolvedGenesisPath := resolveGenesisPath(home, genesisPath)
	cfg, err := loadNodeConfig(resolvedConfigPath)
	if err != nil {
		return startInputs{}, err
	}
	genesis, err := loadGenesis(resolvedGenesisPath)
	if err != nil {
		return startInputs{}, err
	}
	if err := genesis.Validate(cfg.Chain.ChainID); err != nil {
		return startInputs{}, err
	}
	var resolvedKeyPath string
	var resolvedRotationKeyPaths []string
	var keyType string
	var publicKey string
	var signer vexocrypto.Signer
	if cfg.ValidatorID != "" {
		resolvedKeyPath = resolveKeyPath(home, keyPath)
		keyDocument, err := vexocrypto.LoadKeyDocument(resolvedKeyPath)
		if err != nil {
			return startInputs{}, err
		}
		rotationDocuments := []vexocrypto.KeyDocument{keyDocument}
		for _, rotationKeyPath := range rotationKeyPaths {
			resolvedRotationKeyPath := resolveRotationKeyPath(home, rotationKeyPath)
			rotationDocument, err := vexocrypto.LoadKeyDocument(resolvedRotationKeyPath)
			if err != nil {
				return startInputs{}, err
			}
			rotationDocuments = append(rotationDocuments, rotationDocument)
			resolvedRotationKeyPaths = append(resolvedRotationKeyPaths, resolvedRotationKeyPath)
		}
		loadedSigner, err := signerFromKeyDocuments(rotationDocuments)
		if err != nil {
			return startInputs{}, err
		}
		signer = loadedSigner
		keyType = keyDocument.Type
		if len(rotationDocuments) > 1 {
			keyType = "keyring"
		}
		publicKey = keyDocument.PublicKey
		genesis = withLocalValidatorPublicKey(genesis, cfg.ValidatorID, signer.PublicKey())
	}
	plan := startPlanDocument{
		ChainID:          cfg.Chain.ChainID,
		ValidatorID:      string(cfg.ValidatorID),
		DataDir:          cfg.DataDir,
		ConfigPath:       resolvedConfigPath,
		GenesisPath:      resolvedGenesisPath,
		KeyPath:          resolvedKeyPath,
		RotationKeyPaths: resolvedRotationKeyPaths,
		ValidatorN:       len(genesis.Validators),
		KeyType:          keyType,
		PublicKey:        publicKey,
		DryRun:           dryRun,
	}
	return startInputs{
		Config:  cfg,
		Genesis: genesis,
		Signer:  signer,
		Plan:    plan,
	}, nil
}

func loadStartRuntimeConfig(home string, configPath string) (startRuntimeConfig, error) {
	resolvedConfigPath := resolveConfigPath(home, configPath)
	document, err := readConfigDocument(resolvedConfigPath)
	if err != nil {
		return startRuntimeConfig{}, err
	}
	networkDocument, err := loadNetworkConfigForConfig(resolvedConfigPath, document)
	if err != nil {
		return startRuntimeConfig{}, err
	}
	consensusDocument, err := loadConsensusConfigForConfig(resolvedConfigPath, document)
	if err != nil {
		return startRuntimeConfig{}, err
	}
	logDocument, err := loadLogConfigForConfig(resolvedConfigPath, document)
	if err != nil {
		return startRuntimeConfig{}, err
	}
	return runtimeConfigFromDocuments(home, document, networkDocument, consensusDocument, logDocument)
}

func runtimeConfigFromDocument(home string, document configDocument) (startRuntimeConfig, error) {
	return runtimeConfigFromDocuments(home, document, document.LegacyNetwork, document.LegacyConsensus, document.LegacyLog)
}

func runtimeConfigFromDocuments(home string, document configDocument, networkDocument networkConfigDocument, consensusDocument consensusConfigDocument, logDocument logConfigDocument) (startRuntimeConfig, error) {
	runtime := runtimeConfig{
		RPC:       networkDocument.RPC,
		P2P:       networkDocument.P2P,
		Consensus: consensusDocument.Consensus,
		Log:       logDocument.Log,
	}
	if runtimeConfigIsZero(runtime) {
		runtime = defaultRuntimeConfig(document.ValidatorID)
	}
	cfg := startRuntimeConfig{
		RPCEnabled:           runtime.RPC.Enabled,
		RPCAddress:           runtime.RPC.Address,
		RPCAdminToken:        runtime.RPC.AdminToken,
		RPCEnablePprof:       runtime.RPC.EnablePprof,
		RPCMaxRequestBytes:   runtime.RPC.MaxRequestBytes,
		P2PEnabled:           runtime.P2P.Enabled,
		P2PListenAddress:     runtime.P2P.ListenAddress,
		P2PNetworkID:         runtime.P2P.NetworkID,
		P2PMaxMessageBytes:   runtime.P2P.MaxMessageBytes,
		P2PMaxPeers:          runtime.P2P.MaxPeers,
		P2PAuthToken:         runtime.P2P.AuthToken,
		AddrBookPath:         resolveAddrBookPath(home, runtime.P2P.AddrBookPath),
		AddrBookMaxFailures:  runtime.P2P.AddrBookMaxFails,
		P2PPeers:             stringPeerMap(runtime.P2P.Peers),
		P2PSeeds:             stringPeerMap(runtime.P2P.Seeds),
		ConsensusLoopEnabled: runtime.Consensus.LoopEnabled,
		ConsensusLoop: vexonode.ConsensusLoopConfig{
			MaxBlockBytes:     runtime.Consensus.MaxBlockBytes,
			CreateEmptyBlocks: runtime.Consensus.CreateEmptyBlocks,
		},
		LogFormat: runtime.Log.Format,
		LogLevel:  runtime.Log.Level,
	}
	cfg.LogCommitEvents = true
	if runtime.Log.CommitEvents != nil {
		cfg.LogCommitEvents = *runtime.Log.CommitEvents
	}
	cfg.LogPeerEvents = true
	if runtime.Log.PeerEvents != nil {
		cfg.LogPeerEvents = *runtime.Log.PeerEvents
	}
	if cfg.RPCAddress == "" {
		cfg.RPCAddress = defaultRPCAddress
	}
	if cfg.P2PListenAddress == "" {
		cfg.P2PListenAddress = defaultP2PAddress
	}
	if cfg.AddrBookMaxFailures == 0 {
		cfg.AddrBookMaxFailures = 3
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "text"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if runtime.RPC.RequestTimeout != "" {
		duration, err := time.ParseDuration(runtime.RPC.RequestTimeout)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.rpc.request_timeout: %w", err)
		}
		cfg.RPCRequestTimeout = duration
	}
	if runtime.RPC.RateLimitWindow != "" {
		duration, err := time.ParseDuration(runtime.RPC.RateLimitWindow)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.rpc.rate_limit_window: %w", err)
		}
		cfg.RPCRateLimitWindow = duration
	}
	if runtime.Consensus.Interval != "" {
		duration, err := time.ParseDuration(runtime.Consensus.Interval)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.consensus.interval: %w", err)
		}
		cfg.ConsensusLoop.Interval = duration
	}
	if runtime.Consensus.TimeoutPropose != "" {
		duration, err := time.ParseDuration(runtime.Consensus.TimeoutPropose)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.consensus.timeout_propose: %w", err)
		}
		cfg.ConsensusLoop.TimeoutPropose = duration
	}
	if runtime.Consensus.TimeoutPrevote != "" {
		duration, err := time.ParseDuration(runtime.Consensus.TimeoutPrevote)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.consensus.timeout_prevote: %w", err)
		}
		cfg.ConsensusLoop.TimeoutPrevote = duration
	}
	if runtime.Consensus.TimeoutPrecommit != "" {
		duration, err := time.ParseDuration(runtime.Consensus.TimeoutPrecommit)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.consensus.timeout_precommit: %w", err)
		}
		cfg.ConsensusLoop.TimeoutPrecommit = duration
	}
	if runtime.Consensus.TimeoutCommit != "" {
		duration, err := time.ParseDuration(runtime.Consensus.TimeoutCommit)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.consensus.timeout_commit: %w", err)
		}
		cfg.ConsensusLoop.TimeoutCommit = duration
	}
	if runtime.Consensus.RoundTimeout != "" {
		duration, err := time.ParseDuration(runtime.Consensus.RoundTimeout)
		if err != nil {
			return startRuntimeConfig{}, fmt.Errorf("runtime.consensus.round_timeout: %w", err)
		}
		cfg.ConsensusLoop.RoundTimeout = duration
	}
	return cfg, nil
}

func runtimeConfigIsZero(runtime runtimeConfig) bool {
	return runtime.RPC == (runtimeRPCConfig{}) &&
		runtime.P2P.Enabled == false &&
		runtime.P2P.ListenAddress == "" &&
		runtime.P2P.NetworkID == "" &&
		runtime.P2P.MaxMessageBytes == 0 &&
		runtime.P2P.MaxPeers == 0 &&
		runtime.P2P.AuthToken == "" &&
		runtime.P2P.AddrBookPath == "" &&
		runtime.P2P.AddrBookMaxFails == 0 &&
		len(runtime.P2P.Peers) == 0 &&
		len(runtime.P2P.Seeds) == 0 &&
		runtime.Consensus == (runtimeConsensusConfig{}) &&
		runtime.Log == (runtimeLogConfig{})
}

func stringPeerMap(peers map[string]string) map[p2p.PeerID]string {
	if len(peers) == 0 {
		return nil
	}
	out := make(map[p2p.PeerID]string, len(peers))
	for peerID, address := range peers {
		out[p2p.PeerID(peerID)] = address
	}
	return out
}

func signerFromKeyDocuments(documents []vexocrypto.KeyDocument) (vexocrypto.Signer, error) {
	if len(documents) == 1 {
		return documents[0].SignerWithPassphrase(resolvePassphrase(""))
	}
	return vexocrypto.NewKeyRingPolicySignerFromDocuments(resolvePassphrase(""), documents...)
}

func resolveRotationKeyPath(home string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if home == "" {
		home = defaultHomeDir
	}
	return filepath.Join(home, path)
}

func buildStartNode(inputs startInputs) (*vexonode.Node, error) {
	node, _, err := buildRuntimeNode(inputs, startRuntimeConfig{})
	return node, err
}

func buildRuntimeNode(inputs startInputs, runtimeConfig startRuntimeConfig) (*vexonode.Node, *transport.GRPCTransport, error) {
	runtimeConfig = applyNetworkRuntimeDefaults(inputs, runtimeConfig)
	application, err := appmodules.NewRuntimeWithExecution(inputs.Config.Chain.ChainID, inputs.Config.Chain.Application, inputs.Config.Chain.Execution)
	if err != nil {
		return nil, nil, err
	}
	node, err := vexonode.New(inputs.Config, inputs.Genesis, application)
	if err != nil {
		return nil, nil, err
	}
	node.WithSigner(inputs.Signer)
	if runtimeConfig.P2PEnabled {
		wire, err := buildGRPCTransport(inputs, runtimeConfig)
		if err != nil {
			return nil, nil, err
		}
		node.WithTransport(wire)
		return node, wire, nil
	}
	return node, nil, nil
}

func applyNetworkRuntimeDefaults(inputs startInputs, runtimeConfig startRuntimeConfig) startRuntimeConfig {
	if runtimeConfig.RPCAddress == "" || runtimeConfig.RPCAddress == defaultRPCAddress {
		if address := validatorMetadata(inputs.Genesis, inputs.Config.ValidatorID, "rpc_listen_address"); address != "" {
			runtimeConfig.RPCAddress = address
		}
	}
	if runtimeConfig.P2PListenAddress == "" || runtimeConfig.P2PListenAddress == defaultP2PAddress {
		if address := validatorMetadata(inputs.Genesis, inputs.Config.ValidatorID, "p2p_listen_address"); address != "" {
			runtimeConfig.P2PListenAddress = address
		}
	}
	if len(runtimeConfig.P2PPeers) == 0 {
		runtimeConfig.P2PPeers = peersFromGenesis(inputs.Genesis, inputs.Config.ValidatorID)
	}
	return runtimeConfig
}

func validatorMetadata(genesis vexonode.Genesis, validatorID types.ValidatorID, key string) string {
	for _, validatorInfo := range genesis.Validators {
		if validatorInfo.ID != validatorID {
			continue
		}
		return validatorInfo.Metadata[key]
	}
	return ""
}

func peersFromGenesis(genesis vexonode.Genesis, self types.ValidatorID) map[p2p.PeerID]string {
	peers := make(map[p2p.PeerID]string)
	for _, validatorInfo := range genesis.Validators {
		if validatorInfo.ID == self {
			continue
		}
		address := validatorInfo.Metadata["p2p_address"]
		if address == "" {
			continue
		}
		peers[p2p.PeerID(validatorInfo.ID)] = address
	}
	return peers
}

func buildGRPCTransport(inputs startInputs, runtimeConfig startRuntimeConfig) (*transport.GRPCTransport, error) {
	networkID := runtimeConfig.P2PNetworkID
	if networkID == "" {
		networkID = inputs.Config.Chain.ChainID
	}
	addrBook, err := p2p.OpenAddrBookWithPolicy(runtimeConfig.AddrBookPath, runtimeConfig.AddrBookMaxFailures)
	if err != nil {
		return nil, err
	}
	addrBook.Merge(runtimeConfig.P2PPeers, "cli-peer", true)
	addrBook.Merge(runtimeConfig.P2PSeeds, "cli-seed", true)
	if err := addrBook.Save(); err != nil {
		return nil, err
	}
	peers := mergePeerMaps(addrBook.PeerMap(p2p.PeerID(inputs.Config.ValidatorID)), runtimeConfig.P2PPeers, runtimeConfig.P2PSeeds)
	var grpcTransport *transport.GRPCTransport
	grpcTransport, err = transport.NewGRPCTransport(transport.GRPCConfig{
		PeerID:          p2p.PeerID(inputs.Config.ValidatorID),
		ListenAddr:      runtimeConfig.P2PListenAddress,
		Peers:           peers,
		NetworkID:       networkID,
		ChainID:         inputs.Config.Chain.ChainID,
		GenesisHash:     genesisHash(inputs.Genesis),
		MaxMessageBytes: runtimeConfig.P2PMaxMessageBytes,
		MaxPeers:        runtimeConfig.P2PMaxPeers,
		AuthToken:       runtimeConfig.P2PAuthToken,
		PeerLearned: func(peerID p2p.PeerID, address string) {
			addrBook.Add(peerID, address, "handshake", false)
			_ = addrBook.Save()
		},
		PeerAttempted: func(peerID p2p.PeerID) {
			addrBook.MarkAttempt(peerID)
			_ = addrBook.Save()
		},
		PeerDialResult: func(peerID p2p.PeerID, success bool) {
			if success {
				addrBook.MarkSuccess(peerID)
			} else {
				addrBook.MarkFailure(peerID, inputs.Config.Chain.P2P.BanDuration)
				if addrBook.EvictBanned() > 0 && grpcTransport != nil {
					grpcTransport.RemovePeer(peerID)
				}
			}
			_ = addrBook.Save()
		},
		PeerGate: func(ctx context.Context, peerID p2p.PeerID) error {
			if addrBook.IsBanned(peerID) {
				return p2p.ErrPeerBanned
			}
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return grpcTransport, nil
}

func mergePeerMaps(peerMaps ...map[p2p.PeerID]string) map[p2p.PeerID]string {
	merged := make(map[p2p.PeerID]string)
	for _, peers := range peerMaps {
		for peerID, address := range peers {
			if peerID == "" || address == "" {
				continue
			}
			merged[peerID] = address
		}
	}
	return merged
}

func resolveAddrBookPath(home string, path string) string {
	if path != "" {
		return path
	}
	if home == "" {
		home = defaultHomeDir
	}
	return filepath.Join(home, "addrbook.json")
}

func genesisHash(genesis vexonode.Genesis) string {
	data, err := json.Marshal(genesis)
	if err != nil {
		return transport.GenesisHash([]byte(genesis.ChainID))
	}
	return transport.GenesisHash(data)
}

func withLocalValidatorPublicKey(genesis vexonode.Genesis, validatorID types.ValidatorID, publicKey types.PublicKey) vexonode.Genesis {
	for index := range genesis.Validators {
		if genesis.Validators[index].ID == validatorID && len(genesis.Validators[index].PublicKey) == 0 {
			genesis.Validators[index].PublicKey = append(types.PublicKey(nil), publicKey...)
		}
	}
	return genesis
}

func (flags peerFlags) String() string {
	if len(flags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(flags))
	for peerID, address := range flags {
		parts = append(parts, string(peerID)+"="+address)
	}
	return strings.Join(parts, ",")
}

func (flags peerFlags) Set(value string) error {
	peerID, address, err := parsePeerAssignment(value)
	if err != nil {
		return err
	}
	flags[peerID] = address
	return nil
}

func (flags *stringListFlags) String() string {
	if flags == nil || len(*flags) == 0 {
		return ""
	}
	return strings.Join(*flags, ",")
}

func (flags *stringListFlags) Set(value string) error {
	if value == "" {
		return errors.New("value is required")
	}
	*flags = append(*flags, value)
	return nil
}

func parsePeerAssignment(value string) (p2p.PeerID, string, error) {
	peerID, address, found := strings.Cut(value, "=")
	if !found || peerID == "" || address == "" {
		return "", "", fmt.Errorf("invalid peer %q: expected id=host:port", value)
	}
	if err := p2p.ValidatePeerAddress(address); err != nil {
		return "", "", fmt.Errorf("invalid peer %q: %w", value, err)
	}
	return p2p.PeerID(peerID), address, nil
}
