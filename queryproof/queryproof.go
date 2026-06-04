package queryproof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const SchemaVersionV1 = "v1"

var (
	ErrInvalidProof = errors.New("invalid query proof")
	ErrRootMismatch = errors.New("query proof root mismatch")
)

type Proof struct {
	SchemaVersion string       `json:"schema_version"`
	ChainID       string       `json:"chain_id"`
	Height        types.Height `json:"height"`
	Namespace     string       `json:"namespace"`
	Key           []byte       `json:"key"`
	Value         []byte       `json:"value,omitempty"`
	Exists        bool         `json:"exists"`
	StateRoot     types.Hash   `json:"state_root"`
	LeafHash      types.Hash   `json:"leaf_hash"`
}

func Build(ctx context.Context, kv store.KVStore, chainID string, height types.Height, namespace string, key []byte) (Proof, error) {
	if kv == nil || chainID == "" || height == 0 || namespace == "" || len(key) == 0 {
		return Proof{}, ErrInvalidProof
	}
	value, err := kv.Get(ctx, namespace, key)
	exists := err == nil
	if err != nil && !errors.Is(err, store.ErrKeyNotFound) {
		return Proof{}, err
	}
	root, err := kv.Root(ctx, namespace)
	if err != nil {
		return Proof{}, err
	}
	return Proof{
		SchemaVersion: SchemaVersionV1,
		ChainID:       chainID,
		Height:        height,
		Namespace:     namespace,
		Key:           append([]byte(nil), key...),
		Value:         append([]byte(nil), value...),
		Exists:        exists,
		StateRoot:     root,
		LeafHash:      leafHash(namespace, key, value, exists),
	}, nil
}

func Verify(proof Proof, expectedChainID string, expectedHeight types.Height, expectedRoot types.Hash) error {
	if proof.SchemaVersion != SchemaVersionV1 ||
		proof.ChainID == "" ||
		proof.Height == 0 ||
		proof.Namespace == "" ||
		len(proof.Key) == 0 {
		return ErrInvalidProof
	}
	if expectedChainID != "" && proof.ChainID != expectedChainID {
		return ErrInvalidProof
	}
	if expectedHeight != 0 && proof.Height != expectedHeight {
		return ErrInvalidProof
	}
	if expectedRoot != (types.Hash{}) && proof.StateRoot != expectedRoot {
		return ErrRootMismatch
	}
	if proof.LeafHash != leafHash(proof.Namespace, proof.Key, proof.Value, proof.Exists) {
		return ErrInvalidProof
	}
	return nil
}

func Encode(proof Proof) ([]byte, error) {
	return json.Marshal(proof)
}

func Decode(data []byte) (Proof, error) {
	var proof Proof
	if err := json.Unmarshal(data, &proof); err != nil {
		return Proof{}, err
	}
	return proof, nil
}

func leafHash(namespace string, key []byte, value []byte, exists bool) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte(namespace))
	hasher.Write([]byte{0})
	hasher.Write(key)
	hasher.Write([]byte{0})
	if exists {
		hasher.Write([]byte{1})
	} else {
		hasher.Write([]byte{0})
	}
	hasher.Write([]byte{0})
	hasher.Write(value)
	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

func EqualValue(proof Proof, value []byte) bool {
	return bytes.Equal(proof.Value, value)
}
