package evm

import (
	"context"
	"encoding/binary"
	"math/big"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func persistExecutionResult(ctx context.Context, store vexoapp.StateStore, defaultAddress types.Address, result contract.Result) error {
	writes, err := executionResultWrites(ctx, store, defaultAddress, result)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, writes)
}

func executionResultWrites(ctx context.Context, store vexoapp.StateStore, defaultAddress types.Address, result contract.Result) ([]vexostore.KVWrite, error) {
	writes := make([]vexostore.KVWrite, 0, len(result.CodeWrites)+len(result.StorageWrites)+len(result.BalanceWrites)+len(result.NonceWrites)+len(result.AccountDeletions)*3)
	codeWrites, err := codeKVWrites(result.CodeWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, codeWrites...)
	storageWrites, err := storageKVWrites(defaultAddress, result.StorageWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, storageWrites...)
	balanceWrites, err := balanceKVWrites(result.BalanceWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, balanceWrites...)
	nonceWrites, err := nonceKVWrites(result.NonceWrites)
	if err != nil {
		return nil, err
	}
	writes = append(writes, nonceWrites...)
	deletionWrites, err := accountDeletionKVWrites(ctx, store, result.AccountDeletions)
	if err != nil {
		return nil, err
	}
	writes = append(writes, deletionWrites...)
	return writes, nil
}

func persistCodeWrites(ctx context.Context, store vexoapp.StateStore, writes []contract.CodeWrite) error {
	kvWrites, err := codeKVWrites(writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func codeKVWrites(writes []contract.CodeWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		if write.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		if write.Delete {
			kvWrites = append(kvWrites, deleteWrite(ModuleName, codeKey(write.Address)))
			continue
		}
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: ModuleName, Key: codeKey(write.Address), Value: append([]byte(nil), write.Code...)})
	}
	return kvWrites, nil
}

func persistAccountDeletions(ctx context.Context, store vexoapp.StateStore, deletions []contract.AccountDeletion) error {
	writes, err := accountDeletionKVWrites(ctx, store, deletions)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, writes)
}

func accountDeletionKVWrites(ctx context.Context, store vexoapp.StateStore, deletions []contract.AccountDeletion) ([]vexostore.KVWrite, error) {
	writes := make([]vexostore.KVWrite, 0, len(deletions)*3)
	for _, deletion := range deletions {
		if deletion.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		writes = append(writes,
			deleteWrite(ModuleName, codeKey(deletion.Address)),
			deleteWrite("bank", evmBankKey(deletion.Address)),
			deleteWrite("auth", evmNonceKey(deletion.Address)),
		)
		prefixStore, ok := store.(vexostore.PrefixKVStore)
		if !ok {
			continue
		}
		pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, storageAccountPrefix(deletion.Address))
		if err != nil {
			return nil, err
		}
		for _, pair := range pairs {
			writes = append(writes, deleteWrite(ModuleName, pair.Key))
		}
	}
	return writes, nil
}

func persistStorageWrites(ctx context.Context, store vexoapp.StateStore, defaultAddress types.Address, writes []contract.StorageWrite) error {
	kvWrites, err := storageKVWrites(defaultAddress, writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func storageKVWrites(defaultAddress types.Address, writes []contract.StorageWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		address := write.Address
		if address == "" {
			address = defaultAddress
		}
		if address == "" || write.Slot == "" {
			return nil, ErrInvalidEVMTx
		}
		key := storageKey(address, write.Slot)
		if write.Delete {
			kvWrites = append(kvWrites, deleteWrite(ModuleName, key))
			continue
		}
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: ModuleName, Key: key, Value: append([]byte(nil), write.Value...)})
	}
	return kvWrites, nil
}

func persistBalanceWrites(ctx context.Context, store vexoapp.StateStore, writes []contract.BalanceWrite) error {
	kvWrites, err := balanceKVWrites(writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func balanceKVWrites(writes []contract.BalanceWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		if write.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		encoded, err := encodeEthereumBalance(write)
		if err != nil {
			return nil, err
		}
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: "bank", Key: evmBankKey(write.Address), Value: encoded})
	}
	return kvWrites, nil
}

func encodeEthereumBalance(write contract.BalanceWrite) ([]byte, error) {
	if write.BalanceBig != nil {
		if write.BalanceBig.Sign() < 0 || write.BalanceBig.BitLen() > 256 {
			return nil, ErrInvalidEVMTx
		}
		encoded := trimLeftZero(write.BalanceBig.FillBytes(make([]byte, 32)))
		if len(encoded) == 0 {
			return []byte{0}, nil
		}
		return encoded, nil
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], write.Balance)
	return encoded[:], nil
}

func decodeEthereumBalance(value []byte) (uint64, string, error) {
	if len(value) == 0 {
		return 0, "", nil
	}
	if len(value) > 32 {
		return 0, "", ErrInvalidEVMTx
	}
	balance := new(big.Int).SetBytes(value)
	if !balance.IsUint64() {
		return 0, hexQuantityBigLocal(balance), nil
	}
	return balance.Uint64(), "", nil
}

func accountBalanceHex(account ethcompat.AccountState) string {
	if account.BalanceHex != "" {
		return account.BalanceHex
	}
	return hexQuantityLocal(account.Balance)
}

func persistNonceWrites(ctx context.Context, store vexoapp.StateStore, writes []contract.NonceWrite) error {
	kvWrites, err := nonceKVWrites(writes)
	if err != nil {
		return err
	}
	return applyKVWrites(ctx, store, kvWrites)
}

func nonceKVWrites(writes []contract.NonceWrite) ([]vexostore.KVWrite, error) {
	kvWrites := make([]vexostore.KVWrite, 0, len(writes))
	for _, write := range writes {
		if write.Address == "" {
			return nil, ErrInvalidEVMTx
		}
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], write.Nonce)
		kvWrites = append(kvWrites, vexostore.KVWrite{Namespace: "auth", Key: evmNonceKey(write.Address), Value: append([]byte(nil), encoded[:]...)})
	}
	return kvWrites, nil
}
