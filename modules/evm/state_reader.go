package evm

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func queryEthereumStateRoot(ctx vexoapp.Context, roots *stateRootCache, data []byte) vexoapp.QueryResponse {
	var request StateRootRequest
	if len(data) > 0 {
		if err := json.Unmarshal(data, &request); err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
	}
	root, err := roots.stateRoot(ctx.GoContext(), ctx.Store, request.Height)
	if isHistoricalStateUnavailable(err) {
		return vexoapp.QueryResponse{Code: 3, Log: errHistoricalStateNotFound}
	}
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(map[string]string{"state_root": root})
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

func queryEthereumProof(ctx vexoapp.Context, data []byte) vexoapp.QueryResponse {
	var request ProofRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
	}
	if request.Address == "" {
		return vexoapp.QueryResponse{Code: 2, Log: ErrInvalidEVMQuery.Error()}
	}
	accounts, err := ethereumAccountsAtHeight(ctx.GoContext(), ctx.Store, request.Height)
	if isHistoricalStateUnavailable(err) {
		return vexoapp.QueryResponse{Code: 3, Log: errHistoricalStateNotFound}
	}
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	proof, err := ethcompat.GetProof(accounts, request.Address, request.StorageKeys)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

func accountStateRequestHeight(data []byte) (uint64, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return 0, nil
	}
	var request AccountStateRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return 0, err
	}
	return request.Height, nil
}

func ethereumAccountAtHeight(ctx context.Context, stateStore vexoapp.StateStore, address types.Address, height uint64) (ethcompat.AccountState, bool, error) {
	view, err := stateAtHeight(stateStore, height)
	if err != nil {
		return ethcompat.AccountState{}, false, err
	}
	return ethereumAccountFromCurrentState(ctx, view, address)
}

func ethereumAccountFromCurrentState(ctx context.Context, stateStore vexoapp.StateStore, address types.Address) (ethcompat.AccountState, bool, error) {
	if stateStore == nil {
		return ethcompat.AccountState{}, false, ErrStoreMissing
	}
	account := ethcompat.AccountState{Address: canonicalAddressKey(address), Storage: map[string][]byte{}}
	found := false
	value, err := stateStore.Get(ctx, bankNamespace, evmBankKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		err = nil
	}
	if err != nil {
		return ethcompat.AccountState{}, false, err
	}
	if len(value) > 0 {
		balance, balanceHex, err := decodeEthereumBalance(value)
		if err != nil {
			return ethcompat.AccountState{}, false, err
		}
		account.Balance = balance
		account.BalanceHex = balanceHex
		found = true
	}
	value, err = stateStore.Get(ctx, authNamespace, evmNonceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		err = nil
	}
	if err != nil {
		return ethcompat.AccountState{}, false, err
	}
	if len(value) > 0 {
		if len(value) != 8 {
			return ethcompat.AccountState{}, false, ErrInvalidEVMTx
		}
		account.Nonce = binary.BigEndian.Uint64(value)
		found = true
	}
	value, err = stateStore.Get(ctx, ModuleName, codeKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		err = nil
	}
	if err != nil {
		return ethcompat.AccountState{}, false, err
	}
	if len(value) > 0 {
		account.Code = append([]byte(nil), value...)
		found = true
	}
	prefixStore, ok := stateStore.(vexostore.PrefixKVStore)
	if !ok {
		if found {
			return account, true, nil
		}
		return ethcompat.AccountState{}, false, nil
	}
	pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, storageAccountPrefix(address))
	if err != nil {
		return ethcompat.AccountState{}, false, err
	}
	for _, pair := range pairs {
		_, slot, ok := ethereumStorageKeyParts(pair.Key)
		if !ok {
			continue
		}
		account.Storage[slot] = append([]byte(nil), pair.Value...)
		found = true
	}
	if !found {
		return ethcompat.AccountState{}, false, nil
	}
	return account, true, nil
}

