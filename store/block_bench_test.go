package store

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
)

// BenchmarkCommitBlockLikeTheRuntime commits blocks the way the runtime does: the application reads
// the state each transfer changes, the application's app hash and the runtime's state roots each
// ask for the root of every namespace over the block's writes, and then the block commits with its
// writes. Each block makes a transfer's writes for 300
// of 20,000 accounts (both balances, the sender's nonce and the fee collector), over a history that
// earlier blocks already built.
func BenchmarkCommitBlockLikeTheRuntime(b *testing.B) {
	const accounts, transfers, warmup = 20_000, 300, 200
	ctx := context.Background()
	storage, err := OpenPebbleWithOptions(b.TempDir(), PebbleOptions{})
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()
	namespaces := []string{"auth", "bank", "evm", "governance", "ibc", "params", "slashing", "staking", "core", "validators"}
	genesis := make([]KVWrite, 0, 2*accounts)
	for account := range accounts {
		genesis = append(genesis,
			KVWrite{Namespace: "bank", Key: accountKey("balance", account), Value: []byte("1000000")},
			KVWrite{Namespace: "auth", Key: accountKey("nonce", account), Value: []byte("0")})
	}
	if err := storage.SetBatch(ctx, genesis); err != nil {
		b.Fatal(err)
	}
	commit := func(height types.Height) {
		writes := make([]KVWrite, 0, 4*transfers)
		for transfer := range transfers {
			sender := int(uint64(height)*7919+uint64(transfer)*104729) % accounts
			receiver := (sender + 1 + transfer) % accounts
			value := []byte(fmt.Sprintf("%d", height))
			for _, read := range []KVWrite{
				{Namespace: "bank", Key: accountKey("balance", sender)},
				{Namespace: "bank", Key: accountKey("balance", receiver)},
				{Namespace: "auth", Key: accountKey("nonce", sender)},
			} {
				if _, err := storage.Get(ctx, read.Namespace, read.Key); err != nil {
					b.Fatal(err)
				}
			}
			writes = append(writes,
				KVWrite{Namespace: "bank", Key: accountKey("balance", sender), Value: value},
				KVWrite{Namespace: "bank", Key: accountKey("balance", receiver), Value: value},
				KVWrite{Namespace: "auth", Key: accountKey("nonce", sender), Value: value},
				KVWrite{Namespace: "bank", Key: []byte("fee_collector"), Value: value})
		}
		for _, namespace := range namespaces {
			if _, err := storage.RootWithWrites(ctx, namespace, writes); err != nil {
				b.Fatal(err)
			}
		}
		roots := make([]StateRootRecord, 0, len(namespaces))
		for _, namespace := range namespaces {
			root, err := storage.RootWithWrites(ctx, namespace, writes)
			if err != nil {
				b.Fatal(err)
			}
			roots = append(roots, StateRootRecord{Height: height, Namespace: namespace, Root: root})
		}
		var hash types.Hash
		binary.BigEndian.PutUint64(hash[:], uint64(height))
		block := BlockRecord{Block: types.Block{Header: types.Header{Height: height}}, Hash: hash}
		if err := storage.CommitBlockStateWithWrites(ctx, writes, block, StateRecord{Height: height}, roots, nil); err != nil {
			b.Fatal(err)
		}
	}
	for height := types.Height(1); height <= warmup; height++ {
		commit(height)
	}
	b.ResetTimer()
	for iteration := range b.N {
		commit(types.Height(warmup + 1 + iteration))
	}
}

func accountKey(field string, account int) []byte {
	return binary.BigEndian.AppendUint32([]byte(field+"/"), uint32(account))
}
