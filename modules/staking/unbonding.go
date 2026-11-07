package staking

import (
	"context"
	"encoding/binary"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	// defaultMaxUnbondingEntries caps the live unbonding entries of one delegator/validator pair.
	// Every undelegate and withdraw reads all of them, so the cap bounds the work of one transaction.
	defaultMaxUnbondingEntries = 32
	// unbondingEntryGasCost is charged for every entry a transaction reads or writes.
	unbondingEntryGasCost uint64 = 5
	unbondingEntrySize           = 24
)

// unbondingEntry is stake leaving a validator. Entries are keyed by the height that created them, so
// undelegating twice in one block merges into one entry and an undelegate writes exactly one entry.
type unbondingEntry struct {
	CreatedHeight types.Height
	ReleaseHeight types.Height
	Amount        uint64
}

func encodeUnbondingEntry(entry unbondingEntry) []byte {
	encoded := make([]byte, unbondingEntrySize)
	binary.BigEndian.PutUint64(encoded[0:8], entry.Amount)
	binary.BigEndian.PutUint64(encoded[8:16], uint64(entry.ReleaseHeight))
	binary.BigEndian.PutUint64(encoded[16:24], uint64(entry.CreatedHeight))
	return encoded
}

func decodeUnbondingEntry(value []byte) (unbondingEntry, error) {
	if len(value) != unbondingEntrySize {
		return unbondingEntry{}, ErrInvalidStakeRecord
	}
	entry := unbondingEntry{
		Amount:        binary.BigEndian.Uint64(value[0:8]),
		ReleaseHeight: types.Height(binary.BigEndian.Uint64(value[8:16])),
		CreatedHeight: types.Height(binary.BigEndian.Uint64(value[16:24])),
	}
	if entry.Amount == 0 || entry.ReleaseHeight == 0 {
		return unbondingEntry{}, ErrInvalidStakeRecord
	}
	return entry, nil
}

func prefixStoreOf(store vexoapp.StateStore) (vexostore.PrefixKVStore, error) {
	prefixStore, ok := store.(vexostore.PrefixKVStore)
	if !ok {
		return nil, ErrStakingSnapshot
	}
	return prefixStore, nil
}

// loadUnbondingEntries returns the live entries of a delegator/validator pair ordered by creation
// height. Release heights share one delay, so that is also release order.
func loadUnbondingEntries(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) ([]unbondingEntry, error) {
	if store == nil || validateDelegator(delegator) != nil || validateValidatorID(validatorID) != nil {
		return nil, ErrInvalidStakingTx
	}
	prefixStore, err := prefixStoreOf(store)
	if err != nil {
		return nil, err
	}
	pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, unbondingPairPrefix(delegator, validatorID))
	if err != nil {
		return nil, err
	}
	entries := make([]unbondingEntry, 0, len(pairs))
	for _, pair := range pairs {
		_, _, created, ok := parseUnbondingKey(string(pair.Key))
		if !ok {
			return nil, ErrInvalidStakeRecord
		}
		entry, err := decodeUnbondingEntry(pair.Value)
		if err != nil {
			return nil, err
		}
		if entry.CreatedHeight != created {
			return nil, ErrInvalidStakeRecord
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func sumUnbondingEntries(entries []unbondingEntry) (uint64, error) {
	total := uint64(0)
	for _, entry := range entries {
		if total > ^uint64(0)-entry.Amount {
			return 0, ErrStakeOverflow
		}
		total += entry.Amount
	}
	return total, nil
}

// UnbondingReleaseHeight returns the earliest release height among the pair's unbonding entries, or
// zero when nothing is unbonding.
func UnbondingReleaseHeight(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (types.Height, error) {
	entries, err := loadUnbondingEntries(ctx, store, delegator, validatorID)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}
	releaseHeight := entries[0].ReleaseHeight
	for _, entry := range entries[1:] {
		if entry.ReleaseHeight < releaseHeight {
			releaseHeight = entry.ReleaseHeight
		}
	}
	return releaseHeight, nil
}

// UnbondingAmount returns the total stake currently unbonding for the pair.
func UnbondingAmount(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (uint64, error) {
	entries, err := loadUnbondingEntries(ctx, store, delegator, validatorID)
	if err != nil {
		return 0, err
	}
	return sumUnbondingEntries(entries)
}
