package store

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/vexo-network/vexo-consensus/types"
)

func blockHeightKey(height types.Height) []byte {
	key := append([]byte(nil), blockHeightPrefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append(key, buffer[:]...)
}

func blockHeightFromKey(key []byte) (types.Height, bool) {
	if len(key) < len(blockHeightPrefix)+8 {
		return 0, false
	}
	return types.Height(binary.BigEndian.Uint64(key[len(blockHeightPrefix) : len(blockHeightPrefix)+8])), true
}

func blockHashKey(hash types.Hash) []byte {
	key := append([]byte(nil), blockHashPrefix...)
	return append(key, hash[:]...)
}

func stateRootKey(height types.Height, namespace string) []byte {
	key := append([]byte(nil), stateRootPrefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	key = append(key, buffer[:]...)
	key = append(key, ':')
	return append(key, []byte(namespace)...)
}

func stateRootHeightFromKey(key []byte) (types.Height, bool) {
	if len(key) < len(stateRootPrefix)+8 {
		return 0, false
	}
	return types.Height(binary.BigEndian.Uint64(key[len(stateRootPrefix) : len(stateRootPrefix)+8])), true
}

func finalityHeightKey(height types.Height) []byte {
	key := append([]byte(nil), finalityPrefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append(key, buffer[:]...)
}

func upgradePlanHeightKey(height types.Height) []byte {
	key := append([]byte(nil), upgradePlanHeightPrefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append(key, buffer[:]...)
}

func upgradePlanNameKey(name string) []byte {
	key := append([]byte(nil), upgradePlanNamePrefix...)
	return append(key, []byte(name)...)
}

func kvKey(namespace string, key []byte) []byte {
	dbKey := kvNamespacePrefix(namespace)
	return append(dbKey, key...)
}

func kvNamespacePrefix(namespace string) []byte {
	dbKey := append([]byte(nil), kvPrefix...)
	dbKey = append(dbKey, []byte(namespace)...)
	return append(dbKey, ':')
}

func kvHistoryPrefixKey(namespace string, key []byte) []byte {
	dbKey := append([]byte(nil), kvHistoryPrefix...)
	dbKey = append(dbKey, []byte(namespace)...)
	dbKey = append(dbKey, ':')
	dbKey = append(dbKey, []byte(hex.EncodeToString(key))...)
	return append(dbKey, ':')
}

func kvHistoryKey(height types.Height, namespace string, key []byte) []byte {
	dbKey := kvHistoryPrefixKey(namespace, key)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append(dbKey, buffer[:]...)
}

func kvHistoryHeightFromKey(prefix []byte, key []byte) (types.Height, bool) {
	if len(key) != len(prefix)+8 || string(key[:len(prefix)]) != string(prefix) {
		return 0, false
	}
	return types.Height(binary.BigEndian.Uint64(key[len(prefix):])), true
}

func stateHeightKey(height types.Height) []byte {
	key := append([]byte(nil), stateHeightPrefix...)
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return append(key, buffer[:]...)
}

func stateHeightFromKey(key []byte) (types.Height, bool) {
	if len(key) < len(stateHeightPrefix)+8 {
		return 0, false
	}
	return types.Height(binary.BigEndian.Uint64(key[len(stateHeightPrefix) : len(stateHeightPrefix)+8])), true
}

func evidenceKey(key string) []byte {
	dbKey := append([]byte(nil), evidencePrefix...)
	return append(dbKey, []byte(key)...)
}

func evidenceKeyString(key []byte) string {
	if len(key) <= len(evidencePrefix) {
		return ""
	}
	return string(key[len(evidencePrefix):])
}

func stringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}

type byteWriter interface {
	Write([]byte) (int, error)
}
