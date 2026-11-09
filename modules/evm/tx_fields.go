package evm

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/types"
	"golang.org/x/crypto/sha3"
)

func txGasPrice(tx types.Tx) uint64 {
	if gasPrice, found := vexoapp.TxUintTag(tx, ethcompat.TagGasPrice); found {
		return gasPrice
	}
	meta := vexoapp.ParseTxMeta(tx)
	if meta.Gas > 0 && meta.Fee > 0 {
		return meta.Fee / meta.Gas
	}
	return 0
}

func applyInvocationGasPricing(tx types.Tx, invocation *contract.Invocation) {
	if invocation == nil {
		return
	}
	if gasPrice := txGasPriceBig(tx); gasPrice != nil {
		invocation.GasPriceBig = gasPrice
		if gasPrice.IsUint64() {
			invocation.GasPrice = gasPrice.Uint64()
		}
	}
	if gasFeeCap := txGasFeeCapBig(tx); gasFeeCap != nil {
		invocation.GasFeeCapBig = gasFeeCap
		if gasFeeCap.IsUint64() {
			invocation.GasFeeCap = gasFeeCap.Uint64()
		}
	}
	if gasTipCap := txGasTipCapBig(tx); gasTipCap != nil {
		invocation.GasTipCapBig = gasTipCap
		if gasTipCap.IsUint64() {
			invocation.GasTipCap = gasTipCap.Uint64()
		}
	}
	if blobGasFeeCap := txBlobGasFeeCapBig(tx); blobGasFeeCap != nil {
		invocation.BlobGasFeeCapBig = blobGasFeeCap
		if blobGasFeeCap.IsUint64() {
			invocation.BlobGasFeeCap = blobGasFeeCap.Uint64()
		}
	}
}

func txGasPriceBig(tx types.Tx) *big.Int {
	if gasPrice, found := txAmountTagBig(tx, ethcompat.TagGasPrice); found {
		return gasPrice
	}
	meta := vexoapp.ParseTxMeta(tx)
	fee := metaFeeForEVM(meta)
	if meta.Gas > 0 && fee.Sign() > 0 {
		return fee.Div(fee, new(big.Int).SetUint64(meta.Gas))
	}
	return nil
}

func txGasFeeCap(tx types.Tx) uint64 {
	if feeCap, found := vexoapp.TxUintTag(tx, ethcompat.TagMaxFeePerGas); found {
		return feeCap
	}
	return txGasPrice(tx)
}

func txGasFeeCapBig(tx types.Tx) *big.Int {
	if feeCap, found := txAmountTagBig(tx, ethcompat.TagMaxFeePerGas); found {
		return feeCap
	}
	return txGasPriceBig(tx)
}

func txGasTipCap(tx types.Tx) uint64 {
	if tipCap, found := vexoapp.TxUintTag(tx, ethcompat.TagMaxPriorityFeePerGas); found {
		return tipCap
	}
	return txGasPrice(tx)
}

func txGasTipCapBig(tx types.Tx) *big.Int {
	if tipCap, found := txAmountTagBig(tx, ethcompat.TagMaxPriorityFeePerGas); found {
		return tipCap
	}
	return txGasPriceBig(tx)
}

func txBlobGasFeeCap(tx types.Tx) uint64 {
	if feeCap, found := vexoapp.TxUintTag(tx, ethcompat.TagBlobGasFeeCap); found {
		return feeCap
	}
	return 0
}

func txBlobGasFeeCapBig(tx types.Tx) *big.Int {
	if feeCap, found := txAmountTagBig(tx, ethcompat.TagBlobGasFeeCap); found {
		return feeCap
	}
	return nil
}

func txAmountTagBig(tx types.Tx, key string) (*big.Int, bool) {
	value, found := vexoapp.TxAmountBigTag(tx, key)
	if !found || value.Sign() < 0 || value.BitLen() > 256 {
		return nil, false
	}
	return value, true
}

func metaFeeForEVM(meta vexoapp.TxMeta) *big.Int {
	if meta.FeeBig != nil {
		return new(big.Int).Set(meta.FeeBig)
	}
	return new(big.Int).SetUint64(meta.Fee)
}

