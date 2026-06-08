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
	"github.com/vexo-network/vexo-consensus/validator"
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
	inputs, err := loadStartInputs(*home, *configPath, *genesisPath, *keyPath, nil, true)
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
			"go run ./cmd/vexod config tune --validators <n> --tps <target> --regions <r> --latency <duration> --json",
			"go run ./cmd/vexod ops thresholds --json",
			"go run ./cmd/vexod upgrade plan --json --name audit-upgrade --height 100",
			"go run ./cmd/vexod network longrun-plan --validators 4 --duration 168h --regions 3 --hosts 4",
			"go run ./cmd/vexod network longrun --validators 4 --duration 1h --rate 50 --output dist/longrun-evidence.json",
			"go run ./cmd/vexod network chaos-plan --validators 4 --duration 24h",
			"go run ./cmd/vexod network load --validators 4 --duration 1h --rate 50 --dry-run",
			"go run ./cmd/vexod keys verify-remote --home .vexo --challenge audit-challenge --height 1 --type consensus_vote --domain vexo.consensus.vote.v1",
			"make release VERSION=<version>",
			"go run ./cmd/vexod release pack --dist dist --version <version> --longrun-evidence dist/longrun-evidence.json --adversarial-evidence dist/adversarial-evidence.json --fuzz-evidence dist/fuzz-evidence.txt",
			"make sign-release VERSION=<version>",
		},
		Evidence: []string{
			"test output from make check and make fuzz-smoke",
			"JSON output from config audit and consensus adversarial simulation",
			"JSON output from config tune with target validator count, TPS, regions, and measured latency assumptions",
			"threat model, known limitations, assumptions, and safety argument from docs/security/audit-readiness.md",
			"network logs, pids, health/status snapshots, metrics, and pprof captures",
			"snapshot checksums and restore verification output",
			"remote signer policy-bound challenge signature verification output",
			"multi-machine long-run plan with region and host assignment plus generated longrun evidence JSON",
			"P2P scale evidence for discovery, reconnect, NAT, seed, addrbook, ban eviction, and backpressure",
			"state-sync/light-client evidence for validator-set hash binding, snapshot restore, replay, and finality proofs",
			"validator economics evidence for custody, rewards, commission, jail, tombstone, unbonding, and slashing accounting",
			"governance upgrade evidence for proposal approval, migration execution, halt, rollback, and failed-upgrade recovery",
			"MEV/fee-market evidence for base fee, fair ordering, censorship-resistance, spam cost, and mempool WAL replay",
			"SDK conformance evidence for app modules, custom crypto, custom storage, custom transport, RPC versioning, and upgrade hooks",
			"adversarial simulation and fuzz/property evidence files",
			"release checksums, signed checksums, SBOM, release manifest, and Docker image metadata",
		},
		ReviewerNotes: []string{
			"BLS has a reference CIRCL-backed integration and custom adapter hooks; value-bearing networks must link a separately audited adapter and verify dependency, subgroup, rogue-key, and proof-of-possession evidence.",
			"External auditors should treat deterministic crypto as unsafe for public value-bearing networks.",
			"Long-running tests should be executed on independent hosts for network partition coverage.",
		},
	}
}

