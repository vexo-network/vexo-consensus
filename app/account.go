package app

import (
	"context"
	"encoding/binary"
	"errors"

	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidAccountSequence = errors.New("invalid account sequence")

type AccountKeeper struct{}

func NewAccountKeeper() AccountKeeper {
	return AccountKeeper{}
}

func (keeper AccountKeeper) Sequence(ctx context.Context, store StateStore, signer types.Address) (uint64, error) {
	if store == nil || signer == "" {
		return 0, ErrInvalidAccountSequence
	}
	value, err := store.Get(ctx, "auth", nonceKey(signer))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, ErrInvalidAccountSequence
	}
	return binary.BigEndian.Uint64(value), nil
}

func (keeper AccountKeeper) NextSequence(ctx context.Context, store StateStore, signer types.Address) (uint64, error) {
	sequence, err := keeper.Sequence(ctx, store, signer)
	if err != nil {
		return 0, err
	}
	return sequence + 1, nil
}

func (keeper AccountKeeper) SetSequence(ctx context.Context, store StateStore, signer types.Address, sequence uint64) error {
	if store == nil || signer == "" {
		return ErrInvalidAccountSequence
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], sequence)
	return store.Set(ctx, "auth", nonceKey(signer), encoded[:])
}

func nonceKey(signer types.Address) []byte {
	return []byte("nonce/" + string(signer))
}
