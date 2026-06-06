package ethcompat

import (
	"encoding/hex"
	"errors"
	"math/big"
	"sort"
	"strings"

	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
)

var ErrInvalidState = errors.New("invalid Ethereum state")

type AccountState struct {
	Address string
	Balance uint64
	Nonce   uint64
	Code    []byte
	Storage map[string][]byte
}

type AccountProof struct {
	Address      string         `json:"address"`
	AccountProof []string       `json:"accountProof"`
	Balance      string         `json:"balance"`
	CodeHash     string         `json:"codeHash"`
	Nonce        string         `json:"nonce"`
	StorageHash  string         `json:"storageHash"`
	StorageProof []StorageProof `json:"storageProof"`
	StateRoot    string         `json:"stateRoot"`
}

type StorageProof struct {
	Key   string   `json:"key"`
	Value string   `json:"value"`
	Proof []string `json:"proof"`
}

func StateRoot(accounts []AccountState) (string, error) {
	stateTrie, _, err := buildStateTrie(accounts)
	if err != nil {
		return "", err
	}
	return stateTrie.Hash().Hex(), nil
}

func GetProof(accounts []AccountState, address string, storageKeys []string) (AccountProof, error) {
	stateTrie, normalized, err := buildStateTrie(accounts)
	if err != nil {
		return AccountProof{}, err
	}
	targetAddress := gethcommon.HexToAddress(address)
	targetKey := canonicalAddress(targetAddress.Hex())
	account := normalized[targetKey]
	if account.Address == "" {
		account = AccountState{Address: targetAddress.Hex(), Storage: map[string][]byte{}}
	}
	storageTrie, storageRoot, err := buildStorageTrie(account)
	if err != nil {
		return AccountProof{}, err
	}
	accountProof, err := proveStateTrie(stateTrie, gethcrypto.Keccak256(targetAddress.Bytes()))
	if err != nil {
		return AccountProof{}, err
	}
	storageProofs := make([]StorageProof, 0, len(storageKeys))
	for _, key := range storageKeys {
		slot := slotHash(key)
		proof, err := proveStateTrie(storageTrie, gethcrypto.Keccak256(slot.Bytes()))
		if err != nil {
			return AccountProof{}, err
		}
		value := slotValue(account.Storage, slot)
		storageProofs = append(storageProofs, StorageProof{
			Key:   slot.Hex(),
			Value: hashQuantity(value),
			Proof: proof,
		})
	}
	return AccountProof{
		Address:      targetAddress.Hex(),
		AccountProof: accountProof,
		Balance:      hexQuantityBig(new(big.Int).SetUint64(account.Balance)),
		CodeHash:     codeHash(account.Code).Hex(),
		Nonce:        hexQuantityBig(new(big.Int).SetUint64(account.Nonce)),
		StorageHash:  storageRoot.Hex(),
		StorageProof: storageProofs,
		StateRoot:    stateTrie.Hash().Hex(),
	}, nil
}

func buildStateTrie(accounts []AccountState) (*trie.StateTrie, map[string]AccountState, error) {
	database := trieDatabase()
	stateTrie, err := trie.NewStateTrie(trie.TrieID(gethtypes.EmptyRootHash), database)
	if err != nil {
		return nil, nil, err
	}
	normalized := normalizeAccounts(accounts)
	keys := make([]string, 0, len(normalized))
	for key := range normalized {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		account := normalized[key]
		if emptyAccount(account) {
			continue
		}
		_, storageRoot, err := buildStorageTrie(account)
		if err != nil {
			return nil, nil, err
		}
		stateAccount := &gethtypes.StateAccount{
			Nonce:    account.Nonce,
			Balance:  new(uint256.Int).SetUint64(account.Balance),
			Root:     storageRoot,
			CodeHash: codeHash(account.Code).Bytes(),
		}
		if err := stateTrie.UpdateAccount(gethcommon.HexToAddress(account.Address), stateAccount, len(account.Code)); err != nil {
			return nil, nil, err
		}
	}
	return stateTrie, normalized, nil
}

