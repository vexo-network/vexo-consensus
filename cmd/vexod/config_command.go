package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

type pathDocument struct {
	Home         string `json:"home"`
	Config       string `json:"config"`
	ModuleConfig string `json:"module_config"`
	Genesis      string `json:"genesis"`
	Key          string `json:"key"`
	AddrBook     string `json:"addr_book"`
	DataDir      string `json:"data_dir,omitempty"`
}

func runConfig(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand is required")
	}
	switch args[0] {
	case "audit":
		return runConfigAudit(writer, args[1:])
	case "audit-pack":
		return runConfigAuditPack(writer, args[1:])
	case "deployment-template":
		return runConfigDeploymentTemplate(writer, args[1:])
	case "paths":
		return runConfigPaths(writer, args[1:])
	case "show":
		return runConfigShow(writer, args[1:])
	case "tune":
		return runConfigTune(writer, args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
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
	resolvedConfigPath := resolveConfigPath(*home, *configPath)
	moduleConfigPath := resolveModuleConfigPath(*home, "")
	if configDocument, err := readConfigDocument(resolvedConfigPath); err == nil {
		moduleConfigPath = resolveModuleConfigPath(filepath.Dir(resolvedConfigPath), configDocument.ModuleConfigPath)
	}
	document := pathDocument{
		Home:         *home,
		Config:       resolvedConfigPath,
		ModuleConfig: moduleConfigPath,
		Genesis:      resolveGenesisPath(*home, *genesisPath),
		Key:          resolveKeyPath(*home, *keyPath),
		AddrBook:     resolveAddrBookPath(*home, ""),
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
	fmt.Fprintf(writer, "module_config: %s\n", document.ModuleConfig)
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
