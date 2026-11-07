package staking

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	vexovalidator "github.com/vexo-network/vexo-consensus/validator"
)

// genesisAccountMetadataKey names the genesis metadata entry holding the account that controls a
// validator. Genesis validators carry a validator-operator address that no transaction can be signed
// by, while the account derived from the same key can.
const genesisAccountMetadataKey = "account_address"

// validatorRecord is the registration data of a validator. It is written once, when the validator is
// created or seeded from genesis, and never rewritten: delegations only change stake, so nobody but
// the operator at creation time can ever choose the consensus key.
type validatorRecord struct {
	Operator  types.Address     `json:"operator"`
	Address   types.Address     `json:"address"`
	PublicKey []byte            `json:"public_key"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func loadValidator(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (validatorRecord, bool, error) {
	if err := validateValidatorID(validatorID); err != nil {
		return validatorRecord{}, false, err
	}
	encoded, err := store.Get(ctx, ModuleName, validatorRecordKey(validatorID))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return validatorRecord{}, false, nil
	}
	if err != nil {
		return validatorRecord{}, false, err
	}
	return decodeValidatorRecord(encoded)
}

func decodeValidatorRecord(encoded []byte) (validatorRecord, bool, error) {
	var record validatorRecord
	if err := json.Unmarshal(encoded, &record); err != nil || record.Operator == "" || len(record.PublicKey) == 0 {
		return validatorRecord{}, false, ErrInvalidStakeRecord
	}
	return record, true, nil
}

// validatorStatus is the mutable standing of a validator.
type validatorStatus struct {
	power      uint64
	jailEnd    types.Height
	jailed     bool
	tombstoned bool
}

// inactive reports whether the validator is kept out of the validator set, rewards and proposer
// rotation no matter how much stake backs it.
func (status validatorStatus) inactive() bool {
	return status.jailed || status.tombstoned
}

func loadValidatorStatus(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (validatorStatus, error) {
	power, err := ValidatorPower(ctx, store, validatorID)
	if err != nil {
		return validatorStatus{}, err
	}
	status := validatorStatus{power: power}
	rawJail, err := store.Get(ctx, ModuleName, jailKey(validatorID))
	if err != nil && !errors.Is(err, vexostore.ErrKeyNotFound) {
		return validatorStatus{}, err
	}
	if err == nil {
		end, err := decodeUint64(rawJail)
		if err != nil {
			return validatorStatus{}, err
		}
		status.jailed = true
		status.jailEnd = types.Height(end)
	}
	status.tombstoned, err = Tombstoned(ctx, store, validatorID)
	if err != nil {
		return validatorStatus{}, err
	}
	return status, nil
}

// validatorUpdate is the validator set update that reflects the current staking state of a
// validator: full identity so a validator that left the set can rejoin, and zero voting power while it
// is jailed or tombstoned.
func validatorUpdate(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (types.ValidatorUpdate, error) {
	record, found, err := loadValidator(ctx, store, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	if !found {
		return types.ValidatorUpdate{}, ErrValidatorNotFound
	}
	status, err := loadValidatorStatus(ctx, store, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	update := types.ValidatorUpdate{
		ID:        validatorID,
		Address:   record.Address,
		PublicKey: append(types.PublicKey(nil), record.PublicKey...),
		Stake:     status.power,
		Metadata:  cloneMetadata(record.Metadata),
	}
	if !status.inactive() {
		update.VotingPower = types.VotingPower(status.power)
	}
	return update, nil
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

// validatorState is the full staking state of one validator as found by a prefix scan.
type validatorState struct {
	id            types.ValidatorID
	record        validatorRecord
	hasRecord     bool
	status        validatorStatus
	commissionBPS uint64
}

// loadValidatorStates scans every validator once. The result is ordered by validator id.
func loadValidatorStates(ctx context.Context, store vexoapp.StateStore) ([]validatorState, error) {
	prefixStore, err := prefixStoreOf(store)
	if err != nil {
		return nil, err
	}
	pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, []byte(validatorKeyPrefix))
	if err != nil {
		return nil, err
	}
	states := make(map[types.ValidatorID]*validatorState)
	for _, pair := range pairs {
		validatorID, field, ok := parseValidatorKey(string(pair.Key))
		if !ok {
			continue
		}
		state := states[validatorID]
		if state == nil {
			state = &validatorState{id: validatorID}
			states[validatorID] = state
		}
		switch field {
		case validatorFieldPower:
			if state.status.power, err = decodeUint64(pair.Value); err != nil {
				return nil, err
			}
		case validatorFieldRecord:
			if state.record, state.hasRecord, err = decodeValidatorRecord(pair.Value); err != nil {
				return nil, err
			}
		case validatorFieldJail:
			end, err := decodeUint64(pair.Value)
			if err != nil {
				return nil, err
			}
			state.status.jailed = true
			state.status.jailEnd = types.Height(end)
		case validatorFieldTombstone:
			state.status.tombstoned = true
		case validatorFieldCommission:
			if state.commissionBPS, err = decodeUint64(pair.Value); err != nil {
				return nil, err
			}
		}
	}
	ids := make([]types.ValidatorID, 0, len(states))
	for validatorID := range states {
		ids = append(ids, validatorID)
	}
	sort.Slice(ids, func(left int, right int) bool { return ids[left] < ids[right] })
	ordered := make([]validatorState, 0, len(ids))
	for _, validatorID := range ids {
		ordered = append(ordered, *states[validatorID])
	}
	return ordered, nil
}

// activeValidatorSet is the set of registered validators that currently carry voting power. It is the
// set the admission policy is evaluated against.
func activeValidatorSet(ctx context.Context, store vexoapp.StateStore, exclude types.ValidatorID) (vexovalidator.Set, error) {
	states, err := loadValidatorStates(ctx, store)
	if err != nil {
		return nil, err
	}
	validators := make([]vexovalidator.Validator, 0, len(states))
	for _, state := range states {
		if !state.hasRecord || state.id == exclude || state.status.inactive() || state.status.power == 0 {
			continue
		}
		validators = append(validators, vexovalidator.Validator{
			ID:          state.id,
			Address:     state.record.Address,
			PublicKey:   append(types.PublicKey(nil), state.record.PublicKey...),
			VotingPower: types.VotingPower(state.status.power),
			Stake:       state.status.power,
			Metadata:    cloneMetadata(state.record.Metadata),
		})
	}
	return vexovalidator.NewSet(validators), nil
}

// admit applies the validator admission policy before a validator enters the set. Running it in the
// transaction means a candidate the registry would refuse fails visibly instead of being dropped
// when the block's validator updates are applied.
func (module *Module) admit(ctx context.Context, store vexoapp.StateStore, candidate vexovalidator.Candidate, exclude types.ValidatorID) error {
	if module.admission == nil {
		return nil
	}
	set, err := activeValidatorSet(ctx, store, exclude)
	if err != nil {
		return err
	}
	return module.admission.CanJoin(ctx, candidate, set)
}

// touch marks a validator as changed in the current block, so that EndBlock reports its standing.
func (module *Module) touch(validatorID types.ValidatorID) {
	if module.touched == nil {
		module.touched = make(map[types.ValidatorID]struct{})
	}
	module.touched[validatorID] = struct{}{}
}

func consensusKeyMetadata(proof string) (map[string]string, error) {
	if proof == "" {
		return nil, nil
	}
	decoded, err := base64.RawStdEncoding.DecodeString(proof)
	if err != nil || len(decoded) == 0 {
		return nil, fmt.Errorf("%w: proof of possession is not base64", ErrInvalidConsensusKey)
	}
	return map[string]string{vexocrypto.BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString(decoded)}, nil
}

// createValidator registers a new validator whose operator self-delegates amount. It is the only way
// a consensus key enters staking state after genesis.
func (module *Module) createValidator(ctx vexoapp.Context, tx types.Tx, operator types.Address, validatorID types.ValidatorID, amount uint64, publicKey []byte, proof string) error {
	goCtx := ctx.GoContext()
	store := ctx.Store
	if err := authorize(tx, operator); err != nil {
		return err
	}
	_, exists, err := loadValidator(goCtx, store, validatorID)
	if err != nil {
		return err
	}
	if exists {
		return ErrValidatorExists
	}
	metadata, err := consensusKeyMetadata(proof)
	if err != nil {
		return err
	}
	if err := vexocrypto.ValidateConsensusPublicKey(module.cryptoBackend, publicKey, metadata); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConsensusKey, err)
	}
	if _, err := store.Get(goCtx, ModuleName, consensusKeyIndexKey(publicKey)); err == nil {
		return ErrDuplicateConsensusKey
	} else if !errors.Is(err, vexostore.ErrKeyNotFound) {
		return err
	}
	if err := module.admit(goCtx, store, vexovalidator.Candidate{
		Address:   operator,
		PublicKey: publicKey,
		Stake:     amount,
		Metadata:  metadata,
	}, ""); err != nil {
		return err
	}
	writes, err := delegationWrites(goCtx, store, ctx.Height, operator, validatorID, amount)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(validatorRecord{
		Operator:  operator,
		Address:   operator,
		PublicKey: append([]byte(nil), publicKey...),
		Metadata:  metadata,
	})
	if err != nil {
		return err
	}
	writes = append(writes,
		kvbatch.KVWrite{Namespace: ModuleName, Key: validatorRecordKey(validatorID), Value: encoded},
		kvbatch.KVWrite{Namespace: ModuleName, Key: consensusKeyIndexKey(publicKey), Value: []byte(validatorID)},
	)
	if err := applyAtomicWrites(goCtx, store, writes); err != nil {
		return err
	}
	module.touch(validatorID)
	return nil
}

// unjail lets the operator of an expired jail bring the validator back into the set.
func (module *Module) unjail(ctx vexoapp.Context, tx types.Tx, validatorID types.ValidatorID) error {
	goCtx := ctx.GoContext()
	store := ctx.Store
	record, found, err := loadValidator(goCtx, store, validatorID)
	if err != nil {
		return err
	}
	if !found {
		return ErrValidatorNotFound
	}
	if err := authorize(tx, record.Operator); err != nil {
		return err
	}
	status, err := loadValidatorStatus(goCtx, store, validatorID)
	if err != nil {
		return err
	}
	if status.tombstoned {
		return ErrValidatorTombstoned
	}
	if !status.jailed {
		return ErrValidatorNotJailed
	}
	if ctx.Height < status.jailEnd {
		return ErrJailNotExpired
	}
	if status.power == 0 {
		return ErrNoBondedStake
	}
	if err := module.admit(goCtx, store, vexovalidator.Candidate{
		Address:   record.Address,
		PublicKey: record.PublicKey,
		Stake:     status.power,
		Metadata:  record.Metadata,
	}, validatorID); err != nil {
		return err
	}
	if err := applyAtomicWrites(goCtx, store, []kvbatch.KVWrite{{Namespace: ModuleName, Key: jailKey(validatorID), Delete: true}}); err != nil {
		return err
	}
	module.touch(validatorID)
	return nil
}

// setCommission changes the commission of a validator; only its operator may do so.
func (module *Module) setCommission(ctx vexoapp.Context, tx types.Tx, validatorID types.ValidatorID, commissionBPS uint64) error {
	if commissionBPS > module.maxCommissionBPS {
		return ErrInvalidCommission
	}
	record, found, err := loadValidator(ctx.GoContext(), ctx.Store, validatorID)
	if err != nil {
		return err
	}
	if !found {
		return ErrValidatorNotFound
	}
	if err := authorize(tx, record.Operator); err != nil {
		return err
	}
	return setUint64(ctx.GoContext(), ctx.Store, ModuleName, commissionKey(validatorID), commissionBPS)
}

// genesisStake is an explicit stake entry of the genesis app state.
type genesisStake struct {
	delegator   types.Address
	validatorID types.ValidatorID
	amount      uint64
}

// genesisDelegation identifies a delegation by the canonical identity of its delegator.
type genesisDelegation struct {
	delegator   string
	validatorID types.ValidatorID
}

// genesisOperator is the account that controls a genesis validator.
func genesisOperator(update types.ValidatorUpdate, validatorID types.ValidatorID) types.Address {
	if account := update.Metadata[genesisAccountMetadataKey]; account != "" {
		return types.Address(account)
	}
	if update.Address != "" {
		return update.Address
	}
	return types.Address(validatorID)
}

// seedGenesisValidators writes the staking state of the genesis validator set: registration, power
// and the stake that backs the power. Without it the first delegation to a genesis validator would
// replace its power by the delegated amount. Explicit stakes name other delegators of a genesis
// validator; the operator holds the rest of the validator's power.
func seedGenesisValidators(ctx context.Context, store vexoapp.StateStore, validators []types.ValidatorUpdate, stakes []genesisStake) error {
	known := make(map[types.ValidatorID]struct{}, len(validators))
	keys := make(map[string]types.ValidatorID, len(validators))
	explicit := make(map[types.ValidatorID]uint64, len(stakes))
	writes := make([]kvbatch.KVWrite, 0, len(validators)*4+len(stakes))
	for _, update := range validators {
		validatorID := update.ID
		if validatorID == "" {
			validatorID = types.ValidatorID(update.Address)
		}
		if err := validateValidatorID(validatorID); err != nil {
			return fmt.Errorf("genesis validator %q: %w", validatorID, err)
		}
		if _, duplicate := known[validatorID]; duplicate {
			return fmt.Errorf("genesis validator %q: %w", validatorID, ErrValidatorExists)
		}
		if len(update.PublicKey) == 0 {
			return fmt.Errorf("genesis validator %q: %w", validatorID, ErrMissingValidatorKey)
		}
		if previous, duplicate := keys[string(update.PublicKey)]; duplicate {
			return fmt.Errorf("genesis validator %q shares its key with %q: %w", validatorID, previous, ErrDuplicateConsensusKey)
		}
		if update.VotingPower == 0 {
			return fmt.Errorf("genesis validator %q: %w", validatorID, ErrNoBondedStake)
		}
		known[validatorID] = struct{}{}
		keys[string(update.PublicKey)] = validatorID
	}
	sort.Slice(stakes, func(left int, right int) bool {
		if stakes[left].validatorID != stakes[right].validatorID {
			return stakes[left].validatorID < stakes[right].validatorID
		}
		return stakes[left].delegator < stakes[right].delegator
	})
	// A delegation is one canonical delegator and one validator, whatever the spelling of the address.
	// Two entries for one delegation would be written to the same key, the second replacing the first,
	// while both count toward the validator's power: the power would exceed the stake that backs it.
	explicitStakes := make(map[genesisDelegation]uint64, len(stakes))
	for _, stake := range stakes {
		if _, found := known[stake.validatorID]; !found {
			return fmt.Errorf("genesis stake for unknown validator %q: %w", stake.validatorID, ErrValidatorNotFound)
		}
		delegation := genesisDelegation{delegator: identityKey(stake.delegator), validatorID: stake.validatorID}
		if _, duplicate := explicitStakes[delegation]; duplicate {
			return fmt.Errorf("genesis stake of %q in validator %q: %w", delegation.delegator, stake.validatorID, ErrDuplicateGenesisStake)
		}
		if explicit[stake.validatorID] > ^uint64(0)-stake.amount {
			return ErrStakeOverflow
		}
		explicit[stake.validatorID] += stake.amount
		explicitStakes[delegation] = stake.amount
		writes = append(writes, stakeWrites(stake.delegator, stake.validatorID, stake.amount)...)
	}
	for _, update := range validators {
		validatorID := update.ID
		if validatorID == "" {
			validatorID = types.ValidatorID(update.Address)
		}
		power := uint64(update.VotingPower)
		if explicit[validatorID] > power {
			return fmt.Errorf("genesis stakes of validator %q exceed its power: %w", validatorID, ErrInvalidStakeRecord)
		}
		operator := genesisOperator(update, validatorID)
		if err := validateDelegator(operator); err != nil {
			return fmt.Errorf("genesis validator %q operator: %w", validatorID, err)
		}
		address := update.Address
		if address == "" {
			address = types.Address(validatorID)
		}
		encoded, err := json.Marshal(validatorRecord{
			Operator:  operator,
			Address:   address,
			PublicKey: append([]byte(nil), update.PublicKey...),
			Metadata:  cloneMetadata(update.Metadata),
		})
		if err != nil {
			return err
		}
		writes = append(writes,
			kvbatch.KVWrite{Namespace: ModuleName, Key: validatorRecordKey(validatorID), Value: encoded},
			kvbatch.KVWrite{Namespace: ModuleName, Key: consensusKeyIndexKey(update.PublicKey), Value: []byte(validatorID)},
			kvbatch.KVWrite{Namespace: ModuleName, Key: validatorPowerKey(validatorID), Value: encodeUint64(power)},
		)
		if remainder := power - explicit[validatorID]; remainder > 0 {
			selfStake := remainder + explicitStakes[genesisDelegation{delegator: identityKey(operator), validatorID: validatorID}]
			writes = append(writes, stakeWrites(operator, validatorID, selfStake)...)
		}
	}
	return applyAtomicWrites(ctx, store, dedupeWrites(writes))
}

// dedupeWrites keeps the last write of every key, preserving order, so a self-stake computed for the
// operator replaces an explicit entry of the same delegator.
func dedupeWrites(writes []kvbatch.KVWrite) []kvbatch.KVWrite {
	last := make(map[string]int, len(writes))
	for index, write := range writes {
		last[write.Namespace+"\x00"+string(write.Key)] = index
	}
	deduped := make([]kvbatch.KVWrite, 0, len(last))
	for index, write := range writes {
		if last[write.Namespace+"\x00"+string(write.Key)] == index {
			deduped = append(deduped, write)
		}
	}
	return deduped
}
