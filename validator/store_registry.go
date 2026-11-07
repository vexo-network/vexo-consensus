package validator

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrValidatorSetNotFound = errors.New("validator set not found")

const validatorRegistryNamespace = "validator_registry"

// RegistryNamespace is the state namespace that holds the validator-set snapshots of the store
// registry. Code that reads past validator sets from state, such as the slashing module, depends on
// it, so a replay must carry it into its own store.
const RegistryNamespace = validatorRegistryNamespace

type KVStore interface {
	Set(ctx context.Context, namespace string, key []byte, value []byte) error
	Get(ctx context.Context, namespace string, key []byte) ([]byte, error)
	Delete(ctx context.Context, namespace string, key []byte) error
}

type StoreRegistry struct {
	mu              sync.Mutex
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
	} else if !errors.Is(err, ErrValidatorSetNotFound) {
		return nil, err
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
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.setEffectiveHeightLocked(height)
}

func (registry *StoreRegistry) setEffectiveHeightLocked(height types.Height) {
	if height == 0 {
		height = 1
	}
	registry.effectiveHeight = height
}

func (registry *StoreRegistry) ValidatorSet(ctx context.Context, height types.Height) (Set, error) {
	registry.mu.Lock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.mu.Unlock()
	document, err := registry.loadLatest(ctx, height)
	if err != nil {
		return nil, err
	}
	return newSetSnapshot(sortedValidators(document.Validators)), nil
}

func (registry *StoreRegistry) ApplyJoin(ctx context.Context, candidate Candidate) (Validator, error) {
	registry.mu.Lock()
	height := registry.effectiveHeight
	registry.mu.Unlock()
	return registry.ApplyJoinAt(ctx, height, candidate)
}

func (registry *StoreRegistry) ApplyJoinAt(ctx context.Context, height types.Height, candidate Candidate) (Validator, error) {
	if candidate.Address == "" {
		return Validator{}, ErrMissingCandidateID
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.setEffectiveHeightLocked(height)
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
	if err := registry.saveSnapshot(ctx, height, sortedValidatorMap(validators)); err != nil {
		return Validator{}, err
	}
	return validatorInfo, nil
}

func (registry *StoreRegistry) ApplyLeave(ctx context.Context, id types.ValidatorID) error {
	registry.mu.Lock()
	height := registry.effectiveHeight
	registry.mu.Unlock()
	return registry.ApplyLeaveAt(ctx, height, id)
}

func (registry *StoreRegistry) ApplyLeaveAt(ctx context.Context, height types.Height, id types.ValidatorID) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.setEffectiveHeightLocked(height)
	validators, err := registry.currentValidators(ctx)
	if err != nil {
		return err
	}
	if _, found := validators[id]; !found {
		return ErrValidatorNotFound
	}
	delete(validators, id)
	return registry.saveSnapshot(ctx, height, sortedValidatorMap(validators))
}

func (registry *StoreRegistry) UpdateVotingPower(ctx context.Context, id types.ValidatorID, power types.VotingPower) error {
	registry.mu.Lock()
	height := registry.effectiveHeight
	registry.mu.Unlock()
	return registry.UpdateVotingPowerAt(ctx, height, id, power)
}

