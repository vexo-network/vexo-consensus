package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/vexo-network/vexo-consensus/config"
)

type pathDocument struct {
	Home     string `json:"home"`
	Config   string `json:"config"`
	Genesis  string `json:"genesis"`
	Key      string `json:"key"`
	AddrBook string `json:"addr_book"`
	DataDir  string `json:"data_dir,omitempty"`
}

type profilesDocument struct {
	Profiles []profileDocument `json:"profiles"`
}

type profileDocument struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	CommitteeSize        uint64 `json:"committee_size"`
	CommitteeEpochLength uint64 `json:"committee_epoch_length"`
	MempoolMaxTxBytes    int64  `json:"mempool_max_tx_bytes"`
	MempoolMaxTxs        int    `json:"mempool_max_txs"`
	MempoolPriority      bool   `json:"mempool_priority"`
	ExecutionMinFee      uint64 `json:"execution_min_fee"`
	RequireNonce         bool   `json:"require_nonce"`
	RequireSigned        bool   `json:"require_signed"`
	P2PMaxMessagesWindow uint64 `json:"p2p_max_messages_per_window"`
	P2PBanDuration       string `json:"p2p_ban_duration"`
}

func runConfig(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand is required")
	}
	switch args[0] {
	case "audit":
		return runConfigAudit(writer, args[1:])
	case "paths":
		return runConfigPaths(writer, args[1:])
	case "profiles":
		return runConfigProfiles(writer, args[1:])
	case "show":
		return runConfigShow(writer, args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func runConfigProfiles(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config profiles", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	chainID := flags.String("chain-id", defaultChainID, "chain id used to render profile values")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document, err := buildProfilesDocument(*chainID)
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	for _, profile := range document.Profiles {
		fmt.Fprintf(writer, "%s\n", profile.Name)
		fmt.Fprintf(writer, "  description: %s\n", profile.Description)
		fmt.Fprintf(writer, "  committee: size=%d epoch=%d\n", profile.CommitteeSize, profile.CommitteeEpochLength)
		fmt.Fprintf(writer, "  mempool: max_tx_bytes=%d max_txs=%d priority=%t\n", profile.MempoolMaxTxBytes, profile.MempoolMaxTxs, profile.MempoolPriority)
		fmt.Fprintf(writer, "  execution: min_fee=%d require_nonce=%t require_signed=%t\n", profile.ExecutionMinFee, profile.RequireNonce, profile.RequireSigned)
		fmt.Fprintf(writer, "  p2p: max_messages_per_window=%d ban_duration=%s\n", profile.P2PMaxMessagesWindow, profile.P2PBanDuration)
	}
	return nil
}

func buildProfilesDocument(chainID string) (profilesDocument, error) {
	specs := config.Profiles()
	document := profilesDocument{Profiles: make([]profileDocument, 0, len(specs))}
	for _, spec := range specs {
		cfg, err := config.WithProfile(chainID, spec.Name)
		if err != nil {
			return profilesDocument{}, err
		}
		document.Profiles = append(document.Profiles, profileDocument{
			Name:                 string(spec.Name),
			Description:          spec.Description,
			CommitteeSize:        cfg.Committee.CommitteeSize,
			CommitteeEpochLength: cfg.Committee.EpochLength,
			MempoolMaxTxBytes:    cfg.Mempool.MaxTxBytes,
			MempoolMaxTxs:        cfg.Mempool.MaxTxs,
			MempoolPriority:      cfg.Mempool.EnablePriority,
			ExecutionMinFee:      cfg.Execution.MinFee,
			RequireNonce:         cfg.Execution.RequireNonce,
			RequireSigned:        cfg.Execution.RequireSigned,
			P2PMaxMessagesWindow: cfg.P2P.MaxMessagesPerWindow,
			P2PBanDuration:       cfg.P2P.BanDuration.String(),
		})
	}
	return document, nil
}

func runConfigPaths(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config paths", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	genesisPath := flags.String("genesis", "", "genesis file path")
	keyPath := flags.String("key", "", "key file path")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document := pathDocument{
		Home:     *home,
		Config:   resolveConfigPath(*home, *configPath),
		Genesis:  resolveGenesisPath(*home, *genesisPath),
		Key:      resolveKeyPath(*home, *keyPath),
		AddrBook: resolveAddrBookPath(*home, ""),
	}
	if cfg, err := loadNodeConfig(document.Config); err == nil {
		document.DataDir = cfg.DataDir
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	fmt.Fprintf(writer, "home: %s\n", document.Home)
	fmt.Fprintf(writer, "config: %s\n", document.Config)
	fmt.Fprintf(writer, "genesis: %s\n", document.Genesis)
	fmt.Fprintf(writer, "key: %s\n", document.Key)
	fmt.Fprintf(writer, "addr_book: %s\n", document.AddrBook)
	if document.DataDir != "" {
		fmt.Fprintf(writer, "data_dir: %s\n", document.DataDir)
	}
	return nil
}

func runConfigShow(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("config show", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", defaultHomeDir, "node home directory")
	configPath := flags.String("config", "", "config file path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := loadNodeConfig(resolveConfigPath(*home, *configPath))
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}
