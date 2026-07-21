package geth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrExecutionConformanceFailed = errors.New("geth EVM execution conformance fixture failed")

type ExecutionFixture struct {
	Name                  string                     `json:"name"`
	Method                string                     `json:"method"`
	Caller                string                     `json:"caller,omitempty"`
	Contract              string                     `json:"contract,omitempty"`
	Coinbase              string                     `json:"coinbase,omitempty"`
	Code                  string                     `json:"code,omitempty"`
	Input                 string                     `json:"input,omitempty"`
	Salt                  string                     `json:"salt,omitempty"`
	BlobHashes            []string                   `json:"blob_hashes,omitempty"`
	AccessList            []contract.AccessListEntry `json:"access_list,omitempty"`
	GasLimit              uint64                     `json:"gas_limit,omitempty"`
	GasPrice              uint64                     `json:"gas_price,omitempty"`
	GasFeeCap             uint64                     `json:"gas_fee_cap,omitempty"`
	GasTipCap             uint64                     `json:"gas_tip_cap,omitempty"`
	BaseFee               uint64                     `json:"base_fee,omitempty"`
	BlockGasLimit         uint64                     `json:"block_gas_limit,omitempty"`
	Value                 uint64                     `json:"value,omitempty"`
	ValueBig              string                     `json:"value_big,omitempty"`
	CallerBalance         uint64                     `json:"caller_balance,omitempty"`
	CallerBalanceBig      string                     `json:"caller_balance_big,omitempty"`
	CallerNonce           uint64                     `json:"caller_nonce,omitempty"`
	Nonce                 uint64                     `json:"nonce,omitempty"`
	EthereumTx            bool                       `json:"ethereum_tx,omitempty"`
	EthereumSimulation    bool                       `json:"ethereum_simulation,omitempty"`
	WantOutput            string                     `json:"want_output,omitempty"`
	WantDeployedCode      string                     `json:"want_deployed_code,omitempty"`
	WantFailed            bool                       `json:"want_failed,omitempty"`
	WantStorageWrites     int                        `json:"want_storage_writes,omitempty"`
	WantGasUsed           *uint64                    `json:"want_gas_used,omitempty"`
	WantMinGasUsed        *uint64                    `json:"want_min_gas_used,omitempty"`
	WantLogs              *int                       `json:"want_logs,omitempty"`
	WantBalanceWrites     *int                       `json:"want_balance_writes,omitempty"`
	WantNonceWrites       *int                       `json:"want_nonce_writes,omitempty"`
	WantCodeWrites        *int                       `json:"want_code_writes,omitempty"`
	WantAccessListEntries *int                       `json:"want_access_list_entries,omitempty"`
	Categories            []string                   `json:"categories,omitempty"`
}

