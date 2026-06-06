package ethcompat

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/big"
	"strconv"
	"strings"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	DefaultVM = "evm"

	TagHash                 = "eth_hash"
	TagRaw                  = "eth_raw"
	TagType                 = "eth_type"
	TagInput                = "eth_input"
	TagChainID              = "eth_chain_id"
	TagBaseFee              = "eth_base_fee"
	TagGasPrice             = "eth_gas_price"
	TagMaxFeePerGas         = "eth_max_fee_per_gas"
	TagMaxPriorityFeePerGas = "eth_max_priority_fee_per_gas"
	TagAccessList           = "eth_access_list"
)

var (
	ErrInvalidRawTransaction = errors.New("invalid Ethereum raw transaction")
	ErrChainIDMismatch       = errors.New("Ethereum transaction chain ID mismatch")
	ErrValueOverflow         = errors.New("Ethereum transaction value overflows Vexo uint64 amount")
	ErrSignatureMismatch     = errors.New("Ethereum transaction signature does not match canonical tags")
)

type DecodeOptions struct {
	ChainID uint64
	BaseFee uint64
	VM      string
}

type DecodedTransaction struct {
	Tx                   types.Tx
	Hash                 string
	Raw                  string
	From                 types.Address
	To                   *types.Address
	ContractAddress      types.Address
	Type                 uint8
	Nonce                uint64
	Gas                  uint64
	Fee                  uint64
	GasPrice             uint64
	MaxFeePerGas         uint64
	MaxPriorityFeePerGas uint64
	Value                uint64
	Input                string
	AccessList           []contract.AccessListEntry
	ContractCreation     bool
	ChainID              uint64
}

func ChainNumericID(chainID string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(chainID))
	value := hash.Sum64()
	if value == 0 {
		return 1
	}
	return value
}

func DecodeRawTransaction(rawHex string, options DecodeOptions) (DecodedTransaction, error) {
	rawHex = normalizeHex(rawHex)
	raw, err := hex.DecodeString(strings.TrimPrefix(rawHex, "0x"))
	if err != nil || len(raw) == 0 {
		return DecodedTransaction{}, ErrInvalidRawTransaction
	}
	var ethTx gethtypes.Transaction
	if err := ethTx.UnmarshalBinary(raw); err != nil {
		return DecodedTransaction{}, fmt.Errorf("%w: %v", ErrInvalidRawTransaction, err)
	}
	chainID, err := resolvedChainID(&ethTx, options.ChainID)
	if err != nil {
		return DecodedTransaction{}, err
	}
	signer := signerForTransaction(&ethTx, chainID)
	from, err := gethtypes.Sender(signer, &ethTx)
	if err != nil {
		return DecodedTransaction{}, fmt.Errorf("%w: %v", ErrInvalidRawTransaction, err)
	}
	value, err := uint64Big(ethTx.Value())
	if err != nil {
		return DecodedTransaction{}, err
	}
	gasPrice, err := effectiveGasPrice(&ethTx, options.BaseFee)
	if err != nil {
		return DecodedTransaction{}, err
	}
	fee, ok := multiply(ethTx.Gas(), gasPrice)
	if !ok {
		return DecodedTransaction{}, ErrValueOverflow
	}
	maxFee, err := uint64Big(ethTx.GasFeeCap())
	if err != nil {
		return DecodedTransaction{}, err
	}
	maxPriority, err := uint64Big(ethTx.GasTipCap())
	if err != nil {
		return DecodedTransaction{}, err
	}
	input := "0x" + hex.EncodeToString(ethTx.Data())
	hash := ethTx.Hash().Hex()
	vm := options.VM
	if vm == "" {
		vm = DefaultVM
	}
	tags := map[string]string{
		"fee":                   strconv.FormatUint(fee, 10),
		"gas":                   strconv.FormatUint(ethTx.Gas(), 10),
		"signer":                from.Hex(),
		"nonce":                 strconv.FormatUint(ethTx.Nonce(), 10),
		TagHash:                 hash,
		TagRaw:                  rawHex,
		TagType:                 strconv.FormatUint(uint64(ethTx.Type()), 10),
		TagInput:                input,
		TagChainID:              strconv.FormatUint(chainID, 10),
		TagBaseFee:              strconv.FormatUint(options.BaseFee, 10),
		TagGasPrice:             strconv.FormatUint(gasPrice, 10),
		TagMaxFeePerGas:         strconv.FormatUint(maxFee, 10),
		TagMaxPriorityFeePerGas: strconv.FormatUint(maxPriority, 10),
	}
	accessList := contractAccessList(ethTx.AccessList())
	if len(accessList) > 0 {
		encodedAccessList, err := EncodeAccessList(accessList)
		if err != nil {
			return DecodedTransaction{}, err
		}
		tags[TagAccessList] = encodedAccessList
	}
	decoded := DecodedTransaction{
		Hash:                 hash,
		Raw:                  rawHex,
		From:                 types.Address(from.Hex()),
		Type:                 ethTx.Type(),
		Nonce:                ethTx.Nonce(),
		Gas:                  ethTx.Gas(),
		Fee:                  fee,
		GasPrice:             gasPrice,
		MaxFeePerGas:         maxFee,
		MaxPriorityFeePerGas: maxPriority,
		Value:                value,
		Input:                input,
		AccessList:           accessList,
		ChainID:              chainID,
	}
	var canonical vexoapp.CanonicalTx
	if ethTx.To() == nil {
		contractAddress := gethcrypto.CreateAddress(from, ethTx.Nonce())
		decoded.ContractCreation = true
		decoded.ContractAddress = types.Address(contractAddress.Hex())
		canonical = vexoapp.CanonicalTx{
			Module: "evm",
			Action: "eth_deploy",
			Args: []string{
				vm,
				from.Hex(),
				strings.TrimPrefix(input, "0x"),
				strconv.FormatUint(ethTx.Nonce(), 10),
				strconv.FormatUint(value, 10),
			},
			Tags: tags,
		}
	} else {
		to := types.Address(ethTx.To().Hex())
		decoded.To = &to
		canonical = vexoapp.CanonicalTx{
			Module: "evm",
			Action: "call",
			Args: []string{
				vm,
				from.Hex(),
				ethTx.To().Hex(),
				"call",
				strings.TrimPrefix(input, "0x"),
				strconv.FormatUint(ethTx.Gas(), 10),
				strconv.FormatUint(value, 10),
			},
			Tags: tags,
		}
	}
	built, err := vexoapp.BuildCanonicalTx(canonical)
	if err != nil {
		return DecodedTransaction{}, err
	}
	decoded.Tx = built
	return decoded, nil
}

