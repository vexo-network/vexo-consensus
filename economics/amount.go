package economics

import (
	"errors"
	"math"
	"strconv"
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
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return 0, ErrInvalidAmount
	}
	if amount, err := strconv.ParseUint(value, 10, 64); err == nil {
		return amount, nil
	}
	for _, unit := range Units {
		number, found := strings.CutSuffix(value, unit.Denom)
		if !found {
			continue
		}
		return parseDecimalAmount(number, unit)
	}
	return 0, ErrInvalidAmount
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
	number = strings.TrimSpace(number)
	if number == "" || strings.HasPrefix(number, "-") || strings.HasPrefix(number, "+") {
		return 0, ErrInvalidAmount
	}
	whole, fraction, hasFraction := strings.Cut(number, ".")
	if whole == "" {
		whole = "0"
	}
	if whole == "" || !decimalDigits(whole) {
		return 0, ErrInvalidAmount
	}
	if hasFraction && !decimalDigits(fraction) {
		return 0, ErrInvalidAmount
	}
	if len(fraction) > int(unit.Exponent) {
		return 0, ErrInvalidAmount
	}
	wholeValue, err := strconv.ParseUint(whole, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	if wholeValue > math.MaxUint64/unit.Factor {
		return 0, ErrInvalidAmount
	}
	amount := wholeValue * unit.Factor
	if !hasFraction || fraction == "" {
		return amount, nil
	}
	padded := fraction + strings.Repeat("0", int(unit.Exponent)-len(fraction))
	fractionValue, err := strconv.ParseUint(padded, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	if amount > math.MaxUint64-fractionValue {
		return 0, ErrInvalidAmount
	}
	return amount + fractionValue, nil
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