// ethereumAccountsAtHeight returns every Ethereum account as of the committed
// height, or as of the current state for height 0.
func ethereumAccountsAtHeight(ctx context.Context, stateStore vexoapp.StateStore, height uint64) ([]ethcompat.AccountState, error) {
	view, err := stateAtHeight(stateStore, height)
	if err != nil {
		return nil, err
	}
	return ethereumAccountsFromStore(ctx, view)
}

func ethereumAccountsFromStore(ctx context.Context, stateStore vexoapp.StateStore) ([]ethcompat.AccountState, error) {
	prefixStore, ok := stateStore.(vexostore.PrefixKVStore)
	if !ok {
		return nil, ErrStoreMissing
	}
	accounts := map[string]*ethcompat.AccountState{}
	accountFor := func(address types.Address) *ethcompat.AccountState {
		key := canonicalAddressKey(address)
		account := accounts[key]
		if account == nil {
			account = &ethcompat.AccountState{Address: key, Storage: map[string][]byte{}}
			accounts[key] = account
		}
		return account
	}
	balances, err := prefixStore.ExportPrefix(ctx, "bank", nil)
	if err != nil {
		return nil, err
	}
	for _, pair := range balances {
		address := types.Address(string(pair.Key))
		if !strings.HasPrefix(string(address), "0x") || len(pair.Value) == 0 {
			continue
		}
		balance, balanceHex, err := decodeEthereumBalance(pair.Value)
		if err != nil {
			return nil, err
		}
		account := accountFor(address)
		account.Balance = balance
		account.BalanceHex = balanceHex
	}
	nonces, err := prefixStore.ExportPrefix(ctx, "auth", []byte("nonce/"))
	if err != nil {
		return nil, err
	}
	for _, pair := range nonces {
		address := types.Address(strings.TrimPrefix(string(pair.Key), "nonce/"))
		if !strings.HasPrefix(string(address), "0x") {
			continue
		}
		if len(pair.Value) != 8 {
			return nil, ErrInvalidEVMTx
		}
		accountFor(address).Nonce = binary.BigEndian.Uint64(pair.Value)
	}
	codes, err := prefixStore.ExportPrefix(ctx, ModuleName, []byte("code/"))
	if err != nil {
		return nil, err
	}
	for _, pair := range codes {
		address := types.Address(strings.TrimPrefix(string(pair.Key), "code/"))
		if !strings.HasPrefix(string(address), "0x") {
			continue
		}
		accountFor(address).Code = append([]byte(nil), pair.Value...)
	}
	storage, err := prefixStore.ExportPrefix(ctx, ModuleName, []byte("storage/"))
	if err != nil {
		return nil, err
	}
	for _, pair := range storage {
		address, slot, ok := ethereumStorageKeyParts(pair.Key)
		if !ok {
			continue
		}
		accountFor(address).Storage[slot] = append([]byte(nil), pair.Value...)
	}
	result := make([]ethcompat.AccountState, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, *account)
	}
	return result, nil
}

func ethereumStorageKeyParts(key []byte) (types.Address, string, bool) {
	trimmed := strings.TrimPrefix(string(key), "storage/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "0x") || parts[1] == "" {
		return "", "", false
	}
	return types.Address(parts[0]), parts[1], true
}

func createAddress(caller types.Address, code []byte, salt [32]byte) types.Address {
	codeHash := keccak256(code)
	seed := append([]byte{0xff}, addressBytes(caller)...)
	seed = append(seed, salt[:]...)
	seed = append(seed, codeHash...)
	final := keccak256(seed)
	return types.Address("0x" + hex.EncodeToString(final[12:]))
}

func createLegacyAddress(caller types.Address, nonce uint64) types.Address {
	final := keccak256(rlpLegacyCreateAddress(addressBytes(caller), nonce))
	return types.Address("0x" + hex.EncodeToString(final[12:]))
}