func txNonce(tx types.Tx) uint64 {
	if nonce, found := vexoapp.TxUintTag(tx, "nonce"); found {
		return nonce
	}
	return 0
}

func txRawEthereum(tx types.Tx) string {
	raw, _ := vexoapp.TxTag(tx, ethcompat.TagRaw)
	return raw
}

func txGasLimit(tx types.Tx, fallback uint64) uint64 {
	if !ethcompat.IsEthereumTx(tx) {
		return fallback
	}
	if gas, found := vexoapp.TxUintTag(tx, "gas"); found && gas > 0 {
		return gas
	}
	if gas, found := vexoapp.TxUintTag(tx, "gas_limit"); found && gas > 0 {
		return gas
	}
	return fallback
}

func txBlobHashes(tx types.Tx) []types.Hash {
	return parseBlobHashes(txBlobHashStrings(tx))
}

func txBlobHashStrings(tx types.Tx) []string {
	encoded, found := vexoapp.TxTag(tx, ethcompat.TagBlobHashes)
	if !found || encoded == "" {
		return nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	var hashes []string
	if err := json.Unmarshal(raw, &hashes); err != nil {
		return nil
	}
	out := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		if decoded, err := hex.DecodeString(strings.TrimPrefix(hash, "0x")); err == nil && len(decoded) == len(types.Hash{}) {
			out = append(out, "0x"+strings.ToLower(hex.EncodeToString(decoded)))
		}
	}
	return out
}

func sameBlobHashes(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !strings.EqualFold(left[index], right[index]) {
			return false
		}
	}
	return true
}

func parseBlobHashes(raw []string) []types.Hash {
	if len(raw) == 0 {
		return nil
	}
	hashes := make([]types.Hash, 0, len(raw))
	for _, value := range raw {
		decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
		if err != nil || len(decoded) != len(types.Hash{}) {
			return nil
		}
		var hash types.Hash
		copy(hash[:], decoded)
		hashes = append(hashes, hash)
	}
	return hashes
}

func evmBankKey(address types.Address) []byte {
	return []byte(canonicalAddressKey(address))
}

// evmNonceKey is the auth-namespace key of an account's nonce. It is the key
// the ante handler keeps the account sequence under, so a nonce bumped by
// either side is the nonce the other side checks.
func evmNonceKey(address types.Address) []byte {
	return vexoapp.NonceKey(address)
}

func create2Salt(raw string) [32]byte {
	clean := strings.TrimPrefix(raw, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err == nil && len(decoded) <= 32 {
		var salt [32]byte
		copy(salt[32-len(decoded):], decoded)
		return salt
	}
	var salt [32]byte
	copy(salt[:], keccak256([]byte(raw)))
	return salt
}

func addressBytes(address types.Address) []byte {
	clean := strings.TrimPrefix(string(address), "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err == nil && len(decoded) <= 20 {
		out := make([]byte, 20)
		copy(out[20-len(decoded):], decoded)
		return out
	}
	hash := keccak256([]byte(address))
	return hash[12:]
}

func rightMost20(raw []byte) []byte {
	if len(raw) > 20 {
		return raw[len(raw)-20:]
	}
	if len(raw) == 20 {
		return raw
	}
	padded := make([]byte, 20)
	copy(padded[20-len(raw):], raw)
	return padded
}

// canonicalAddressKey is the identity an account is stored under. It is the
// identity the bank module and the ante handler use too, so the EVM sees the
// balance and nonce of exactly the accounts they do.
func canonicalAddressKey(address types.Address) string {
	return vexoapp.NormalizeActor(address)
}

func normalizeSlot(slot string) string {
	clean := strings.TrimPrefix(slot, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 32 {
		decoded = keccak256([]byte(slot))
	}
	padded := make([]byte, 32)
	copy(padded[32-len(decoded):], decoded)
	return "0x" + hex.EncodeToString(padded)
}

func keccak256(values ...[]byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	for _, value := range values {
		_, _ = hasher.Write(value)
	}
	return hasher.Sum(nil)
}

func cloneMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
