package main

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"

	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/economics"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/node"
)

func maybeEncryptKeyDocument(document vexocrypto.KeyDocument, encrypt bool, passphrase string) (vexocrypto.KeyDocument, error) {
	if !encrypt {
		return document, nil
	}
	return document.Encrypted(passphrase)
}

func defaultConfigDocument(chainID string, dataDir string, validatorID string) configDocument {
	return configDocument{
		SchemaVersion:        configSchemaVersion,
		RequireNetworkSafety: true,
		DataDir:              dataDir,
		ValidatorID:          validatorID,
		ChainID:              chainID,
		ModuleConfigPath:     moduleConfigFileName,
		NetworkConfigPath:    networkConfigFileName,
		ConsensusConfigPath:  consensusConfigFileName,
		MempoolConfigPath:    mempoolConfigFileName,
		LogConfigPath:        logConfigFileName,
		RetainBlocks:         new(node.DefaultRetainBlocks),
	}
}

func defaultModuleConfigDocument(chainID string) moduleConfigDocument {
	return defaultModuleConfigDocumentWithWeb3(chainID, false)
}

func defaultModuleConfigDocumentWithWeb3(chainID string, web3DevAccounts bool) moduleConfigDocument {
	cfg := config.Default(chainID)
	applyDefaultNetworkSafetyModuleConfig(&cfg)
	if web3DevAccounts {
		cfg.Application.Modules = appendModuleIfMissing(cfg.Application.Modules, "evm")
		// Local Web3 networks must accept bytecode emitted by current Remix and
		// Solidity releases. Production networks keep the explicit London default
		// and should schedule fork changes through normal governance.
		cfg.Execution.EVMForkPreset = "latest"
	}
	return moduleConfigFromConfig(cfg)
}

func appendModuleIfMissing(modules []string, name string) []string {
	for _, module := range modules {
		if module == name {
			return modules
		}
	}
	return append(modules, name)
}

func applyDefaultNetworkSafetyModuleConfig(cfg *config.Config) {
	cfg.Execution.RequireSigned = true
	cfg.Execution.RequireNonce = true
	cfg.Execution.MinFee = 1
	cfg.Execution.BaseFee = 1
	cfg.Execution.MinBaseFee = 1
	cfg.Execution.DynamicBaseFee = true
	cfg.Execution.MinGas = 1
	cfg.Execution.BlobBaseFee = 1
	cfg.Execution.MinBlobBaseFee = 1
	cfg.Execution.DynamicBlobBaseFee = true
	cfg.Execution.StrictEVMStateRoot = true
	cfg.Execution.AllowUnprotectedLegacyTx = false
	cfg.Bank.MintAuthority = "governance"
	cfg.Governance.RequireDeposit = true
	cfg.Governance.MinDeposit = "1avxo"
	cfg.Governance.DepositDenom = economics.AtomicDenom
	cfg.Governance.DepositEscrow = "module:governance:deposit_escrow"
	cfg.Governance.RejectedDeposits = "module:governance:rejected_deposits"
}

func defaultNetworkConfigDocument(chainID string, dataDir string, validatorID string) networkConfigDocument {
	cfg := config.Default(chainID)
	runtime := defaultRuntimeConfig(validatorID)
	return networkConfigDocument{
		SchemaVersion: networkSchemaVersion,
		RPC:           runtime.RPC,
		P2P:           runtime.P2P,
		StateSync:     runtime.StateSync,
		PeerScoring:   cfg.P2P,
	}
}

func defaultConsensusConfigDocument(chainID string, dataDir string, validatorID string) consensusConfigDocument {
	cfg := config.Default(chainID)
	applyDefaultNetworkSafetyConsensusConfig(&cfg)
	runtime := defaultRuntimeConfig(validatorID)
	return consensusConfigDocument{
		SchemaVersion: consensusSchemaVersion,
		Consensus:     runtime.Consensus,
		Crypto:        cfg.Crypto,
		VRF:           cfg.VRF,
		Validator:     cfg.Validator,
		Committee:     cfg.Committee,
	}
}

