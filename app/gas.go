package app

import (
	"errors"
	"math"
)

var ErrOutOfGas = errors.New("out of gas")

type GasMeter struct {
	limit uint64
	used  uint64
}

func NewGasMeter(limit uint64) *GasMeter {
	return &GasMeter{limit: limit}
}

func (meter *GasMeter) Consume(amount uint64) error {
	if meter == nil || amount == 0 {
		return nil
	}
	if meter.used > math.MaxUint64-amount {
		return ErrOutOfGas
	}
	next := meter.used + amount
	if meter.limit > 0 && next > meter.limit {
		return ErrOutOfGas
	}
	meter.used = next
	return nil
}

func (meter *GasMeter) Used() uint64 {
	if meter == nil {
		return 0
	}
	return meter.used
}

func (meter *GasMeter) Limit() uint64 {
	if meter == nil {
		return 0
	}
	return meter.limit
}
