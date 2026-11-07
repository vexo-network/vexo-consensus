package validator

import (
	"context"
	"fmt"
)

var (
	ErrInsufficientStake   error = invalidUpdateError("insufficient stake")
	ErrValidatorSetFull    error = invalidUpdateError("validator set full")
	ErrCandidateDenied     error = invalidUpdateError("candidate denied")
	ErrMissingCandidateKey error = invalidUpdateError("candidate public key is required")
)

type AdmissionConfig struct {
	Permissionless   bool
	MinStake         uint64
	MaxValidators    int
	Whitelist        map[string]bool
	RequirePublicKey bool
}

type ConfigurableAdmissionPolicy struct {
	config AdmissionConfig
}

func NewConfigurableAdmissionPolicy(config AdmissionConfig) ConfigurableAdmissionPolicy {
	return ConfigurableAdmissionPolicy{config: config}
}

func (policy ConfigurableAdmissionPolicy) CanJoin(ctx context.Context, candidate Candidate, currentSet Set) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if policy.config.MaxValidators > 0 && len(currentSet.List()) >= policy.config.MaxValidators {
		return ErrValidatorSetFull
	}
	if candidate.Stake < policy.config.MinStake {
		return ErrInsufficientStake
	}
	if policy.config.RequirePublicKey && len(candidate.PublicKey) == 0 {
		return ErrMissingCandidateKey
	}
	if policy.config.Permissionless {
		return nil
	}
	if policy.config.Whitelist[string(candidate.Address)] {
		return nil
	}
	return fmt.Errorf("%w: address is not whitelisted", ErrCandidateDenied)
}