func ValidateCanonicalTx(tx types.Tx, expectedChainID uint64) error {
	raw, found := vexoapp.TxTag(tx, TagRaw)
	if !found || raw == "" {
		return ErrInvalidRawTransaction
	}
	decoded, err := DecodeRawTransaction(raw, DecodeOptions{ChainID: expectedChainID})
	if err != nil {
		return err
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		return err
	}
	decodedCanonical, err := vexoapp.ParseCanonicalTx(decoded.Tx)
	if err != nil {
		return err
	}
	if canonical.Module != decodedCanonical.Module || canonical.Action != decodedCanonical.Action {
		return ErrSignatureMismatch
	}
	if !sameStrings(canonical.Args, decodedCanonical.Args) {
		return ErrSignatureMismatch
	}
	for _, key := range []string{"gas", "signer", "nonce", TagHash, TagRaw, TagType, TagInput, TagChainID, TagMaxFeePerGas, TagMaxPriorityFeePerGas, TagAccessList} {
		if canonical.Tags[key] != decodedCanonical.Tags[key] {
			return ErrSignatureMismatch
		}
	}
	return nil
}

func EncodeAccessList(entries []contract.AccessListEntry) (string, error) {
	if len(entries) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(encoded), nil
}

func DecodeAccessList(encoded string) ([]contract.AccessListEntry, error) {
	if encoded == "" {
		return nil, nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var entries []contract.AccessListEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func contractAccessList(entries gethtypes.AccessList) []contract.AccessListEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]contract.AccessListEntry, 0, len(entries))
	for _, entry := range entries {
		item := contract.AccessListEntry{
			Address:     types.Address(entry.Address.Hex()),
			StorageKeys: make([]string, 0, len(entry.StorageKeys)),
		}
		for _, slot := range entry.StorageKeys {
			item.StorageKeys = append(item.StorageKeys, slot.Hex())
		}
		out = append(out, item)
	}
	return out
}

func IsEthereumTx(tx types.Tx) bool {
	_, found := vexoapp.TxTag(tx, TagRaw)
	return found
}

func normalizeHex(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "0X") {
		value = "0x" + strings.TrimPrefix(value, "0X")
	}
	if !strings.HasPrefix(value, "0x") {
		value = "0x" + value
	}
	trimmed := strings.TrimPrefix(value, "0x")
	if len(trimmed)%2 == 1 {
		trimmed = "0" + trimmed
	}
	return "0x" + strings.ToLower(trimmed)
}

func resolvedChainID(tx *gethtypes.Transaction, expected uint64) (uint64, error) {
	actual := tx.ChainId()
	if actual == nil || actual.Sign() == 0 {
		if expected > 0 {
			return expected, nil
		}
		return 1, nil
	}
	actualUint, err := uint64Big(actual)
	if err != nil {
		return 0, err
	}
	if expected > 0 && actualUint != expected {
		return 0, ErrChainIDMismatch
	}
	return actualUint, nil
}

func signerForTransaction(tx *gethtypes.Transaction, chainID uint64) gethtypes.Signer {
	if tx.Type() == gethtypes.LegacyTxType && !tx.Protected() {
		return gethtypes.HomesteadSigner{}
	}
	return gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID))
}

func effectiveGasPrice(tx *gethtypes.Transaction, baseFee uint64) (uint64, error) {
	if tx.Type() == gethtypes.LegacyTxType || tx.Type() == gethtypes.AccessListTxType || baseFee == 0 {
		return uint64Big(tx.GasPrice())
	}
	base := new(big.Int).SetUint64(baseFee)
	candidate := new(big.Int).Add(base, tx.GasTipCap())
	if candidate.Cmp(tx.GasFeeCap()) > 0 {
		candidate = tx.GasFeeCap()
	}
	if candidate.Sign() < 0 {
		return 0, ErrInvalidRawTransaction
	}
	return uint64Big(candidate)
}

func uint64Big(value *big.Int) (uint64, error) {
	if value == nil {
		return 0, nil
	}
	if value.Sign() < 0 || !value.IsUint64() {
		return 0, ErrValueOverflow
	}
	return value.Uint64(), nil
}

func multiply(left uint64, right uint64) (uint64, bool) {
	if left == 0 || right == 0 {
		return 0, true
	}
	if left > math.MaxUint64/right {
		return 0, false
	}
	return left * right, true
}

func sameStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func AddressHex(address types.Address) string {
	return gethcommon.HexToAddress(string(address)).Hex()
}
