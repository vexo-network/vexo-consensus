package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
)

type deploymentTemplateDocument struct {
	SchemaVersion string                       `json:"schema_version"`
	Chain         deploymentChainTemplate      `json:"chain"`
	Runtime       deploymentRuntimeTemplate    `json:"runtime"`
	Validation    deploymentValidationTemplate `json:"validation"`
	Notes         []string                     `json:"notes"`
}

type deploymentChainTemplate struct {
	Execution deploymentExecutionTemplate `json:"execution"`
	Mempool   deploymentMempoolTemplate   `json:"mempool"`
	Validator deploymentValidatorTemplate `json:"validator"`
	Committee deploymentCommitteeTemplate `json:"committee"`
}

type deploymentExecutionTemplate struct {
	RequireSigned            bool   `json:"require_signed"`
	RequireNonce             bool   `json:"require_nonce"`
	MinFee                   uint64 `json:"min_fee"`
	BaseFee                  uint64 `json:"base_fee"`
	BlobBaseFee              uint64 `json:"blob_base_fee"`
	DynamicBaseFee           bool   `json:"dynamic_base_fee"`
	DynamicBlobBaseFee       bool   `json:"dynamic_blob_base_fee"`
	TargetGas                uint64 `json:"target_gas"`
	TargetBlobGas            uint64 `json:"target_blob_gas"`
	MaxBlobGas               uint64 `json:"max_blob_gas"`
	BaseFeeChangeDenominator uint64 `json:"base_fee_change_denominator"`
	BlobFeeChangeDenominator uint64 `json:"blob_fee_change_denominator"`
	MinBaseFee               uint64 `json:"min_base_fee"`
	MaxBaseFee               uint64 `json:"max_base_fee"`
	MinBlobBaseFee           uint64 `json:"min_blob_base_fee"`
	MaxBlobBaseFee           uint64 `json:"max_blob_base_fee"`
	MinGas                   uint64 `json:"min_gas"`
	MaxGas                   uint64 `json:"max_gas"`
	FeeDenom                 string `json:"fee_denom"`
	DisplayDenom             string `json:"display_denom"`
	DisplayExponent          uint8  `json:"display_exponent"`
	GasDenom                 string `json:"gas_denom"`
	StrictEVMStateRoot       bool   `json:"strict_evm_state_root"`
}

type deploymentMempoolTemplate struct {
	MinFee             uint64 `json:"min_fee"`
	EnablePriority     bool   `json:"enable_priority"`
	EnableReplacement  bool   `json:"enable_replacement"`
	ReplacementBumpBPS uint64 `json:"replacement_bump_bps"`
	MaxTxs             int    `json:"max_txs"`
	SeenTTL            string `json:"seen_ttl"`
	WALPath            string `json:"wal_path"`
}

type deploymentValidatorTemplate struct {
	Permissionless bool   `json:"permissionless"`
	MinStake       uint64 `json:"min_stake"`
	RemoteSigner   bool   `json:"remote_signer"`
}

type deploymentCommitteeTemplate struct {
	Size        uint64 `json:"size"`
	EpochLength uint64 `json:"epoch_length"`
	Regions     int    `json:"regions"`
}

type deploymentRuntimeTemplate struct {
	RPCMaxRequestBytes      int64  `json:"rpc_max_request_bytes"`
	RPCRateLimitMaxRequests int    `json:"rpc_rate_limit_max_requests"`
	P2PMaxMessageBytes      uint64 `json:"p2p_max_message_bytes"`
	P2PAuthTokenRequired    bool   `json:"p2p_auth_token_required"`
	RPCAdminTokenRequired   bool   `json:"rpc_admin_token_required"`
	PprofLoopbackOnly       bool   `json:"pprof_loopback_only"`
}

type deploymentValidationTemplate struct {
	Preflight []string `json:"preflight"`
	LongRun   []string `json:"long_run"`
}

