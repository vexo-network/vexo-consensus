package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

func runTx(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("tx subcommand is required")
	}
	switch args[0] {
	case "build":
		return runTxBuild(writer, args[1:])
	case "parse":
		return runTxParse(writer, args[1:])
	default:
		return fmt.Errorf("unknown tx subcommand %q", args[0])
	}
}

func runTxBuild(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("tx build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	module := flags.String("module", "", "transaction module")
	action := flags.String("action", "", "transaction action")
	rawArgs := flags.String("args", "", "comma-separated positional arguments")
	tags := flags.String("tags", "", "comma-separated key=value tags")
	jsonOutput := flags.Bool("json", false, "write parsed canonical transaction JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	tx, err := vexoapp.BuildCanonicalTx(vexoapp.CanonicalTx{
		Module: *module,
		Action: *action,
		Args:   splitCSV(*rawArgs),
		Tags:   parseTxTagList(*tags),
	})
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeParsedCanonicalTxJSON(writer, tx)
	}
	fmt.Fprintf(writer, "tx: %s\n", tx)
	return nil
}

func runTxParse(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("tx parse", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	txValue := flags.String("tx", "", "raw or signed transaction")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *txValue == "" {
		return errors.New("tx is required")
	}
	if *jsonOutput {
		return writeParsedCanonicalTxJSON(writer, types.Tx(*txValue))
	}
	parsed, err := vexoapp.ParseCanonicalTx(types.Tx(*txValue))
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, "module: %s\n", parsed.Module)
	fmt.Fprintf(writer, "action: %s\n", parsed.Action)
	if len(parsed.Args) > 0 {
		fmt.Fprintf(writer, "args: %s\n", strings.Join(parsed.Args, ","))
	}
	for _, key := range vexoapp.CanonicalTagKeys(parsed.Tags) {
		fmt.Fprintf(writer, "%s: %s\n", key, parsed.Tags[key])
	}
	return nil
}

func writeParsedCanonicalTxJSON(writer io.Writer, tx types.Tx) error {
	parsed, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(parsed)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseTxTagList(value string) map[string]string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	tags := make(map[string]string)
	for _, part := range strings.Split(value, ",") {
		key, tagValue, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found || key == "" {
			continue
		}
		tags[key] = tagValue
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}
