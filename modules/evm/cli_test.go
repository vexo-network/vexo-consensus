package evm

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLICommandsBuildEVMTransactionsAndQueries(t *testing.T) {
	command := evmCLICommand()
	var output bytes.Buffer
	if err := command.Execute(&output, []string{"tx", "call", "evm", "0xaaaa", "0xbbbb", "transfer", "aabb", "100000", "--fee", "1", "--gas", "100000", "--signer", "0xaaaa", "--nonce", "1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: evm:call:evm:0xaaaa:0xbbbb:transfer:aabb:100000:fee=1:gas=100000:signer=0xaaaa:nonce=1") {
		t.Fatalf("unexpected call output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"tx", "deploy", "evm", "0xaaaa", "6001", "salt"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tx: evm:deploy:evm:0xaaaa:6001:salt") {
		t.Fatalf("unexpected deploy output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"tx", "call", "evm", "0xaaaa", "0xbbbb", "transfer", "aabb", "100000", "0x10000000000000000"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), ":18446744073709551616") {
		t.Fatalf("expected 256-bit value to be normalized to decimal, got %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"query", "receipt", "0xhash"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: evm/receipt/0xhash" {
		t.Fatalf("unexpected query output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"query", "code", "0xcontract"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: evm/code/0xcontract" {
		t.Fatalf("unexpected code query output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"query", "storage", "0xcontract", "0x0"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: evm/storage/0xcontract/0x0" {
		t.Fatalf("unexpected storage query output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"query", "logs"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: evm/logs" {
		t.Fatalf("unexpected global logs query output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"query", "logs", "0xcontract"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: evm/logs/0xcontract" {
		t.Fatalf("unexpected address logs query output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"tx", "call", "evm", "0xaaaa", "0xbbbb", "transfer", "aabb", "bad"}); err != ErrInvalidEVMTx {
		t.Fatalf("expected invalid call gas rejection, got %v", err)
	}
	if err := command.Execute(&output, []string{"tx", "deploy", "evm", "0xaaaa", "6001", "salt", "bad"}); err != ErrInvalidEVMTx {
		t.Fatalf("expected invalid deploy value rejection, got %v", err)
	}
	if err := command.Execute(&output, []string{"tx", "call", "evm", "0xaaaa", "0xbbbb", "transfer", "aabb", "100000", "--bad", "1"}); err == nil {
		t.Fatalf("expected unknown flag rejection")
	}
	if commands := NewModule().CLICommands(); len(commands) != 1 || commands[0].Name != ModuleName {
		t.Fatalf("unexpected module CLI commands: %+v", commands)
	}
}
