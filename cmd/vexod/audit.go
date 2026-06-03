package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
)

type auditDocument struct {
	OK     bool                 `json:"ok"`
	Strict bool                 `json:"strict"`
	Checks []auditCheckDocument `json:"checks"`
}

type auditCheckDocument struct {
	Name     string `json:"name"`
	Severity string `json:"severity"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
}

type auditPackDocument struct {
	SchemaVersion string   `json:"schema_version"`
	Scope         []string `json:"scope"`
	Commands      []string `json:"commands"`
	Evidence      []string `json:"evidence"`
	ReviewerNotes []string `json:"reviewer_notes"`
}

func runConfigAudit(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config audit", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	keyPath := flags.String("key", "", "key file path")
	rpcEnabled := flags.Bool("rpc", true, "audit HTTP RPC server exposure")
	rpcAddress := flags.String("rpc-address", defaultRPCAddress, "HTTP RPC listen address")
	rpcAdminToken := flags.String("rpc-admin-token", "", "admin token required for protected RPC endpoints")
	rpcEnablePprof := flags.Bool("rpc-pprof", false, "audit net/http/pprof exposure")
	rpcMaxRequestBytes := flags.Int64("rpc-max-request-bytes", 0, "maximum HTTP RPC request body bytes")
	rpcRateLimitMaxRequests := flags.Int("rpc-rate-limit-max", 0, "maximum HTTP RPC requests per client per window")
	p2pEnabled := flags.Bool("p2p", true, "audit gRPC P2P transport exposure")
	p2pListenAddress := flags.String("p2p-listen", defaultP2PAddress, "gRPC P2P listen address")
	p2pAuthToken := flags.String("p2p-auth-token", "", "shared P2P handshake auth token")
	p2pMaxMessageBytes := flags.Uint64("p2p-max-message-bytes", 0, "maximum P2P message bytes")
	addrBookMaxFailures := flags.Int("addr-book-max-failures", 3, "failed dial attempts before addr book bans a peer")
	strict := flags.Bool("strict", false, "return an error when production checks fail")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	inputs, err := loadStartInputs(*home, *configPath, *genesisPath, *keyPath, true)
	if err != nil {
		return err
	}
	document := auditDeployment(inputs, startRuntimeConfig{
		RPCEnabled:              *rpcEnabled,
		RPCAddress:              *rpcAddress,
		RPCAdminToken:           *rpcAdminToken,
		RPCEnablePprof:          *rpcEnablePprof,
		RPCMaxRequestBytes:      *rpcMaxRequestBytes,
		RPCRateLimitMaxRequests: *rpcRateLimitMaxRequests,
		P2PEnabled:              *p2pEnabled,
		P2PListenAddress:        *p2pListenAddress,
		P2PAuthToken:            *p2pAuthToken,
		P2PMaxMessageBytes:      *p2pMaxMessageBytes,
		AddrBookMaxFailures:     *addrBookMaxFailures,
	}, *strict)
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(document); err != nil {
			return err
		}
	} else {
		writeAuditDocument(writer, document)
	}
	if *strict && !document.OK {
		return fmt.Errorf("production audit failed with %d failed checks", failedAuditChecks(document.Checks))
	}
	return nil
}

func runConfigAuditPack(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config audit-pack", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document := buildAuditPackDocument()
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	fmt.Fprintf(writer, "external security audit pack\n")
	fmt.Fprintf(writer, "scope:\n")
	for _, item := range document.Scope {
		fmt.Fprintf(writer, "- %s\n", item)
	}
	fmt.Fprintf(writer, "commands:\n")
	for _, command := range document.Commands {
		fmt.Fprintf(writer, "- %s\n", command)
	}
	fmt.Fprintf(writer, "evidence:\n")
	for _, item := range document.Evidence {
		fmt.Fprintf(writer, "- %s\n", item)
	}
	return nil
}

func buildAuditPackDocument() auditPackDocument {
	return auditPackDocument{
		SchemaVersion: "v1",
		Scope: []string{
			"consensus safety, fork choice, timeout certificates, accountable evidence",
			"p2p handshake, peer scoring, reconnect/backoff, rate-limit and ban behavior",
			"RPC admin authorization, strict JSON decoding, request limits and pprof exposure",
			"key management, local encrypted keys, remote signer/KMS documents and signing flow",
			"state sync snapshot export/verify/restore and LevelDB recovery/pruning",
			"fuzz targets for tx envelopes, wire messages, DA, fair ordering, mempool and RPC decoders",
		},
		Commands: []string{
			"make check",
			"make fuzz-smoke",
			"go run ./cmd/vexod config audit --home .vexo --strict --json",
			"go run ./cmd/vexod consensus adversarial --json",
			"go run ./cmd/vexod config deployment-template --json",
			"go run ./cmd/vexod ops thresholds --json",
			"go run ./cmd/vexod upgrade plan --json --name audit-upgrade --height 100",
			"go run ./cmd/vexod network longrun-plan --validators 4 --duration 168h --regions 3 --hosts 4",
			"go run ./cmd/vexod network chaos-plan --validators 4 --duration 24h",
			"go run ./cmd/vexod network load --validators 4 --duration 1h --rate 50 --dry-run",
			"go run ./cmd/vexod keys verify-remote --home .vexo --challenge audit-challenge --height 1 --type consensus_vote --domain vexo.consensus.vote.v1",
			"make release VERSION=<version>",
			"make sign-release VERSION=<version>",
		},
		Evidence: []string{
			"test output from make check and make fuzz-smoke",
			"JSON output from config audit and consensus adversarial simulation",
			"threat model, known limitations, assumptions, and safety argument from docs/security/audit-readiness.md",
			"network logs, pids, health/status snapshots, metrics, and pprof captures",
			"snapshot checksums and restore verification output",
			"remote signer policy-bound challenge signature verification output",
			"multi-machine long-run plan with region and host assignment",
			"release checksums, signed checksums, SBOM, release manifest, and Docker image metadata",
		},
		ReviewerNotes: []string{
			"BLS adapter is intentionally unavailable until a production implementation is linked.",
			"External auditors should treat deterministic crypto as non-production.",
			"Long-running tests should be executed on independent hosts for network partition coverage.",
		},
	}
}

func auditDeployment(inputs startInputs, runtimeConfig startRuntimeConfig, strict bool) auditDocument {
	runtimeConfig = applyNetworkRuntimeDefaults(inputs, runtimeConfig)
	document := auditDocument{OK: true, Strict: strict}
	keyDocument, keyErr := vexocrypto.LoadKeyDocument(inputs.Plan.KeyPath)
	document.addCheck("config_valid", "error", inputs.Config.Chain.Validate() == nil, "chain config must pass validation")
	document.addCheck("production_config", strictSeverity(strict), inputs.Config.Chain.ValidateProduction() == nil, "production config must use non-dev crypto, signed/nonced txs, fees, gas floor, and priority mempool")
	document.addCheck("genesis_valid", "error", inputs.Genesis.Validate(inputs.Config.Chain.ChainID) == nil, "genesis must match chain id and validator set")
	document.addCheck("key_loadable", "error", keyErr == nil, "validator key document must be loadable")

	document.addCheck("crypto_backend", strictSeverity(strict), inputs.Config.Chain.Crypto.Backend != config.CryptoBackendDeterministic, "use ed25519, bls, or remote signer in non-dev deployments")
	if keyErr == nil {
		document.addCheck("key_encrypted_or_remote", strictSeverity(strict), keyDocument.Type == vexocrypto.KeyTypeRemote || keyDocument.Encryption != nil, "avoid unencrypted local private keys in production")
	}
	if runtimeConfig.RPCEnabled {
		document.addCheck("rpc_admin_token", strictSeverity(strict), runtimeConfig.RPCAdminToken != "", "protect admin RPC endpoints with an admin token")
		document.addCheck("rpc_private_or_admin", strictSeverity(strict), isPrivateListenAddress(runtimeConfig.RPCAddress) || runtimeConfig.RPCAdminToken != "", "public RPC listeners need admin-token protection")
		document.addCheck("rpc_pprof_loopback", strictSeverity(strict), !runtimeConfig.RPCEnablePprof || isLoopbackListenAddress(runtimeConfig.RPCAddress), "pprof should only be exposed on loopback interfaces")
		document.addCheck("rpc_request_limit", "warning", runtimeConfig.RPCMaxRequestBytes > 0, "set --rpc-max-request-bytes to bound request memory")
		document.addCheck("rpc_rate_limit", "warning", runtimeConfig.RPCRateLimitMaxRequests > 0, "set --rpc-rate-limit-max to reduce RPC floods")
	}
	if runtimeConfig.P2PEnabled {
		document.addCheck("p2p_auth_token", strictSeverity(strict), runtimeConfig.P2PAuthToken != "", "set a P2P auth token to harden handshakes")
		document.addCheck("p2p_message_limit", "warning", runtimeConfig.P2PMaxMessageBytes > 0, "set --p2p-max-message-bytes to bound peer payloads")
		document.addCheck("p2p_peer_discovery", "warning", len(runtimeConfig.P2PPeers)+len(runtimeConfig.P2PSeeds) > 0, "configure peers or seeds for non-local networks")
		document.addCheck("addr_book_failure_policy", "warning", runtimeConfig.AddrBookMaxFailures > 0, "keep addr book failure banning enabled to evict repeatedly failing peers")
	}
	document.addCheck("mempool_seen_ttl", "warning", inputs.Config.Chain.Mempool.SeenTTL > 0, "set mempool seen TTL to suppress replay gossip")
	document.addCheck("mempool_min_fee", "warning", inputs.Config.Chain.Mempool.MinFee > 0, "set minimum fee for public networks")
	document.addCheck("mempool_priority", "warning", inputs.Config.Chain.Mempool.EnablePriority, "enable priority ordering when fee markets are active")
	document.addCheck("execution_min_fee", "warning", inputs.Config.Chain.Execution.MinFee > 0, "set ante minimum fee for transaction execution")
	document.addCheck("execution_gas_limit", "warning", inputs.Config.Chain.Execution.MaxGas > 0, "set ante gas bounds for transaction execution")
	document.addCheck("execution_nonce_required", "warning", inputs.Config.Chain.Execution.RequireNonce, "require signer nonces to prevent replay")
	document.addCheck("execution_signed_required", "warning", inputs.Config.Chain.Execution.RequireSigned, "require signed transaction envelopes on public networks")
	return document
}

func (document *auditDocument) addCheck(name string, severity string, ok bool, message string) {
	check := auditCheckDocument{Name: name, Severity: severity, OK: ok, Message: message}
	if !ok && severity == "error" {
		document.OK = false
	}
	document.Checks = append(document.Checks, check)
}

func writeAuditDocument(writer io.Writer, document auditDocument) {
	status := "ok"
	if !document.OK {
		status = "failed"
	}
	fmt.Fprintf(writer, "audit %s\n", status)
	for _, check := range document.Checks {
		checkStatus := "ok"
		if !check.OK {
			checkStatus = "failed"
		}
		fmt.Fprintf(writer, "check %s: %s [%s] %s\n", check.Name, checkStatus, check.Severity, check.Message)
	}
}

func failedAuditChecks(checks []auditCheckDocument) int {
	count := 0
	for _, check := range checks {
		if !check.OK && check.Severity == "error" {
			count++
		}
	}
	return count
}

func strictSeverity(strict bool) string {
	if strict {
		return "error"
	}
	return "warning"
}

func isPrivateListenAddress(address string) bool {
	host := address
	if splitHost, _, err := net.SplitHostPort(address); err == nil {
		host = splitHost
	}
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

func isLoopbackListenAddress(address string) bool {
	host := address
	if splitHost, _, err := net.SplitHostPort(address); err == nil {
		host = splitHost
	}
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
