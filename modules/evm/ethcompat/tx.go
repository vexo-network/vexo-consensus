package ethcompat

import (
	"crypto/sha256"
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
	gethcore "github.com/ethereum/go-ethereum/core"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	gethparams "github.com/ethereum/go-ethereum/params"
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
	TagValue                = "eth_value"
	TagMaxFeePerGas         = "eth_max_fee_per_gas"
	TagMaxPriorityFeePerGas = "eth_max_priority_fee_per_gas"
	TagAccessList           = "eth_access_list"
	TagBlobBaseFee          = "eth_blob_base_fee"
	TagBlobGas              = "eth_blob_gas"
	TagBlobGasFeeCap        = "eth_blob_gas_fee_cap"
	TagBlobHashes           = "eth_blob_hashes"
	TagBlobSidecar          = "eth_blob_sidecar"
)

var (
	ErrInvalidRawTransaction = errors.New("invalid Ethereum raw transaction")
	ErrChainIDMismatch       = errors.New("Ethereum transaction chain ID mismatch")
	ErrValueOverflow         = errors.New("Ethereum transaction value overflows Vexo uint64 amount")
	ErrSignatureMismatch     = errors.New("Ethereum transaction signature does not match canonical tags")
	ErrFeeCapTooLow          = errors.New("Ethereum fee cap is below base fee")
	ErrTipCapAboveFeeCap     = errors.New("Ethereum priority fee cap is above fee cap")
	ErrUnprotectedLegacyTx   = errors.New("unprotected Ethereum legacy transaction is disabled")
	ErrBlobFeeCapTooLow      = errors.New("Ethereum blob fee cap is below blob base fee")
	ErrInvalidBlobSidecar    = errors.New("Ethereum blob sidecar proof is invalid")
)

type DecodeOptions struct {
	ChainID                uint64
	BaseFee                uint64
	BlobBaseFee            uint64
	VM                     string
	AllowUnprotectedLegacy bool
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
	BlobGas              uint64
	BlobGasFeeCap        uint64
	BlobFee              uint64
	BlobHashes           []string
	Value                uint64
	ValueBig             *big.Int
	Input                string
	AccessList           []contract.AccessListEntry
	ContractCreation     bool
	ChainID              uint64
}

