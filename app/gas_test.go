package app

import (
	"errors"
	"math"
	"testing"
)

func TestGasMeterConsumesWithinLimit(t *testing.T) {
	meter := NewGasMeter(10)
	if err := meter.Consume(4); err != nil {
		t.Fatal(err)
	}
	if err := meter.Consume(6); err != nil {
		t.Fatal(err)
	}
	if meter.Used() != 10 || meter.Limit() != 10 {
		t.Fatalf("unexpected meter state: used=%d limit=%d", meter.Used(), meter.Limit())
	}
}

func TestGasMeterRejectsOverLimitAndOverflow(t *testing.T) {
	meter := NewGasMeter(10)
	if err := meter.Consume(11); !errors.Is(err, ErrOutOfGas) {
		t.Fatalf("expected out of gas, got %v", err)
	}
	unlimited := NewGasMeter(0)
	if err := unlimited.Consume(math.MaxUint64); err != nil {
		t.Fatal(err)
	}
	if err := unlimited.Consume(1); !errors.Is(err, ErrOutOfGas) {
		t.Fatalf("expected overflow out of gas, got %v", err)
	}
}