func rlpLegacyCreateAddress(address []byte, nonce uint64) []byte {
	address = rightMost20(address)
	noncePayload := rlpEncodeUint(nonce)
	payloadLen := 1 + len(address) + len(noncePayload)
	encoded := make([]byte, 0, 1+payloadLen)
	encoded = append(encoded, byte(0xc0+payloadLen))
	encoded = append(encoded, byte(0x80+len(address)))
	encoded = append(encoded, address...)
	encoded = append(encoded, noncePayload...)
	return encoded
}

func rlpEncodeUint(value uint64) []byte {
	if value == 0 {
		return []byte{0x80}
	}
	raw := make([]byte, 8)
	for index := len(raw) - 1; index >= 0; index-- {
		raw[index] = byte(value)
		value >>= 8
	}
	raw = trimLeftZero(raw)
	if len(raw) == 1 && raw[0] < 0x80 {
		return raw
	}
	encoded := make([]byte, 0, 1+len(raw))
	encoded = append(encoded, byte(0x80+len(raw)))
	encoded = append(encoded, raw...)
	return encoded
}

func trimLeftZero(value []byte) []byte {
	for len(value) > 0 && value[0] == 0 {
		value = value[1:]
	}
	return value
}

func loadCode(ctx context.Context, store vexoapp.StateStore, address types.Address) ([]byte, error) {
	if store == nil {
		return nil, ErrStoreMissing
	}
	code, err := store.Get(ctx, ModuleName, codeKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil, ErrInvalidEVMTx
	}
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), code...), nil
}

type evmStateReader struct {
	store vexoapp.StateStore
}

type callStateReader interface {
	contract.StateReader
	contract.BalanceBigReader
	contract.BalanceReader
	contract.NonceReader
	contract.BlockHashReader
}

func stateReaderForCall(ctx vexoapp.Context, height uint64) (callStateReader, error) {
	if ctx.Store == nil {
		return evmStateReader{}, nil
	}
	view, err := stateAtHeight(ctx.Store, height)
	if err != nil {
		return nil, err
	}
	return evmStateReader{store: view}, nil
}

type overrideStateReader struct {
	base     callStateReader
	accounts map[string]CallStateOverride
}

func newOverrideStateReader(base callStateReader, overrides map[string]CallStateOverride) callStateReader {
	if len(overrides) == 0 {
		return base
	}
	normalized := make(map[string]CallStateOverride, len(overrides))
	for address, override := range overrides {
		if len(override.State) > 0 {
			state := make(map[string]string, len(override.State))
			for slot, value := range override.State {
				state[normalizeSlot(slot)] = value
			}
			override.State = state
		}
		if len(override.StateDiff) > 0 {
			stateDiff := make(map[string]string, len(override.StateDiff))
			for slot, value := range override.StateDiff {
				stateDiff[normalizeSlot(slot)] = value
			}
			override.StateDiff = stateDiff
		}
		normalized[canonicalAddressKey(types.Address(address))] = override
	}
	return overrideStateReader{base: base, accounts: normalized}
}

func (reader overrideStateReader) override(address types.Address) (CallStateOverride, bool) {
	override, found := reader.accounts[canonicalAddressKey(address)]
	return override, found
}

func (reader overrideStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	if override, found := reader.override(address); found && override.Code != "" {
		return decodeOverrideBytes(override.Code)
	}
	return reader.base.Code(ctx, address)
}

func (reader overrideStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	override, found := reader.override(address)
	if !found {
		return reader.base.Storage(ctx, address, slot)
	}
	normalized := normalizeSlot(slot)
	if override.State != nil {
		return decodeOverrideBytes(override.State[normalized])
	}
	if override.StateDiff != nil {
		if value, ok := override.StateDiff[normalized]; ok {
			return decodeOverrideBytes(value)
		}
	}
	return reader.base.Storage(ctx, address, slot)
}

