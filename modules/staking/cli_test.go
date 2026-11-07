package staking

import (
	"bytes"
	"strings"
	"testing"
)

func TestStakingCLICommands(t *testing.T) {
	command := stakingCLICommand()
	if command.Name != ModuleName || len(command.Children) != 2 {
		t.Fatalf("unexpected staking command: %+v", command)
	}
	if commands := NewModule().CLICommands(); len(commands) != 1 || commands[0].Name != ModuleName {
		t.Fatalf("unexpected module commands: %+v", commands)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"create-validator", []string{"tx", "create-validator", "operator-1", "validator-1", "40", "dmFsaWRhdG9yLWtleQ", "--signer", "operator-1"}, "tx: staking:create-validator:operator-1:validator-1:40:dmFsaWRhdG9yLWtleQ:signer=operator-1"},
		{"create-validator with proof", []string{"tx", "create-validator", "operator-1", "validator-1", "40", "a2V5", "cHJvb2Y"}, "tx: staking:create-validator:operator-1:validator-1:40:a2V5:cHJvb2Y"},
		{"delegate", []string{"tx", "delegate", "alice", "validator-1", "40", "--fee", "1avxo", "--gas", "1000", "--signer", "alice", "--nonce", "7"}, "tx: staking:delegate:alice:validator-1:40:fee=1avxo:gas=1000:signer=alice:nonce=7"},
		{"undelegate", []string{"tx", "undelegate", "alice", "validator-1", "15"}, "tx: staking:undelegate:alice:validator-1:15"},
		{"unjail", []string{"tx", "unjail", "validator-1"}, "tx: staking:unjail:validator-1"},
		{"claim-rewards", []string{"tx", "claim-rewards", "alice", "validator-1"}, "tx: staking:claim-rewards:alice:validator-1"},
		{"withdraw-unbonded", []string{"tx", "withdraw-unbonded", "alice", "validator-1"}, "tx: staking:withdraw-unbonded:alice:validator-1"},
		{"set-commission", []string{"tx", "set-commission", "validator-1", "500", "--signer", "operator-1"}, "tx: staking:set-commission:validator-1:500:signer=operator-1"},
		{"stake query", []string{"query", "stake", "alice", "validator-1"}, "query_path: staking/stake/alice/validator-1"},
		{"validator query", []string{"query", "validator", "validator-1"}, "query_path: staking/validator/validator-1"},
		{"unbonding query", []string{"query", "unbonding", "alice", "validator-1"}, "query_path: staking/unbonding/alice/validator-1"},
		{"unbonding-balance query", []string{"query", "unbonding-balance", "alice", "validator-1"}, "query_path: staking/unbonding-balance/alice/validator-1"},
		{"rewards query", []string{"query", "rewards", "alice", "validator-1"}, "query_path: staking/rewards/alice/validator-1"},
		{"commission query", []string{"query", "commission", "validator-1"}, "query_path: staking/commission/validator-1"},
		{"tombstone query", []string{"query", "tombstone", "validator-1"}, "query_path: staking/tombstone/validator-1"},
		{"jail query", []string{"query", "jail", "validator-1"}, "query_path: staking/jail/validator-1"},
		{"operator query", []string{"query", "operator", "validator-1"}, "query_path: staking/operator/validator-1"},
	} {
		var output bytes.Buffer
		if err := command.Execute(&output, tc.args); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if strings.TrimSpace(output.String()) != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, output.String())
		}
	}
	for _, args := range [][]string{
		{"tx", "delegate", "alice", "validator-1", "40", "extra-public-key"},
		{"tx", "create-validator", "operator-1", "validator-1", "40"},
		{"tx", "set-commission", "validator-1", "10001"},
		{"tx", "delegate", "alice", "validator-1", "bad"},
	} {
		if err := command.Execute(&bytes.Buffer{}, args); err == nil {
			t.Fatalf("expected %v to fail", args)
		}
	}
	if _, _, err := splitExecutionTags([]string{"--unknown", "1"}); err == nil {
		t.Fatalf("expected unknown flag failure")
	}
	if _, _, err := splitExecutionTags([]string{"--fee"}); err == nil {
		t.Fatalf("expected missing flag value failure")
	}
}
