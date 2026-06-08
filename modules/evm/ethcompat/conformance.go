package ethcompat

import (
	"encoding/json"
	"errors"
)

var ErrConformanceFailed = errors.New("Ethereum compatibility conformance fixture failed")

type TransactionFixture struct {
	Name        string `json:"name"`
	Raw         string `json:"raw"`
	ChainID     uint64 `json:"chain_id,omitempty"`
	BaseFee     uint64 `json:"base_fee,omitempty"`
	BlobBaseFee uint64 `json:"blob_base_fee,omitempty"`
	WantHash    string `json:"want_hash,omitempty"`
	WantFrom    string `json:"want_from,omitempty"`
	WantError   string `json:"want_error,omitempty"`
}

type TransactionFixtureResult struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Hash string `json:"hash,omitempty"`
	From string `json:"from,omitempty"`
	Err  string `json:"error,omitempty"`
}

type TransactionConformanceReport struct {
	OK      bool                       `json:"ok"`
	Total   int                        `json:"total"`
	Passed  int                        `json:"passed"`
	Failed  int                        `json:"failed"`
	Results []TransactionFixtureResult `json:"results"`
}

func RunTransactionFixturesJSON(raw []byte) (TransactionConformanceReport, error) {
	var fixtures []TransactionFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		return TransactionConformanceReport{}, err
	}
	return RunTransactionFixtures(fixtures), nil
}

func RunTransactionFixtures(fixtures []TransactionFixture) TransactionConformanceReport {
	report := TransactionConformanceReport{OK: true, Total: len(fixtures), Results: make([]TransactionFixtureResult, 0, len(fixtures))}
	for _, fixture := range fixtures {
		result := runTransactionFixture(fixture)
		if result.OK {
			report.Passed++
		} else {
			report.OK = false
			report.Failed++
		}
		report.Results = append(report.Results, result)
	}
	return report
}

func runTransactionFixture(fixture TransactionFixture) TransactionFixtureResult {
	result := TransactionFixtureResult{Name: fixture.Name}
	decoded, err := DecodeRawTransaction(fixture.Raw, DecodeOptions{
		ChainID:     fixture.ChainID,
		BaseFee:     fixture.BaseFee,
		BlobBaseFee: fixture.BlobBaseFee,
	})
	if err != nil {
		result.Err = err.Error()
		result.OK = fixture.WantError != "" && errors.Is(err, namedFixtureError(fixture.WantError))
		if !result.OK && fixture.WantError != "" {
			result.OK = result.Err == fixture.WantError
		}
		return result
	}
	result.Hash = decoded.Hash
	result.From = string(decoded.From)
	if fixture.WantError != "" {
		result.Err = "expected error " + fixture.WantError
		return result
	}
	if fixture.WantHash != "" && fixture.WantHash != decoded.Hash {
		result.Err = "hash mismatch"
		return result
	}
	if fixture.WantFrom != "" && fixture.WantFrom != string(decoded.From) {
		result.Err = "sender mismatch"
		return result
	}
	if err := ValidateCanonicalTx(decoded.Tx, decoded.ChainID); err != nil {
		result.Err = err.Error()
		return result
	}
	result.OK = true
	return result
}

func namedFixtureError(name string) error {
	switch name {
	case ErrInvalidRawTransaction.Error():
		return ErrInvalidRawTransaction
	case ErrChainIDMismatch.Error():
		return ErrChainIDMismatch
	case ErrValueOverflow.Error():
		return ErrValueOverflow
	case ErrSignatureMismatch.Error():
		return ErrSignatureMismatch
	case ErrBlobFeeCapTooLow.Error():
		return ErrBlobFeeCapTooLow
	case ErrInvalidBlobSidecar.Error():
		return ErrInvalidBlobSidecar
	default:
		return errors.New(name)
	}
}
