package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/finality"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
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
		appHash, err := runtime.AppHashAt(ctx, setHeight-1)
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
	args := append([]string{"sync", "--home", home, "--url", server.URL + "/snapshot/export"}, chain.trustFlags()...)
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
	if err != nil || state.Height != 1 || state.AppHash != chain.anchor.AppHash {
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
}

// Whoever serves a snapshot is not trusted: a snapshot is restored only when it is the state the
// trusted block commits, and a rejected one writes nothing.
func TestSnapshotRestoreRefusesASnapshotThatIsNotTheStateOfTheTrustedBlock(t *testing.T) {
	ctx := context.Background()
	chain := startCommittedSnapshotChain(t)
	snapshot, err := chain.node.StateSnapshot(ctx, chain.anchor.Height)
	if err != nil {
		t.Fatal(err)
	}
	genuine := snapshotDocumentFromState(snapshot.ChainID, snapshot.Namespaces, snapshot.Header, snapshot.State, snapshot.StateRoots, snapshot.KV)
	home := freshSnapshotHome(t)
	cfg, err := loadNodeConfig(filepath.Join(home, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshotTrust(ctx, genuine, cfg.Chain, chain.trust()); err != nil {
		t.Fatalf("expected the genuine snapshot to be the state of the trusted block, got %v", err)
	}

	// reroot recomputes the roots and the checksum, so only the trust check can catch the change.
	reroot := func(document snapshotDocument) snapshotDocument {
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
		return snapshotDocumentFromState(document.ChainID, document.Modules, document.Header, document.State, document.StateRoots, document.KV)
	}
	clone := func() snapshotDocument {
		document := genuine
		document.StateRoots = append([]store.StateRootRecord(nil), genuine.StateRoots...)
		document.KV = make([]store.KVPair, len(genuine.KV))
		for index, pair := range genuine.KV {
			document.KV[index] = store.KVPair{Namespace: pair.Namespace, Key: append([]byte(nil), pair.Key...), Value: append([]byte(nil), pair.Value...)}
		}
		return document
	}
	cases := map[string]struct {
		document snapshotDocument
		trust    snapshotTrust
	}{}
	cases["another trusted block"] = struct {
		document snapshotDocument
		trust    snapshotTrust
	}{genuine, snapshotTrust{Height: chain.anchor.Height, Hash: types.Hash{1}}}

	forged := clone()
	forged.KV = append(forged.KV, store.KVPair{Namespace: "bank", Key: []byte("balance/mallory"), Value: []byte("1000000")})
	cases["a minted balance"] = struct {
		document snapshotDocument
		trust    snapshotTrust
	}{reroot(forged), chain.trust()}

	// The registry is outside the app hash; the header's validator set hash still binds it.
	registry := clone()
	for index, pair := range registry.KV {
		if pair.Namespace == validator.RegistryNamespace && string(pair.Key) == "heights" {
			registry.KV[index].Value = []byte("[1,2,9]")
		}
	}
	cases["a validator set planted for a later height"] = struct {
		document snapshotDocument
		trust    snapshotTrust
	}{reroot(registry), chain.trust()}

	extra := clone()
	extra.Modules = append(extra.Modules, "evm_receipts")
	extra.KV = append(extra.KV, store.KVPair{Namespace: "evm_receipts", Key: []byte("receipt"), Value: []byte("forged")})
	extra.StateRoots = append(extra.StateRoots, store.StateRootRecord{Height: extra.State.Height, Namespace: "evm_receipts"})
	cases["a namespace outside the replicated state"] = struct {
		document snapshotDocument
		trust    snapshotTrust
	}{reroot(extra), chain.trust()}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			target, err := store.OpenLevelDB(filepath.Join(t.TempDir(), "target"))
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			err = restoreSnapshotDocumentWithContext(ctx, target, testCase.document, cfg.Chain, testCase.trust)
			if !errors.Is(err, ErrUntrustedSnapshot) {
				t.Fatalf("expected the snapshot to be refused, got %v", err)
			}
			if _, err := target.LatestState(ctx); !errors.Is(err, store.ErrStateNotFound) {
				t.Fatalf("a refused snapshot must write nothing, got %v", err)
			}
		})
	}

	if err := runSnapshot(&bytes.Buffer{}, []string{"sync", "--home", home, "--url", "http://127.0.0.1:1/snapshot/export"}); !errors.Is(err, ErrUntrustedSnapshot) {
		t.Fatalf("expected snapshot sync without a trusted block to be refused, got %v", err)
	}
}
