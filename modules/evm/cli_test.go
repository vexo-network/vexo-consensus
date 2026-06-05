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
	if err := command.Execute(&output, []string{"query", "receipt", "0xhash"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: evm/receipt/0xhash" {
		t.Fatalf("unexpected query output: %s", output.String())
	}
	output.Reset()
	if err := command.Execute(&output, []string{"query", "logs"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) != "query_path: evm/logs" {
		t.Fatalf("unexpected global logs query output: %s", output.String())
	}
}