type ExecutionFixtureResult struct {
	Name              string `json:"name"`
	OK                bool   `json:"ok"`
	Output            string `json:"output,omitempty"`
	Failed            bool   `json:"failed,omitempty"`
	GasUsed           uint64 `json:"gas_used,omitempty"`
	Logs              int    `json:"logs,omitempty"`
	StorageWrites     int    `json:"storage_writes,omitempty"`
	BalanceWrites     int    `json:"balance_writes,omitempty"`
	NonceWrites       int    `json:"nonce_writes,omitempty"`
	CodeWrites        int    `json:"code_writes,omitempty"`
	AccessListEntries int    `json:"access_list_entries,omitempty"`
	Err               string `json:"error,omitempty"`
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

var defaultRequiredExecutionFixtureCategories = []string{
	"call_return",
	"contract_create",
	"create2",
	"revert",
	"revert_data",
	"storage_write",
	"gas_accounting",
	"event_logs",
	"value_transfer",
	"precompile",
	"access_list",
}

type ExecutionFixtureDocument struct {
	SchemaVersion      string             `json:"schema_version,omitempty"`
	RequiredCategories []string           `json:"required_categories,omitempty"`
	Fixtures           []ExecutionFixture `json:"fixtures"`
}

func DefaultExecutionFixtures() []ExecutionFixture {
	return []ExecutionFixture{
		{
			Name:           "default call returns 42 and consumes gas",
			Method:         "call",
			Code:           "0x602a60005260206000f3",
			GasLimit:       100_000,
			WantOutput:     "0x000000000000000000000000000000000000000000000000000000000000002a",
			WantMinGasUsed: uint64Ptr(1),
			Categories:     []string{"call_return", "gas_accounting"},
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
			Name:             "default create2 returns runtime code",
			Method:           "deploy",
			Code:             "0x600a600c600039600a6000f3602a60005260206000f3",
			Salt:             "0x0000000000000000000000000000000000000000000000000000000000000001",
			GasLimit:         100_000,
			WantDeployedCode: "0x602a60005260206000f3",
			Categories:       []string{"create2"},
		},
		{
			Name:       "default revert reports failure and data",
			Method:     "call",
			Code:       "0x602a60005260206000fd",
			GasLimit:   100_000,
			WantFailed: true,
			WantOutput: "0x000000000000000000000000000000000000000000000000000000000000002a",
			Categories: []string{"revert", "revert_data"},
		},
		{
			Name:              "default storage write is captured",
			Method:            "call",
			Code:              "0x6001600055",
			GasLimit:          100_000,
			WantStorageWrites: 1,
			Categories:        []string{"storage_write"},
		},
		{
			Name:       "default log opcode emits event",
			Method:     "call",
			Code:       "0x602a600052600160206000a1",
			GasLimit:   100_000,
			WantLogs:   intPtr(1),
			Categories: []string{"event_logs"},
		},
		{
			Name:              "default value transfer updates balances",
			Method:            "call",
			Value:             7,
			CallerBalance:     100,
			GasLimit:          100_000,
			WantBalanceWrites: intPtr(2),
			Categories:        []string{"value_transfer"},
		},
		{
			Name:       "default identity precompile returns input",
			Method:     "call",
			Contract:   "0x0000000000000000000000000000000000000004",
			Input:      "0xabcdef",
			GasLimit:   100_000,
			WantOutput: "0xabcdef",
			Categories: []string{"precompile"},
		},
		{
			Name:                  "default access list is surfaced",
			Method:                "call",
			Code:                  "0x00",
			GasLimit:              100_000,
			AccessList:            []contract.AccessListEntry{{Address: "0x000000000000000000000000000000000000bbbb", StorageKeys: []string{"0x01"}}},
			WantAccessListEntries: intPtr(1),
			Categories:            []string{"access_list"},
		},
	}
}

func RunExecutionFixtures(fixtures []ExecutionFixture) ExecutionConformanceReport {
	return RunExecutionFixturesWithRequired(fixtures, defaultRequiredExecutionFixtureCategories)
}

func RunExecutionFixturesWithRequired(fixtures []ExecutionFixture, required []string) ExecutionConformanceReport {
	report := ExecutionConformanceReport{
		OK:                 true,
		Total:              len(fixtures),
		RequiredCategories: append([]string(nil), required...),
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
	for _, category := range report.RequiredCategories {
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

func RunExecutionFixturesJSON(raw []byte) (ExecutionConformanceReport, error) {
	var document ExecutionFixtureDocument
	if err := json.Unmarshal(raw, &document); err == nil && len(document.Fixtures) > 0 {
		required := document.RequiredCategories
		if len(required) == 0 {
			required = defaultRequiredExecutionFixtureCategories
		}
		return RunExecutionFixturesWithRequired(document.Fixtures, required), nil
	}
	var fixtures []ExecutionFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		return ExecutionConformanceReport{}, err
	}
	return RunExecutionFixtures(fixtures), nil
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
	salt, err := decodeFixtureHex(fixture.Salt)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	blobHashes, err := decodeFixtureHashes(fixture.BlobHashes)
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
	caller := types.Address(fixture.Caller)
	if caller == "" {
		caller = types.Address("0x000000000000000000000000000000000000aaaa")
	}
	contractAddress := types.Address(fixture.Contract)
	if contractAddress == "" {
		contractAddress = types.Address("0x000000000000000000000000000000000000bbbb")
	}
	value, err := parseFixtureBigInt(fixture.ValueBig, fixture.Value)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	state, err := fixtureStateReader(fixture, caller)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	executed, err := New().Execute(context.Background(), contract.Invocation{
		Method:             method,
		Caller:             caller,
		Contract:           contractAddress,
		Coinbase:           types.Address(fixture.Coinbase),
		Code:               code,
		Input:              input,
		Salt:               salt,
		BlobHashes:         blobHashes,
		AccessList:         fixture.AccessList,
		GasLimit:           gasLimit,
		GasPrice:           fixture.GasPrice,
		GasFeeCap:          fixture.GasFeeCap,
		GasTipCap:          fixture.GasTipCap,
		BaseFee:            fixture.BaseFee,
		BlockGasLimit:      fixture.BlockGasLimit,
		Value:              fixture.Value,
		ValueBig:           value,
		Nonce:              fixture.Nonce,
		EthereumTx:         fixture.EthereumTx,
		EthereumSimulation: fixture.EthereumSimulation,
		State:              state,
	})
	if err != nil {
		result.Err = err.Error()
		return result
	}
	result.Output = "0x" + hex.EncodeToString(executed.Output)
	result.Failed = executed.Failed
	result.GasUsed = executed.GasUsed
	result.Logs = len(executed.Logs)
	result.StorageWrites = len(executed.StorageWrites)
	result.BalanceWrites = len(executed.BalanceWrites)
	result.NonceWrites = len(executed.NonceWrites)
	result.CodeWrites = len(executed.CodeWrites)
	result.AccessListEntries = len(executed.AccessList)
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
	if fixture.WantGasUsed != nil && *fixture.WantGasUsed != executed.GasUsed {
		result.Err = "gas used mismatch"
		return result
	}
	if fixture.WantMinGasUsed != nil && executed.GasUsed < *fixture.WantMinGasUsed {
		result.Err = "minimum gas used mismatch"
		return result
	}
	if fixture.WantLogs != nil && *fixture.WantLogs != len(executed.Logs) {
		result.Err = "log count mismatch"
		return result
	}
	if fixture.WantBalanceWrites != nil && *fixture.WantBalanceWrites != len(executed.BalanceWrites) {
		result.Err = "balance write count mismatch"
		return result
	}
	if fixture.WantNonceWrites != nil && *fixture.WantNonceWrites != len(executed.NonceWrites) {
		result.Err = "nonce write count mismatch"
		return result
	}
	if fixture.WantCodeWrites != nil && *fixture.WantCodeWrites != len(executed.CodeWrites) {
		result.Err = "code write count mismatch"
		return result
	}
	if fixture.WantAccessListEntries != nil && len(executed.AccessList) < *fixture.WantAccessListEntries {
		result.Err = "access list entry count mismatch"
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

func decodeFixtureHashes(values []string) ([]types.Hash, error) {
	if len(values) == 0 {
		return nil, nil
	}
	hashes := make([]types.Hash, 0, len(values))
	for _, value := range values {
		raw, err := decodeFixtureHex(value)
		if err != nil || len(raw) != len(types.Hash{}) {
			return nil, errors.New("invalid fixture hash")
		}
		var hash types.Hash
		copy(hash[:], raw)
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

func parseFixtureBigInt(raw string, fallback uint64) (*big.Int, error) {
	if strings.TrimSpace(raw) == "" {
		if fallback == 0 {
			return nil, nil
		}
		return new(big.Int).SetUint64(fallback), nil
	}
	base := 10
	valueText := strings.TrimSpace(raw)
	if strings.HasPrefix(valueText, "0x") || strings.HasPrefix(valueText, "0X") {
		base = 16
		valueText = valueText[2:]
	}
	value, ok := new(big.Int).SetString(valueText, base)
	if !ok || value.Sign() < 0 || value.BitLen() > 256 {
		return nil, contract.ErrInvalidInvocation
	}
	return value, nil
}

func fixtureStateReader(fixture ExecutionFixture, caller types.Address) (contract.StateReader, error) {
	if fixture.CallerBalance == 0 && strings.TrimSpace(fixture.CallerBalanceBig) == "" && fixture.CallerNonce == 0 {
		return nil, nil
	}
	balance, err := parseFixtureBigInt(fixture.CallerBalanceBig, fixture.CallerBalance)
	if err != nil {
		return nil, err
	}
	reader := executionFixtureStateReader{
		balances: make(map[types.Address]*big.Int),
		nonces:   make(map[types.Address]uint64),
	}
	if balance != nil {
		reader.balances[caller] = balance
	}
	if fixture.CallerNonce > 0 {
		reader.nonces[caller] = fixture.CallerNonce
	}
	return reader, nil
}

type executionFixtureStateReader struct {
	balances map[types.Address]*big.Int
	nonces   map[types.Address]uint64
}

func (reader executionFixtureStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	return nil, nil
}

func (reader executionFixtureStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	return nil, nil
}

func (reader executionFixtureStateReader) BalanceBig(ctx context.Context, address types.Address) (*big.Int, error) {
	if value := reader.balances[address]; value != nil {
		return new(big.Int).Set(value), nil
	}
	return new(big.Int), nil
}

func (reader executionFixtureStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	return reader.nonces[address], nil
}

func intPtr(value int) *int {
	return &value
}

func uint64Ptr(value uint64) *uint64 {
	return &value
}
