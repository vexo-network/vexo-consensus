package rpc

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const (
	defaultSnapshotChunkSize = 10_000
	maxSnapshotChunkSize     = 100_000
)

// snapshotExportCacheSize bounds how many snapshot exports stay cached: the latest one, which state
// sync clients fetch, and a few heights that clients anchored to a recent trusted block ask for.
const snapshotExportCacheSize = 4

// snapshotExports serves snapshot exports anchored to committed blocks. Building an export copies,
// sorts and hashes the whole application state, so the exports of the most recently requested
// blocks are cached: every export and chunk request for one block shares its export, and concurrent
// requests wait for the one build instead of each starting their own. A committed block never
// changes, so neither does its export.
type snapshotExports struct {
	provider SnapshotProvider

	mu     sync.Mutex
	cached []SnapshotExportResponse
}

func newSnapshotExports(provider StatusProvider) *snapshotExports {
	snapshotProvider, ok := provider.(SnapshotProvider)
	if !ok {
		return nil
	}
	return &snapshotExports{provider: snapshotProvider}
}

// at returns the export anchored to committed block blockHeight, the latest committed block for 0.
func (exports *snapshotExports) at(ctx context.Context, blockHeight types.Height) (SnapshotExportResponse, error) {
	if blockHeight == 0 {
		head, err := exports.provider.StateSnapshotHead(ctx)
		if err != nil {
			return SnapshotExportResponse{}, err
		}
		blockHeight = head.State.Height
	}
	exports.mu.Lock()
	defer exports.mu.Unlock()
	for index, cached := range exports.cached {
		if cached.Header.Height == blockHeight {
			exports.cached = append(append([]SnapshotExportResponse{cached}, exports.cached[:index]...), exports.cached[index+1:]...)
			return cached, nil
		}
	}
	snapshot, err := exports.provider.StateSnapshot(ctx, blockHeight)
	if err != nil {
		return SnapshotExportResponse{}, err
	}
	export := snapshotExportResponse(snapshot)
	exports.cached = append([]SnapshotExportResponse{export}, exports.cached...)
	if len(exports.cached) > snapshotExportCacheSize {
		exports.cached = exports.cached[:snapshotExportCacheSize]
	}
	return export, nil
}

// writeSnapshotError writes the response for a failed snapshot read and reports whether err was one.
func writeSnapshotError(writer http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrStateNotFound):
		writeError(writer, http.StatusNotFound, "snapshot not found")
	case errors.Is(err, store.ErrBlockNotFound):
		writeError(writer, http.StatusNotFound, "snapshot block not found")
	case errors.Is(err, node.ErrSnapshotNotAnchored):
		writeError(writer, http.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrStateRootNotFound):
		writeError(writer, http.StatusNotFound, "snapshot state root not found")
	default:
		writeError(writer, http.StatusInternalServerError, err.Error())
	}
	return true
}
