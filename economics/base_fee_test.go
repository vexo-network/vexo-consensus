package economics

import "testing"

func TestNextBaseFeeAdjustsAroundTargetGas(t *testing.T) {
	params := BaseFeeParams{
		CurrentBaseFee:    100,
		TargetGas:         1000,
		ChangeDenominator: 8,
		MinBaseFee:        1,
	}
	if next := NextBaseFee(params); next != 88 {
		t.Fatalf("expected base fee decrease below target, got %d", next)
	}
	params.GasUsed = 1000
	if next := NextBaseFee(params); next != 100 {
		t.Fatalf("expected base fee unchanged at target, got %d", next)
	}
	params.GasUsed = 2000
	if next := NextBaseFee(params); next != 112 {
		t.Fatalf("expected base fee increase above target, got %d", next)
	}
}

func TestNextBaseFeeClampsBounds(t *testing.T) {
	params := BaseFeeParams{
		CurrentBaseFee:    10,
		GasUsed:           0,
		TargetGas:         1000,
		ChangeDenominator: 1,
		MinBaseFee:        5,
	}
	if next := NextBaseFee(params); next != 5 {
		t.Fatalf("expected min base fee clamp, got %d", next)
	}
	params = BaseFeeParams{
		CurrentBaseFee:    100,
		GasUsed:           10_000,
		TargetGas:         1000,
		ChangeDenominator: 1,
		MaxBaseFee:        120,
	}
	if next := NextBaseFee(params); next != 120 {
		t.Fatalf("expected max base fee clamp, got %d", next)
	}
}
