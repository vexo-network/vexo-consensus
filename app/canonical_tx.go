package app

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
)

// The transaction grammar is parsed by txcodec, the single parser shared with
// the mempool, so every layer reads a transaction identically.
var (
	ErrInvalidCanonicalTx = txcodec.ErrInvalidCanonicalTx
	ErrMissingTxModule    = txcodec.ErrMissingTxModule
	ErrMissingTxAction    = txcodec.ErrMissingTxAction
)

// CanonicalTx is a parsed `module:action:arg...:key=value...` payload.
type CanonicalTx = txcodec.Canonical

// ParseCanonicalTx strictly parses tx, unwrapping a signed envelope first. A
// signed transaction that does not decode and a payload that repeats a tag key
// are errors.
func ParseCanonicalTx(tx types.Tx) (CanonicalTx, error) {
	return txcodec.ParseCanonical(tx)
}

// BuildCanonicalTx serializes tx. Components that contain the field separator
// or, for tag keys, the key/value separator cannot be represented unambiguously
// and are rejected rather than silently reinterpreted by the parser.
func BuildCanonicalTx(tx CanonicalTx) (types.Tx, error) {
	if tx.Module == "" {
		return nil, ErrMissingTxModule
	}
	if tx.Action == "" {
		return nil, ErrMissingTxAction
	}
	parts := []string{tx.Module, tx.Action}
	if strings.Contains(tx.Module, ":") || strings.Contains(tx.Action, ":") {
		return nil, fmt.Errorf("%w: separator in module or action", ErrInvalidCanonicalTx)
	}
	for _, arg := range tx.Args {
		if strings.Contains(arg, ":") {
			return nil, fmt.Errorf("%w: separator in argument", ErrInvalidCanonicalTx)
		}
		parts = append(parts, arg)
	}
	for _, key := range CanonicalTagKeys(tx.Tags) {
		if key == "" || strings.ContainsAny(key, ":=") || strings.Contains(tx.Tags[key], ":") {
			return nil, fmt.Errorf("%w: invalid tag %q", ErrInvalidCanonicalTx, key)
		}
		parts = append(parts, key+"="+tx.Tags[key])
	}
	return types.Tx(strings.Join(parts, ":")), nil
}

func CanonicalTagKeys(tags map[string]string) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left int, right int) bool {
		return tagSortRank(keys[left]) < tagSortRank(keys[right]) ||
			(tagSortRank(keys[left]) == tagSortRank(keys[right]) && keys[left] < keys[right])
	})
	return keys
}

func TxTag(tx types.Tx, key string) (string, bool) {
	canonical, err := ParseCanonicalTx(tx)
	if err != nil || canonical.Tags == nil {
		return "", false
	}
	value, found := canonical.Tags[key]
	return value, found
}

func TxUintTag(tx types.Tx, key string) (uint64, bool) {
	value, found := TxTag(tx, key)
	if !found {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}

func TxAmountTag(tx types.Tx, key string) (uint64, bool) {
	parsed, found := TxAmountBigTag(tx, key)
	if !found || !parsed.IsUint64() {
		return 0, false
	}
	return parsed.Uint64(), true
}

func TxAmountBigTag(tx types.Tx, key string) (*big.Int, bool) {
	value, found := TxTag(tx, key)
	if !found {
		return nil, false
	}
	parsed, err := txcodec.ParseAmount(value)
	return parsed, err == nil
}

func tagSortRank(key string) int {
	switch key {
	case "fee":
		return 10
	case "gas":
		return 20
	case "gas_limit":
		return 21
	case "signer":
		return 30
	case "nonce":
		return 40
	case "priority":
		return 50
	default:
		return 100
	}
}
