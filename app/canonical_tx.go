package app

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrInvalidCanonicalTx = errors.New("invalid canonical transaction")
	ErrMissingTxModule    = errors.New("missing transaction module")
	ErrMissingTxAction    = errors.New("missing transaction action")
)

type CanonicalTx struct {
	Module string            `json:"module"`
	Action string            `json:"action"`
	Args   []string          `json:"args,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

func ParseCanonicalTx(tx types.Tx) (CanonicalTx, error) {
	payload := TxPayload(tx)
	parts := strings.Split(string(payload), ":")
	if len(parts) < 2 || parts[0] == "" {
		return CanonicalTx{}, ErrMissingTxModule
	}
	if parts[1] == "" {
		return CanonicalTx{}, ErrMissingTxAction
	}
	canonical := CanonicalTx{
		Module: parts[0],
		Action: parts[1],
		Tags:   make(map[string]string),
	}
	for _, part := range parts[2:] {
		key, value, found := strings.Cut(part, "=")
		if found {
			if key == "" {
				return CanonicalTx{}, ErrInvalidCanonicalTx
			}
			canonical.Tags[key] = value
			continue
		}
		canonical.Args = append(canonical.Args, part)
	}
	if len(canonical.Tags) == 0 {
		canonical.Tags = nil
	}
	return canonical, nil
}

func BuildCanonicalTx(tx CanonicalTx) (types.Tx, error) {
	if tx.Module == "" {
		return nil, ErrMissingTxModule
	}
	if tx.Action == "" {
		return nil, ErrMissingTxAction
	}
	parts := []string{tx.Module, tx.Action}
	parts = append(parts, tx.Args...)
	for _, key := range CanonicalTagKeys(tx.Tags) {
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

func tagSortRank(key string) int {
	switch key {
	case "fee":
		return 10
	case "gas":
		return 20
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
