package validator

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrValidatorExists    = errors.New("validator already exists")
	ErrValidatorNotFound  = errors.New("validator not found")
	ErrZeroVotingPower    = errors.New("voting power must be greater than zero")
	ErrMissingCandidateID = errors.New("candidate address is required")
)

type InMemoryRegistry struct {
	policy     AdmissionPolicy
	validators map[types.ValidatorID]Validator
}

func NewInMemoryRegistry(policy AdmissionPolicy, initialValidators []Validator) (*InMemoryRegistry, error) {
	registry := &InMemoryRegistry{
		policy:     policy,
		validators: make(map[types.ValidatorID]Validator, len(initialValidators)),
	}
	for _, validatorInfo := range initialValidators {
		if validatorInfo.VotingPower == 0 {
			return nil, ErrZeroVotingPower
		}
		registry.validators[validatorInfo.ID] = validatorInfo
	}
	return registry, nil
}

func (registry *InMemoryRegistry) ValidatorSet(ctx context.Context, height types.Height) (Set, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return registry.snapshot(), nil
}

func (registry *InMemoryRegistry) ApplyJoin(ctx context.Context, candidate Candidate) (Validator, error) {
	select {
	case <-ctx.Done():
		return Validator{}, ctx.Err()
	default:
	}

	if candidate.Address == "" {
		return Validator{}, ErrMissingCandidateID
	}
	validatorID := types.ValidatorID(candidate.Address)
	if _, found := registry.validators[validatorID]; found {
		return Validator{}, ErrValidatorExists
	}
	if registry.policy != nil {
		if err := registry.policy.CanJoin(ctx, candidate, registry.snapshot()); err != nil {
			return Validator{}, err
		}
	}

	validatorInfo := Validator{
		ID:          validatorID,
		Address:     candidate.Address,
		PublicKey:   candidate.PublicKey,
		VotingPower: types.VotingPower(candidate.Stake),
		Stake:       candidate.Stake,
		Metadata:    candidate.Metadata,
	}
	if validatorInfo.VotingPower == 0 {
		return Validator{}, ErrZeroVotingPower
	}

	registry.validators[validatorID] = validatorInfo
	return validatorInfo, nil
}

func (registry *InMemoryRegistry) ApplyLeave(ctx context.Context, id types.ValidatorID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, found := registry.validators[id]; !found {
		return ErrValidatorNotFound
	}
	delete(registry.validators, id)
	return nil
}

func (registry *InMemoryRegistry) UpdateVotingPower(ctx context.Context, id types.ValidatorID, power types.VotingPower) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if power == 0 {
		return ErrZeroVotingPower
	}
	validatorInfo, found := registry.validators[id]
	if !found {
		return ErrValidatorNotFound
	}
	validatorInfo.VotingPower = power
	registry.validators[id] = validatorInfo
	return nil
}

func (registry *InMemoryRegistry) snapshot() setSnapshot {
	validators := make([]Validator, 0, len(registry.validators))
	for _, validatorInfo := range registry.validators {
		validators = append(validators, validatorInfo)
	}
	sort.Slice(validators, func(left, right int) bool {
		return validators[left].ID < validators[right].ID
	})
	return newSetSnapshot(validators)
}
