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
	for _, update := range updates {
		if update.ID == "" {
			update.ID = types.ValidatorID(update.Address)
		}
		if update.Address == "" {
			update.Address = types.Address(update.ID)
		}
		if update.ID == "" {
			return nil, nil, ErrMissingCandidateID
		}
		if update.VotingPower == 0 {
			if _, found := validators[update.ID]; !found {
				return nil, nil, ErrValidatorNotFound
			}
			delete(validators, update.ID)
			continue
		}
		if validatorInfo, found := validators[update.ID]; found {
			validatorInfo.VotingPower = update.VotingPower
			if update.Stake > 0 {
				validatorInfo.Stake = update.Stake
			}
			if len(update.PublicKey) > 0 {
				validatorInfo.PublicKey = append(types.PublicKey(nil), update.PublicKey...)
			}
			if update.Metadata != nil {
				validatorInfo.Metadata = cloneMetadata(update.Metadata)
			}
			validators[update.ID] = validatorInfo
			continue
		}
		stake := update.Stake
		if stake == 0 {
			stake = uint64(update.VotingPower)
		}
		candidate := Candidate{
			Address:   update.Address,
			PublicKey: update.PublicKey,
			Stake:     stake,
			Metadata:  update.Metadata,
		}
		if registry.policy != nil {
			if err := registry.policy.CanJoin(ctx, candidate, newSetSnapshot(sortedValidatorMap(validators))); err != nil {
				return nil, nil, err
			}
		}
		validatorInfo := Validator{
			ID:          update.ID,
			Address:     update.Address,
			PublicKey:   append(types.PublicKey(nil), update.PublicKey...),
			VotingPower: update.VotingPower,
			Stake:       stake,
			Metadata:    cloneMetadata(update.Metadata),
		}
		if validatorInfo.VotingPower == 0 {
			return nil, nil, ErrZeroVotingPower
		}
		validators[update.ID] = validatorInfo
	}
	set := newSetSnapshot(sortedValidatorMap(validators))
	writes, err := registry.snapshotWrites(ctx, height, set.List())
	if err != nil {
		return nil, nil, err
	}
	registry.stageRotationEvents(height, previous, set, updates)
	eventWrites, err := registry.rotationEventWrites(ctx, registry.pendingEvents[height])
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, eventWrites...)
	return set, writes, nil
}

func (registry *StoreRegistry) CommitStagedValidatorUpdates(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if height == 0 {
		height = registry.effectiveHeight
	}
	registry.setEffectiveHeightLocked(height)
	if len(registry.pendingEvents[height]) > 0 {
		registry.events = append(registry.events, registry.pendingEvents[height]...)
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
	currentHash := current.Hash()
	for _, update := range updates {
		if update.ID == "" {
			update.ID = types.ValidatorID(update.Address)
		}
		if update.ID == "" {
			continue
		}
		currentValidator, currentFound := current.Get(update.ID)
		_, previousFound := previous.Get(update.ID)
		switch {
		case !currentFound && previousFound:
			registry.recordEventLocked(height, RotationEventLeave, update.ID, 0, currentHash)
		case currentFound && !previousFound:
			registry.recordEventLocked(height, RotationEventJoin, update.ID, currentValidator.VotingPower, currentHash)
		case currentFound && previousFound:
			registry.recordEventLocked(height, RotationEventPowerChange, update.ID, currentValidator.VotingPower, currentHash)
		}
	}
	return nil
}

func (registry *StoreRegistry) stageRotationEvents(height types.Height, previous Set, current Set, updates []types.ValidatorUpdate) {
	if registry.pendingEvents == nil {
		registry.pendingEvents = make(map[types.Height][]RotationEvent)
	}
	events := make([]RotationEvent, 0, len(updates))
	currentHash := current.Hash()
	for _, update := range updates {
		if update.ID == "" {
			update.ID = types.ValidatorID(update.Address)
		}
		if update.ID == "" {
			continue
		}
		currentValidator, currentFound := current.Get(update.ID)
		_, previousFound := previous.Get(update.ID)
		switch {
		case !currentFound && previousFound:
			events = append(events, RotationEvent{Height: height, Type: RotationEventLeave, ValidatorID: update.ID, ValidatorSetHash: currentHash})
		case currentFound && !previousFound:
			events = append(events, RotationEvent{Height: height, Type: RotationEventJoin, ValidatorID: update.ID, VotingPower: currentValidator.VotingPower, ValidatorSetHash: currentHash})
		case currentFound && previousFound:
			events = append(events, RotationEvent{Height: height, Type: RotationEventPowerChange, ValidatorID: update.ID, VotingPower: currentValidator.VotingPower, ValidatorSetHash: currentHash})
		}
	}
	registry.pendingEvents[height] = events
}

func (registry *StoreRegistry) RotationEvents() []RotationEvent {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return append([]RotationEvent(nil), registry.events...)
}

func (registry *StoreRegistry) recordEvent(ctx context.Context, height types.Height, eventType RotationEventType, validatorID types.ValidatorID, power types.VotingPower) {
	set, err := registry.ValidatorSet(ctx, height)
	if err != nil {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.recordEventLocked(height, eventType, validatorID, power, set.Hash())
}

func (registry *StoreRegistry) recordEventLocked(height types.Height, eventType RotationEventType, validatorID types.ValidatorID, power types.VotingPower, validatorSetHash types.Hash) {
	registry.events = append(registry.events, RotationEvent{
		Height:           height,
		Type:             eventType,
		ValidatorID:      validatorID,
		VotingPower:      power,
		ValidatorSetHash: validatorSetHash,
	})
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
