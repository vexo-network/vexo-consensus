package governance

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/upgrade"
)

// Size limits of a proposal. The governance state is rewritten by every operation, so a proposal must
// not be able to grow it without bound.
const (
	MaxProposalTitleBytes       = 256
	MaxProposalDescriptionBytes = 4096
	MaxProposalChanges          = 16
	MaxParameterChangeBytes     = 8192
)

// Defaults of the tally thresholds in basis points of the bonded power: a third of the stake must
// take part, half of the stake must vote yes, and a third of the stake vetoes.
const (
	DefaultQuorumBPS       uint64 = 3340
	DefaultYesThresholdBPS uint64 = 5000
	DefaultVetoBPS         uint64 = 3340
)

var (
	ErrProposalTooLarge       = errors.New("proposal exceeds the size limits")
	ErrUpgradePlanNotInFuture = errors.New("upgrade plan height is not in the future")
	ErrUpgradePlanConflict    = errors.New("an upgrade plan is already scheduled for the height")
	ErrInvalidTallyPolicy     = errors.New("invalid governance tally policy")
)

// Validate checks that a proposal is complete, within the size limits and that every upgrade plan it
// carries is well formed.
func (proposal Proposal) Validate() error {
	if proposal.Title == "" {
		return ErrMissingProposalTitle
	}
	if proposal.Submitter == "" {
		return ErrMissingSubmitter
	}
	if len(proposal.Changes) == 0 {
		return ErrNoProposalChanges
	}
	if len(proposal.Title) > MaxProposalTitleBytes ||
		len(proposal.Description) > MaxProposalDescriptionBytes ||
		len(proposal.Changes) > MaxProposalChanges {
		return ErrProposalTooLarge
	}
	for _, change := range proposal.Changes {
		if len(change.Module)+len(change.Key)+len(change.Value) > MaxParameterChangeBytes {
			return ErrProposalTooLarge
		}
	}
	_, err := upgradePlans(proposal.Changes)
	return err
}

// Validate checks the thresholds of a tally policy.
func (policy TallyPolicy) Validate() error {
	if policy.QuorumBPS == 0 || policy.QuorumBPS > BasisPoints ||
		policy.YesThresholdBPS == 0 || policy.YesThresholdBPS > BasisPoints ||
		policy.VetoBPS > BasisPoints {
		return fmt.Errorf("%w: thresholds are basis points of the bonded power in 1..%d", ErrInvalidTallyPolicy, BasisPoints)
	}
	return nil
}

// upgradePlans extracts the upgrade plans a list of parameter changes schedules.
func upgradePlans(changes []ParameterChange) ([]upgrade.Plan, error) {
	var plans []upgrade.Plan
	for _, change := range changes {
		if change.Module != "upgrade" || change.Key != "plan" {
			continue
		}
		var plan upgrade.Plan
		if err := json.Unmarshal(change.Value, &plan); err != nil {
			return nil, err
		}
		if err := upgrade.ValidatePlan(plan); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}
