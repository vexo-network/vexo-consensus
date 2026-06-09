package ethcompat

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
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
	WantAction  string `json:"want_action,omitempty"`
	WantType    string `json:"want_type,omitempty"`
	WantTo      string `json:"want_to,omitempty"`
	WantValue   string `json:"want_value,omitempty"`
	WantFee     string `json:"want_fee,omitempty"`
	WantGas     uint64 `json:"want_gas,omitempty"`
	WantError   string `json:"want_error,omitempty"`
}

type TransactionFixtureResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Hash   string `json:"hash,omitempty"`
	From   string `json:"from,omitempty"`
	Type   string `json:"type,omitempty"`
	Action string `json:"action,omitempty"`
	Err    string `json:"error,omitempty"`
}

type TransactionConformanceReport struct {
	OK      bool                       `json:"ok"`
	Total   int                        `json:"total"`
	Passed  int                        `json:"passed"`
	Failed  int                        `json:"failed"`
	Results []TransactionFixtureResult `json:"results"`
}

func DefaultTransactionFixtures() ([]TransactionFixture, error) {
	callRaw, callHash, err := signedFixtureRawTransaction(7, false)
	if err != nil {
		return nil, err
	}
	createRaw, createHash, err := signedFixtureRawTransaction(7, true)
	if err != nil {
		return nil, err
	}
	accessListRaw, accessListHash, err := signedFixtureAccessListTransaction(7)
	if err != nil {
		return nil, err
	}
	legacyRaw, legacyHash, err := signedFixtureLegacyTransaction(7, true)
	if err != nil {
		return nil, err
	}
	unprotectedLegacyRaw, _, err := signedFixtureLegacyTransaction(7, false)
	if err != nil {
		return nil, err
	}
	tipAboveFeeRaw, _, err := signedFixtureRawTransactionWithFees(7, false, 30, 20)
	if err != nil {
		return nil, err
	}
	return []TransactionFixture{
		{
			Name:       "default dynamic fee call",
			Raw:        callRaw,
			ChainID:    7,
			BaseFee:    11,
			WantHash:   callHash,
			WantAction: "call",
			WantType:   "2",
			WantTo:     "0x000000000000000000000000000000000000bEEF",
			WantValue:  "3",
			WantFee:    "273000",
			WantGas:    21_000,
		},
		{
			Name:       "default dynamic fee contract creation",
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
		{
			Name:       "default access-list metadata preservation",
			Raw:        accessListRaw,
			ChainID:    7,
			BaseFee:    11,
			WantHash:   accessListHash,
			WantAction: "call",
			WantType:   "1",
			WantTo:     "0x000000000000000000000000000000000000bEEF",
			WantValue:  "3",
			WantFee:    "273000",
			WantGas:    21_000,
		},
		{
			Name:       "default protected legacy call",
			Raw:        legacyRaw,
			ChainID:    7,
			WantHash:   legacyHash,
			WantAction: "call",
			WantType:   "0",
			WantTo:     "0x000000000000000000000000000000000000bEEF",
			WantValue:  "3",
			WantFee:    "273000",
			WantGas:    21_000,
		},
		{Name: "default wrong chain rejection", Raw: callRaw, ChainID: 8, WantError: ErrChainIDMismatch.Error()},
		{Name: "default invalid raw rejection", Raw: "0x", WantError: ErrInvalidRawTransaction.Error()},
		{Name: "default base fee cap rejection", Raw: callRaw, ChainID: 7, BaseFee: 21, WantError: ErrFeeCapTooLow.Error()},
		{Name: "default priority fee cap rejection", Raw: tipAboveFeeRaw, ChainID: 7, BaseFee: 1, WantError: ErrTipCapAboveFeeCap.Error()},
		{Name: "default unprotected legacy rejection", Raw: unprotectedLegacyRaw, ChainID: 7, WantError: ErrUnprotectedLegacyTx.Error()},
	}, nil
}

func DefaultTransactionFixturesJSON() ([]byte, error) {
	fixtures, err := DefaultTransactionFixtures()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(fixtures, "", "  ")
}

func RunTransactionFixturesJSON(raw []byte) (TransactionConformanceReport, error) {
	var fixtures []TransactionFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		return TransactionConformanceReport{}, err
	}
	return RunTransactionFixtures(fixtures), nil
}

