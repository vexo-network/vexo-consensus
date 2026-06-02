package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunCommandHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		var stdout bytes.Buffer
		if err := runCommand(&stdout, &bytes.Buffer{}, args); err != nil {
			t.Fatal(err)
		}
		output := stdout.String()
		for _, expected := range []string{"Usage:", "init", "config paths", "start", "version", "Module Commands:", "bank tx mint", "bank query balance"} {
			if !strings.Contains(output, expected) {
				t.Fatalf("expected help output to contain %q, got:\n%s", expected, output)
			}
		}
	}

	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "vexod "+version {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}
}

func TestRunCommandDispatchesModuleCLI(t *testing.T) {
	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"bank", "tx", "send", "alice", "bob", "25"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "tx: bank:send:alice:bob:25" {
		t.Fatalf("unexpected module cli output: %q", stdout.String())
	}
}

func TestRunCommandShowsModuleHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"bank", "--help"}); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, expected := range []string{"bank module commands", "Usage:", "Commands:", "tx", "query", "bank tx mint"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected module help to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunCommandShowsNestedModuleHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := runCommand(&stdout, &bytes.Buffer{}, []string{"bank", "tx", "mint", "--help"}); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, expected := range []string{"build a mint transaction payload", "Arguments:", "to", "amount", "bank tx mint <to> <amount>"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected nested module help to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRunCommandReportsModuleCLIError(t *testing.T) {
	var stderr bytes.Buffer
	if err := runCommand(&bytes.Buffer{}, &stderr, []string{"bank", "tx", "mint", "alice", "0"}); err == nil {
		t.Fatal("expected module cli error")
	}
	if !strings.Contains(stderr.String(), "bank failed") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunCommandRejectsUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	if err := runCommand(&bytes.Buffer{}, &stderr, []string{"nope"}); err == nil {
		t.Fatal("expected unknown command error")
	}
	if !strings.Contains(stderr.String(), "unknown command") || !strings.Contains(stderr.String(), "vexod help") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunConfigPathsAndShow(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}

	var pathsOutput bytes.Buffer
	if err := runConfig(&pathsOutput, []string{"paths", "--home", home, "--json"}); err != nil {
		t.Fatal(err)
	}
	var paths pathDocument
	if err := json.Unmarshal(pathsOutput.Bytes(), &paths); err != nil {
		t.Fatal(err)
	}
	if paths.Config == "" || paths.Genesis == "" || paths.Key == "" || paths.DataDir == "" {
		t.Fatalf("unexpected paths document: %+v", paths)
	}

	var configOutput bytes.Buffer
	if err := runConfig(&configOutput, []string{"show", "--home", home}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(configOutput.String(), `"ChainID": "vexo-test"`) {
		t.Fatalf("unexpected config output:\n%s", configOutput.String())
	}
}

func TestRunConfigRejectsInvalidSubcommand(t *testing.T) {
	if err := runConfig(&bytes.Buffer{}, nil); err == nil {
		t.Fatal("expected missing config subcommand error")
	}
	if err := runConfig(&bytes.Buffer{}, []string{"unknown"}); err == nil {
		t.Fatal("expected unknown config subcommand error")
	}
}

func TestRunStartDryRun(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runStart(&output, []string{"--home", home, "--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	var plan startPlanDocument
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.ChainID != "vexo-test" || plan.ValidatorID != "alice" || plan.ValidatorN != 1 || plan.KeyType == "" || plan.PublicKey == "" || !plan.DryRun {
		t.Fatalf("unexpected start plan: %+v", plan)
	}

	output.Reset()
	if err := runCommand(&output, &bytes.Buffer{}, []string{"start", "--home", home, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "startup dry-run valid") || !strings.Contains(output.String(), "validator_id: alice") {
		t.Fatalf("unexpected start output:\n%s", output.String())
	}
}

func TestBuildStartNodeLoadsValidatorSigner(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	inputs, err := loadStartInputs(home, "", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs.Genesis.Validators) != 1 || len(inputs.Genesis.Validators[0].PublicKey) == 0 {
		t.Fatalf("expected local validator public key to be patched from key file: %+v", inputs.Genesis.Validators)
	}
	if string(inputs.Genesis.Validators[0].PublicKey) != string(inputs.Signer.PublicKey()) {
		t.Fatal("expected genesis public key to match loaded signer")
	}
	startNode, err := buildStartNode(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if startNode == nil {
		t.Fatal("expected start node")
	}

	var output bytes.Buffer
	if err := runStart(&output, []string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "validator signer loaded") {
		t.Fatalf("expected signer loaded output, got:\n%s", output.String())
	}
}

func TestRunStartRunStartsAndStopsNode(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test", "--validator", "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := runKeys(&bytes.Buffer{}, []string{"gen", "--home", home}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	output := &cancelOnNeedleWriter{
		needle: "node running",
		cancel: cancel,
	}
	if err := runStartWithContext(ctx, output, []string{"--home", home, "--run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "node running") || !strings.Contains(output.String(), "node stopped") {
		t.Fatalf("unexpected run output:\n%s", output.String())
	}
}

func TestRunStartRequiresKey(t *testing.T) {
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home}); err != nil {
		t.Fatal(err)
	}
	if err := runStart(&bytes.Buffer{}, []string{"--home", home, "--dry-run"}); err == nil {
		t.Fatal("expected missing key error")
	}
}

type cancelOnNeedleWriter struct {
	bytes.Buffer
	needle string
	cancel context.CancelFunc
}

func (writer *cancelOnNeedleWriter) Write(data []byte) (int, error) {
	count, err := writer.Buffer.Write(data)
	if strings.Contains(writer.Buffer.String(), writer.needle) {
		writer.cancel()
	}
	return count, err
}
