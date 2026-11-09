package rpc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

// Snapshots for state sync.
//
// A restoring node first reads the manifest of the snapshot a committed block anchors
// (/snapshot/manifest), checks it against the block hash it trusts, and then reads the pairs of
// every namespace page by page (/snapshot/pairs). Each request reads only what it returns from the
// store's versions, so serving snapshots needs neither a copy of the state nor a cache of exports.

// SnapshotSchemaVersion is the schema of the snapshot manifest and its pages.
const SnapshotSchemaVersion = "v2"

const defaultSnapshotPairsPage = 1_000

// SnapshotManifestResponse is node.StateSnapshotManifest on the wire.
type SnapshotManifestResponse struct {
	SchemaVersion string   `json:"schema_version"`
	ChainID       string   `json:"chain_id"`
	Modules       []string `json:"modules"`
	// Header is the committed block header whose app hash commits State.
	Header     types.Header            `json:"header"`
	Links      []types.Header          `json:"links,omitempty"`
	Block      store.BlockRecord       `json:"block"`
	State      store.StateRecord       `json:"state"`
	StateRoots []store.StateRootRecord `json:"state_roots"`
}

// SnapshotPairsResponse is a page of the pairs of one namespace of a snapshot; More reports that
// pairs follow the last one, after which the next page starts.
type SnapshotPairsResponse struct {
	SchemaVersion string         `json:"schema_version"`
	Height        uint64         `json:"height"`
	Namespace     string         `json:"namespace"`
	Pairs         []store.KVPair `json:"pairs"`
	More          bool           `json:"more"`
}

// SnapshotErrorResponse reports why no snapshot is served; EarliestHeight is set when the anchor
// asked for is pruned and names the lowest anchor this node serves.
type SnapshotErrorResponse struct {
	Error          string `json:"error"`
	EarliestHeight uint64 `json:"earliest_height,omitempty"`
}

// NewSnapshotManifestResponse is manifest on the wire.
func NewSnapshotManifestResponse(manifest node.StateSnapshotManifest) SnapshotManifestResponse {
	return SnapshotManifestResponse{
		SchemaVersion: SnapshotSchemaVersion,
		ChainID:       manifest.ChainID,
		Modules:       sortedStrings(manifest.Namespaces),
		Header:        manifest.Header,
		Links:         manifest.Links,
		Block:         manifest.Block,
		State:         manifest.State,
		StateRoots:    sortedStateRoots(manifest.StateRoots),
	}
}

func registerSnapshotRoutes(mux *http.ServeMux, provider StatusProvider) {
	snapshots, _ := provider.(SnapshotProvider)
	mux.HandleFunc("/snapshot/latest", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		if snapshots == nil {
			writeError(writer, http.StatusNotImplemented, "snapshot query is unavailable")
			return
		}
		head, err := snapshots.StateSnapshotHead(request.Context())
		if writeSnapshotError(writer, request, snapshots, err) {
			return
		}
		writeJSON(writer, http.StatusOK, stateSnapshotResponse(head))
	})
	mux.HandleFunc("/snapshot/manifest", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		if snapshots == nil {
			writeError(writer, http.StatusNotImplemented, "snapshot export is unavailable")
			return
		}
		blockHeight, err := parseOptionalUintQuery(request, "height", 0)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		manifest, err := snapshots.StateSnapshotManifest(request.Context(), types.Height(blockHeight))
		if writeSnapshotError(writer, request, snapshots, err) {
			return
		}
		writeJSON(writer, http.StatusOK, NewSnapshotManifestResponse(manifest))
	})
	mux.HandleFunc("/snapshot/pairs", func(writer http.ResponseWriter, request *http.Request) {
		if !allowGet(writer, request) {
			return
		}
		if snapshots == nil {
			writeError(writer, http.StatusNotImplemented, "snapshot export is unavailable")
			return
		}
		blockHeight, err := parseOptionalUintQuery(request, "height", 0)
		if err == nil && blockHeight == 0 {
			err = errors.New("height is required")
		}
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		limit, err := parseOptionalUintQuery(request, "limit", defaultSnapshotPairsPage)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if limit == 0 || limit > node.MaxSnapshotPairsPage {
			writeError(writer, http.StatusBadRequest, fmt.Sprintf("snapshot page limit must be between 1 and %d", node.MaxSnapshotPairsPage))
			return
		}
		namespace := request.URL.Query().Get("namespace")
		var after []byte
		if raw := request.URL.Query().Get("after"); raw != "" {
			if after, err = hex.DecodeString(raw); err != nil || len(after) == 0 {
				writeError(writer, http.StatusBadRequest, "after must be a hex encoded key")
				return
			}
		}
		pairs, more, err := snapshots.StateSnapshotPairs(request.Context(), types.Height(blockHeight), namespace, after, int(limit))
		if writeSnapshotError(writer, request, snapshots, err) {
			return
		}
		writeJSON(writer, http.StatusOK, SnapshotPairsResponse{SchemaVersion: SnapshotSchemaVersion, Height: blockHeight, Namespace: namespace, Pairs: pairs, More: more})
	})
}

// writeSnapshotError writes the response for a failed snapshot read and reports whether err was
// one. A pruned anchor is 410 Gone with the lowest anchor this node serves, so a client can pick a
// newer trusted block or another source.
func writeSnapshotError(writer http.ResponseWriter, request *http.Request, snapshots SnapshotProvider, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, node.ErrSnapshotPruned):
		response := SnapshotErrorResponse{Error: "the state of the requested snapshot is pruned: " + err.Error()}
		if earliest, earliestErr := snapshots.EarliestSnapshotHeight(request.Context()); earliestErr == nil {
			response.EarliestHeight = uint64(earliest)
		}
		writeJSON(writer, http.StatusGone, response)
	case errors.Is(err, store.ErrStateNotFound):
		writeError(writer, http.StatusNotFound, "snapshot not found")
	case errors.Is(err, store.ErrStateRootNotFound):
		writeError(writer, http.StatusNotFound, "snapshot state root not found")
	case errors.Is(err, store.ErrBlockNotFound):
		writeError(writer, http.StatusNotFound, "snapshot block not found")
	case errors.Is(err, node.ErrSnapshotNotAnchored), errors.Is(err, node.ErrSnapshotAnchorTooFar):
		writeError(writer, http.StatusNotFound, err.Error())
	case errors.Is(err, node.ErrSnapshotPage):
		writeError(writer, http.StatusBadRequest, err.Error())
	default:
		writeError(writer, http.StatusInternalServerError, err.Error())
	}
	return true
}
