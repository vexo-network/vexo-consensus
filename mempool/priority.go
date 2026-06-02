package mempool

import (
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
	for _, part := range strings.Split(string(tx), ":") {
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
