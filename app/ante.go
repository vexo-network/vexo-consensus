package app

import (
	"context"
	"encoding/binary"
	"errors"
	"math"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInsufficientFee        = errors.New("insufficient transaction fee")
	ErrInsufficientFeeBalance = errors.New("insufficient fee balance")
	ErrInvalidGas             = errors.New("invalid transaction gas")
	ErrMissingNonce           = errors.New("missing transaction nonce")
	ErrInvalidNonce           = errors.New("invalid transaction nonce")
)

type AnteConfig struct {
	MinFee        uint64
	BaseFee       uint64
	MinGas        uint64
	MaxGas        uint64
	RequireNonce  bool
	FeeCollector  string
	RequireSigned bool
}

type TxMeta struct {
	Signer   types.Address
	Nonce    uint64
	Fee      uint64
	Gas      uint64
	HasNonce bool
}

type AnteHandler interface {
	CheckTx(ctx Context, tx types.Tx) error
	CheckBlock(ctx Context, txs []types.Tx) error
	BeforeTx(ctx Context, tx types.Tx) error
	AfterTx(ctx Context, tx types.Tx) error
	GasUsed(tx types.Tx) uint64
	GasLimit(tx types.Tx) uint64
	FeePaid(tx types.Tx) uint64
}

type AnteKeeper struct {
	config   AnteConfig
	accounts AccountKeeper
}

func NewAnteKeeper(config AnteConfig) *AnteKeeper {
	return &AnteKeeper{config: config, accounts: NewAccountKeeper()}
}

func ParseTxMeta(tx types.Tx) TxMeta {
	var meta TxMeta
	if signer, found := TxTag(tx, "signer"); found {
		meta.Signer = types.Address(signer)
	}
	if nonce, found := TxUintTag(tx, "nonce"); found {
		meta.Nonce = nonce
		meta.HasNonce = true
	}
	if fee, found := TxAmountTag(tx, "fee"); found {
		meta.Fee = fee
	}
	if gas, found := TxUintTag(tx, "gas"); found {
		meta.Gas = gas
	} else if gasLimit, found := TxUintTag(tx, "gas_limit"); found {
		meta.Gas = gasLimit
	}
	return meta
}

func (keeper AnteKeeper) CheckTx(ctx Context, tx types.Tx) error {
	if err := keeper.verifySignature(ctx, tx); err != nil {
		return err
	}
	meta := ParseTxMeta(tx)
	if err := keeper.validateMeta(meta); err != nil {
		return err
	}
	if ctx.Store == nil || meta.Signer == "" || !meta.HasNonce {
		return nil
	}
	expected, err := keeper.accounts.NextSequence(ctx.GoContext(), ctx.Store, meta.Signer)
	if err != nil {
		return err
	}
	if meta.Nonce != expected {
		return ErrInvalidNonce
	}
	return nil
}

func (keeper AnteKeeper) CheckBlock(ctx Context, txs []types.Tx) error {
	nextBySigner := make(map[types.Address]uint64)
	for _, tx := range txs {
		if err := keeper.verifySignature(ctx, tx); err != nil {
			return err
		}
		meta := ParseTxMeta(tx)
		if err := keeper.validateMeta(meta); err != nil {
			return err
		}
		if ctx.Store == nil || meta.Signer == "" || !meta.HasNonce {
			continue
		}
		expected, found := nextBySigner[meta.Signer]
		if !found {
			var err error
			expected, err = keeper.accounts.NextSequence(ctx.GoContext(), ctx.Store, meta.Signer)
			if err != nil {
				return err
			}
		}
		if meta.Nonce != expected {
			return ErrInvalidNonce
		}
		nextBySigner[meta.Signer] = expected + 1
	}
	return nil
}

func (keeper AnteKeeper) BeforeTx(ctx Context, tx types.Tx) error {
	return keeper.CheckTx(ctx, tx)
}

