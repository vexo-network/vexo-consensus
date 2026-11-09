package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/stateproof"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// committedSnapshotChain is a running node of chain vexo-test that executed three blocks. Block 3
// names the validator set of registry height 2, so its header commits the app hash of the state
// after block 1: the state a snapshot anchored to block 3 carries.
type committedSnapshotChain struct {
	home   string
	node   *vexonode.Node
	anchor types.Header
}

// stop stops the node, so the commands under test can open the store of its home.
func (chain committedSnapshotChain) stop(t *testing.T) {
	t.Helper()
	if err := chain.node.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func startCommittedSnapshotChain(t *testing.T) committedSnapshotChain {
	t.Helper()
	ctx := context.Background()
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadStartInputs(home, "", "", "", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	node, err := buildStartNode(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := node.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { node.Stop(context.Background()) })
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	var parent types.Hash
	var anchor types.Header
	for height := types.Height(1); height <= 3; height++ {
		setHeight := finality.FirstValidatorSetHeight
		if height == 3 {
			setHeight = 2
		}
		appHash, err := runtime.HeaderAppHashAt(ctx, setHeight-1)
		if err != nil {
			t.Fatal(err)
		}
		validatorSet, err := runtime.Validators.ValidatorSet(ctx, setHeight)
		if err != nil {
			t.Fatal(err)
		}
		block := types.Block{Header: types.Header{
			ChainID:            "vexo-test",
			Height:             height,
			PreviousBlockHash:  parent,
			AppHash:            appHash,
			ValidatorSetHeight: setHeight,
			ValidatorSetHash:   validatorSet.Hash(),
			TxRoot:             consensus.TxRoot(nil),
		}}
		if _, err := runtime.ExecuteBlock(ctx, block); err != nil {
			t.Fatal(err)
		}
		parent = consensus.HashBlock(block)
		anchor = block.Header
	}
	return committedSnapshotChain{home: home, node: node, anchor: anchor}
}

func (chain committedSnapshotChain) trustFlags() []string {
	hash := finality.HeaderHash(chain.anchor)
	return []string{"--trust-height", strconv.FormatUint(uint64(chain.anchor.Height), 10), "--trust-hash", hex.EncodeToString(hash[:])}
}

func (chain committedSnapshotChain) trust() snapshotTrust {
	return snapshotTrust{Height: chain.anchor.Height, Hash: finality.HeaderHash(chain.anchor)}
}

func freshSnapshotHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := runInit(&bytes.Buffer{}, []string{"--home", home, "--chain-id", "vexo-test"}); err != nil {
		t.Fatal(err)
	}
	return home
}

// A node restored from a snapshot holds the state of the trusted block, every namespace of the
// replicated state included: the account sequences, the fee market and the validator registry next
// to the module state.
func TestSnapshotSyncRestoresTheStateOfTheTrustedBlock(t *testing.T) {
	ctx := context.Background()
	chain := startCommittedSnapshotChain(t)
	server := httptest.NewServer(vexorpc.NewHandler(chain.node))
	defer server.Close()

	home := freshSnapshotHome(t)
	var output bytes.Buffer
	args := append([]string{"sync", "--home", home, "--url", server.URL}, chain.trustFlags()...)
	if err := runSnapshot(&output, args); err != nil {
		t.Fatal(err)
	}
	runtime, err := chain.node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	target, err := store.OpenLevelDB(cfg.StoreDir())
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	state, err := target.LatestState(ctx)
	sourceState, err2 := runtime.Store.StateByHeight(ctx, 1)
	if err2 != nil {
		t.Fatal(err2)
	}
	if err != nil || state != sourceState {
		t.Fatalf("expected the state after block 1 restored, got %+v (%v)", state, err)
	}
	for _, namespace := range runtime.StateNamespaces() {
		want, err := runtime.Store.ExportNamespaceAt(ctx, 1, namespace)
		if err != nil {
			t.Fatal(err)
		}
		got, err := target.ExportNamespace(ctx, namespace)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("namespace %s: restored %d pairs, the state of height 1 has %d", namespace, len(got), len(want))
		}
		for index := range want {
			if !bytes.Equal(got[index].Key, want[index].Key) || !bytes.Equal(got[index].Value, want[index].Value) {
				t.Fatalf("namespace %s: restored %+v, want %+v", namespace, got[index], want[index])
			}
		}
	}
	// The restored node continues the chain from the block of the state.
	sourceBlock, err := runtime.Store.BlockByHeight(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	block, err := target.BlockByHeight(ctx, 1)
	if err != nil || block.Hash != sourceBlock.Hash || consensus.HashBlock(block.Block) != sourceBlock.Hash || block.QuorumCert != nil {
		t.Fatalf("expected block 1 restored without a certificate, got %+v (%v)", block, err)
	}
	target.Close()

	inputs, err := loadStartInputs(home, "", "", "", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := buildStartNode(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer restored.Stop(context.Background())
	if _, err := restored.StepConsensus(ctx, 1<<20); err != nil {
		t.Fatalf("expected the restored node to step from the restored block, got %v", err)
	}
	// The restored node holds no state below the snapshot, yet judges headers that name the
	// genesis state, as blocks 1 and 2 did, by the app hash the chain certified for it.
	restoredRuntime, err := restored.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	want, err := runtime.HeaderAppHashAt(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := restoredRuntime.HeaderAppHashAt(ctx, 0); err != nil || got != want {
		t.Fatalf("expected the restored node to know the certified genesis app hash %x, got %x (%v)", want, got, err)
	}
	if _, err := restoredRuntime.HeaderAppHashAt(ctx, 5); !errors.Is(err, vexoruntime.ErrAppHashUnavailable) {
		t.Fatalf("expected a state no block named to be unavailable, got %v", err)
	}
}

// Whoever serves a snapshot is not trusted: a snapshot is restored only when it is the state the
// trusted block commits, and a rejected one writes nothing.
func TestSnapshotRestoreRefusesASnapshotThatIsNotTheStateOfTheTrustedBlock(t *testing.T) {
	ctx := context.Background()
	chain := startCommittedSnapshotChain(t)
	genuine := readWholeSnapshot(t, chain.node, chain.anchor.Height)
	home := freshSnapshotHome(t)
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshotTrust(genuine.snapshotManifest, chain.trust()); err != nil {
		t.Fatalf("expected the genuine snapshot to be anchored to the trusted block, got %v", err)
	}
	if err := checkSnapshotManifest(ctx, genuine.snapshotManifest, cfg.Chain); err != nil {
		t.Fatalf("expected the genuine snapshot to be the state its anchor commits, got %v", err)
	}

	// reroot recomputes the roots of the pairs, so only the app hash they give can catch the change.
	reroot := func(document wholeSnapshot) wholeSnapshot {
		for index, root := range document.StateRoots {
			var pairs []stateproof.Pair
			for _, pair := range document.KV {
				if pair.Namespace == root.Namespace {
					pairs = append(pairs, stateproof.Pair{Key: pair.Key, Value: pair.Value})
				}
			}
			value, err := stateproof.Root(root.Namespace, pairs)
			if err != nil {
				t.Fatal(err)
			}
			document.StateRoots[index].Root = value
		}
		return document
	}
	clone := func() wholeSnapshot {
		document := genuine
		document.StateRoots = append([]store.StateRootRecord(nil), genuine.StateRoots...)
		document.Links = append([]types.Header(nil), genuine.Links...)
		document.Block.Block.Txs = append([]types.Tx(nil), genuine.Block.Block.Txs...)
		document.KV = make([]store.KVPair, len(genuine.KV))
		for index, pair := range genuine.KV {
			document.KV[index] = store.KVPair{Namespace: pair.Namespace, Key: append([]byte(nil), pair.Key...), Value: append([]byte(nil), pair.Value...)}
		}
		return document
	}
	cases := map[string]struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{}
	cases["another trusted block"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{genuine, snapshotTrust{Height: chain.anchor.Height, Hash: types.Hash{1}}}

	minted := clone()
	minted.KV = append(minted.KV, store.KVPair{Namespace: "bank", Key: []byte("balance/mallory"), Value: []byte("1000000")})
	cases["pairs with a minted balance"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{minted, chain.trust()}

	forged := clone()
	forged.KV = append(forged.KV, store.KVPair{Namespace: "bank", Key: []byte("balance/mallory"), Value: []byte("1000000")})
	cases["a minted balance"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{reroot(forged), chain.trust()}

	// The header app hash commits the validator registry root, so no registry entry can be planted.
	registry := clone()
	for index, pair := range registry.KV {
		if pair.Namespace == validator.RegistryNamespace && string(pair.Key) == "heights" {
			registry.KV[index].Value = []byte("[1,2,9]")
		}
	}
	cases["a validator set planted for a later height"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{reroot(registry), chain.trust()}

	// Evidence of a past height is judged against the validator set of that height. A registry that
	// forges the set of height 1, and keeps the genuine set as the one of height 2 the header names,
	// agrees with the header's validator set hash; only the registry root the header commits tells it
	// apart.
	history := clone()
	setKey := func(height uint64) []byte {
		key := []byte("set/")
		return binary.BigEndian.AppendUint64(key, height)
	}
	var genuineSet []byte
	for _, pair := range history.KV {
		if pair.Namespace == validator.RegistryNamespace && bytes.Equal(pair.Key, setKey(1)) {
			genuineSet = pair.Value
		}
	}
	if genuineSet == nil {
		t.Fatal("expected the registry to hold the genesis validator set")
	}
	var forgedSet struct {
		Height     types.Height          `json:"height"`
		Validators []validator.Validator `json:"validators"`
	}
	if err := json.Unmarshal(genuineSet, &forgedSet); err != nil {
		t.Fatal(err)
	}
	forgedSet.Height = 2
	genuineAtTwo, err := json.Marshal(forgedSet)
	if err != nil {
		t.Fatal(err)
	}
	forgedSet.Height = 1
	for index := range forgedSet.Validators {
		forgedSet.Validators[index].PublicKey = append(types.PublicKey(nil), bytes.Repeat([]byte{0x42}, len(forgedSet.Validators[index].PublicKey))...)
	}
	forgedAtOne, err := json.Marshal(forgedSet)
	if err != nil {
		t.Fatal(err)
	}
	historyKV := []store.KVPair{
		{Namespace: validator.RegistryNamespace, Key: []byte("heights"), Value: []byte("[1,2]")},
		{Namespace: validator.RegistryNamespace, Key: setKey(1), Value: forgedAtOne},
		{Namespace: validator.RegistryNamespace, Key: setKey(2), Value: genuineAtTwo},
	}
	for _, pair := range history.KV {
		if pair.Namespace == validator.RegistryNamespace && (bytes.HasPrefix(pair.Key, []byte("set/")) || string(pair.Key) == "heights") {
			continue
		}
		historyKV = append(historyKV, pair)
	}
	history.KV = historyKV
	forgedRegistry := registryPairs{}
	for _, pair := range history.KV {
		if pair.Namespace == validator.RegistryNamespace {
			forgedRegistry[string(pair.Key)] = pair.Value
		}
	}
	atHeader, err := validator.LoadValidatorSet(ctx, forgedRegistry, chain.anchor.ValidatorSetHeight)
	if err != nil || atHeader.Hash() != chain.anchor.ValidatorSetHash {
		t.Fatalf("expected the forged registry to agree with the header's validator set, got %v", err)
	}
	if atOne, err := validator.LoadValidatorSet(ctx, forgedRegistry, 1); err != nil || atOne.Hash() == chain.anchor.ValidatorSetHash {
		t.Fatalf("expected the forged registry to hold another set for height 1, got %v", err)
	}
	cases["a forged validator set of a past height"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{reroot(history), chain.trust()}

	extra := clone()
	extra.Modules = append(extra.Modules, "evm_receipts")
	extra.KV = append(extra.KV, store.KVPair{Namespace: "evm_receipts", Key: []byte("receipt"), Value: []byte("forged")})
	extra.StateRoots = append(extra.StateRoots, store.StateRootRecord{Height: extra.State.Height, Namespace: "evm_receipts"})
	cases["a namespace outside the replicated state"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{reroot(extra), chain.trust()}

	// The state record and the block a restored node continues from are bound to the anchor too.
	lastBlock := clone()
	lastBlock.State.LastBlockHash = types.Hash{0xee}
	cases["a state record naming another last block"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{lastBlock, chain.trust()}
	appHash := clone()
	appHash.State.AppHash = types.Hash{0xee}
	cases["a state record naming another app hash"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{appHash, chain.trust()}
	block := clone()
	block.Block.Block.Txs = append(block.Block.Block.Txs, types.Tx("bank:send:forged"))
	cases["a block of the state with a forged transaction"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{block, chain.trust()}
	rootless := clone()
	rootless.StateRoots = rootless.StateRoots[1:]
	cases["a namespace without its root"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{rootless, chain.trust()}
	link := clone()
	link.Links[0].TimeUnixNano++
	cases["a forged link to the anchor"] = struct {
		document wholeSnapshot
		trust    snapshotTrust
	}{link, chain.trust()}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "target")
			local, err := store.OpenLevelDB(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := local.Set(ctx, "bank", []byte("local"), []byte("kept")); err != nil {
				t.Fatal(err)
			}
			local.Close()
			_, _, err = restoreSnapshot(ctx, dir, testCase.document.source(), cfg.Chain, testCase.trust, nil)
			if !errors.Is(err, ErrUntrustedSnapshot) {
				t.Fatalf("expected the snapshot to be refused, got %v", err)
			}
			target, err := store.OpenLevelDB(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			if _, err := target.LatestState(ctx); !errors.Is(err, store.ErrStateNotFound) {
				t.Fatalf("a refused snapshot must write nothing, got %v", err)
			}
			if value, err := target.Get(ctx, "bank", []byte("local")); err != nil || string(value) != "kept" {
				t.Fatalf("a refused snapshot must leave the store as it was, got %q %v", value, err)
			}
		})
	}

	if err := runSnapshot(&bytes.Buffer{}, []string{"sync", "--home", home, "--url", "http://127.0.0.1:1"}); !errors.Is(err, ErrUntrustedSnapshot) {
		t.Fatalf("expected snapshot sync without a trusted block to be refused, got %v", err)
	}
}

// wholeSnapshot is a snapshot held whole in memory, served one page per namespace, so a test can
// change any part of it.
type wholeSnapshot struct {
	snapshotManifest
	KV []store.KVPair
}

// readWholeSnapshot reads the snapshot node serves for block blockHeight through its RPC handler.
func readWholeSnapshot(t *testing.T, node *vexonode.Node, blockHeight types.Height) wholeSnapshot {
	t.Helper()
	server := httptest.NewServer(vexorpc.NewHandler(node))
	defer server.Close()
	source, err := newHTTPSnapshotSource(server.URL, blockHeight, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := source.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := wholeSnapshot{snapshotManifest: manifest}
	if err := readSnapshotPages(context.Background(), source, manifest, func(page snapshotPage) error {
		snapshot.KV = append(snapshot.KV, page.Pairs...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func (snapshot wholeSnapshot) source() snapshotSource {
	return wholeSnapshotSource{snapshot}
}

type wholeSnapshotSource struct {
	snapshot wholeSnapshot
}

func (source wholeSnapshotSource) Manifest(context.Context) (snapshotManifest, error) {
	return source.snapshot.snapshotManifest, nil
}

func (source wholeSnapshotSource) Page(_ context.Context, namespace string, _ []byte) (snapshotPage, error) {
	var pairs []store.KVPair
	for _, pair := range source.snapshot.KV {
		if pair.Namespace == namespace {
			pairs = append(pairs, pair)
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return bytes.Compare(pairs[i].Key, pairs[j].Key) < 0 })
	return snapshotPage{SchemaVersion: vexorpc.SnapshotSchemaVersion, Height: uint64(source.snapshot.Header.Height), Namespace: namespace, Pairs: pairs}, nil
}

// registryPairs is a read-only validator registry store over the key-value pairs of a snapshot.
type registryPairs map[string][]byte

func (pairs registryPairs) Get(_ context.Context, _ string, key []byte) ([]byte, error) {
	value, found := pairs[string(key)]
	if !found {
		return nil, store.ErrKeyNotFound
	}
	return value, nil
}

func (registryPairs) Set(context.Context, string, []byte, []byte) error {
	return errors.New("read-only registry")
}

func (registryPairs) Delete(context.Context, string, []byte) error {
	return errors.New("read-only registry")
}
