package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strconv"

	vexoconfig "github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/finality"
	appmodules "github.com/vexo-network/vexo-consensus/modules"
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

// snapshotExportURL returns the export URL of the snapshot anchored to the trusted block: the
// snapshot export endpoint serves the state of the block its height parameter names.
func snapshotExportURL(rawURL string, trustHeight types.Height) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("snapshot URL %q: %w", rawURL, err)
	}
	query := parsed.Query()
	query.Set("height", strconv.FormatUint(uint64(trustHeight), 10))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// verifySnapshotTrust checks that document is the state the trusted block commits:
//
//   - the snapshot header is the trusted block, by height and hash;
//   - the key-value pairs have the namespace roots the snapshot names;
//   - those roots give the header app hash the header commits: the app hash recomputed the way the
//     application of chain computes it, together with the validator registry root
//     (finality.HeaderAppHash), so every namespace, the registry with the validator sets of past
//     heights included, is exactly the trusted state.
//
// The snapshot must carry exactly the namespaces of the replicated state, so no namespace outside
// what the checks above cover is imported.
func verifySnapshotTrust(ctx context.Context, document snapshotDocument, chain vexoconfig.Config, trust snapshotTrust) error {
	header := document.Header
	if header.Height != trust.Height || finality.HeaderHash(header) != trust.Hash {
		return fmt.Errorf("%w: snapshot header is block %d %x, trusted block is %d %x", ErrUntrustedSnapshot, header.Height, finality.HeaderHash(header), trust.Height, trust.Hash)
	}
	if header.ChainID != chain.ChainID {
		return fmt.Errorf("%w: snapshot header is of chain %q, node runs %q", ErrUntrustedSnapshot, header.ChainID, chain.ChainID)
	}
	height, ok := finality.AppHashHeight(header)
	if !ok || height != document.State.Height {
		return fmt.Errorf("%w: header of block %d does not commit the state of height %d", ErrUntrustedSnapshot, header.Height, document.State.Height)
	}
	if len(document.KV) == 0 {
		return fmt.Errorf("%w: snapshot carries no state", ErrUntrustedSnapshot)
	}
	application, err := appmodules.NewRuntimeWithChainConfig(chain.ChainID, chain)
	if err != nil {
		return err
	}
	namespaces := make([]string, 0, len(application.Modules())+3)
	for _, module := range application.Modules() {
		namespaces = append(namespaces, module.Name())
	}
	if want := snapshotNamespaces(namespaces); !sameStringSet(document.Modules, want) {
		return fmt.Errorf("%w: snapshot namespaces %v, the replicated state has %v", ErrUntrustedSnapshot, sortedStrings(document.Modules), want)
	}
	if err := validateSnapshotKVRoots(document); err != nil {
		return fmt.Errorf("%w: %w", ErrUntrustedSnapshot, err)
	}
	appHash, err := application.AppHashOfRoots(ctx, height, snapshotRoots(document.StateRoots))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUntrustedSnapshot, err)
	}
	headerAppHash, err := snapshotHeaderAppHash(appHash, document.StateRoots)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUntrustedSnapshot, err)
	}
	if headerAppHash != header.AppHash {
		return fmt.Errorf("%w: snapshot state has header app hash %x, block %d commits %x", ErrUntrustedSnapshot, headerAppHash, header.Height, header.AppHash)
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
