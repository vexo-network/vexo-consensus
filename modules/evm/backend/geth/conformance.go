package geth

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrExecutionConformanceFailed = errors.New("geth EVM execution conformance fixture failed")

type ExecutionFixture struct {
	Name              string   `json:"name"`
	Method            string   `json:"method"`
	Code              string   `json:"code,omitempty"`
	Input             string   `json:"input,omitempty"`
	GasLimit          uint64   `json:"gas_limit,omitempty"`
	WantOutput        string   `json:"want_output,omitempty"`
	WantDeployedCode  string   `json:"want_deployed_code,omitempty"`
	WantFailed        bool     `json:"want_failed,omitempty"`
	WantStorageWrites int      `json:"want_storage_writes,omitempty"`
	Categories        []string `json:"categories,omitempty"`
}

type ExecutionFixtureResult struct {
	Name          string `json:"name"`
	OK            bool   `json:"ok"`
	Output        string `json:"output,omitempty"`
	Failed        bool   `json:"failed,omitempty"`
	StorageWrites int    `json:"storage_writes,omitempty"`
	Err           string `json:"error,omitempty"`
}

type ExecutionConformanceReport struct {
	OK                 bool                     `json:"ok"`
	Total              int                      `json:"total"`
	Passed             int                      `json:"passed"`
	Failed             int                      `json:"failed"`
	CoverageOK         bool                     `json:"coverage_ok"`
	RequiredCategories []string                 `json:"required_categories"`
	CoveredCategories  []string                 `json:"covered_categories"`
	MissingCategories  []string                 `json:"missing_categories,omitempty"`
	Results            []ExecutionFixtureResult `json:"results"`
}

var defaultRequiredExecutionFixtureCategories = []string{"call_return", "contract_create", "revert", "storage_write"}

func DefaultExecutionFixtures() []ExecutionFixture {
	return []ExecutionFixture{
		{
			Name:       "default call returns 42",
			Method:     "call",
			Code:       "0x602a60005260206000f3",
			GasLimit:   100_000,
			WantOutput: "0x000000000000000000000000000000000000000000000000000000000000002a",
			Categories: []string{"call_return"},
		},
		{
			Name:             "default contract creation returns runtime code",
			Method:           "deploy",
			Code:             "0x600a600c600039600a6000f3602a60005260206000f3",
			GasLimit:         100_000,
			WantDeployedCode: "0x602a60005260206000f3",
			Categories:       []string{"contract_create"},
		},
		{
			Name:       "default revert reports failure",
			Method:     "call",
			Code:       "0x60006000fd",
			GasLimit:   100_000,
			WantFailed: true,
			Categories: []string{"revert"},
		},
		{
			Name:              "default storage write is captured",
			Method:            "call",
			Code:              "0x6001600055",
			GasLimit:          100_000,
			WantStorageWrites: 1,
			Categories:        []string{"storage_write"},
		},
	}
}

func RunExecutionFixtures(fixtures []ExecutionFixture) ExecutionConformanceReport {
	report := ExecutionConformanceReport{
		OK:                 true,
		Total:              len(fixtures),
		RequiredCategories: append([]string(nil), defaultRequiredExecutionFixtureCategories...),
		Results:            make([]ExecutionFixtureResult, 0, len(fixtures)),
	}
	covered := make(map[string]struct{})
	for _, fixture := range fixtures {
		result := runExecutionFixture(fixture)
		if result.OK {
			report.Passed++
			for _, category := range fixture.Categories {
				covered[category] = struct{}{}
			}
		} else {
			report.OK = false
			report.Failed++
		}
		report.Results = append(report.Results, result)
	}
	for _, category := range defaultRequiredExecutionFixtureCategories {
		if _, ok := covered[category]; ok {
			report.CoveredCategories = append(report.CoveredCategories, category)
		} else {
			report.MissingCategories = append(report.MissingCategories, category)
		}
	}
	report.CoverageOK = len(report.MissingCategories) == 0
	report.OK = report.OK && report.CoverageOK
	return report
}

func runExecutionFixture(fixture ExecutionFixture) ExecutionFixtureResult {
	result := ExecutionFixtureResult{Name: fixture.Name}
	code, err := decodeFixtureHex(fixture.Code)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	input, err := decodeFixtureHex(fixture.Input)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	method := fixture.Method
	if method == "" {
		method = "call"
	}
	if method == "deploy" && len(input) == 0 {
		input = code
		code = nil
	}
	gasLimit := fixture.GasLimit
	if gasLimit == 0 {
		gasLimit = 100_000
	}
	executed, err := New().Execute(context.Background(), contract.Invocation{
		Method:   method,
		Caller:   types.Address("0x000000000000000000000000000000000000aaaa"),
		Contract: types.Address("0x000000000000000000000000000000000000bbbb"),
		Code:     code,
		Input:    input,
		GasLimit: gasLimit,
	})
	if err != nil {
		result.Err = err.Error()
		return result
	}
	result.Output = "0x" + hex.EncodeToString(executed.Output)
	result.Failed = executed.Failed
	result.StorageWrites = len(executed.StorageWrites)
	if fixture.WantFailed != executed.Failed {
		result.Err = "failure flag mismatch"
		return result
	}
	if fixture.WantOutput != "" && !strings.EqualFold(fixture.WantOutput, result.Output) {
		result.Err = "output mismatch"
		return result
	}
	if fixture.WantDeployedCode != "" && !strings.EqualFold(fixture.WantDeployedCode, "0x"+hex.EncodeToString(executed.DeployedCode)) {
		result.Err = "deployed code mismatch"
		return result
	}
	if fixture.WantStorageWrites >= 0 && fixture.WantStorageWrites != len(executed.StorageWrites) {
		result.Err = "storage write count mismatch"
		return result
	}
	result.OK = true
	return result
}

func decodeFixtureHex(value string) ([]byte, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if value == "" {
		return nil, nil
	}
	return hex.DecodeString(value)
}