func signedFixtureRawTransaction(chainID uint64, create bool) (string, string, error) {
	return signedFixtureRawTransactionWithFees(chainID, create, 2, 20)
}

func signedFixtureRawTransactionWithFees(chainID uint64, create bool, tipCap int64, feeCap int64) (string, string, error) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		return "", "", err
	}
	var to *gethcommon.Address
	data := []byte{0x12, 0x34}
	if create {
		data = []byte{0x60, 0x00}
	} else {
		address := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
		to = &address
	}
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     7,
		GasTipCap: big.NewInt(tipCap),
		GasFeeCap: big.NewInt(feeCap),
		Gas:       21_000,
		To:        to,
		Value:     big.NewInt(3),
		Data:      data,
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		return "", "", err
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return "", "", err
	}
	return "0x" + hex.EncodeToString(raw), signed.Hash().Hex(), nil
}

func signedFixtureAccessListTransaction(chainID uint64) (string, string, error) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		return "", "", err
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	tx := gethtypes.NewTx(&gethtypes.AccessListTx{
		ChainID:  new(big.Int).SetUint64(chainID),
		Nonce:    7,
		GasPrice: big.NewInt(13),
		Gas:      21_000,
		To:       &to,
		Value:    big.NewInt(3),
		Data:     []byte{0x12, 0x34},
		AccessList: gethtypes.AccessList{{
			Address:     to,
			StorageKeys: []gethcommon.Hash{gethcommon.HexToHash("0x01")},
		}},
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		return "", "", err
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return "", "", err
	}
	return "0x" + hex.EncodeToString(raw), signed.Hash().Hex(), nil
}

func signedFixtureLegacyTransaction(chainID uint64, protected bool) (string, string, error) {
	key, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		return "", "", err
	}
	to := gethcommon.HexToAddress("0x000000000000000000000000000000000000bEEF")
	tx := gethtypes.NewTx(&gethtypes.LegacyTx{
		Nonce:    7,
		GasPrice: big.NewInt(13),
		Gas:      21_000,
		To:       &to,
		Value:    big.NewInt(3),
		Data:     []byte{0x12, 0x34},
	})
	signer := gethtypes.Signer(gethtypes.HomesteadSigner{})
	if protected {
		signer = gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID))
	}
	signed, err := gethtypes.SignTx(tx, signer, key)
	if err != nil {
		return "", "", err
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return "", "", err
	}
	return "0x" + hex.EncodeToString(raw), signed.Hash().Hex(), nil
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
	result.Type = strconv.FormatUint(uint64(decoded.Type), 10)
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
	canonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		result.Err = err.Error()
		return result
	}
	result.Action = canonical.Action
	if fixture.WantAction != "" && fixture.WantAction != canonical.Action {
		result.Err = "action mismatch"
		return result
	}
	if fixture.WantType != "" && fixture.WantType != canonical.Tags[TagType] {
		result.Err = "type mismatch"
		return result
	}
	if fixture.WantTo != "" && (canonical.Action != "call" || len(canonical.Args) < 3 || canonical.Args[2] != fixture.WantTo) {
		result.Err = "recipient mismatch"
		return result
	}
	if fixture.WantValue != "" && canonical.Tags[TagValue] != fixture.WantValue {
		result.Err = "value mismatch"
		return result
	}
	if fixture.WantFee != "" && canonical.Tags["fee"] != fixture.WantFee {
		result.Err = "fee mismatch"
		return result
	}
	if fixture.WantGas != 0 && canonical.Tags["gas"] != strconv.FormatUint(fixture.WantGas, 10) {
		result.Err = "gas mismatch"
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
	case ErrFeeCapTooLow.Error():
		return ErrFeeCapTooLow
	case ErrTipCapAboveFeeCap.Error():
		return ErrTipCapAboveFeeCap
	case ErrUnprotectedLegacyTx.Error():
		return ErrUnprotectedLegacyTx
	case ErrBlobFeeCapTooLow.Error():
		return ErrBlobFeeCapTooLow
	case ErrInvalidBlobSidecar.Error():
		return ErrInvalidBlobSidecar
	default:
		return errors.New(name)
	}
}