func buildStorageTrie(account AccountState) (*trie.StateTrie, gethcommon.Hash, error) {
	database := trieDatabase()
	storageTrie, err := trie.NewStateTrie(trie.TrieID(gethtypes.EmptyRootHash), database)
	if err != nil {
		return nil, gethcommon.Hash{}, err
	}
	slots := make([]string, 0, len(account.Storage))
	for slot := range account.Storage {
		slots = append(slots, slot)
	}
	sort.Strings(slots)
	address := gethcommon.HexToAddress(account.Address)
	for _, slot := range slots {
		key := slotHash(slot)
		value := gethcommon.BytesToHash(account.Storage[slot])
		if value == (gethcommon.Hash{}) {
			continue
		}
		if err := storageTrie.UpdateStorage(address, key.Bytes(), gethcommon.TrimLeftZeroes(value.Bytes())); err != nil {
			return nil, gethcommon.Hash{}, err
		}
	}
	return storageTrie, storageTrie.Hash(), nil
}

func trieDatabase() *triedb.Database {
	return triedb.NewDatabase(rawdb.NewMemoryDatabase(), triedb.HashDefaults)
}

func proveStateTrie(stateTrie *trie.StateTrie, key []byte) ([]string, error) {
	proofDB := memorydb.New()
	if err := stateTrie.Prove(key, proofDB); err != nil {
		return nil, err
	}
	iterator := proofDB.NewIterator(nil, nil)
	defer iterator.Release()
	proof := make([]string, 0)
	for iterator.Next() {
		proof = append(proof, "0x"+hex.EncodeToString(iterator.Value()))
	}
	if err := iterator.Error(); err != nil {
		return nil, err
	}
	return proof, nil
}

func normalizeAccounts(accounts []AccountState) map[string]AccountState {
	normalized := make(map[string]AccountState, len(accounts))
	for _, account := range accounts {
		address := gethcommon.HexToAddress(account.Address)
		key := canonicalAddress(address.Hex())
		current := normalized[key]
		current.Address = address.Hex()
		if account.Balance != 0 {
			current.Balance = account.Balance
		}
		if account.Nonce != 0 {
			current.Nonce = account.Nonce
		}
		if len(account.Code) > 0 {
			current.Code = append([]byte(nil), account.Code...)
		}
		if current.Storage == nil {
			current.Storage = map[string][]byte{}
		}
		for slot, value := range account.Storage {
			current.Storage[slotHash(slot).Hex()] = append([]byte(nil), value...)
		}
		normalized[key] = current
	}
	return normalized
}

func emptyAccount(account AccountState) bool {
	return account.Balance == 0 && account.Nonce == 0 && len(account.Code) == 0 && len(account.Storage) == 0
}

func canonicalAddress(address string) string {
	return strings.ToLower(gethcommon.HexToAddress(address).Hex())
}

func codeHash(code []byte) gethcommon.Hash {
	if len(code) == 0 {
		return gethtypes.EmptyCodeHash
	}
	return gethcrypto.Keccak256Hash(code)
}

func slotHash(slot string) gethcommon.Hash {
	clean := strings.TrimPrefix(slot, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 32 {
		return gethcrypto.Keccak256Hash([]byte(slot))
	}
	return gethcommon.BytesToHash(decoded)
}

func slotValue(storage map[string][]byte, slot gethcommon.Hash) gethcommon.Hash {
	for key, value := range storage {
		if slotHash(key) == slot {
			return gethcommon.BytesToHash(value)
		}
	}
	return gethcommon.Hash{}
}

func hashQuantity(value gethcommon.Hash) string {
	return hexQuantityBytes(value.Bytes())
}

func hexQuantityBig(value *big.Int) string {
	if value == nil || value.Sign() == 0 {
		return "0x0"
	}
	return "0x" + value.Text(16)
}

func hexQuantityBytes(value []byte) string {
	trimmed := gethcommon.TrimLeftZeroes(value)
	if len(trimmed) == 0 {
		return "0x0"
	}
	return "0x" + hex.EncodeToString(trimmed)
}