func (reader overrideStateReader) Balance(ctx context.Context, address types.Address) (uint64, error) {
	balance, err := reader.BalanceBig(ctx, address)
	if err != nil {
		return 0, err
	}
	if balance == nil || balance.Sign() == 0 {
		return 0, nil
	}
	if !balance.IsUint64() {
		return 0, ErrInvalidEVMTx
	}
	return balance.Uint64(), nil
}

func (reader overrideStateReader) BalanceBig(ctx context.Context, address types.Address) (*big.Int, error) {
	if override, found := reader.override(address); found && override.Balance != "" {
		return parseOverrideBig(override.Balance)
	}
	return reader.base.BalanceBig(ctx, address)
}

func (reader overrideStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	if override, found := reader.override(address); found && override.Nonce != nil {
		return *override.Nonce, nil
	}
	return reader.base.Nonce(ctx, address)
}

func (reader overrideStateReader) BlockHash(ctx context.Context, height uint64) (types.Hash, error) {
	return reader.base.BlockHash(ctx, height)
}

func decodeOverrideBytes(value string) ([]byte, error) {
	if value == "" || value == "0x" {
		return nil, nil
	}
	clean := strings.TrimPrefix(value, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil {
		return nil, ErrInvalidEVMQuery
	}
	return decoded, nil
}

func parseOverrideBig(value string) (*big.Int, error) {
	if value == "" {
		return new(big.Int), nil
	}
	base := 10
	clean := value
	if strings.HasPrefix(value, "0x") {
		base = 16
		clean = strings.TrimPrefix(value, "0x")
	}
	parsed, ok := new(big.Int).SetString(clean, base)
	if !ok || parsed.Sign() < 0 || parsed.BitLen() > 256 {
		return nil, ErrInvalidEVMQuery
	}
	return parsed, nil
}

func (reader evmStateReader) Code(ctx context.Context, address types.Address) ([]byte, error) {
	if reader.store == nil {
		return nil, ErrStoreMissing
	}
	code, err := reader.store.Get(ctx, ModuleName, codeKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), code...), nil
}

func (reader evmStateReader) Storage(ctx context.Context, address types.Address, slot string) ([]byte, error) {
	if reader.store == nil {
		return nil, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, ModuleName, storageKey(address, slot))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), value...), nil
}

func (reader evmStateReader) Balance(ctx context.Context, address types.Address) (uint64, error) {
	balance, err := reader.BalanceBig(ctx, address)
	if err != nil {
		return 0, err
	}
	if balance == nil || balance.Sign() == 0 {
		return 0, nil
	}
	if !balance.IsUint64() {
		return 0, ErrInvalidEVMTx
	}
	return balance.Uint64(), nil
}

func (reader evmStateReader) BalanceBig(ctx context.Context, address types.Address) (*big.Int, error) {
	if reader.store == nil {
		return nil, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, "bank", evmBankKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return new(big.Int), nil
	}
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return new(big.Int), nil
	}
	if len(value) > 32 {
		return nil, ErrInvalidEVMTx
	}
	return new(big.Int).SetBytes(value), nil
}

func (reader evmStateReader) Nonce(ctx context.Context, address types.Address) (uint64, error) {
	if reader.store == nil {
		return 0, ErrStoreMissing
	}
	value, err := reader.store.Get(ctx, "auth", evmNonceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, ErrInvalidEVMTx
	}
	return binary.BigEndian.Uint64(value), nil
}

func (reader evmStateReader) BlockHash(ctx context.Context, height uint64) (types.Hash, error) {
	var zero types.Hash
	if reader.store == nil {
		return zero, ErrStoreMissing
	}
	blockStore, ok := reader.store.(interface {
		BlockByHeight(context.Context, types.Height) (vexostore.BlockRecord, error)
	})
	if !ok {
		return zero, nil
	}
	record, err := blockStore.BlockByHeight(ctx, types.Height(height))
	if errors.Is(err, vexostore.ErrBlockNotFound) {
		return zero, nil
	}
	if err != nil {
		return zero, err
	}
	return record.Hash, nil
}