func (registry *StoreRegistry) UpdateVotingPowerAt(ctx context.Context, height types.Height, id types.ValidatorID, power types.VotingPower) error {
	if power == 0 {
		return ErrZeroVotingPower
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.setEffectiveHeightLocked(height)
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
	return registry.saveSnapshot(ctx, height, sortedValidatorMap(validators))
}

// StageValidatorUpdatesAt computes the validator set that results from applying updates on top of
// the set in effect at height and returns the writes that persist it. Staging never rejects an
// update because of its content: block execution runs after a quorum certified the block, so an
// update that cannot be applied is skipped deterministically instead. Only storage failures and a
// cancelled context are returned as errors.
//
// Updates apply in two passes. Removals (voting power zero) of known validators run first, so the
// slots they free are visible to admission, and then upserts run in order: a known validator takes
// the new power, stake, key (when the update carries one) and metadata entries; an unknown validator
// joins only when the update carries a public key and the admission policy accepts the candidate.
// A removal that would leave the set empty is ignored.
func (registry *StoreRegistry) StageValidatorUpdatesAt(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) (Set, []vexostore.KVWrite, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	validators, err := registry.validatorsAt(ctx, height)
	if err != nil {
		return nil, nil, err
	}
	if err := registry.applyValidatorUpdates(ctx, validators, normalizeValidatorUpdates(updates)); err != nil {
		return nil, nil, err
	}
	set := newSetSnapshot(sortedValidatorMap(validators))
	writes, err := registry.snapshotWrites(ctx, height, set.List())
	if err != nil {
		return nil, nil, err
	}
	return set, writes, nil
}

func normalizeValidatorUpdates(updates []types.ValidatorUpdate) []types.ValidatorUpdate {
	normalized := make([]types.ValidatorUpdate, 0, len(updates))
	for _, update := range updates {
		if update.ID == "" {
			update.ID = types.ValidatorID(update.Address)
		}
		if update.Address == "" {
			update.Address = types.Address(update.ID)
		}
		if update.ID == "" {
			continue
		}
		normalized = append(normalized, update)
	}
	return normalized
}

func (registry *StoreRegistry) applyValidatorUpdates(ctx context.Context, validators map[types.ValidatorID]Validator, updates []types.ValidatorUpdate) error {
	removed := make(map[types.ValidatorID]Validator)
	for _, update := range updates {
		if update.VotingPower != 0 {
			continue
		}
		if validatorInfo, found := validators[update.ID]; found {
			removed[update.ID] = validatorInfo
			delete(validators, update.ID)
		}
	}
	for _, update := range updates {
		if update.VotingPower == 0 {
			continue
		}
		if validatorInfo, found := validators[update.ID]; found {
			validators[update.ID] = updatedValidator(validatorInfo, update)
			continue
		}
		if len(update.PublicKey) == 0 {
			continue
		}
		stake := update.Stake
		if stake == 0 {
			stake = uint64(update.VotingPower)
		}
		if registry.policy != nil {
			candidate := Candidate{
				Address:   update.Address,
				PublicKey: update.PublicKey,
				Stake:     stake,
				Metadata:  update.Metadata,
			}
			if err := registry.policy.CanJoin(ctx, candidate, newSetSnapshot(sortedValidatorMap(validators))); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ctxErr
				}
				continue
			}
		}
		validators[update.ID] = Validator{
			ID:          update.ID,
			Address:     update.Address,
			PublicKey:   append(types.PublicKey(nil), update.PublicKey...),
			VotingPower: update.VotingPower,
			Stake:       stake,
			Metadata:    cloneMetadata(update.Metadata),
		}
	}
	if len(validators) == 0 {
		for id, validatorInfo := range removed {
			validators[id] = validatorInfo
		}
	}
	return nil
}

func updatedValidator(validatorInfo Validator, update types.ValidatorUpdate) Validator {
	validatorInfo.VotingPower = update.VotingPower
	if update.Stake > 0 {
		validatorInfo.Stake = update.Stake
	}
	if len(update.PublicKey) > 0 {
		validatorInfo.PublicKey = append(types.PublicKey(nil), update.PublicKey...)
	}
	if len(update.Metadata) > 0 {
		merged := cloneMetadata(validatorInfo.Metadata)
		if merged == nil {
			merged = make(map[string]string, len(update.Metadata))
		}
		for key, value := range update.Metadata {
			merged[key] = value
		}
		validatorInfo.Metadata = merged
	}
	return validatorInfo
}

// CommitStagedValidatorUpdates makes height, the registry height of the set staged by
// StageValidatorUpdatesAt, the effective height once the writes it returned are committed.
func (registry *StoreRegistry) CommitStagedValidatorUpdates(height types.Height) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.setEffectiveHeightLocked(height)
}

func (registry *StoreRegistry) currentValidators(ctx context.Context) (map[types.ValidatorID]Validator, error) {
	return registry.validatorsAt(ctx, registry.effectiveHeight)
}

func (registry *StoreRegistry) validatorsAt(ctx context.Context, height types.Height) (map[types.ValidatorID]Validator, error) {
	document, err := registry.loadLatest(ctx, height)
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
	writes, err := registry.snapshotWrites(ctx, height, validators)
	if err != nil {
		return err
	}
	if batchStore, ok := registry.store.(vexostore.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := registry.store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func (registry *StoreRegistry) snapshotWrites(ctx context.Context, height types.Height, validators []Validator) ([]vexostore.KVWrite, error) {
	document := validatorSetDocument{Height: height, Validators: sortedValidators(validators)}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	heights, err := registry.loadHeights(ctx)
	if errors.Is(err, ErrValidatorSetNotFound) {
		heights = nil
	} else if err != nil {
		return nil, err
	}
	if !containsHeight(heights, height) {
		heights = append(heights, height)
		sort.Slice(heights, func(left int, right int) bool { return heights[left] < heights[right] })
	}
	encodedHeights, err := json.Marshal(heights)
	if err != nil {
		return nil, err
	}
	return []vexostore.KVWrite{
		{Namespace: validatorRegistryNamespace, Key: validatorSetKey(height), Value: encoded},
		{Namespace: validatorRegistryNamespace, Key: []byte("heights"), Value: encodedHeights},
	}, nil
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
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return nil, ErrValidatorSetNotFound
		}
		return nil, err
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

// LoadValidatorSet reads the persisted validator set in effect at height. Unlike NewStoreRegistry it
// never creates registry state: it returns ErrValidatorSetNotFound when no set was persisted yet.
func LoadValidatorSet(ctx context.Context, store KVStore, height types.Height) (Set, error) {
	if store == nil {
		return nil, errors.New("validator registry store is required")
	}
	registry := &StoreRegistry{store: store}
	document, err := registry.loadLatest(ctx, height)
	if err != nil {
		return nil, err
	}
	return newSetSnapshot(sortedValidators(document.Validators)), nil
}
