package ethcompat

import (
	"encoding/json"
	"testing"
)

func TestRunTransactionFixturesJSON(t *testing.T) {
	raw, hash := signedRawTestTx(t, 7, false)
	fixtures, err := json.Marshal([]TransactionFixture{
		{Name: "dynamic fee call", Raw: raw, ChainID: 7, BaseFee: 11, WantHash: hash},
		{Name: "wrong chain", Raw: raw, ChainID: 8, WantError: ErrChainIDMismatch.Error()},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := RunTransactionFixturesJSON(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Total != 2 || report.Passed != 2 || report.Failed != 0 {
		t.Fatalf("unexpected conformance report: %+v", report)
	}
}
