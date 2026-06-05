package queryproof

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const SchemaVersionV1 = "v1"

var (
	ErrInvalidProof = errors.New("invalid query proof")
	ErrRootMismatch = errors.New("query proof root mismatch")
)

type Proof struct {
	SchemaVersion   string                      `json:"schema_version"`
	ChainID         string                      `json:"chain_id"`
	Height          types.Height                `json:"height"`
	Namespace       string                      `json:"namespace"`
	Key             []byte                      `json:"key"`
	Value           []byte                      `json:"value,omitempty"`
	Exists          bool                        `json:"exists"`
	StateRoot       types.Hash                  `json:"state_root"`
	LeafHash        types.Hash                  `json:"leaf_hash"`
	MerklePath      []stateproof.Step           `json:"merkle_path,omitempty"`
	NamespaceLeaves []stateproof.Pair           `json:"namespace_leaves,omitempty"`
	AbsenceLeft     *stateproof.AbsenceNeighbor `json:"absence_left,omitempty"`
	AbsenceRight    *stateproof.AbsenceNeighbor `json:"absence_right,omitempty"`
}

func Build(ctx context.Context, kv store.KVStore, chainID string, height types.Height, namespace string, key []byte) (Proof, error) {
	if kv == nil || chainID == "" || height == 0 || namespace == "" || len(key) == 0 {
		return Proof{}, ErrInvalidProof
	}
	snapshot, ok := kv.(store.SnapshotKVStore)
	if !ok {
		return Proof{}, ErrInvalidProof
	}
	pairs, err := snapshot.ExportNamespace(ctx, namespace)
	if err != nil {
		return Proof{}, err
	}
	root, err := kv.Root(ctx, namespace)
	if err != nil {
		return Proof{}, err
	}
	return BuildFromKVPairs(chainID, height, namespace, key, pairs, root)
}

func BuildFromKVPairs(chainID string, height types.Height, namespace string, key []byte, pairs []store.KVPair, stateRoot types.Hash) (Proof, error) {
	return BuildFromPairs(chainID, height, namespace, key, storePairsToProofPairs(pairs), stateRoot)
}

func BuildFromPairs(chainID string, height types.Height, namespace string, key []byte, pairs []stateproof.Pair, stateRoot types.Hash) (Proof, error) {
	if chainID == "" || height == 0 || namespace == "" || len(key) == 0 || stateRoot == (types.Hash{}) {
		return Proof{}, ErrInvalidProof
	}
	root, value, exists, path, err := stateproof.BuildMembership(namespace, pairs, key)
	if err != nil {
		return Proof{}, err
	}
	if root != stateRoot {
		return Proof{}, ErrRootMismatch
	}
	proofPairs := []stateproof.Pair(nil)
	var absenceLeft *stateproof.AbsenceNeighbor
	var absenceRight *stateproof.AbsenceNeighbor
	if !exists {
		absenceRoot, left, right, absenceExists, err := stateproof.BuildNonMembership(namespace, pairs, key)
		if err != nil {
			return Proof{}, err
		}
		if absenceExists || absenceRoot != stateRoot {
			return Proof{}, ErrInvalidProof
		}
		absenceLeft = left
		absenceRight = right
	}
	return Proof{
		SchemaVersion:   SchemaVersionV1,
		ChainID:         chainID,
		Height:          height,
		Namespace:       namespace,
		Key:             append([]byte(nil), key...),
		Value:           append([]byte(nil), value...),
		Exists:          exists,
		StateRoot:       stateRoot,
		LeafHash:        leafHash(namespace, key, value),
		MerklePath:      append([]stateproof.Step(nil), path...),
		NamespaceLeaves: proofPairs,
		AbsenceLeft:     absenceLeft,
		AbsenceRight:    absenceRight,
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
	if proof.LeafHash != leafHash(proof.Namespace, proof.Key, proof.Value) {
		return ErrInvalidProof
	}
	if proof.Exists {
		if !stateproof.VerifyMembership(proof.Namespace, proof.Key, proof.Value, proof.MerklePath, proof.StateRoot) {
			return ErrInvalidProof
		}
		return nil
	}
	if proof.AbsenceLeft != nil || proof.AbsenceRight != nil {
		if !stateproof.VerifyNonMembershipCompact(proof.Namespace, proof.Key, proof.StateRoot, proof.AbsenceLeft, proof.AbsenceRight) {
			return ErrInvalidProof
		}
		return nil
	}
	if !stateproof.VerifyNonMembership(proof.Namespace, proof.NamespaceLeaves, proof.Key, proof.StateRoot) {
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

func leafHash(namespace string, key []byte, value []byte) types.Hash {
	return stateproof.LeafHash(namespace, key, value)
}

func EqualValue(proof Proof, value []byte) bool {
	return bytes.Equal(proof.Value, value)
}

func storePairsToProofPairs(pairs []store.KVPair) []stateproof.Pair {
	proofPairs := make([]stateproof.Pair, 0, len(pairs))
	for _, pair := range pairs {
		proofPairs = append(proofPairs, stateproof.Pair{
			Key:   append([]byte(nil), pair.Key...),
			Value: append([]byte(nil), pair.Value...),
		})
	}
	return proofPairs
}
