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
	AppealDeadline types.Height             `json:"appeal_deadline"`
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
	policy := slashing.DefaultPenaltyPolicy()
	penalty, found := policy[evidenceType]
	if !found {
		return slashingLifecyclePlanDocument{}, slashing.ErrUnknownEvidenceType
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
		AppealDeadline: evidenceHeight + lifecycle.AppealWindow,
		CurrentPower:   currentPower,
		RemainingPower: remainingPower,
		Penalty:        penalty,
		Steps: []string{
			"validate evidence type, validator id, height, and proof before gossiping",
			"deduplicate by stable evidence key before accepting into durable storage",
			"hold evidence through the appeal window before final penalty execution",
			"apply stake slash, jail validator, and block unbonding until release height",
			"persist penalty receipt and validator set update proof",
			"expire unapplied evidence after max age and archive all receipts for audit",
		},
	}
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "validator", OK: validator != "", Message: "validator id is required"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "height", OK: evidenceHeight > 0, Message: "evidence height is required"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "not_expired", OK: currentHeight < document.ExpiresAt, Message: "evidence must not be expired before penalty"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "appeal_window", OK: currentHeight >= document.AppealDeadline, Message: "appeal window should close before penalty is final"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "stake_accounting", OK: remainingPower <= currentPower, Message: "remaining power must not exceed current power"})
	document.Checks = append(document.Checks, slashingLifecycleCheck{Name: "runtime_proof_verifier", OK: !document.PlanOnly, Message: "unsupported evidence types are plan-only until concrete proof decoder and verifier are implemented"})
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
		evidenceType == slashing.EvidenceConflictingTimeoutVote
}

func writeSlashingLifecyclePlan(writer io.Writer, document slashingLifecyclePlanDocument) {
	fmt.Fprintf(writer, "slashing lifecycle plan\n")
	fmt.Fprintf(writer, "type: %s\n", document.EvidenceType)
	fmt.Fprintf(writer, "plan_only: %t\n", document.PlanOnly)
	fmt.Fprintf(writer, "validator: %s\n", document.Validator)
	fmt.Fprintf(writer, "height: %d\n", document.EvidenceHeight)
	fmt.Fprintf(writer, "current_height: %d\n", document.CurrentHeight)
	fmt.Fprintf(writer, "expires_at: %d\n", document.ExpiresAt)
	fmt.Fprintf(writer, "appeal_deadline: %d\n", document.AppealDeadline)
	fmt.Fprintf(writer, "power: %d -> %d\n", document.CurrentPower, document.RemainingPower)
	fmt.Fprintf(writer, "slash_fraction: %s\n", document.Penalty.SlashFraction)
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
