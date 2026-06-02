package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
)

type mainnetTemplateDocument struct {
	SchemaVersion string                    `json:"schema_version"`
	Chain         mainnetChainTemplate      `json:"chain"`
	Runtime       mainnetRuntimeTemplate    `json:"runtime"`
	Validation    mainnetValidationTemplate `json:"validation"`
	Notes         []string                  `json:"notes"`
}

type mainnetChainTemplate struct {
	Execution mainnetExecutionTemplate `json:"execution"`
	Mempool   mainnetMempoolTemplate   `json:"mempool"`
	Validator mainnetValidatorTemplate `json:"validator"`
	Committee mainnetCommitteeTemplate `json:"committee"`
}

type mainnetExecutionTemplate struct {
	RequireSigned bool   `json:"require_signed"`
	RequireNonce  bool   `json:"require_nonce"`
	MinFee        uint64 `json:"min_fee"`
	MinGas        uint64 `json:"min_gas"`
	MaxGas        uint64 `json:"max_gas"`
}

type mainnetMempoolTemplate struct {
	MinFee         uint64 `json:"min_fee"`
	EnablePriority bool   `json:"enable_priority"`
	MaxTxs         int    `json:"max_txs"`
	SeenTTL        string `json:"seen_ttl"`
}

type mainnetValidatorTemplate struct {
	Permissionless bool   `json:"permissionless"`
	MinStake       uint64 `json:"min_stake"`
	RemoteSigner   bool   `json:"remote_signer"`
}

type mainnetCommitteeTemplate struct {
	Size        uint64 `json:"size"`
	EpochLength uint64 `json:"epoch_length"`
	Regions     int    `json:"regions"`
}

type mainnetRuntimeTemplate struct {
	RPCMaxRequestBytes      int64  `json:"rpc_max_request_bytes"`
	RPCRateLimitMaxRequests int    `json:"rpc_rate_limit_max_requests"`
	P2PMaxMessageBytes      uint64 `json:"p2p_max_message_bytes"`
	P2PAuthTokenRequired    bool   `json:"p2p_auth_token_required"`
	RPCAdminTokenRequired   bool   `json:"rpc_admin_token_required"`
	PprofLoopbackOnly       bool   `json:"pprof_loopback_only"`
}

type mainnetValidationTemplate struct {
	Preflight []string `json:"preflight"`
	LongRun   []string `json:"long_run"`
}

func runConfigMainnetTemplate(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config mainnet-template", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document := buildMainnetTemplateDocument()
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	fmt.Fprintf(writer, "mainnet parameter template\n")
	fmt.Fprintf(writer, "execution: require_signed=%t require_nonce=%t min_fee=%d min_gas=%d max_gas=%d\n", document.Chain.Execution.RequireSigned, document.Chain.Execution.RequireNonce, document.Chain.Execution.MinFee, document.Chain.Execution.MinGas, document.Chain.Execution.MaxGas)
	fmt.Fprintf(writer, "mempool: min_fee=%d priority=%t max_txs=%d seen_ttl=%s\n", document.Chain.Mempool.MinFee, document.Chain.Mempool.EnablePriority, document.Chain.Mempool.MaxTxs, document.Chain.Mempool.SeenTTL)
	fmt.Fprintf(writer, "validator: permissionless=%t min_stake=%d remote_signer=%t\n", document.Chain.Validator.Permissionless, document.Chain.Validator.MinStake, document.Chain.Validator.RemoteSigner)
	fmt.Fprintf(writer, "committee: size=%d epoch_length=%d regions=%d\n", document.Chain.Committee.Size, document.Chain.Committee.EpochLength, document.Chain.Committee.Regions)
	fmt.Fprintf(writer, "runtime: rpc_max_request_bytes=%d rpc_rate_limit_max=%d p2p_max_message_bytes=%d\n", document.Runtime.RPCMaxRequestBytes, document.Runtime.RPCRateLimitMaxRequests, document.Runtime.P2PMaxMessageBytes)
	fmt.Fprintf(writer, "validation:\n")
	for _, command := range document.Validation.Preflight {
		fmt.Fprintf(writer, "- %s\n", command)
	}
	for _, command := range document.Validation.LongRun {
		fmt.Fprintf(writer, "- %s\n", command)
	}
	return nil
}

func buildMainnetTemplateDocument() mainnetTemplateDocument {
	return mainnetTemplateDocument{
		SchemaVersion: "v1",
		Chain: mainnetChainTemplate{
			Execution: mainnetExecutionTemplate{
				RequireSigned: true,
				RequireNonce:  true,
				MinFee:        1,
				MinGas:        1,
				MaxGas:        10_000_000,
			},
			Mempool: mainnetMempoolTemplate{
				MinFee:         1,
				EnablePriority: true,
				MaxTxs:         50_000,
				SeenTTL:        "10m0s",
			},
			Validator: mainnetValidatorTemplate{
				Permissionless: true,
				MinStake:       1_000,
				RemoteSigner:   true,
			},
			Committee: mainnetCommitteeTemplate{
				Size:        128,
				EpochLength: 1_000,
				Regions:     3,
			},
		},
		Runtime: mainnetRuntimeTemplate{
			RPCMaxRequestBytes:      1_048_576,
			RPCRateLimitMaxRequests: 100,
			P2PMaxMessageBytes:      1_048_576,
			P2PAuthTokenRequired:    true,
			RPCAdminTokenRequired:   true,
			PprofLoopbackOnly:       true,
		},
		Validation: mainnetValidationTemplate{
			Preflight: []string{
				"make check",
				"make fuzz-smoke",
				"vexod config audit --home .vexo --strict --json",
				"vexod keys verify-remote --home .vexo --challenge mainnet-kms",
			},
			LongRun: []string{
				"vexod localnet longrun-plan --validators 4 --duration 168h --regions 3 --hosts 4",
				"vexod localnet load --validators 4 --duration 24h --rate 50",
				"vexod localnet chaos-plan --validators 4 --duration 24h --regions 3",
				"vexod consensus adversarial --json",
			},
		},
		Notes: []string{
			"Tune fee, stake, and rate-limit values against real traffic before launch.",
			"Use remote signer/KMS or encrypted local keys; never ship unencrypted validator keys.",
			"BLS can be selected only after a production BLS adapter is linked and audited.",
		},
	}
}