type BlobSidecarBundle struct {
	BlobHashes  []string `json:"blob_hashes"`
	Blobs       []string `json:"blobs"`
	Commitments []string `json:"commitments"`
	Proofs      []string `json:"proofs"`
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
	if ethTx.Type() == gethtypes.LegacyTxType && !ethTx.Protected() && !options.AllowUnprotectedLegacy {
		return DecodedTransaction{}, ErrUnprotectedLegacyTx
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
	valueBig := new(big.Int).Set(ethTx.Value())
	value := uint64(0)
	if valueBig.IsUint64() {
		value = valueBig.Uint64()
	}
	gasPrice, err := effectiveGasPrice(&ethTx, options.BaseFee)
	if err != nil {
		return DecodedTransaction{}, err
	}
	fee, ok := multiply(ethTx.Gas(), gasPrice)
	if !ok {
		return DecodedTransaction{}, ErrValueOverflow
	}
	blobGas := ethTx.BlobGas()
	blobGasFeeCap, err := uint64Big(ethTx.BlobGasFeeCap())
	if err != nil {
		return DecodedTransaction{}, err
	}
	blobFee := uint64(0)
	if blobGas > 0 {
		if options.BlobBaseFee > 0 && blobGasFeeCap < options.BlobBaseFee {
			return DecodedTransaction{}, ErrBlobFeeCapTooLow
		}
		blobUnitPrice := blobGasFeeCap
		if options.BlobBaseFee > 0 {
			blobUnitPrice = options.BlobBaseFee
		}
		var ok bool
		blobFee, ok = multiply(blobGas, blobUnitPrice)
		if !ok || fee > math.MaxUint64-blobFee {
			return DecodedTransaction{}, ErrValueOverflow
		}
		fee += blobFee
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
		TagValue:                valueBig.String(),
		TagMaxFeePerGas:         strconv.FormatUint(maxFee, 10),
		TagMaxPriorityFeePerGas: strconv.FormatUint(maxPriority, 10),
		TagBlobBaseFee:          strconv.FormatUint(options.BlobBaseFee, 10),
		TagBlobGas:              strconv.FormatUint(blobGas, 10),
		TagBlobGasFeeCap:        strconv.FormatUint(blobGasFeeCap, 10),
	}
	blobHashes := blobHashesHex(ethTx.BlobHashes())
	if sidecar := ethTx.BlobTxSidecar(); sidecar != nil {
		if err := VerifyBlobSidecar(sidecar, blobHashes); err != nil {
			return DecodedTransaction{}, err
		}
	}
	if len(blobHashes) > 0 {
		encodedHashes, err := json.Marshal(blobHashes)
		if err != nil {
			return DecodedTransaction{}, err
		}
		tags[TagBlobHashes] = base64.RawStdEncoding.EncodeToString(encodedHashes)
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
		BlobGas:              blobGas,
		BlobGasFeeCap:        blobGasFeeCap,
		BlobFee:              blobFee,
		BlobHashes:           blobHashes,
		Value:                value,
		ValueBig:             valueBig,
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
				valueBig.String(),
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
				valueBig.String(),
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

func VerifyBlobSidecar(sidecar *gethtypes.BlobTxSidecar, expectedHashes []string) error {
	if sidecar == nil {
		if len(expectedHashes) > 0 {
			return ErrInvalidBlobSidecar
		}
		return nil
	}
	if len(sidecar.Blobs) == 0 ||
		len(sidecar.Blobs) != len(sidecar.Commitments) ||
		len(sidecar.Blobs) != len(sidecar.Proofs) ||
		len(expectedHashes) != len(sidecar.Blobs) {
		return ErrInvalidBlobSidecar
	}
	commitmentHashes := make([]gethcommon.Hash, len(sidecar.Commitments))
	for index := range sidecar.Blobs {
		if err := kzg4844.VerifyBlobProof(&sidecar.Blobs[index], sidecar.Commitments[index], sidecar.Proofs[index]); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidBlobSidecar, err)
		}
		versionedHash := kzg4844.CalcBlobHashV1(sha256.New(), &sidecar.Commitments[index])
		if !kzg4844.IsValidVersionedHash(versionedHash[:]) {
			return ErrInvalidBlobSidecar
		}
		commitmentHashes[index] = gethcommon.Hash(versionedHash)
		if !strings.EqualFold(commitmentHashes[index].Hex(), expectedHashes[index]) {
			return ErrInvalidBlobSidecar
		}
	}
	if err := sidecar.ValidateBlobCommitmentHashes(commitmentHashes); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBlobSidecar, err)
	}
	return nil
}

func VerifyBlobSidecarBytes(expectedHashes []string, blobs [][]byte, commitments [][]byte, proofs [][]byte) error {
	if len(blobs) != len(commitments) || len(blobs) != len(proofs) {
		return ErrInvalidBlobSidecar
	}
	sidecar := &gethtypes.BlobTxSidecar{
		Blobs:       make([]kzg4844.Blob, len(blobs)),
		Commitments: make([]kzg4844.Commitment, len(commitments)),
		Proofs:      make([]kzg4844.Proof, len(proofs)),
	}
	for index := range blobs {
		if len(blobs[index]) != len(kzg4844.Blob{}) ||
			len(commitments[index]) != len(kzg4844.Commitment{}) ||
			len(proofs[index]) != len(kzg4844.Proof{}) {
			return ErrInvalidBlobSidecar
		}
		copy(sidecar.Blobs[index][:], blobs[index])
		copy(sidecar.Commitments[index][:], commitments[index])
		copy(sidecar.Proofs[index][:], proofs[index])
	}
	return VerifyBlobSidecar(sidecar, expectedHashes)
}

func EncodeBlobSidecarBundle(bundle BlobSidecarBundle) (string, error) {
	normalized, err := NormalizeBlobSidecarBundle(bundle)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(encoded), nil
}

func DecodeBlobSidecarBundle(encoded string) (BlobSidecarBundle, error) {
	if encoded == "" {
		return BlobSidecarBundle{}, ErrInvalidBlobSidecar
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return BlobSidecarBundle{}, err
	}
	return DecodeBlobSidecarBundleJSON(raw)
}

func DecodeBlobSidecarBundleJSON(raw []byte) (BlobSidecarBundle, error) {
	var bundle BlobSidecarBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return BlobSidecarBundle{}, err
	}
	return NormalizeBlobSidecarBundle(bundle)
}