func applyDefaultNetworkSafetyConsensusConfig(cfg *config.Config) {
	cfg.Crypto.Backend = config.CryptoBackendBLS
	cfg.Crypto.ProductionAdapter = true
	cfg.Crypto.AdapterName = config.NetworkSafeBLSAdapterBLST
	cfg.Crypto.AuditReport = config.NetworkSafeBLSAuditReport
	cfg.Crypto.DependencyAudit = config.NetworkSafeBLSDependencyAudit
	cfg.Crypto.AuditEvidenceSHA256 = config.NetworkSafeBLSAuditEvidence
	cfg.Committee.Backend = committee.BackendVRF
	cfg.VRF.ProductionAdapter = true
	cfg.VRF.AdapterName = config.NetworkSafeVRFAdapterECVRFP256
	cfg.VRF.AuditReport = config.NetworkSafeVRFAuditReport
	cfg.VRF.DependencyAudit = config.NetworkSafeVRFDependencyAudit
	cfg.VRF.AuditEvidenceSHA256 = config.NetworkSafeVRFAuditEvidence
	cfg.VRF.KeySource = "consensus_config.vrf_key_paths"
}

func applyConsensusCryptoForKeyType(document *consensusConfigDocument, keyType string) {
	if document == nil {
		return
	}
	switch keyType {
	case vexocrypto.KeyTypeBLS:
		document.Crypto.Backend = config.CryptoBackendBLS
		document.Crypto.ProductionAdapter = true
		document.Crypto.AdapterName = config.NetworkSafeBLSAdapterBLST
		document.Crypto.AuditReport = config.NetworkSafeBLSAuditReport
		document.Crypto.DependencyAudit = config.NetworkSafeBLSDependencyAudit
		document.Crypto.AuditEvidenceSHA256 = config.NetworkSafeBLSAuditEvidence
	case vexocrypto.KeyTypeEd25519:
		document.Crypto.Backend = config.CryptoBackendEd25519
		document.Crypto.ProductionAdapter = false
		document.Crypto.AdapterName = ""
		document.Crypto.AuditReport = ""
		document.Crypto.DependencyAudit = ""
		document.Crypto.AuditEvidenceSHA256 = ""
	}
}

func defaultMempoolConfigDocument(chainID string, dataDir string) mempoolConfigDocument {
	cfg := config.Default(chainID)
	applyDefaultNetworkSafetyMempoolConfig(&cfg, dataDir)
	return mempoolConfigDocument{
		SchemaVersion: mempoolSchemaVersion,
		Mempool:       cfg.Mempool,
	}
}

func applyDefaultNetworkSafetyMempoolConfig(cfg *config.Config, dataDir string) {
	cfg.Mempool.MinFee = 1
	cfg.Mempool.EnablePriority = true
	cfg.Mempool.EnableReplacement = true
	cfg.Mempool.ReplacementBumpBPS = 1000
	cfg.Mempool.SeenTTL = time.Hour
	if dataDir != "" {
		cfg.Mempool.WALPath = filepath.Join(dataDir, "mempool.wal")
	} else {
		cfg.Mempool.WALPath = "mempool.wal"
	}
}

func defaultLogConfigDocument(chainID string, dataDir string, validatorID string) logConfigDocument {
	runtime := defaultRuntimeConfig(validatorID)
	return logConfigDocument{
		SchemaVersion: logSchemaVersion,
		Log:           runtime.Log,
	}
}

