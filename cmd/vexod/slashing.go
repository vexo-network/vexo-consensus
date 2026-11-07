package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
)

type slashingLifecyclePlanDocument struct {
	SchemaVersion  string                   `json:"schema_version"`
	EvidenceType   slashing.EvidenceType    `json:"evidence_type"`
	PlanOnly       bool                     `json:"plan_only"`
	Validator      types.ValidatorID        `json:"validator"`
	EvidenceHeight types.Height             `json:"evidence_height"`
	CurrentHeight  types.Height             `json:"current_height"`
	ExpiresAt      types.Height             `json:"expires_at"`
	CurrentPower   types.VotingPower        `json:"current_power"`
	RemainingPower types.VotingPower        `json:"remaining_power"`
	Penalty        slashing.Penalty         `json:"penalty"`
	Checks         []slashingLifecycleCheck `json:"checks"`
	Steps          []string                 `json:"steps"`
	Warnings       []string                 `json:"warnings,omitempty"`
}

type slashingLifecycleCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func runSlashing(writer io.Writer, args []string) error {
	if len(args) == 0 {
		return errors.New("slashing subcommand is required")
	}
	switch args[0] {
	case "lifecycle-plan":
		return runSlashingLifecyclePlan(writer, args[1:])
	default:
		return fmt.Errorf("unknown slashing subcommand %q", args[0])
	}
}

func runSlashingLifecyclePlan(writer io.Writer, args []string) error {
	flags := flag.NewFlagSet("slashing lifecycle-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	evidenceType := flags.String("type", string(slashing.EvidenceConflictingVote), "evidence type")
	validator := flags.String("validator", "validator-1", "validator id")
	height := flags.Uint64("height", 1, "evidence height")
	currentHeight := flags.Uint64("current-height", 1, "current chain height")
	currentPower := flags.Uint64("current-power", 100, "current validator voting power")
	jsonOutput := flags.Bool("json", false, "write JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	document, err := buildSlashingLifecyclePlanDocument(slashing.EvidenceType(*evidenceType), types.ValidatorID(*validator), types.Height(*height), types.Height(*currentHeight), types.VotingPower(*currentPower))
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(document)
	}
	writeSlashingLifecyclePlan(writer, document)
	return nil
}

func buildSlashingLifecyclePlanDocument(evidenceType slashing.EvidenceType, validator types.ValidatorID, evidenceHeight types.Height, currentHeight types.Height, currentPower types.VotingPower) (slashingLifecyclePlanDocument, error) {
	penalty, err := slashing.DefaultPenaltyPolicy().Penalty(evidenceType)
	if err != nil {
		return slashingLifecyclePlanDocument{}, err
	}
	lifecycle := slashing.DefaultLifecyclePolicy()
	remainingPower, err := slashing.ApplySlash(currentPower, penalty)
	if err != nil {
		return slashingLifecyclePlanDocument{}, err
	}
	document := slashingLifecyclePlanDocument{
		SchemaVersion:  "v1",
		EvidenceType:   evidenceType,
		PlanOnly:       !supportsRuntimeSlashing(evidenceType),
		Validator:      validator,
		EvidenceHeight: evidenceHeight,
		CurrentHeight:  currentHeight,
		ExpiresAt:      evidenceHeight + lifecycle.EvidenceMaxAge,
		CurrentPower:   currentPower,
		RemainingPower: remainingPower,
		Penalty:        penalty,
		Steps: []string{
			"validate evidence type, validator id, height, and proof before gossiping",
			"deduplicate by stable offence key before accepting into the evidence pool",
			"submit the evidence as a signed, fee-paying slashing transaction; nodes never apply a penalty outside a block",
			"re-verify the evidence against the validator set of its height while executing the including block",
			"apply stake slash and jail the validator; the voting power change takes effect at the next height",
			"drop evidence once it is applied or older than the maximum age",
		},
	}
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "validator", OK: validator != "", Message: "validator id is required"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "height", OK: evidenceHeight > 0, Message: "evidence height is required"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "not_expired", OK: currentHeight < document.ExpiresAt, Message: "evidence must not be expired before penalty"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "not_from_future", OK: evidenceHeight <= currentHeight, Message: "evidence must not claim a height the chain has not reached"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "stake_accounting", OK: remainingPower <= currentPower, Message: "remaining power must not exceed current power"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "runtime_proof_verifier", OK: !document.PlanOnly, Message: "evidence type has a concrete runtime proof decoder and verifier"})
	for _, check := range document.Checks {
		if !check.OK {
			document.Warnings = append(document.Warnings, check.Message)
		}
	}
	return document, nil
}

func supportsRuntimeSlashing(evidenceType slashing.EvidenceType) bool {
	return evidenceType == slashing.EvidenceDoubleSign ||
		evidenceType == slashing.EvidenceConflictingVote ||
		evidenceType == slashing.EvidenceConflictingTimeoutVote ||
		evidenceType == slashing.EvidenceInvalidProposal ||
		evidenceType == slashing.EvidenceUnavailableData ||
		evidenceType == slashing.EvidenceFinalityConflict
}

func writeSlashingLifecyclePlan(writer io.Writer, document slashingLifecyclePlanDocument) {
	fmt.Fprintf(writer, "slashing lifecycle plan\n")
	fmt.Fprintf(writer, "type: %s\n", document.EvidenceType)
	fmt.Fprintf(writer, "plan_only: %t\n", document.PlanOnly)
	fmt.Fprintf(writer, "validator: %s\n", document.Validator)
	fmt.Fprintf(writer, "height: %d\n", document.EvidenceHeight)
	fmt.Fprintf(writer, "current_height: %d\n", document.CurrentHeight)
	fmt.Fprintf(writer, "expires_at: %d\n", document.ExpiresAt)
	fmt.Fprintf(writer, "power: %d -> %d\n", document.CurrentPower, document.RemainingPower)
	fmt.Fprintf(writer, "slash_bps: %d\n", document.Penalty.SlashBPS)
	fmt.Fprintf(writer, "jail_duration: %d\n", document.Penalty.JailDuration)
	fmt.Fprintf(writer, "checks:\n")
	for _, check := range document.Checks {
		fmt.Fprintf(writer, "- %s ok=%t %s\n", check.Name, check.OK, check.Message)
	}
	fmt.Fprintf(writer, "steps:\n")
	for index, step := range document.Steps {
		fmt.Fprintf(writer, "%d. %s\n", index+1, step)
	}
	if len(document.Warnings) > 0 {
		fmt.Fprintf(writer, "warnings:\n")
		for _, warning := range document.Warnings {
			fmt.Fprintf(writer, "- %s\n", warning)
		}
	}
}