func auditDeployment(inputs startInputs, runtimeConfig startRuntimeConfig, strict bool) auditDocument {
	runtimeConfig = applyNetworkRuntimeDefaults(inputs, runtimeConfig)
	document := auditDocument{OK: true, Strict: strict}
	keyDocument, keyErr := vexocrypto.LoadKeyDocument(inputs.Plan.KeyPath)
	document.addCheck("config_valid", "error", inputs.Config.Chain.Validate() == nil, "chain config must pass validation")
	document.addCheck("network_safety_config", strictSeverity(strict), inputs.Config.Chain.ValidateNetworkSafety() == nil, "network safety config must use non-deterministic crypto, signed/nonced txs, min fee, base fee, gas floor, priority mempool, and durable mempool WAL")
	document.addCheck("genesis_valid", "error", inputs.Genesis.Validate(inputs.Config.Chain.ChainID) == nil, "genesis must match chain id and validator set")
	document.addCheck("key_loadable", "error", keyErr == nil, "validator key document must be loadable")

	document.addCheck("crypto_backend", strictSeverity(strict), inputs.Config.Chain.Crypto.Backend != config.CryptoBackendDeterministic, "use ed25519, bls, or remote signer for public value-bearing networks")
	if inputs.Config.Chain.Crypto.Backend == config.CryptoBackendBLS {
		document.addCheck("bls_production_adapter", strictSeverity(strict), inputs.Config.Chain.Crypto.ProductionAdapter, "BLS requires an audited production adapter with dependency audit metadata")
		document.addCheck("bls_genesis_pop", strictSeverity(strict), genesisHasBLSPoP(inputs.Genesis.Validators), "every genesis validator must include bls_pop proof-of-possession metadata")
	}
	if keyErr == nil {
		document.addCheck("key_encrypted_or_remote", strictSeverity(strict), keyDocument.Type == vexocrypto.KeyTypeRemote || keyDocument.Encryption != nil, "avoid unencrypted local private keys in production")
		if keyDocument.Type == vexocrypto.KeyTypeRemote {
			document.addCheck("remote_signer_auth", strictSeverity(strict), keyDocument.Metadata.AuthTokenEnv != "", "remote signer key metadata should reference an auth token environment variable")
			document.addCheck("remote_signer_policy", strictSeverity(strict), keyDocument.Metadata.RequirePolicy, "remote signer should require height/round/type/domain sign policy")
			document.addCheck("remote_signer_guard", strictSeverity(strict), keyDocument.Metadata.GuardPath != "", "remote signer should use a durable double-sign guard path")
		}
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
		document.addCheck("p2p_tls_identity", strictSeverity(strict), runtimeConfig.P2PTLSCertPath != "" && runtimeConfig.P2PTLSKeyPath != "", "configure P2P TLS cert/key identity for encrypted peer transport")
		document.addCheck("p2p_mtls_ca", strictSeverity(strict), runtimeConfig.P2PTLSCAPath != "", "configure P2P TLS CA trust roots so peers verify client certificates")
		document.addCheck("p2p_message_limit", "warning", runtimeConfig.P2PMaxMessageBytes > 0, "set --p2p-max-message-bytes to bound peer payloads")
		document.addCheck("p2p_peer_discovery", "warning", len(runtimeConfig.P2PPeers)+len(runtimeConfig.P2PSeeds) > 0, "configure peers or seeds for non-local networks")
		document.addCheck("addr_book_failure_policy", "warning", runtimeConfig.AddrBookMaxFailures > 0, "keep addr book failure banning enabled to evict repeatedly failing peers")
	}
	document.addCheck("p2p_score_cap", strictSeverity(strict), inputs.Config.Chain.P2P.MaxScore > 0, "cap peer scores to prevent unbounded score growth")
	document.addCheck("p2p_backpressure", strictSeverity(strict), inputs.Config.Chain.P2P.MaxTotalMessagesPerWindow > inputs.Config.Chain.P2P.MaxMessagesPerWindow, "set total-window backpressure above per-peer rate limits")
	document.addCheck("p2p_ban_recovery", strictSeverity(strict), inputs.Config.Chain.P2P.BanDuration > 0 && inputs.Config.Chain.P2P.ScoreRecovery > 0, "set ban duration and score recovery for stable peer rehabilitation")
	document.addCheck("mempool_seen_ttl", strictSeverity(strict), inputs.Config.Chain.Mempool.SeenTTL > 0, "set mempool seen TTL to suppress replay gossip")
	document.addCheck("mempool_min_fee", "warning", inputs.Config.Chain.Mempool.MinFee > 0, "set minimum fee for public networks")
	document.addCheck("mempool_priority", "warning", inputs.Config.Chain.Mempool.EnablePriority, "enable priority ordering when fee markets are active")
	document.addCheck("mempool_wal", "warning", inputs.Config.Chain.Mempool.WALPath != "", "set mempool WAL path so pending transactions survive restart")
	document.addCheck("execution_min_fee", "warning", inputs.Config.Chain.Execution.MinFee > 0, "set ante minimum fee for transaction execution")
	document.addCheck("execution_base_fee", "warning", inputs.Config.Chain.Execution.BaseFee > 0, "set base fee per gas for transaction execution")
	document.addCheck("execution_gas_limit", "warning", inputs.Config.Chain.Execution.MaxGas > 0, "set ante gas bounds for transaction execution")
	document.addCheck("execution_nonce_required", "warning", inputs.Config.Chain.Execution.RequireNonce, "require signer nonces to prevent replay")
	document.addCheck("execution_signed_required", "warning", inputs.Config.Chain.Execution.RequireSigned, "require signed transaction envelopes on public networks")
	document.addCheck("bank_mint_authority", strictSeverity(strict), inputs.Config.Chain.Bank.MintAuthority != "", "set bank mint authority to prevent permissionless supply creation")
	return document
}

func genesisHasBLSPoP(validators []validator.Validator) bool {
	if len(validators) == 0 {
		return false
	}
	for _, validatorInfo := range validators {
		if validatorInfo.Metadata["bls_pop"] == "" {
			return false
		}
	}
	return true
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
