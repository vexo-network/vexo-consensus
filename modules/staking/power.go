package staking

import (
	"context"
	"errors"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// DelegatedPower returns the total stake the delegator has bonded to validators.
func DelegatedPower(ctx context.Context, store vexoapp.StateStore, delegator types.Address) (types.VotingPower, error) {
	if store == nil || validateDelegator(delegator) != nil {
		return 0, ErrInvalidStakingTx
	}
	prefixStore, err := prefixStoreOf(store)
	if err != nil {
		return 0, err
	}
	pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, []byte(stakeKeyPrefix+identityKey(delegator)+"/"))
	if err != nil {
		return 0, err
	}
	total := uint64(0)
	for _, pair := range pairs {
		parsedDelegator, _, ok := parseStakeKey(string(pair.Key))
		if !ok || parsedDelegator != types.Address(identityKey(delegator)) {
			continue
		}
		stake, err := decodeUint64(pair.Value)
		if err != nil {
			return 0, err
		}
		if total > ^uint64(0)-stake {
			return 0, ErrStakeOverflow
		}
		total += stake
	}
	return types.VotingPower(total), nil
}

// TotalBondedPower returns the stake bonded across all validators, jailed ones included: the
// delegators of a jailed validator keep their stake and their governance rights.
func TotalBondedPower(ctx context.Context, store vexoapp.StateStore) (types.VotingPower, error) {
	if store == nil {
		return 0, ErrStakingStoreRequired
	}
	states, err := loadValidatorStates(ctx, store)
	if err != nil {
		return 0, err
	}
	total := uint64(0)
	for _, state := range states {
		if total > ^uint64(0)-state.status.power {
			return 0, ErrStakeOverflow
		}
		total += state.status.power
	}
	return types.VotingPower(total), nil
}

// VotingPowerAt returns the voting power the delegator carries in a vote whose power is snapshotted
// at snapshotHeight: the delegator's bonded stake, unless any of it was added at or after
// snapshotHeight. Stake bought after a proposal exists therefore cannot vote on it, which stops a
// flash delegation from swinging a tally.
func VotingPowerAt(ctx context.Context, store vexoapp.StateStore, delegator types.Address, snapshotHeight types.Height) (types.VotingPower, error) {
	power, err := DelegatedPower(ctx, store, delegator)
	if err != nil || power == 0 {
		return 0, err
	}
	raw, err := store.Get(ctx, ModuleName, delegationHeightKey(delegator))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return power, nil
	}
	if err != nil {
		return 0, err
	}
	lastIncrease, err := decodeUint64(raw)
	if err != nil {
		return 0, err
	}
	if types.Height(lastIncrease) >= snapshotHeight {
		return 0, nil
	}
	return power, nil
}
