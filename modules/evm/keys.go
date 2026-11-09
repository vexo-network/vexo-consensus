package evm

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/modules/evm/ethcompat"
	"github.com/vexo-network/vexo-consensus/types"
)

// TxHash returns the identity a transaction's receipt is stored under. An
// Ethereum transaction is identified by the Ethereum hash of its signed raw
// bytes and every other transaction by the SHA-256 of its payload. The eth_hash
// tag is never trusted: it is unsigned, so honoring it would let any
// transaction claim the hash of another and overwrite that transaction's
// receipt.
func TxHash(tx types.Tx) string {
	if raw, found := vexoapp.TxTag(tx, ethcompat.TagRaw); found {
		if hash, err := ethcompat.RawTransactionHash(raw); err == nil {
			return hash
		}
	}
	hash := sha256.Sum256(tx)
	return "0x" + hex.EncodeToString(hash[:])
}

func receiptKey(hash string) []byte {
	return []byte("receipts/" + strings.TrimPrefix(hash, "0x"))
}

// receiptsPrunedBelowKey holds the height below which the receipts are pruned.
var receiptsPrunedBelowKey = []byte("receipt_pruned_below")

// receiptHeightPrefix lists the receipts of the block of height.
func receiptHeightPrefix(height uint64) []byte {
	return binary.BigEndian.AppendUint64([]byte("receipt_height/"), height)
}

func receiptHeightKey(height uint64, hash string) []byte {
	return append(receiptHeightPrefix(height), strings.TrimPrefix(hash, "0x")...)
}

func receiptIndexKey(hash string) []byte {
	return []byte("receipt_index/" + strings.TrimPrefix(hash, "0x"))
}

func blobSidecarKey(hash string) []byte {
	return []byte("blob_sidecars/by_tx/" + strings.ToLower(strings.TrimPrefix(hash, "0x")))
}

func blobSidecarHashIndexKey(hash string) []byte {
	return []byte("blob_sidecars/by_blob/" + strings.ToLower(strings.TrimPrefix(hash, "0x")))
}

func codeKey(address types.Address) []byte {
	return []byte("code/" + canonicalAddressKey(address))
}

// globalLogHeightPrefix is the prefix of every log of one block, in the index
// that spans all addresses.
func globalLogHeightPrefix(height uint64) []byte {
	return []byte(fmt.Sprintf("logs/by_height/%020d/", height))
}

// addressLogHeightPrefix is the prefix of the logs one address emitted in one
// block. The address is canonical, so a filter finds the logs of an address
// whatever the case it spells it in.
func addressLogHeightPrefix(address types.Address, height uint64) []byte {
	return []byte(fmt.Sprintf("logs/by_address/%s/%020d/", canonicalAddressKey(address), height))
}

func globalLogKey(log Log) []byte {
	return append(globalLogHeightPrefix(log.BlockNumber), []byte(logOrderSuffix(log))...)
}

func addressLogKey(log Log) []byte {
	return append(addressLogHeightPrefix(types.Address(log.Address), log.BlockNumber), []byte(logOrderSuffix(log))...)
}

func logOrderSuffix(log Log) string {
	return fmt.Sprintf("%s/%020d", strings.TrimPrefix(log.TransactionHash, "0x"), log.LogIndex)
}

func storageKey(address types.Address, slot string) []byte {
	return []byte("storage/" + canonicalAddressKey(address) + "/" + strings.TrimPrefix(normalizeSlot(slot), "0x"))
}

func storageAccountPrefix(address types.Address) []byte {
	return []byte("storage/" + canonicalAddressKey(address) + "/")
}

func headerUnixSeconds(header types.Header) uint64 {
	if header.TimeUnixNano <= 0 {
		return 0
	}
	return uint64(header.TimeUnixNano / 1_000_000_000)
}
