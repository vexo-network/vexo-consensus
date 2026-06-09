package ethcompat

import (
	"encoding/json"
	"testing"
)

func TestRunTransactionFixturesJSON(t *testing.T) {
	raw, hash := signedRawTestTx(t, 7, false)
	createRaw, createHash := signedRawTestTx(t, 7, true)
	fixtures, err := json.Marshal([]TransactionFixture{
		{
			Name:       "dynamic fee call",
			Raw:        raw,
			ChainID:    7,
			BaseFee:    11,
			WantHash:   hash,
			WantAction: "call",
			WantType:   "2",
			WantTo:     "0x000000000000000000000000000000000000bEEF",
			WantValue:  "3",
			WantFee:    "273000",
			WantGas:    21_000,
		},
		{
			Name:       "dynamic fee contract creation",
			Raw:        createRaw,
			ChainID:    7,
			BaseFee:    11,
			WantHash:   createHash,
			WantAction: "eth_deploy",
			WantType:   "2",
			WantValue:  "3",
			WantFee:    "273000",
			WantGas:    21_000,
		},
		{Name: "wrong chain", Raw: raw, ChainID: 8, WantError: ErrChainIDMismatch.Error()},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := RunTransactionFixturesJSON(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Total != 3 || report.Passed != 3 || report.Failed != 0 {
		t.Fatalf("unexpected conformance report: %+v", report)
	}
}

func TestRunTransactionFixturesRejectsCanonicalMismatch(t *testing.T) {
	raw, _ := signedRawTestTx(t, 7, false)
	report := RunTransactionFixtures([]TransactionFixture{{
		Name:       "wrong action",
		Raw:        raw,
		ChainID:    7,
		BaseFee:    11,
		WantAction: "eth_deploy",
	}})
	if report.OK || report.Passed != 0 || report.Failed != 1 || report.Results[0].Err != "action mismatch" {
		t.Fatalf("expected canonical action mismatch: %+v", report)
	}
}

func TestDefaultTransactionFixturesCoverSuccessAndFeeFailures(t *testing.T) {
	fixtures, err := DefaultTransactionFixtures()
	if err != nil {
		t.Fatal(err)
	}
	report := RunTransactionFixtures(fixtures)
	if !report.OK || report.Total < 9 || report.Passed != report.Total {
		t.Fatalf("expected default fixtures to pass as a corpus: %+v", report)
	}
	foundAccessList := false
	foundLegacy := false
	foundUnprotectedLegacy := false
	foundFeeCap := false
	foundTipCap := false
	for _, result := range report.Results {
		switch result.Name {
		case "default access-list metadata preservation":
			foundAccessList = result.OK && result.Type == "1" && result.Action == "call"
		case "default protected legacy call":
			foundLegacy = result.OK && result.Type == "0" && result.Action == "call"
		case "default unprotected legacy rejection":
			foundUnprotectedLegacy = result.OK
		case "default base fee cap rejection":
			foundFeeCap = result.OK
		case "default priority fee cap rejection":
			foundTipCap = result.OK
		}
	}
	if !foundAccessList || !foundLegacy || !foundUnprotectedLegacy || !foundFeeCap || !foundTipCap {
		t.Fatalf("expected access-list, legacy, and fee failure fixtures, got %+v", report.Results)
	}
}
