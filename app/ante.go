package app

import (
	"context"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"

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
	FeePaid(tx types.Tx) uint64
}

type AnteKeeper struct {
	config AnteConfig
}

func NewAnteKeeper(config AnteConfig) AnteKeeper {
	return AnteKeeper{config: config}
}

func ParseTxMeta(tx types.Tx) TxMeta {
	tx = TxPayload(tx)
	var meta TxMeta
	for _, part := range strings.Split(string(tx), ":") {
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		switch key {
		case "signer":
			meta.Signer = types.Address(value)
		case "nonce":
			nonce, err := strconv.ParseUint(value, 10, 64)
			if err == nil {
				meta.Nonce = nonce
				meta.HasNonce = true
			}
		case "fee":
			meta.Fee, _ = strconv.ParseUint(value, 10, 64)
		case "gas":
			meta.Gas, _ = strconv.ParseUint(value, 10, 64)
		}
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
	expected, err := keeper.nextNonce(context.Background(), ctx.Store, meta.Signer)
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
			expected, err = keeper.nextNonce(context.Background(), ctx.Store, meta.Signer)
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
	if err := keeper.collectFee(context.Background(), ctx.Store, meta); err != nil {
		return err
	}
	if meta.Signer == "" || !meta.HasNonce {
		return nil
	}
	return keeper.setNonce(context.Background(), ctx.Store, meta.Signer, meta.Nonce)
}

func (keeper AnteKeeper) GasUsed(tx types.Tx) uint64 {
	meta := ParseTxMeta(tx)
	if meta.Gas > 0 {
		return meta.Gas
	}
	return uint64(len(tx))
}

func (keeper AnteKeeper) FeePaid(tx types.Tx) uint64 {
	return ParseTxMeta(tx).Fee
}

func (keeper AnteKeeper) validateMeta(meta TxMeta) error {
	if keeper.config.MinFee > 0 && meta.Fee < keeper.config.MinFee {
		return ErrInsufficientFee
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

func (keeper AnteKeeper) nextNonce(ctx context.Context, store StateStore, signer types.Address) (uint64, error) {
	value, err := store.Get(ctx, "auth", nonceKey(signer))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, ErrInvalidNonce
	}
	return binary.BigEndian.Uint64(value) + 1, nil
}

func (keeper AnteKeeper) setNonce(ctx context.Context, store StateStore, signer types.Address, nonce uint64) error {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], nonce)
	return store.Set(ctx, "auth", nonceKey(signer), encoded[:])
}

func nonceKey(signer types.Address) []byte {
	return []byte("nonce/" + string(signer))
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
