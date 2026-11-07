package app

import (
	"context"
	"encoding/binary"
	"errors"
	"math"

	"github.com/vexo-network/vexo-consensus/economics"
	vexostore "github.com/vexo-network/vexo-consensus/store"
)

// feeMarketNamespace holds the fee market state. The base fee is consensus
// state: every transaction of block N is priced against the value the previous
// block's FinalizeBlock stored, so a node that restarts, replays or validates
// a proposal reads the same price as the node that produced the block.
const feeMarketNamespace = "feemarket"

var (
	feeMarketStateKey = []byte("state")

	ErrInvalidFeeMarketState = errors.New("invalid fee market state")
)

// FeeMarketConfig is the rule that moves the base fees from block to block.
type FeeMarketConfig struct {
	// BlobBaseFee is the initial blob base fee. The initial base fee is
	// AnteConfig.BaseFee.
	BlobBaseFee              uint64
	DynamicBaseFee           bool
	DynamicBlobBaseFee       bool
	TargetGas                uint64
	TargetBlobGas            uint64
	MaxBlobGas               uint64
	BaseFeeChangeDenominator uint64
	BlobFeeChangeDenominator uint64
	MinBaseFee               uint64
	MaxBaseFee               uint64
	MinBlobBaseFee           uint64
	MaxBlobBaseFee           uint64
}

type feeMarketState struct {
	baseFee     uint64
	blobBaseFee uint64
}

// CoreNamespaces lists the state namespaces the application runtime owns in
// addition to the modules' own: account sequences and the fee market. They are
// part of the app hash and must be exported with every snapshot and replay.
func CoreNamespaces() []string {
	return []string{"auth", feeMarketNamespace}
}

// loadFeeMarket returns the stored fee market state, or the configured initial
// values for a chain whose first block has not been finalized yet.
func (keeper *AnteKeeper) loadFeeMarket(ctx context.Context, store StateStore) (feeMarketState, error) {
	initial := feeMarketState{baseFee: keeper.config.BaseFee, blobBaseFee: keeper.config.FeeMarket.BlobBaseFee}
	if store == nil {
		return initial, nil
	}
	value, err := store.Get(ctx, feeMarketNamespace, feeMarketStateKey)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return initial, nil
	}
	if err != nil {
		return feeMarketState{}, err
	}
	if len(value) != 16 {
		return feeMarketState{}, ErrInvalidFeeMarketState
	}
	return feeMarketState{
		baseFee:     binary.BigEndian.Uint64(value[:8]),
		blobBaseFee: binary.BigEndian.Uint64(value[8:]),
	}, nil
}

func (keeper *AnteKeeper) storeFeeMarket(ctx context.Context, store StateStore, state feeMarketState) error {
	var encoded [16]byte
	binary.BigEndian.PutUint64(encoded[:8], state.baseFee)
	binary.BigEndian.PutUint64(encoded[8:], state.blobBaseFee)
	return store.Set(ctx, feeMarketNamespace, feeMarketStateKey, encoded[:])
}

// nextFeeMarket is the state the next block is priced against, given what this
// block consumed. It is integer arithmetic over consensus inputs only.
func (keeper *AnteKeeper) nextFeeMarket(current feeMarketState, gasUsed uint64, blobGasUsed uint64) feeMarketState {
	rule := keeper.config.FeeMarket
	next := current
	if rule.DynamicBaseFee {
		next.baseFee = economics.NextBaseFee(economics.BaseFeeParams{
			CurrentBaseFee:    current.baseFee,
			GasUsed:           gasUsed,
			TargetGas:         rule.TargetGas,
			ChangeDenominator: rule.BaseFeeChangeDenominator,
			MinBaseFee:        rule.MinBaseFee,
			MaxBaseFee:        rule.MaxBaseFee,
		})
	}
	if rule.DynamicBlobBaseFee {
		next.blobBaseFee = economics.NextBaseFee(economics.BaseFeeParams{
			CurrentBaseFee:    current.blobBaseFee,
			GasUsed:           blobGasUsed,
			TargetGas:         rule.TargetBlobGas,
			ChangeDenominator: rule.BlobFeeChangeDenominator,
			MinBaseFee:        rule.MinBlobBaseFee,
			MaxBaseFee:        rule.MaxBlobBaseFee,
		})
	}
	return next
}

// addSaturating adds amounts, saturating at the uint64 maximum.
func addSaturating(total uint64, amount uint64) uint64 {
	if total > math.MaxUint64-amount {
		return math.MaxUint64
	}
	return total + amount
}