func defaultRuntimeConfig(validatorID string) runtimeConfig {
	loopEnabled := validatorID != ""
	return runtimeConfig{
		RPC: runtimeRPCConfig{
			Enabled:              true,
			Address:              defaultRPCAddress,
			ShutdownTimeout:      "10s",
			Web3MaxSubscriptions: 256,
			Web3IdleTimeout:      "2m",
			Web3FilterSnapshot:   filepath.Join("data", "web3_filters.json"),
		},
		P2P: runtimeP2PConfig{
			Enabled:          true,
			NodeID:           defaultP2PNodeID(validatorID, ""),
			NodeKeyPath:      nodeKeyFileName,
			ListenAddress:    defaultP2PAddress,
			DialTimeout:      "10s",
			AuthReplayPath:   filepath.Join("data", "p2p_auth_replay.jsonl"),
			AddrBookMaxFails: 3,
			Peers:            map[string]string{},
			Seeds:            map[string]string{},
		},
		StateSync: runtimeStateSyncConfig{
			Enabled:           false,
			Timeout:           defaultStateSyncTimeout.String(),
			RequireFresh:      true,
			TrustLocalHigher:  true,
			MaxSnapshotBytes:  defaultStateSyncSnapshotMaxBytes,
			RetryAllSnapshots: true,
		},
		Consensus: runtimeConsensusConfig{
			LoopEnabled:                 loopEnabled,
			Interval:                    "50ms",
			TimeoutPropose:              "3s",
			TimeoutPrevote:              "1s",
			TimeoutPrecommit:            "1s",
			TimeoutCommit:               "1s",
			RoundTimeout:                "3s",
			MaxBlockBytes:               1024 * 1024,
			CreateEmptyBlocks:           false,
			ExecutionCommit:             string(node.ExecutionCommitModeFinalized),
			AdaptiveRoundTimeoutEnabled: boolPtr(true),
			RecoveryFinalityGateEnabled: boolPtr(true),
		},
		Log: runtimeLogConfig{
			Format:       "text",
			Level:        "info",
			CommitEvents: boolPtr(true),
			PeerEvents:   boolPtr(true),
		},
	}
}

func defaultP2PNodeID(validatorID string, home string) string {
	if validatorID != "" {
		return validatorID
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "archive-node"
	}
	sum := sha256.Sum256([]byte(filepath.Clean(home)))
	return "archive-" + hex.EncodeToString(sum[:6])
}

func chainConfigFromConfig(cfg config.Config) chainConfigDocument {
	return chainConfigDocument{
		ChainID:   cfg.ChainID,
		Crypto:    cfg.Crypto,
		VRF:       cfg.VRF,
		Validator: cfg.Validator,
		Committee: cfg.Committee,
		Mempool:   cfg.Mempool,
		P2P:       cfg.P2P,
	}
}

func configFromChainConfigDocument(document chainConfigDocument) config.Config {
	cfg := config.Default(document.ChainID)
	cfg.ChainID = document.ChainID
	cfg.Crypto = document.Crypto
	cfg.VRF = document.VRF
	cfg.Validator = document.Validator
	cfg.Committee = document.Committee
	cfg.Mempool = document.Mempool
	cfg.P2P = document.P2P
	return cfg
}

func moduleConfigFromConfig(cfg config.Config) moduleConfigDocument {
	return moduleConfigDocument{
		SchemaVersion: moduleSchemaVersion,
		Application:   cfg.Application,
		Execution:     cfg.Execution,
		Bank:          cfg.Bank,
		Staking:       cfg.Staking,
		Governance:    cfg.Governance,
	}
}

func networkConfigFromConfig(cfg config.Config, runtime runtimeConfig) networkConfigDocument {
	return networkConfigDocument{
		SchemaVersion: networkSchemaVersion,
		RPC:           runtime.RPC,
		P2P:           runtime.P2P,
		StateSync:     runtime.StateSync,
		PeerScoring:   cfg.P2P,
	}
}

func consensusConfigFromConfig(cfg config.Config, runtime runtimeConfig) consensusConfigDocument {
	return consensusConfigDocument{
		SchemaVersion: consensusSchemaVersion,
		Consensus:     runtime.Consensus,
		Crypto:        cfg.Crypto,
		VRF:           cfg.VRF,
		Validator:     cfg.Validator,
		Committee:     cfg.Committee,
	}
}

func mempoolConfigFromConfig(cfg config.Config) mempoolConfigDocument {
	return mempoolConfigDocument{
		SchemaVersion: mempoolSchemaVersion,
		Mempool:       cfg.Mempool,
	}
}

func logConfigFromRuntime(runtime runtimeConfig) logConfigDocument {
	return logConfigDocument{
		SchemaVersion: logSchemaVersion,
		Log:           runtime.Log,
	}
}

