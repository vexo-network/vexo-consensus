package economics

import "math"

type BaseFeeParams struct {
	CurrentBaseFee    uint64
	GasUsed           uint64
	TargetGas         uint64
	ChangeDenominator uint64
	MinBaseFee        uint64
	MaxBaseFee        uint64
}

func NextBaseFee(params BaseFeeParams) uint64 {
	current := params.CurrentBaseFee
	if current == 0 || params.TargetGas == 0 || params.ChangeDenominator == 0 {
		return current
	}
	if params.GasUsed == params.TargetGas {
		return clampBaseFee(current, params.MinBaseFee, params.MaxBaseFee)
	}
	if params.GasUsed > params.TargetGas {
		delta := baseFeeDelta(current, params.GasUsed-params.TargetGas, params.TargetGas, params.ChangeDenominator)
		if delta == 0 {
			delta = 1
		}
		if current > math.MaxUint64-delta {
			return clampBaseFee(math.MaxUint64, params.MinBaseFee, params.MaxBaseFee)
		}
		return clampBaseFee(current+delta, params.MinBaseFee, params.MaxBaseFee)
	}
	delta := baseFeeDelta(current, params.TargetGas-params.GasUsed, params.TargetGas, params.ChangeDenominator)
	if delta >= current {
		return clampBaseFee(0, params.MinBaseFee, params.MaxBaseFee)
	}
	return clampBaseFee(current-delta, params.MinBaseFee, params.MaxBaseFee)
}

func baseFeeDelta(current uint64, gasDelta uint64, targetGas uint64, denominator uint64) uint64 {
	if current == 0 || gasDelta == 0 || targetGas == 0 || denominator == 0 {
		return 0
	}
	if current > math.MaxUint64/gasDelta {
		return math.MaxUint64 / denominator
	}
	return (current * gasDelta / targetGas) / denominator
}

func clampBaseFee(value uint64, minBaseFee uint64, maxBaseFee uint64) uint64 {
	if value < minBaseFee {
		value = minBaseFee
	}
	if maxBaseFee > 0 && value > maxBaseFee {
		value = maxBaseFee
	}
	return value
}
