package validator

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

var ErrValidatorSetNotFound = errors.New("validator set not found")

const validatorRegistryNamespace = "validator_registry"

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
}

type StoreRegistry struct {
	store           KVStore
	policy          AdmissionPolicy
	effectiveHeight types.Height
}

type validatorSetDocument struct {
	Height     types.Height `json:"height"`
	Validators []Validator  `json:"validators"`
}

func NewStoreRegistry(ctx context.Context, store KVStore, policy AdmissionPolicy, initialHeight types.Height, initialValidators []Validator) (*StoreRegistry, error) {
	if store == nil {
		return nil, errors.New("validator registry store is required")
	}
	if initialHeight == 0 {
		initialHeight = 1
	}
	registry := &StoreRegistry{store: store, policy: policy, effectiveHeight: initialHeight}
	if _, err := registry.loadLatest(ctx, initialHeight); err == nil {
		return registry, nil
	}
	for _, validatorInfo := range initialValidators {
		if validatorInfo.VotingPower == 0 {
			return nil, ErrZeroVotingPower
		}
	}
	if err := registry.saveSnapshot(ctx, initialHeight, initialValidators); err != nil {
		return nil, err
	}
	return registry, nil
}

func (registry *StoreRegistry) SetEffectiveHeight(height types.Height) {
	if height == 0 {
		height = 1
	}
	registry.effectiveHeight = height
}

func (registry *StoreRegistry) ValidatorSet(ctx context.Context, height types.Height) (Set, error) {
	if height == 0 {
		height = registry.effectiveHeight
	}
	document, err := registry.loadLatest(ctx, height)
	if err != nil {
		return nil, err
	}
	return newSetSnapshot(sortedValidators(document.Validators)), nil
}

func (registry *StoreRegistry) ApplyJoin(ctx context.Context, candidate Candidate) (Validator, error) {
	if candidate.Address == "" {
		return Validator{}, ErrMissingCandidateID
	}
	validators, err := registry.currentValidators(ctx)
	if err != nil {
		return Validator{}, err
	}
	validatorID := types.ValidatorID(candidate.Address)
	if _, found := validators[validatorID]; found {
		return Validator{}, ErrValidatorExists
	}
	if registry.policy != nil {
		if err := registry.policy.CanJoin(ctx, candidate, newSetSnapshot(sortedValidatorMap(validators))); err != nil {
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
	validators[validatorID] = validatorInfo
	if err := registry.saveSnapshot(ctx, registry.effectiveHeight, sortedValidatorMap(validators)); err != nil {
		return Validator{}, err
	}
	return validatorInfo, nil
}

func (registry *StoreRegistry) ApplyLeave(ctx context.Context, id types.ValidatorID) error {
	validators, err := registry.currentValidators(ctx)
	if err != nil {
		return err
	}
	if _, found := validators[id]; !found {
		return ErrValidatorNotFound
	}
	delete(validators, id)
	return registry.saveSnapshot(ctx, registry.effectiveHeight, sortedValidatorMap(validators))
}

func (registry *StoreRegistry) UpdateVotingPower(ctx context.Context, id types.ValidatorID, power types.VotingPower) error {
	if power == 0 {
		return ErrZeroVotingPower
	}
	validators, err := registry.currentValidators(ctx)
	if err != nil {
		return err
	}
	validatorInfo, found := validators[id]
	if !found {
		return ErrValidatorNotFound
	}
	validatorInfo.VotingPower = power
	validators[id] = validatorInfo
	return registry.saveSnapshot(ctx, registry.effectiveHeight, sortedValidatorMap(validators))
}

func (registry *StoreRegistry) currentValidators(ctx context.Context) (map[types.ValidatorID]Validator, error) {
	document, err := registry.loadLatest(ctx, registry.effectiveHeight)
	if err != nil {
		return nil, err
	}
	validators := make(map[types.ValidatorID]Validator, len(document.Validators))
	for _, validatorInfo := range document.Validators {
		validatorInfo.PublicKey = append(types.PublicKey(nil), validatorInfo.PublicKey...)
		validatorInfo.Metadata = cloneMetadata(validatorInfo.Metadata)
		validators[validatorInfo.ID] = validatorInfo
	}
	return validators, nil
}

func (registry *StoreRegistry) saveSnapshot(ctx context.Context, height types.Height, validators []Validator) error {
	document := validatorSetDocument{Height: height, Validators: sortedValidators(validators)}
	encoded, err := json.Marshal(document)
	if err != nil {
		return err
	}
	if err := registry.store.Set(ctx, validatorRegistryNamespace, validatorSetKey(height), encoded); err != nil {
		return err
	}
	heights, _ := registry.loadHeights(ctx)
	if !containsHeight(heights, height) {
		heights = append(heights, height)
		sort.Slice(heights, func(left int, right int) bool { return heights[left] < heights[right] })
	}
	encodedHeights, err := json.Marshal(heights)
	if err != nil {
		return err
	}
	return registry.store.Set(ctx, validatorRegistryNamespace, []byte("heights"), encodedHeights)
}

func (registry *StoreRegistry) loadLatest(ctx context.Context, height types.Height) (validatorSetDocument, error) {
	heights, err := registry.loadHeights(ctx)
	if err != nil {
		return validatorSetDocument{}, err
	}
	var selected types.Height
	for _, candidate := range heights {
		if candidate <= height && candidate >= selected {
			selected = candidate
		}
	}
	if selected == 0 {
		return validatorSetDocument{}, ErrValidatorSetNotFound
	}
	encoded, err := registry.store.Get(ctx, validatorRegistryNamespace, validatorSetKey(selected))
	if err != nil {
		return validatorSetDocument{}, err
	}
	var document validatorSetDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		return validatorSetDocument{}, err
	}
	return document, nil
}

func (registry *StoreRegistry) loadHeights(ctx context.Context) ([]types.Height, error) {
	encoded, err := registry.store.Get(ctx, validatorRegistryNamespace, []byte("heights"))
	if err != nil {
		return nil, ErrValidatorSetNotFound
	}
	var heights []types.Height
	if err := json.Unmarshal(encoded, &heights); err != nil {
		return nil, err
	}
	return heights, nil
}

func validatorSetKey(height types.Height) []byte {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append([]byte("set/"), buffer[:]...)
}

func sortedValidatorMap(validators map[types.ValidatorID]Validator) []Validator {
	list := make([]Validator, 0, len(validators))
	for _, validatorInfo := range validators {
		list = append(list, validatorInfo)
	}
	return sortedValidators(list)
}

func sortedValidators(validators []Validator) []Validator {
	list := append([]Validator(nil), validators...)
	sort.Slice(list, func(left int, right int) bool {
		return list[left].ID < list[right].ID
	})
	for index := range list {
		list[index].PublicKey = append(types.PublicKey(nil), list[index].PublicKey...)
		list[index].Metadata = cloneMetadata(list[index].Metadata)
	}
	return list
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func containsHeight(heights []types.Height, height types.Height) bool {
	for _, existing := range heights {
		if existing == height {
			return true
		}
	}
	return false
}