func configFromConfigDocuments(document configDocument, moduleDocument moduleConfigDocument, networkDocument networkConfigDocument, consensusDocument consensusConfigDocument, mempoolDocument mempoolConfigDocument) config.Config {
	chainID := document.ChainID
	if chainID == "" {
		chainID = document.Chain.ChainID
	}
	cfg := config.Default(chainID)
	if len(moduleDocument.Application.CoreModules) > 0 {
		cfg.Application.CoreModules = append([]string(nil), moduleDocument.Application.CoreModules...)
	}
	if moduleDocument.Application.Modules != nil {
		cfg.Application.Modules = append([]string(nil), moduleDocument.Application.Modules...)
	}
	cfg.Execution = moduleDocument.Execution
	cfg.Execution = normalizeExecutionConfig(cfg.Execution)
	cfg.Bank = moduleDocument.Bank
	if moduleDocument.Staking != (config.StakingConfig{}) {
		cfg.Staking = moduleDocument.Staking
	}
	if moduleDocument.Governance != (governance.TallyPolicy{}) {
		cfg.Governance = moduleDocument.Governance
	}
	cfg.P2P = networkDocument.PeerScoring
	cfg.Crypto = consensusDocument.Crypto
	cfg.VRF = consensusDocument.VRF
	cfg.Validator = consensusDocument.Validator
	cfg.Committee = consensusDocument.Committee
	cfg.Mempool = mempoolDocument.Mempool
	return cfg
}

func normalizeExecutionConfig(execution config.ExecutionConfig) config.ExecutionConfig {
	defaults := config.Default(defaultChainID).Execution
	if execution.EVMChainID == 0 {
		execution.EVMChainID = defaults.EVMChainID
	}
	if execution.TargetGas == 0 {
		execution.TargetGas = defaults.TargetGas
	}
	if execution.MaxGas == 0 {
		execution.MaxGas = defaults.MaxGas
	}
	if execution.BaseFeeChangeDenominator == 0 {
		execution.BaseFeeChangeDenominator = defaults.BaseFeeChangeDenominator
	}
	if execution.BlobBaseFee == 0 {
		execution.BlobBaseFee = defaults.BlobBaseFee
	}
	if execution.TargetBlobGas == 0 {
		execution.TargetBlobGas = defaults.TargetBlobGas
	}
	if execution.MaxBlobGas == 0 {
		execution.MaxBlobGas = defaults.MaxBlobGas
	}
	if execution.BlobFeeChangeDenominator == 0 {
		execution.BlobFeeChangeDenominator = defaults.BlobFeeChangeDenominator
	}
	if execution.MinBaseFee == 0 {
		execution.MinBaseFee = defaults.MinBaseFee
	}
	if execution.MinBlobBaseFee == 0 {
		execution.MinBlobBaseFee = defaults.MinBlobBaseFee
	}
	if execution.MinGas == 0 {
		execution.MinGas = defaults.MinGas
	}
	if execution.FeeCollector == "" {
		execution.FeeCollector = defaults.FeeCollector
	}
	if execution.FeeDenom == "" {
		execution.FeeDenom = defaults.FeeDenom
	}
	if execution.DisplayDenom == "" {
		execution.DisplayDenom = defaults.DisplayDenom
	}
	if execution.DisplayExponent == 0 {
		execution.DisplayExponent = defaults.DisplayExponent
	}
	if execution.GasDenom == "" {
		execution.GasDenom = defaults.GasDenom
	}
	if execution.EVMForkPreset == "" {
		execution.EVMForkPreset = defaults.EVMForkPreset
	}
	if execution.MaxBlobSidecarBlobs == 0 {
		execution.MaxBlobSidecarBlobs = defaults.MaxBlobSidecarBlobs
	}
	if execution.MaxBlobSidecarBytes == 0 {
		execution.MaxBlobSidecarBytes = defaults.MaxBlobSidecarBytes
	}
	if execution.RPCGasCap == 0 {
		execution.RPCGasCap = defaults.RPCGasCap
	}
	return execution
}

func boolPtr(value bool) *bool {
	return &value
}

func defaultGenesisDocument(chainID string, validatorID string) genesisDocument {
	return genesisDocument{
		SchemaVersion: genesisSchemaVersion,
		ChainID:       chainID,
		Validators: []validatorDocument{
			{
				ID:          validatorID,
				Address:     validatorID,
				VotingPower: 1,
				Stake:       1,
			},
		},
		Governance: map[string]uint64{validatorID: 1},
	}
}
