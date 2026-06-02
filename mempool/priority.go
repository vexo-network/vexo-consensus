package mempool

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
)

func TxFee(tx types.Tx) uint64 {
	return txNumericTag(tx, "fee")
}

func TxPriority(tx types.Tx) uint64 {
	return txNumericTag(tx, "priority")
}

func txNumericTag(tx types.Tx, key string) uint64 {
	payload := mempoolTxPayload(tx)
	for _, part := range strings.Split(string(payload), ":") {
		tagKey, tagValue, found := strings.Cut(part, "=")
		if !found || tagKey != key {
			continue
		}
		value, err := strconv.ParseUint(tagValue, 10, 64)
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
