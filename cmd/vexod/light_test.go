package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/finality"
	vexonode "github.com/vexo-network/vexo-consensus/node"
	vexorpc "github.com/vexo-network/vexo-consensus/rpc"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
)

// startRunningChain starts the validator of a fresh home and lets its consensus loop commit blocks
// until height.
func startRunningChain(t *testing.T, height types.Height) *vexonode.Node {
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
	wire, err := transport.NewInMemoryBus().NewPeer("light-test-validator")
	if err != nil {
		t.Fatal(err)
	}
	node.WithTransport(wire)
	if err := node.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { node.Stop(context.Background()) })
	loop := vexonode.ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Second, MaxBlockBytes: 1 << 20, CreateEmptyBlocks: true, ExecutionCommitMode: vexonode.ExecutionCommitModeFinalized}
	if err := node.StartConsensusLoop(ctx, loop); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for node.Status(ctx).LatestHeight < height {
		if time.Now().After(deadline) {
			t.Fatalf("chain stalled at height %d", node.Status(ctx).LatestHeight)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := node.StopConsensusLoop(ctx); err != nil {
		t.Fatal(err)
	}
	return node
}

func lightTrustFlags(t *testing.T, node *vexonode.Node, height types.Height) []string {
	t.Helper()
	record, err := node.BlockByHeight(context.Background(), height)
	if err != nil {
		t.Fatal(err)
	}
	hash := finality.HeaderHash(record.Block.Header)
	return []string{"--trust-height", strconv.FormatUint(uint64(height), 10), "--trust-hash", hex.EncodeToString(hash[:])}
}

// light verify follows a running chain over its RPC from a trusted block to the requested block.
func TestLightVerifyFollowsAChainOverRPC(t *testing.T) {
	node := startRunningChain(t, 8)
	server := httptest.NewServer(vexorpc.NewHandler(node))
	defer server.Close()

	var output bytes.Buffer
	args := append([]string{"verify", "--rpc", server.URL, "--height", "6", "--trusting-period", "1h"}, lightTrustFlags(t, node, 2)...)
	if err := runLight(&output, args); err != nil {
		t.Fatalf("%v: %s", err, output.String())
	}
	var result lightVerifyResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	record, err := node.BlockByHeight(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	blockHash := finality.HeaderHash(record.Block.Header)
	if result.Height != 6 || result.BlockHash != hex.EncodeToString(blockHash[:]) || result.Fetched == 0 {
		t.Fatalf("expected block 6 %x verified, got %+v", blockHash, result)
	}
}

// The trusted block is the one the operator names: a node serving another block at that height is
// refused, and so is a node whose answer does not verify.
func TestLightVerifyRefusesAnUntrustedNode(t *testing.T) {
	node := startRunningChain(t, 8)
	server := httptest.NewServer(vexorpc.NewHandler(node))
	defer server.Close()

	wrongTrust := []string{"--trust-height", "2", "--trust-hash", strings.Repeat("ab", 32)}
	args := append([]string{"verify", "--rpc", server.URL, "--height", "6", "--trusting-period", "1h"}, wrongTrust...)
	if err := runLight(&bytes.Buffer{}, args); err == nil || !strings.Contains(err.Error(), ErrUntrustedSnapshot.Error()) {
		t.Fatalf("expected a block other than the trusted one to be refused, got %v", err)
	}

	honest := vexorpc.NewHandler(node)
	forging := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasSuffix(request.URL.Path, "/light/6") {
			honest.ServeHTTP(writer, request)
			return
		}
		recorder := httptest.NewRecorder()
		honest.ServeHTTP(recorder, request)
		var block vexorpc.LightBlockResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &block); err != nil {
			t.Error(err)
			return
		}
		block.Proof.Header.AppHash[0] ^= 0xff
		block.Proof.BlockHash = finality.HeaderHash(block.Proof.Header)
		_ = json.NewEncoder(writer).Encode(block)
	}))
	defer forging.Close()
	args = append([]string{"verify", "--rpc", forging.URL, "--height", "6", "--trusting-period", "1h"}, lightTrustFlags(t, node, 2)...)
	if err := runLight(&bytes.Buffer{}, args); err == nil {
		t.Fatal("expected a forged block to be refused")
	}
	if err := runLight(&bytes.Buffer{}, append([]string{"verify", "--rpc", server.URL, "--height", "6"}, lightTrustFlags(t, node, 2)...)); err == nil {
		t.Fatal("expected the trusting period to be required")
	}
}