func (keeper AnteKeeper) AfterTx(ctx Context, tx types.Tx) error {
	meta := ParseTxMeta(tx)
	if ctx.Store == nil {
		return nil
	}
	if err := keeper.collectFee(ctx.GoContext(), ctx.Store, meta); err != nil {
		return err
	}
	if meta.Signer == "" || !meta.HasNonce {
		return nil
	}
	return keeper.accounts.SetSequence(ctx.GoContext(), ctx.Store, meta.Signer, meta.Nonce)
}

func (keeper AnteKeeper) GasUsed(tx types.Tx) uint64 {
	return keeper.GasLimit(tx)
}

func (keeper AnteKeeper) GasLimit(tx types.Tx) uint64 {
	meta := ParseTxMeta(tx)
	if meta.Gas > 0 {
		return meta.Gas
	}
	return uint64(len(tx))
}

func (keeper AnteKeeper) FeePaid(tx types.Tx) uint64 {
	return ParseTxMeta(tx).Fee
}

func (keeper *AnteKeeper) SetBaseFee(baseFee uint64) {
	if keeper == nil {
		return
	}
	keeper.config.BaseFee = baseFee
}

func (keeper AnteKeeper) validateMeta(meta TxMeta) error {
	if keeper.config.MinFee > 0 && meta.Fee < keeper.config.MinFee {
		return ErrInsufficientFee
	}
	if keeper.config.BaseFee > 0 {
		if meta.Gas == 0 {
			return ErrInvalidGas
		}
		requiredFee, ok := multiplyGasPrice(meta.Gas, keeper.config.BaseFee)
		if !ok || meta.Fee < requiredFee {
			return ErrInsufficientFee
		}
	}
	if keeper.config.MinGas > 0 && meta.Gas < keeper.config.MinGas {
		return ErrInvalidGas
	}
	if keeper.config.MaxGas > 0 && meta.Gas > keeper.config.MaxGas {
		return ErrInvalidGas
	}
	if keeper.config.RequireNonce && (meta.Signer == "" || !meta.HasNonce) {
		return ErrMissingNonce
	}
	return nil
}

func multiplyGasPrice(gas uint64, baseFee uint64) (uint64, bool) {
	if gas == 0 || baseFee == 0 {
		return 0, true
	}
	if gas > math.MaxUint64/baseFee {
		return 0, false
	}
	return gas * baseFee, true
}

func (keeper AnteKeeper) collectFee(ctx context.Context, store StateStore, meta TxMeta) error {
	if meta.Fee == 0 {
		return nil
	}
	if meta.Signer == "" {
		return ErrMissingNonce
	}
	collector := types.Address(keeper.config.FeeCollector)
	if collector == "" {
		collector = "fee_collector"
	}
	balance, err := bankBalance(ctx, store, meta.Signer)
	if err != nil {
		return err
	}
	if balance < meta.Fee {
		return ErrInsufficientFeeBalance
	}
	collectorBalance, err := bankBalance(ctx, store, collector)
	if err != nil {
		return err
	}
	if collectorBalance > ^uint64(0)-meta.Fee {
		return ErrInsufficientFeeBalance
	}
	if err := setBankBalance(ctx, store, meta.Signer, balance-meta.Fee); err != nil {
		return err
	}
	return setBankBalance(ctx, store, collector, collectorBalance+meta.Fee)
}

func (keeper AnteKeeper) verifySignature(ctx Context, tx types.Tx) error {
	if !IsSignedTx(tx) {
		if keeper.config.RequireSigned {
			return ErrInvalidSignedTx
		}
		return nil
	}
	return VerifySignedTx(ctx.ChainID, tx, keeper.signatureVerifier())
}

func (keeper AnteKeeper) signatureVerifier() vexocrypto.Signer {
	return vexocrypto.Ed25519Signer{}
}

func bankBalance(ctx context.Context, store StateStore, address types.Address) (uint64, error) {
	value, err := store.Get(ctx, "bank", []byte(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if len(value) == 0 {
		return 0, nil
	}
	if len(value) != 8 {
		return 0, ErrInsufficientFeeBalance
	}
	return binary.BigEndian.Uint64(value), nil
}

func setBankBalance(ctx context.Context, store StateStore, address types.Address, balance uint64) error {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], balance)
	return store.Set(ctx, "bank", []byte(address), encoded[:])
}
