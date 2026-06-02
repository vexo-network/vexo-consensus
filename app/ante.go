package app

import (
	"context"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"

	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInsufficientFee = errors.New("insufficient transaction fee")
	ErrInvalidGas      = errors.New("invalid transaction gas")
	ErrMissingNonce    = errors.New("missing transaction nonce")
	ErrInvalidNonce    = errors.New("invalid transaction nonce")
)

type AnteConfig struct {
	MinFee       uint64
	MinGas       uint64
	MaxGas       uint64
	RequireNonce bool
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
}

type AnteKeeper struct {
	config AnteConfig
}

func NewAnteKeeper(config AnteConfig) AnteKeeper {
	return AnteKeeper{config: config}
}

func ParseTxMeta(tx types.Tx) TxMeta {
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
	if ctx.Store == nil || meta.Signer == "" || !meta.HasNonce {
		return nil
	}
	return keeper.setNonce(context.Background(), ctx.Store, meta.Signer, meta.Nonce)
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
