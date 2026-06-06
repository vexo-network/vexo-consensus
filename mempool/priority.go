package mempool

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/vexo-network/vexo-consensus/economics"
	"github.com/vexo-network/vexo-consensus/types"
)

func TxFee(tx types.Tx) uint64 {
	return txAmountTag(tx, "fee")
}

func TxPriority(tx types.Tx) uint64 {
	return txNumericTag(tx, "priority")
}

func TxNonce(tx types.Tx) (uint64, bool) {
	return txNumericTagFound(tx, "nonce")
}

func TxSigner(tx types.Tx) (string, bool) {
	return txStringTag(tx, "signer")
}

func ReplacementKey(tx types.Tx) (string, bool) {
	signer, found := TxSigner(tx)
	if !found || signer == "" {
		return "", false
	}
	nonce, found := TxNonce(tx)
	if !found {
		return "", false
	}
	return signer + "/" + strconv.FormatUint(nonce, 10), true
}

func txNumericTag(tx types.Tx, key string) uint64 {
	value, _ := txNumericTagFound(tx, key)
	return value
}

func txNumericTagFound(tx types.Tx, key string) (uint64, bool) {
	payload := mempoolTxPayload(tx)
	for _, part := range strings.Split(string(payload), ":") {
		tagKey, tagValue, found := strings.Cut(part, "=")
		if !found || tagKey != key {
			continue
		}
		value, err := strconv.ParseUint(tagValue, 10, 64)
		if err != nil {
			return 0, false
		}
		return value, true
	}
	return 0, false
}

func txStringTag(tx types.Tx, key string) (string, bool) {
	payload := mempoolTxPayload(tx)
	for _, part := range strings.Split(string(payload), ":") {
		tagKey, tagValue, found := strings.Cut(part, "=")
		if !found || tagKey != key {
			continue
		}
		return tagValue, true
	}
	return "", false
}

func txAmountTag(tx types.Tx, key string) uint64 {
	payload := mempoolTxPayload(tx)
	for _, part := range strings.Split(string(payload), ":") {
		tagKey, tagValue, found := strings.Cut(part, "=")
		if !found || tagKey != key {
			continue
		}
		value, err := economics.ParseAmount(tagValue)
		if err != nil {
			return 0
		}
		return value
	}
	return 0
}

func mempoolTxPayload(tx types.Tx) types.Tx {
	if !strings.HasPrefix(string(tx), "signed:") {
		return tx
	}
	document, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(string(tx), "signed:"))
	if err != nil {
		return tx
	}
	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		Payload       string `json:"payload"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		return tx
	}
	if envelope.SchemaVersion != "v1" || envelope.Payload == "" {
		return tx
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return tx
	}
	return types.Tx(payload)
}

func replacementPriceBumped(oldTx types.Tx, newTx types.Tx, bumpBPS uint64) bool {
	if bumpBPS == 0 {
		bumpBPS = 1000
	}
	oldFee := TxFee(oldTx)
	newFee := TxFee(newTx)
	oldPriority := TxPriority(oldTx)
	newPriority := TxPriority(newTx)
	if oldFee == 0 && oldPriority == 0 {
		return newFee > 0 || newPriority > 0
	}
	return bumpedByBPS(newFee, oldFee, bumpBPS) || bumpedByBPS(newPriority, oldPriority, bumpBPS)
}

func bumpedByBPS(newValue uint64, oldValue uint64, bumpBPS uint64) bool {
	if oldValue == 0 {
		return newValue > 0
	}
	maxUint64 := ^uint64(0)
	if oldValue > maxUint64/(10000+bumpBPS) {
		return newValue == maxUint64
	}
	required := (oldValue*(10000+bumpBPS) + 9999) / 10000
	return newValue >= required
}
