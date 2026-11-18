package store

import (
	"encoding/binary"

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

// blockHashValue is what a block hash key holds: the height of the block, whose record the height
// key keeps. The record is stored once.
func blockHashValue(height types.Height) []byte {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return buffer[:]
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

// kvHistoryPrefixKey is the prefix every version of key shares. The key is in its ordered
// encoding (appendOrderedKey), so the version groups of a namespace sort by key.
func kvHistoryPrefixKey(namespace string, key []byte) []byte {
	return appendOrderedKey(kvHistoryNamespacePrefix(namespace), key)
}

// appendOrderedKey appends the ordered encoding of key to dst: every zero byte is escaped as
// 0x00 0xff and the key ends with 0x00 0x01. Encoded keys sort exactly as the keys do, and no
// encoding is a prefix of another, so a key's encoding delimits a group of database keys.
func appendOrderedKey(dst []byte, key []byte) []byte {
	return append(appendOrderedKeyBody(dst, key), 0x00, 0x01)
}

// appendOrderedKeyBody appends the encoding of key without its terminator: the prefix that the
// encodings of every key starting with key share.
func appendOrderedKeyBody(dst []byte, key []byte) []byte {
	for _, value := range key {
		if value == 0x00 {
			dst = append(dst, 0x00, 0xff)
			continue
		}
		dst = append(dst, value)
	}
	return dst
}

// decodeOrderedKey decodes a complete ordered encoding.
func decodeOrderedKey(encoded []byte) ([]byte, bool) {
	key := make([]byte, 0, len(encoded))
	for index := 0; index < len(encoded); index++ {
		if encoded[index] != 0x00 {
			key = append(key, encoded[index])
			continue
		}
		if index+1 >= len(encoded) {
			return nil, false
		}
		switch encoded[index+1] {
		case 0xff:
			key = append(key, 0x00)
			index++
		case 0x01:
			return key, index+2 == len(encoded) && len(key) > 0
		default:
			return nil, false
		}
	}
	return nil, false
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

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}

type byteWriter interface {
	Write([]byte) (int, error)
}
