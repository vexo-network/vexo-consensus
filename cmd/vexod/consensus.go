package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	vexoconsensus "github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func runConsensus(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("consensus subcommand is required")
	}
	switch args[0] {
	case "adversarial":
		return runConsensusAdversarial(writer, args[1:])
	default:
		return fmt.Errorf("unknown consensus subcommand %q", args[0])
	}
}

func runConsensusAdversarial(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("consensus adversarial", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	validatorCount := flags.Int("validators", 4, "number of equal-power validators to simulate")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	report, err := vexoconsensus.RunAdversarialSimulation(context.Background(), vexoconsensus.AdversarialSimulationConfig{
		Validators: simulationValidators(*validatorCount),
	})
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	fmt.Fprintf(writer, "consensus adversarial simulation\n")
	fmt.Fprintf(writer, "validators: %d\n", *validatorCount)
	fmt.Fprintf(writer, "safety_ok: %t\n", report.SafetyOK)
	fmt.Fprintf(writer, "liveness_ok: %t\n", report.LivenessOK)
	fmt.Fprintf(writer, "evidence_count: %d\n", report.EvidenceCount)
	fmt.Fprintf(writer, "quorum_failures: %d\n", report.QuorumFailures)
	fmt.Fprintf(writer, "rejected_attacks: %d\n", report.RejectedAttacks)
	for _, scenario := range report.Scenarios {
		fmt.Fprintf(writer, "scenario: %s ok=%t safety=%t liveness=%t quorum=%t rejected=%t evidence=%d finalized=%d\n", scenario.Name, scenario.OK, scenario.SafetyOK, scenario.LivenessOK, scenario.QuorumReached, scenario.Rejected, scenario.Evidence, scenario.Finalized)
		if scenario.Error != "" {
			fmt.Fprintf(writer, "  error: %s\n", scenario.Error)
		}
	}
	return nil
}

func simulationValidators(count int) []validator.Validator {
	if count <= 0 {
		return nil
	}
	validators := make([]validator.Validator, 0, count)
	for index := 0; index < count; index++ {
		id := types.ValidatorID(fmt.Sprintf("validator-%d", index+1))
		validators = append(validators, validator.Validator{
			ID:          id,
			Address:     types.Address(id),
			VotingPower: 1,
			Stake:       1,
		})
	}
	return validators
}
