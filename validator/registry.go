package validator

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

// ErrInvalidUpdate matches, through errors.Is, every error that rejects the content of a
// validator update (as opposed to a storage failure or a cancelled context). Callers that must
// not abort on bad content, such as block execution, use it to tell the two apart.
var ErrInvalidUpdate = errors.New("invalid validator update")

// invalidUpdateError is a sentinel for a validator update that is rejected on its content.
type invalidUpdateError string

func (err invalidUpdateError) Error() string {
	return string(err)
}

func (err invalidUpdateError) Is(target error) bool {
	return target == ErrInvalidUpdate
}

var (
	ErrValidatorExists    error = invalidUpdateError("validator already exists")
	ErrValidatorNotFound  error = invalidUpdateError("validator not found")
	ErrZeroVotingPower    error = invalidUpdateError("voting power must be greater than zero")
	ErrMissingCandidateID error = invalidUpdateError("candidate address is required")
)

type InMemoryRegistry struct {
	policy          AdmissionPolicy
	validators      map[types.ValidatorID]Validator
	effectiveHeight types.Height
	versions        []validatorSetVersion
}

type VersionedRegistry interface {
	Registry
	SetEffectiveHeight(height types.Height)
	ApplyJoinAt(ctx context.Context, height types.Height, candidate Candidate) (Validator, error)
	ApplyLeaveAt(ctx context.Context, height types.Height, id types.ValidatorID) error
	UpdateVotingPowerAt(ctx context.Context, height types.Height, id types.ValidatorID, power types.VotingPower) error
}

type validatorSetVersion struct {
	height     types.Height
	validators map[types.ValidatorID]Validator
}

func NewInMemoryRegistry(policy AdmissionPolicy, initialValidators []Validator) (*InMemoryRegistry, error) {
	registry := &InMemoryRegistry{
		policy:          policy,
		validators:      make(map[types.ValidatorID]Validator, len(initialValidators)),
		effectiveHeight: 1,
	}
	for _, validatorInfo := range initialValidators {
		if validatorInfo.VotingPower == 0 {
			return nil, ErrZeroVotingPower
		}
		registry.validators[validatorInfo.ID] = cloneValidator(validatorInfo)
	}
	registry.recordVersion(1)
	return registry, nil
}

func (registry *InMemoryRegistry) SetEffectiveHeight(height types.Height) {
	if height == 0 {
		height = 1
	}
	registry.effectiveHeight = height
}

func (registry *InMemoryRegistry) ValidatorSet(ctx context.Context, height types.Height) (Set, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if height == 0 {
		height = registry.effectiveHeight
	}
	return registry.snapshotAt(height), nil
}

func (registry *InMemoryRegistry) ApplyJoin(ctx context.Context, candidate Candidate) (Validator, error) {
	return registry.ApplyJoinAt(ctx, registry.effectiveHeight, candidate)
}

func (registry *InMemoryRegistry) ApplyJoinAt(ctx context.Context, height types.Height, candidate Candidate) (Validator, error) {
	select {
	case <-ctx.Done():
		return Validator{}, ctx.Err()
	default:
	}

	if candidate.Address == "" {
		return Validator{}, ErrMissingCandidateID
	}
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.validators = registry.snapshotAt(height).toMap()
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
		PublicKey:   append(types.PublicKey(nil), candidate.PublicKey...),
		VotingPower: types.VotingPower(candidate.Stake),
		Stake:       candidate.Stake,
		Metadata:    cloneMetadata(candidate.Metadata),
	}
	if validatorInfo.VotingPower == 0 {
		return Validator{}, ErrZeroVotingPower
	}

	registry.validators[validatorID] = cloneValidator(validatorInfo)
	registry.recordMutation(height)
	return cloneValidator(validatorInfo), nil
}

func (registry *InMemoryRegistry) ApplyLeave(ctx context.Context, id types.ValidatorID) error {
	return registry.ApplyLeaveAt(ctx, registry.effectiveHeight, id)
}

func (registry *InMemoryRegistry) ApplyLeaveAt(ctx context.Context, height types.Height, id types.ValidatorID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.validators = registry.snapshotAt(height).toMap()
	if _, found := registry.validators[id]; !found {
		return ErrValidatorNotFound
	}
	delete(registry.validators, id)
	registry.recordMutation(height)
	return nil
}

func (registry *InMemoryRegistry) UpdateVotingPower(ctx context.Context, id types.ValidatorID, power types.VotingPower) error {
	return registry.UpdateVotingPowerAt(ctx, registry.effectiveHeight, id, power)
}

func (registry *InMemoryRegistry) UpdateVotingPowerAt(ctx context.Context, height types.Height, id types.ValidatorID, power types.VotingPower) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if power == 0 {
		return ErrZeroVotingPower
	}
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.validators = registry.snapshotAt(height).toMap()
	validatorInfo, found := registry.validators[id]
	if !found {
		return ErrValidatorNotFound
	}
	validatorInfo.VotingPower = power
	registry.validators[id] = validatorInfo
	registry.recordMutation(height)
	return nil
}

func (registry *InMemoryRegistry) snapshot() setSnapshot {
	return newSetSnapshot(sortedValidatorMap(registry.validators))
}

func (registry *InMemoryRegistry) snapshotAt(height types.Height) setSnapshot {
	if len(registry.versions) == 0 {
		return registry.snapshot()
	}
	var selected *validatorSetVersion
	for index := range registry.versions {
		version := &registry.versions[index]
		if version.height <= height {
			selected = version
		}
	}
	if selected == nil {
		return newSetSnapshot(nil)
	}
	return newSetSnapshot(sortedValidatorMap(selected.validators))
}

func (registry *InMemoryRegistry) recordMutation(height types.Height) {
	registry.recordVersion(height)
	registry.effectiveHeight = height
}

func (registry *InMemoryRegistry) recordVersion(height types.Height) {
	if height == 0 {
		height = 1
	}
	version := validatorSetVersion{height: height, validators: registry.snapshot().toMap()}
	for index := range registry.versions {
		if registry.versions[index].height == height {
			registry.versions[index] = version
			return
		}
	}
	registry.versions = append(registry.versions, version)
	sort.Slice(registry.versions, func(left, right int) bool {
		return registry.versions[left].height < registry.versions[right].height
	})
}

func (set setSnapshot) toMap() map[types.ValidatorID]Validator {
	validators := make(map[types.ValidatorID]Validator, len(set.validators))
	for _, validatorInfo := range set.validators {
		validators[validatorInfo.ID] = cloneValidator(validatorInfo)
	}
	return validators
}
