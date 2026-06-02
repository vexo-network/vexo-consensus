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

func auditDeployment(inputs startInputs, runtimeConfig startRuntimeConfig, strict bool) auditDocument {
	runtimeConfig = applyLocalnetRuntimeDefaults(inputs, runtimeConfig)
	document := auditDocument{OK: true, Strict: strict}
	keyDocument, keyErr := vexocrypto.LoadKeyDocument(inputs.Plan.KeyPath)
	document.addCheck("config_valid", "error", inputs.Config.Chain.Validate() == nil, "chain config must pass validation")
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
	}
	document.addCheck("mempool_seen_ttl", "warning", inputs.Config.Chain.Mempool.SeenTTL > 0, "set mempool seen TTL to suppress replay gossip")
	document.addCheck("mempool_min_fee", "warning", inputs.Config.Chain.Mempool.MinFee > 0, "set minimum fee for public networks")
	document.addCheck("mempool_priority", "warning", inputs.Config.Chain.Mempool.EnablePriority, "enable priority ordering when fee markets are active")
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