func NormalizeBlobSidecarBundle(bundle BlobSidecarBundle) (BlobSidecarBundle, error) {
	bundle.BlobHashes = normalizeHashList(bundle.BlobHashes)
	bundle.Blobs = normalizeHexList(bundle.Blobs)
	bundle.Commitments = normalizeHexList(bundle.Commitments)
	bundle.Proofs = normalizeHexList(bundle.Proofs)
	if err := VerifyBlobSidecarBundle(bundle); err != nil {
		return BlobSidecarBundle{}, err
	}
	return bundle, nil
}

func VerifyBlobSidecarBundle(bundle BlobSidecarBundle) error {
	blobs, err := decodeFixedHexList(bundle.Blobs, len(kzg4844.Blob{}))
	if err != nil {
		return err
	}
	commitments, err := decodeFixedHexList(bundle.Commitments, len(kzg4844.Commitment{}))
	if err != nil {
		return err
	}
	proofs, err := decodeFixedHexList(bundle.Proofs, len(kzg4844.Proof{}))
	if err != nil {
		return err
	}
	return VerifyBlobSidecarBytes(bundle.BlobHashes, blobs, commitments, proofs)
}

func BlobSidecarBundleFromGeth(sidecar *gethtypes.BlobTxSidecar, expectedHashes []string) (BlobSidecarBundle, error) {
	if err := VerifyBlobSidecar(sidecar, expectedHashes); err != nil {
		return BlobSidecarBundle{}, err
	}
	bundle := BlobSidecarBundle{
		BlobHashes:  normalizeHashList(expectedHashes),
		Blobs:       make([]string, 0, len(sidecar.Blobs)),
		Commitments: make([]string, 0, len(sidecar.Commitments)),
		Proofs:      make([]string, 0, len(sidecar.Proofs)),
	}
	for _, blob := range sidecar.Blobs {
		bundle.Blobs = append(bundle.Blobs, "0x"+hex.EncodeToString(blob[:]))
	}
	for _, commitment := range sidecar.Commitments {
		bundle.Commitments = append(bundle.Commitments, "0x"+hex.EncodeToString(commitment[:]))
	}
	for _, proof := range sidecar.Proofs {
		bundle.Proofs = append(bundle.Proofs, "0x"+hex.EncodeToString(proof[:]))
	}
	return bundle, nil
}

func ValidateCanonicalTx(tx types.Tx, expectedChainID uint64) error {
	return ValidateCanonicalTxWithOptions(tx, DecodeOptions{ChainID: expectedChainID})
}

func ValidateCanonicalTxWithOptions(tx types.Tx, options DecodeOptions) error {
	raw, found := vexoapp.TxTag(tx, TagRaw)
	if !found || raw == "" {
		return ErrInvalidRawTransaction
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		return err
	}
	baseFee, _ := parseUintTag(canonical.Tags, TagBaseFee)
	blobBaseFee, _ := parseUintTag(canonical.Tags, TagBlobBaseFee)
	options.BaseFee = baseFee
	options.BlobBaseFee = blobBaseFee
	decoded, err := DecodeRawTransaction(raw, options)
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
	for _, key := range []string{"fee", "gas", "signer", "nonce", TagHash, TagRaw, TagType, TagInput, TagChainID, TagBaseFee, TagGasPrice, TagValue, TagMaxFeePerGas, TagMaxPriorityFeePerGas, TagAccessList, TagBlobBaseFee, TagBlobGas, TagBlobGasFeeCap, TagBlobHashes} {
		if canonical.Tags[key] != decodedCanonical.Tags[key] {
			return ErrSignatureMismatch
		}
	}
	return nil
}

func ValidateCanonicalTxForExecution(tx types.Tx, options DecodeOptions) error {
	raw, found := vexoapp.TxTag(tx, TagRaw)
	if !found || raw == "" {
		return ErrInvalidRawTransaction
	}
	canonical, err := vexoapp.ParseCanonicalTx(tx)
	if err != nil {
		return err
	}
	decoded, err := DecodeRawTransaction(raw, options)
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
	for _, key := range []string{"gas", "signer", "nonce", TagHash, TagRaw, TagType, TagInput, TagChainID, TagValue, TagMaxFeePerGas, TagMaxPriorityFeePerGas, TagAccessList, TagBlobGas, TagBlobGasFeeCap, TagBlobHashes} {
		if canonical.Tags[key] != decodedCanonical.Tags[key] {
			return ErrSignatureMismatch
		}
	}
	return nil
}

