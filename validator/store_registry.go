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
	events          []RotationEvent
	pendingEvents   map[types.Height][]RotationEvent
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
	registry := &StoreRegistry{store: store, policy: policy, effectiveHeight: initialHeight, pendingEvents: make(map[types.Height][]RotationEvent)}
	if _, err := registry.loadLatest(ctx, initialHeight); err == nil {
		events, err := registry.loadRotationEvents(ctx)
		if err != nil {
			return nil, err
		}
		registry.events = events
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
	set := newSetSnapshot(sortedValidatorMap(validators))
	event := RotationEvent{Height: height, Type: RotationEventJoin, ValidatorID: validatorID, VotingPower: validatorInfo.VotingPower, ValidatorSetHash: set.Hash()}
	if err := registry.saveSnapshotWithEvents(ctx, height, set.List(), []RotationEvent{event}); err != nil {
		return Validator{}, err
	}
	registry.events = append(registry.events, event)
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
	set := newSetSnapshot(sortedValidatorMap(validators))
	event := RotationEvent{Height: height, Type: RotationEventLeave, ValidatorID: id, ValidatorSetHash: set.Hash()}
	if err := registry.saveSnapshotWithEvents(ctx, height, set.List(), []RotationEvent{event}); err != nil {
		return err
	}
	registry.events = append(registry.events, event)
	return nil
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
	set := newSetSnapshot(sortedValidatorMap(validators))
	event := RotationEvent{Height: height, Type: RotationEventPowerChange, ValidatorID: id, VotingPower: power, ValidatorSetHash: set.Hash()}
	if err := registry.saveSnapshotWithEvents(ctx, height, set.List(), []RotationEvent{event}); err != nil {
		return err
	}
	registry.events = append(registry.events, event)
	return nil
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
	previous := newSetSnapshot(sortedValidatorMap(validators))
	normalized := normalizeValidatorUpdates(updates)
	if err := registry.applyValidatorUpdates(ctx, validators, normalized); err != nil {
		return nil, nil, err
	}
	set := newSetSnapshot(sortedValidatorMap(validators))
	writes, err := registry.snapshotWrites(ctx, height, set.List())
	if err != nil {
		return nil, nil, err
	}
	registry.stageRotationEvents(height, previous, set, normalized)
	eventWrites, err := registry.rotationEventWrites(ctx, registry.pendingEvents[height])
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, eventWrites...)
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

func (registry *StoreRegistry) CommitStagedValidatorUpdates(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.setEffectiveHeightLocked(height)
	if staged, found := registry.pendingEvents[height]; found {
		registry.events = append(registry.events, staged...)
		delete(registry.pendingEvents, height)
		return nil
	}
	previous := Set(newSetSnapshot(nil))
	if height > 1 {
		if validators, err := registry.validatorsAt(ctx, height-1); err == nil {
			previous = newSetSnapshot(sortedValidatorMap(validators))
		}
	}
	validators, err := registry.validatorsAt(ctx, height)
	if err != nil {
		return err
	}
	current := newSetSnapshot(sortedValidatorMap(validators))
	registry.events = append(registry.events, rotationEvents(height, previous, current, normalizeValidatorUpdates(updates))...)
	return nil
}

func (registry *StoreRegistry) stageRotationEvents(height types.Height, previous Set, current Set, updates []types.ValidatorUpdate) {
	if registry.pendingEvents == nil {
		registry.pendingEvents = make(map[types.Height][]RotationEvent)
	}
	registry.pendingEvents[height] = rotationEvents(height, previous, current, updates)
}

// rotationEvents reports what the updates changed between two sets: a join, a leave, or a voting
// power change for each updated validator once, in order of first appearance.
func rotationEvents(height types.Height, previous Set, current Set, updates []types.ValidatorUpdate) []RotationEvent {
	events := make([]RotationEvent, 0, len(updates))
	currentHash := current.Hash()
	seen := make(map[types.ValidatorID]struct{}, len(updates))
	for _, update := range updates {
		if _, duplicate := seen[update.ID]; duplicate {
			continue
		}
		seen[update.ID] = struct{}{}
		currentValidator, currentFound := current.Get(update.ID)
		previousValidator, previousFound := previous.Get(update.ID)
		switch {
		case !currentFound && previousFound:
			events = append(events, RotationEvent{Height: height, Type: RotationEventLeave, ValidatorID: update.ID, ValidatorSetHash: currentHash})
		case currentFound && !previousFound:
			events = append(events, RotationEvent{Height: height, Type: RotationEventJoin, ValidatorID: update.ID, VotingPower: currentValidator.VotingPower, ValidatorSetHash: currentHash})
		case currentFound && previousFound && currentValidator.VotingPower != previousValidator.VotingPower:
			events = append(events, RotationEvent{Height: height, Type: RotationEventPowerChange, ValidatorID: update.ID, VotingPower: currentValidator.VotingPower, ValidatorSetHash: currentHash})
		}
	}
	return events
}

func (registry *StoreRegistry) RotationEvents() []RotationEvent {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return append([]RotationEvent(nil), registry.events...)
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
	return registry.saveSnapshotWithEvents(ctx, height, validators, nil)
}

func (registry *StoreRegistry) saveSnapshotWithEvents(ctx context.Context, height types.Height, validators []Validator, events []RotationEvent) error {
	writes, err := registry.snapshotWrites(ctx, height, validators)
	if err != nil {
		return err
	}
	eventWrites, err := registry.rotationEventWrites(ctx, events)
	if err != nil {
		return err
	}
	writes = append(writes, eventWrites...)
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

func (registry *StoreRegistry) rotationEventWrites(ctx context.Context, events []RotationEvent) ([]vexostore.KVWrite, error) {
	if len(events) == 0 {
		return nil, nil
	}
	persisted, err := registry.loadRotationEvents(ctx)
	if err != nil {
		return nil, err
	}
	persisted = append(persisted, events...)
	encoded, err := json.Marshal(persisted)
	if err != nil {
		return nil, err
	}
	return []vexostore.KVWrite{{Namespace: validatorRegistryNamespace, Key: []byte("events"), Value: encoded}}, nil
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

func (registry *StoreRegistry) loadRotationEvents(ctx context.Context) ([]RotationEvent, error) {
	encoded, err := registry.store.Get(ctx, validatorRegistryNamespace, []byte("events"))
	if err != nil {
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var events []RotationEvent
	if err := json.Unmarshal(encoded, &events); err != nil {
		return nil, err
	}
	return events, nil
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
