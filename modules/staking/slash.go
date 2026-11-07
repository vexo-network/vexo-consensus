package staking

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/bits"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// FractionDenominatorBPS is the denominator of slash fractions: 10000 basis points slash everything.
const FractionDenominatorBPS uint64 = commissionDenominatorBPS

const maxOffenceKeyLength = 1024

// remainingAfterSlash returns amount minus its slashed share, rounding the remainder down so the
// slash rounds up. The multiplication is 128 bit wide, so no amount overflows.
func remainingAfterSlash(amount uint64, fractionBPS uint64) uint64 {
	high, low := bits.Mul64(amount, FractionDenominatorBPS-fractionBPS)
	remaining, _ := bits.Div64(high, low, FractionDenominatorBPS)
	return remaining
}

// SlashValidator punishes a validator for the offence identified by offenceKey. It is idempotent per
// offence key: replaying an offence changes nothing and reports applied == false, while the returned
// update still describes the validator's current standing, so a caller that crashed before feeding
// the first update to the validator set can feed it again.
//
// The slash takes fractionBPS basis points of every delegation to the validator and of every
// unbonding entry created at or after infractionHeight, using integer math only. The validator's
// power becomes the sum of the stake that remains, so power is never backed by less stake than it
// claims. fractionBPS == 10000 tombstones the validator.
//
// A jailUntil above currentHeight jails the validator until that height. A jail never ends earlier
// than an existing one. A jailed validator is reported with zero voting power, stays out of the
// validator set, proposer rotation and reward distribution, and can only return through an unjail
// transaction of its operator once the jail has expired.
func SlashValidator(ctx context.Context, store vexoapp.StateStore, offenceKey string, validatorID types.ValidatorID, infractionHeight types.Height, currentHeight types.Height, fractionBPS uint64, jailUntil types.Height) (types.ValidatorUpdate, bool, error) {
	if store == nil {
		return types.ValidatorUpdate{}, false, ErrStakingStoreRequired
	}
	if offenceKey == "" || len(offenceKey) > maxOffenceKeyLength || fractionBPS > FractionDenominatorBPS {
		return types.ValidatorUpdate{}, false, ErrInvalidSlash
	}
	if err := validateValidatorID(validatorID); err != nil {
		return types.ValidatorUpdate{}, false, err
	}
	if _, found, err := loadValidator(ctx, store, validatorID); err != nil {
		return types.ValidatorUpdate{}, false, err
	} else if !found {
		return types.ValidatorUpdate{}, false, ErrValidatorNotFound
	}
	marker, err := store.Get(ctx, ModuleName, slashMarkerKey(offenceKey))
	if err == nil {
		if !bytes.Equal(marker, []byte(validatorID)) {
			return types.ValidatorUpdate{}, false, fmt.Errorf("%w: offence key already used for another validator", ErrInvalidSlash)
		}
		update, err := validatorUpdate(ctx, store, validatorID)
		return update, false, err
	}
	if !errors.Is(err, vexostore.ErrKeyNotFound) {
		return types.ValidatorUpdate{}, false, err
	}
	status, err := loadValidatorStatus(ctx, store, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, false, err
	}
	prefixStore, err := prefixStoreOf(store)
	if err != nil {
		return types.ValidatorUpdate{}, false, err
	}
	writes := []kvbatch.KVWrite{{Namespace: ModuleName, Key: slashMarkerKey(offenceKey), Value: []byte(validatorID)}}

	// Only the delegations and unbonding entries of this validator are read: the indexes are keyed by
	// validator, so the cost of a slash follows the validator's own stake and not the size of the chain.
	stakes, err := prefixStore.ExportPrefix(ctx, ModuleName, validatorStakePrefix(validatorID))
	if err != nil {
		return types.ValidatorUpdate{}, false, err
	}
	remainingPower := uint64(0)
	for _, pair := range stakes {
		indexed, delegator, ok := parseValidatorStakeKey(string(pair.Key))
		if !ok || indexed != validatorID {
			return types.ValidatorUpdate{}, false, ErrInvalidStakeRecord
		}
		stake, err := decodeUint64(pair.Value)
		if err != nil {
			return types.ValidatorUpdate{}, false, err
		}
		remaining := remainingAfterSlash(stake, fractionBPS)
		if remainingPower > ^uint64(0)-remaining {
			return types.ValidatorUpdate{}, false, ErrStakeOverflow
		}
		remainingPower += remaining
		if remaining == stake {
			continue
		}
		writes = append(writes, stakeWrites(delegator, validatorID, remaining)...)
	}
	writes = append(writes, kvbatch.KVWrite{Namespace: ModuleName, Key: validatorPowerKey(validatorID), Value: encodeUint64(remainingPower)})

	unbonding, err := prefixStore.ExportPrefix(ctx, ModuleName, unbondingValidatorPrefix(validatorID))
	if err != nil {
		return types.ValidatorUpdate{}, false, err
	}
	for _, pair := range unbonding {
		delegator, entryValidator, created, ok := parseUnbondingKey(string(pair.Key))
		if !ok || entryValidator != validatorID {
			return types.ValidatorUpdate{}, false, ErrInvalidStakeRecord
		}
		if created < infractionHeight {
			continue
		}
		entry, err := decodeUnbondingEntry(pair.Value)
		if err != nil {
			return types.ValidatorUpdate{}, false, err
		}
		remaining := remainingAfterSlash(entry.Amount, fractionBPS)
		if remaining == entry.Amount {
			continue
		}
		write := kvbatch.KVWrite{Namespace: ModuleName, Key: unbondingEntryKey(delegator, validatorID, created)}
		if remaining == 0 {
			write.Delete = true
		} else {
			entry.Amount = remaining
			write.Value = encodeUnbondingEntry(entry)
		}
		writes = append(writes, write)
	}

	jailEnd := status.jailEnd
	jailed := status.jailed
	if jailUntil > currentHeight {
		jailed = true
		if jailUntil > jailEnd {
			jailEnd = jailUntil
		}
	}
	if jailed && (!status.jailed || jailEnd != status.jailEnd) {
		writes = append(writes, kvbatch.KVWrite{Namespace: ModuleName, Key: jailKey(validatorID), Value: encodeUint64(uint64(jailEnd))})
	}
	if fractionBPS == FractionDenominatorBPS && !status.tombstoned {
		writes = append(writes, kvbatch.KVWrite{Namespace: ModuleName, Key: tombstoneKey(validatorID), Value: encodeUint64(uint64(infractionHeight))})
	}
	if err := applyAtomicWrites(ctx, store, writes); err != nil {
		return types.ValidatorUpdate{}, false, err
	}
	update, err := validatorUpdate(ctx, store, validatorID)
	return update, true, err
}
