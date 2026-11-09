package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// ErrUntrustedSnapshot reports a snapshot that does not prove it is the state of the trusted block.
var ErrUntrustedSnapshot = errors.New("snapshot is not the state of the trusted block")

// snapshotTrust is the block an operator trusts, learned outside the snapshot source: from a
// validator it trusts, a block explorer, or a node it runs. A snapshot is restored only when it is
// the state that block's header commits the app hash of.
type snapshotTrust struct {
	Height types.Height
	Hash   types.Hash
}

// snapshotTrustFlags registers the trusted block flags of a restoring command.
func snapshotTrustFlags(flags *flag.FlagSet) (height *uint64, hash *string) {
	height = flags.Uint64("trust-height", 0, "height of the trusted block the snapshot is anchored to")
	hash = flags.String("trust-hash", "", "hex hash of the trusted block the snapshot is anchored to")
	return height, hash
}

func parseSnapshotTrust(height uint64, hash string) (snapshotTrust, error) {
	if height == 0 || hash == "" {
		return snapshotTrust{}, fmt.Errorf("%w: a trusted block height and hash are required to restore a snapshot", ErrUntrustedSnapshot)
	}
	decoded, err := hex.DecodeString(hash)
	if err != nil || len(decoded) != len(types.Hash{}) {
		return snapshotTrust{}, fmt.Errorf("trusted block hash must be %d hex-encoded bytes", len(types.Hash{}))
	}
	var trust snapshotTrust
	trust.Height = types.Height(height)
	copy(trust.Hash[:], decoded)
	return trust, nil
}

// verifySnapshotTrust checks that the snapshot of manifest is anchored to the trusted block, by
// height and hash. That the snapshot is the state the block commits is checked by
// checkSnapshotManifest and by the roots of its imported pairs.
func verifySnapshotTrust(manifest snapshotManifest, trust snapshotTrust) error {
	header := manifest.Header
	if header.Height != trust.Height || finality.HeaderHash(header) != trust.Hash {
		return fmt.Errorf("%w: snapshot header is block %d %x, trusted block is %d %x", ErrUntrustedSnapshot, header.Height, finality.HeaderHash(header), trust.Height, trust.Hash)
	}
	return nil
}

// snapshotHeaderAppHash returns the header app hash (finality.HeaderAppHash) of a snapshot state
// with app hash appHash: the root the snapshot names for the validator registry completes it.
func snapshotHeaderAppHash(appHash types.Hash, roots []store.StateRootRecord) (types.Hash, error) {
	registryRoot, err := snapshotRoots(roots).Root(context.Background(), validator.RegistryNamespace)
	if err != nil {
		return types.Hash{}, err
	}
	return finality.HeaderAppHash(appHash, registryRoot), nil
}

// snapshotRoots serves the namespace roots a snapshot names.
type snapshotRoots []store.StateRootRecord

func (roots snapshotRoots) Root(_ context.Context, namespace string) (types.Hash, error) {
	for _, root := range roots {
		if root.Namespace == namespace {
			return root.Root, nil
		}
	}
	return types.Hash{}, fmt.Errorf("snapshot has no root for namespace %q", namespace)
}