func runConfigDeploymentTemplate(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config deployment-template", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document := buildDeploymentTemplateDocument()
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	fmt.Fprintf(writer, "deployment parameter template\n")
	fmt.Fprintf(writer, "execution: require_signed=%t require_nonce=%t min_fee=%d base_fee=%d blob_base_fee=%d dynamic_base_fee=%t dynamic_blob_base_fee=%t target_gas=%d target_blob_gas=%d min_gas=%d max_gas=%d fee_denom=%s gas_denom=%s strict_evm_state_root=%t\n", document.Chain.Execution.RequireSigned, document.Chain.Execution.RequireNonce, document.Chain.Execution.MinFee, document.Chain.Execution.BaseFee, document.Chain.Execution.BlobBaseFee, document.Chain.Execution.DynamicBaseFee, document.Chain.Execution.DynamicBlobBaseFee, document.Chain.Execution.TargetGas, document.Chain.Execution.TargetBlobGas, document.Chain.Execution.MinGas, document.Chain.Execution.MaxGas, document.Chain.Execution.FeeDenom, document.Chain.Execution.GasDenom, document.Chain.Execution.StrictEVMStateRoot)
	fmt.Fprintf(writer, "mempool: min_fee=%d priority=%t replacement=%t replacement_bump_bps=%d max_txs=%d seen_ttl=%s wal_path=%s\n", document.Chain.Mempool.MinFee, document.Chain.Mempool.EnablePriority, document.Chain.Mempool.EnableReplacement, document.Chain.Mempool.ReplacementBumpBPS, document.Chain.Mempool.MaxTxs, document.Chain.Mempool.SeenTTL, document.Chain.Mempool.WALPath)
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

func buildDeploymentTemplateDocument() deploymentTemplateDocument {
	return deploymentTemplateDocument{
		SchemaVersion: "v1",
		Chain: deploymentChainTemplate{
			Execution: deploymentExecutionTemplate{
				RequireSigned:            true,
				RequireNonce:             true,
				MinFee:                   1,
				BaseFee:                  1,
				BlobBaseFee:              1,
				DynamicBaseFee:           true,
				DynamicBlobBaseFee:       true,
				TargetGas:                5_000_000,
				TargetBlobGas:            393_216,
				MaxBlobGas:               786_432,
				BaseFeeChangeDenominator: 8,
				BlobFeeChangeDenominator: 6,
				MinBaseFee:               1,
				MinBlobBaseFee:           1,
				MinGas:                   1,
				MaxGas:                   10_000_000,
				FeeDenom:                 "avxo",
				DisplayDenom:             "vexo",
				DisplayExponent:          18,
				GasDenom:                 "gas",
				StrictEVMStateRoot:       false,
			},
			Mempool: deploymentMempoolTemplate{
				MinFee:             1,
				EnablePriority:     true,
				EnableReplacement:  true,
				ReplacementBumpBPS: 1000,
				MaxTxs:             50_000,
				SeenTTL:            "10m0s",
				WALPath:            "data/mempool.wal",
			},
			Validator: deploymentValidatorTemplate{
				Permissionless: true,
				MinStake:       1_000,
				RemoteSigner:   true,
			},
			Committee: deploymentCommitteeTemplate{
				Size:        128,
				EpochLength: 1_000,
				Regions:     3,
			},
		},
		Runtime: deploymentRuntimeTemplate{
			RPCMaxRequestBytes:      1_048_576,
			RPCRateLimitMaxRequests: 100,
			P2PMaxMessageBytes:      1_048_576,
			P2PAuthTokenRequired:    true,
			RPCAdminTokenRequired:   true,
			PprofLoopbackOnly:       true,
		},
		Validation: deploymentValidationTemplate{
			Preflight: []string{
				"make check",
				"make fuzz-smoke",
				"vexod config audit --home .vexo --strict --json",
				"vexod keys verify-remote --home .vexo --challenge deployment-kms",
			},
			LongRun: []string{
				"vexod network longrun-plan --validators 4 --duration 168h --regions 3 --hosts 4",
				"vexod network load --validators 4 --duration 24h --rate 50",
				"vexod network chaos-plan --validators 4 --duration 24h --regions 3",
				"vexod consensus adversarial --json",
			},
		},
		Notes: []string{
			"Tune fee, stake, and rate-limit values against real traffic before launch.",
			"Use remote signer/KMS or encrypted local keys; never ship unencrypted validator keys.",
			"BLS requires a separately audited adapter linked into the binary plus proof-of-possession metadata before launch; the built-in reference adapter is not a launch waiver.",
		},
	}
}