func IntrinsicGas(data []byte, accessList []contract.AccessListEntry, contractCreation bool, timestamp uint64) (uint64, error) {
	return IntrinsicGasWithChainConfigJSON(data, accessList, contractCreation, timestamp, "")
}

func IntrinsicGasWithChainConfigJSON(data []byte, accessList []contract.AccessListEntry, contractCreation bool, timestamp uint64, chainConfigJSON string) (uint64, error) {
	chainConfig := gethparams.AllDevChainProtocolChanges
	if strings.TrimSpace(chainConfigJSON) != "" {
		var custom gethparams.ChainConfig
		if err := json.Unmarshal([]byte(chainConfigJSON), &custom); err != nil {
			return 0, err
		}
		if err := custom.CheckConfigForkOrder(); err != nil {
			return 0, err
		}
		chainConfig = &custom
	}
	rules := chainConfig.Rules(new(big.Int), true, timestamp)
	cost, err := gethcore.IntrinsicGas(data, gethAccessList(accessList), nil, contractCreation, rules.IsHomestead, rules.IsIstanbul, rules.IsShanghai, rules.IsAmsterdam)
	if err != nil {
		return 0, err
	}
	required := cost.Sum()
	if rules.IsPrague {
		floor, err := gethcore.FloorDataGas(rules, data, gethAccessList(accessList))
		if err != nil {
			return 0, err
		}
		if floor > required {
			required = floor
		}
	}
	return required, nil
}

func gethAccessList(entries []contract.AccessListEntry) gethtypes.AccessList {
	if len(entries) == 0 {
		return nil
	}
	out := make(gethtypes.AccessList, 0, len(entries))
	for _, entry := range entries {
		item := gethtypes.AccessTuple{
			Address:     gethcommon.HexToAddress(string(entry.Address)),
			StorageKeys: make([]gethcommon.Hash, 0, len(entry.StorageKeys)),
		}
		for _, slot := range entry.StorageKeys {
			item.StorageKeys = append(item.StorageKeys, gethcommon.HexToHash(slot))
		}
		out = append(out, item)
	}
	return out
}

func parseUintTag(tags map[string]string, key string) (uint64, bool) {
	if len(tags) == 0 {
		return 0, false
	}
	value, found := tags[key]
	if !found || value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}

func decodeFixedHexList(items []string, expectedBytes int) ([][]byte, error) {
	if len(items) == 0 {
		return nil, ErrInvalidBlobSidecar
	}
	out := make([][]byte, 0, len(items))
	for _, item := range items {
		raw, err := hex.DecodeString(strings.TrimPrefix(normalizeHex(item), "0x"))
		if err != nil || len(raw) != expectedBytes {
			return nil, ErrInvalidBlobSidecar
		}
		out = append(out, raw)
	}
	return out, nil
}

func normalizeHashList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, gethcommon.HexToHash(item).Hex())
	}
	return out
}

func normalizeHexList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, normalizeHex(item))
	}
	return out
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

func blobHashesHex(hashes []gethcommon.Hash) []string {
	if len(hashes) == 0 {
		return nil
	}
	out := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		out = append(out, hash.Hex())
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
	if tx.Type() == gethtypes.LegacyTxType || tx.Type() == gethtypes.AccessListTxType {
		gasPrice, err := uint64Big(tx.GasPrice())
		if err != nil {
			return 0, err
		}
		if baseFee > 0 && gasPrice < baseFee {
			return 0, ErrFeeCapTooLow
		}
		return gasPrice, nil
	}
	feeCap, err := uint64Big(tx.GasFeeCap())
	if err != nil {
		return 0, err
	}
	tipCap, err := uint64Big(tx.GasTipCap())
	if err != nil {
		return 0, err
	}
	if tipCap > feeCap {
		return 0, ErrTipCapAboveFeeCap
	}
	if feeCap < baseFee {
		return 0, ErrFeeCapTooLow
	}
	if baseFee == 0 {
		return feeCap, nil
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
