package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
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
	ChainID     string `json:"chain_id"`
	ValidatorID string `json:"validator_id,omitempty"`
	DataDir     string `json:"data_dir"`
	ConfigPath  string `json:"config_path"`
	GenesisPath string `json:"genesis_path"`
	KeyPath     string `json:"key_path"`
	ValidatorN  int    `json:"validator_count"`
	KeyType     string `json:"key_type,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
	DryRun      bool   `json:"dry_run"`
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
	dryRun := flags.Bool("dry-run", false, "validate startup inputs without running a node")
	run := flags.Bool("run", false, "start the node and block until context cancellation")
	rpcEnabled := flags.Bool("rpc", true, "run HTTP RPC server with node")
	rpcAddress := flags.String("rpc-address", defaultRPCAddress, "HTTP RPC listen address")
	rpcAdminToken := flags.String("rpc-admin-token", "", "admin token required for protected RPC endpoints")
	rpcEnablePprof := flags.Bool("rpc-pprof", false, "enable net/http/pprof endpoints under /debug/pprof")
	rpcRequestTimeout := flags.Duration("rpc-request-timeout", 0, "HTTP RPC request timeout")
	rpcMaxRequestBytes := flags.Int64("rpc-max-request-bytes", 0, "maximum HTTP RPC request body bytes")
	rpcRateLimitWindow := flags.Duration("rpc-rate-limit-window", 0, "HTTP RPC rate limit window")
	rpcRateLimitMaxRequests := flags.Int("rpc-rate-limit-max", 0, "maximum HTTP RPC requests per client per window")
	consensusLoopEnabled := flags.Bool("consensus-loop", true, "start local consensus loop with node")
	consensusInterval := flags.Duration("consensus-interval", 0, "local consensus loop tick interval")
	roundTimeout := flags.Duration("consensus-round-timeout", 0, "local consensus loop timeout round duration")
	maxBlockBytes := flags.Int64("consensus-max-block-bytes", 0, "maximum bytes to include when building a block")
	p2pEnabled := flags.Bool("p2p", true, "run gRPC P2P transport with node")
	p2pListenAddress := flags.String("p2p-listen", defaultP2PAddress, "gRPC P2P listen address")
	p2pNetworkID := flags.String("p2p-network", "", "P2P network id; defaults to chain id")
	p2pMaxMessageBytes := flags.Uint64("p2p-max-message-bytes", 0, "maximum P2P message bytes")
	p2pMaxPeers := flags.Int("p2p-max-peers", 0, "maximum configured P2P peers")
	p2pAuthToken := flags.String("p2p-auth-token", "", "shared P2P handshake auth token")
	addrBookPath := flags.String("addr-book", "", "P2P address book path; defaults to <home>/addrbook.json")
	addrBookMaxFailures := flags.Int("addr-book-max-failures", 3, "failed dial attempts before addr book bans a peer")
	strictProduction := flags.Bool("strict-production", false, "fail startup when production-readiness checks fail")
	production := flags.Bool("production", false, "enable node-level production safety gates before startup")
	logFormat := flags.String("log-format", "text", "operational log format: text or json")
	logLevel := flags.String("log-level", "info", "operational log level")
	peers := peerFlags{}
	flags.Var(peers, "peer", "persistent peer in id=host:port form; may be repeated")
	seeds := peerFlags{}
	flags.Var(seeds, "seed", "seed peer used for bootstrap discovery in id=host:port form; may be repeated")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	inputs, err := loadStartInputs(*home, *configPath, *genesisPath, *keyPath, *dryRun)
	if err != nil {
		return err
	}
	if *production {
		inputs.Config.Production = true
	}
	runtimeConfig := startRuntimeConfig{
		RPCEnabled:              *rpcEnabled,
		RPCAddress:              *rpcAddress,
		RPCAdminToken:           *rpcAdminToken,
		RPCEnablePprof:          *rpcEnablePprof,
		RPCRequestTimeout:       *rpcRequestTimeout,
		RPCMaxRequestBytes:      *rpcMaxRequestBytes,
		RPCRateLimitWindow:      *rpcRateLimitWindow,
		RPCRateLimitMaxRequests: *rpcRateLimitMaxRequests,
		LogFormat:               *logFormat,
		LogLevel:                *logLevel,
		ConsensusLoopEnabled:    *consensusLoopEnabled,
		ConsensusLoop: vexonode.ConsensusLoopConfig{
			Interval:      *consensusInterval,
			RoundTimeout:  *roundTimeout,
			MaxBlockBytes: *maxBlockBytes,
		},
		P2PEnabled:          *p2pEnabled,
		P2PListenAddress:    *p2pListenAddress,
		P2PNetworkID:        *p2pNetworkID,
		P2PMaxMessageBytes:  *p2pMaxMessageBytes,
		P2PMaxPeers:         *p2pMaxPeers,
		P2PAuthToken:        *p2pAuthToken,
		AddrBookPath:        resolveAddrBookPath(*home, *addrBookPath),
		AddrBookMaxFailures: *addrBookMaxFailures,
		P2PPeers:            peers,
		P2PSeeds:            seeds,
	}
	if *strictProduction {
		audit := auditDeployment(inputs, runtimeConfig, true)
		if !audit.OK {
			return fmt.Errorf("strict production checks failed with %d failed checks; run `vexod config audit --strict --json` for details", failedAuditChecks(audit.Checks))
		}
	}
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
		fmt.Fprintf(writer, "validator signer loaded\n")
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

func runStartNode(ctx context.Context, writer io.Writer, inputs startInputs, runtimeConfig startRuntimeConfig) error {
	runtimeConfig = applyNetworkRuntimeDefaults(inputs, runtimeConfig)
	if runtimeConfig.LogFormat != "" && runtimeConfig.LogFormat != "text" && runtimeConfig.LogFormat != "json" {
		return fmt.Errorf("unsupported log format %q", runtimeConfig.LogFormat)
	}
	if runtimeConfig.LogLevel == "" {
		runtimeConfig.LogLevel = "info"
	}
	node, p2pWire, err := buildRuntimeNode(inputs, runtimeConfig)
	if err != nil {
		return err
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
		writeOperationalLog(writer, runtimeConfig.LogFormat, runtimeConfig.LogLevel, "consensus_loop_running", map[string]any{"chain_id": inputs.Plan.ChainID})
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
		writeOperationalLog(writer, runtimeConfig.LogFormat, runtimeConfig.LogLevel, "rpc_listening", map[string]any{"rpc_address": address, "pprof": runtimeConfig.RPCEnablePprof})
	}
	if p2pWire != nil {
		writeOperationalLog(writer, runtimeConfig.LogFormat, runtimeConfig.LogLevel, "p2p_listening", map[string]any{"p2p_address": p2pWire.Address(), "p2p_peers": len(runtimeConfig.P2PPeers), "p2p_seeds": len(runtimeConfig.P2PSeeds)})
	}
	writeOperationalLog(writer, runtimeConfig.LogFormat, runtimeConfig.LogLevel, "node_running", map[string]any{"chain_id": inputs.Plan.ChainID, "validator_id": inputs.Plan.ValidatorID, "data_dir": inputs.Plan.DataDir, "pid": os.Getpid(), "version": version, "go_version": runtime.Version()})
	select {
	case <-ctx.Done():
	case err := <-serverErr:
		_ = node.Stop(context.Background())
		return err
	}
	writeOperationalLog(writer, runtimeConfig.LogFormat, runtimeConfig.LogLevel, "shutdown_requested", map[string]any{"chain_id": inputs.Plan.ChainID})
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
	writeOperationalLog(writer, runtimeConfig.LogFormat, runtimeConfig.LogLevel, "node_stopped", map[string]any{"chain_id": inputs.Plan.ChainID})
	return nil
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
	inputs, err := loadStartInputs(home, configPath, genesisPath, keyPath, dryRun)
	if err != nil {
		return startPlanDocument{}, err
	}
	return inputs.Plan, nil
}

func loadStartInputs(home string, configPath string, genesisPath string, keyPath string, dryRun bool) (startInputs, error) {
	resolvedConfigPath := resolveConfigPath(home, configPath)
	resolvedGenesisPath := resolveGenesisPath(home, genesisPath)
	resolvedKeyPath := resolveKeyPath(home, keyPath)
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
	keyDocument, err := vexocrypto.LoadKeyDocument(resolvedKeyPath)
	if err != nil {
		return startInputs{}, err
	}
	signer, err := signerFromKeyDocument(keyDocument)
	if err != nil {
		return startInputs{}, err
	}
	genesis = withLocalValidatorPublicKey(genesis, cfg.ValidatorID, signer.PublicKey())
	plan := startPlanDocument{
		ChainID:     cfg.Chain.ChainID,
		ValidatorID: string(cfg.ValidatorID),
		DataDir:     cfg.DataDir,
		ConfigPath:  resolvedConfigPath,
		GenesisPath: resolvedGenesisPath,
		KeyPath:     resolvedKeyPath,
		ValidatorN:  len(genesis.Validators),
		KeyType:     keyDocument.Type,
		PublicKey:   keyDocument.PublicKey,
		DryRun:      dryRun,
	}
	return startInputs{
		Config:  cfg,
		Genesis: genesis,
		Signer:  signer,
		Plan:    plan,
	}, nil
}

func signerFromKeyDocument(document vexocrypto.KeyDocument) (vexocrypto.Signer, error) {
	return document.SignerWithPassphrase(resolvePassphrase(""))
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
		if address := validatorMetadata(inputs.Genesis, inputs.Config.ValidatorID, "rpc_address"); address != "" {
			runtimeConfig.RPCAddress = address
		}
	}
	if runtimeConfig.P2PListenAddress == "" || runtimeConfig.P2PListenAddress == defaultP2PAddress {
		if address := validatorMetadata(inputs.Genesis, inputs.Config.ValidatorID, "p2p_address"); address != "" {
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
	peerID, address, found := strings.Cut(value, "=")
	if !found || peerID == "" || address == "" {
		return fmt.Errorf("invalid peer %q: expected id=host:port", value)
	}
	flags[p2p.PeerID(peerID)] = address
	return nil
}
