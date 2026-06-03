package economics

import (
	"errors"
	"testing"
)

func TestParseAmountSupportsAtomicAndDisplayUnits(t *testing.T) {
	cases := map[string]uint64{
		"7":        7,
		"7avxo":    7,
		"1gvxo":    GigaFactor,
		"1.25gvxo": 1_250_000_000,
		"1vexo":    DisplayFactor,
		"0.5vexo":  500_000_000_000_000_000,
	}
	for input, expected := range cases {
		actual, err := ParseAmount(input)
		if err != nil {
			t.Fatalf("parse %q: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("parse %q = %d, want %d", input, actual, expected)
		}
	}
}

func TestParseAmountRejectsInvalidOrLossyValues(t *testing.T) {
	cases := []string{"", "abc", "-1avxo", "0.1avxo", "1.0000000001gvxo", "18446744073709551616"}
	for _, input := range cases {
		if _, err := ParseAmount(input); !errors.Is(err, ErrInvalidAmount) {
			t.Fatalf("expected invalid amount for %q, got %v", input, err)
		}
	}
}
