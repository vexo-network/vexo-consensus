package economics

import (
	"errors"
	"math/big"
	"strings"
)

const (
	AtomicDenom  = "avxo"
	GigaDenom    = "gvxo"
	DisplayDenom = "vexo"

	GigaFactor    uint64 = 1_000_000_000
	DisplayFactor uint64 = 1_000_000_000_000_000_000
)

var ErrInvalidAmount = errors.New("invalid amount")

type DenomUnit struct {
	Denom    string
	Exponent uint8
	Factor   uint64
}

var Units = []DenomUnit{
	{Denom: DisplayDenom, Exponent: 18, Factor: DisplayFactor},
	{Denom: GigaDenom, Exponent: 9, Factor: GigaFactor},
	{Denom: AtomicDenom, Exponent: 0, Factor: 1},
}

func ParseAmount(input string) (uint64, error) {
	amount, err := ParseAmountBig(input)
	if err != nil || !amount.IsUint64() {
		return 0, ErrInvalidAmount
	}
	return amount.Uint64(), nil
}

func ParseAmountBig(input string) (*big.Int, error) {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return nil, ErrInvalidAmount
	}
	if decimalDigits(value) {
		amount, ok := new(big.Int).SetString(value, 10)
		if !ok {
			return nil, ErrInvalidAmount
		}
		return amount, nil
	}
	for _, unit := range Units {
		number, found := strings.CutSuffix(value, unit.Denom)
		if !found {
			continue
		}
		return parseDecimalAmountBig(number, unit)
	}
	return nil, ErrInvalidAmount
}

func DenomFactor(denom string) (uint64, bool) {
	normalized := strings.TrimSpace(strings.ToLower(denom))
	for _, unit := range Units {
		if unit.Denom == normalized {
			return unit.Factor, true
		}
	}
	return 0, false
}

func parseDecimalAmount(number string, unit DenomUnit) (uint64, error) {
	amount, err := parseDecimalAmountBig(number, unit)
	if err != nil || !amount.IsUint64() {
		return 0, ErrInvalidAmount
	}
	return amount.Uint64(), nil
}

func parseDecimalAmountBig(number string, unit DenomUnit) (*big.Int, error) {
	number = strings.TrimSpace(number)
	if number == "" || strings.HasPrefix(number, "-") || strings.HasPrefix(number, "+") {
		return nil, ErrInvalidAmount
	}
	whole, fraction, hasFraction := strings.Cut(number, ".")
	if whole == "" {
		whole = "0"
	}
	if whole == "" || !decimalDigits(whole) {
		return nil, ErrInvalidAmount
	}
	if hasFraction && !decimalDigits(fraction) {
		return nil, ErrInvalidAmount
	}
	if len(fraction) > int(unit.Exponent) {
		return nil, ErrInvalidAmount
	}
	wholeValue, ok := new(big.Int).SetString(whole, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}
	amount := new(big.Int).Mul(wholeValue, new(big.Int).SetUint64(unit.Factor))
	if !hasFraction || fraction == "" {
		return amount, nil
	}
	padded := fraction + strings.Repeat("0", int(unit.Exponent)-len(fraction))
	fractionValue, ok := new(big.Int).SetString(padded, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}
	return amount.Add(amount, fractionValue), nil
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
