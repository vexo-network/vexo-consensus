package node

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

// nodeLightSource serves a node's light blocks, optionally tampered with on the way.
type nodeLightSource struct {
	node   *Node
	tamper func(*finality.LightBlock)
}

func (source nodeLightSource) LightBlock(ctx context.Context, height types.Height) (finality.LightBlock, error) {
	block, err := source.node.LightBlock(ctx, height)
	if err == nil && source.tamper != nil {
		source.tamper(&block)
	}
	return block, err
}

// A light client that trusts an early block of a running chain verifies a later block with the
// proofs and validator sets a full node serves, and refuses the same data once it is tampered with.
func TestLightClientFollowsARunningChainFromATrustedBlock(t *testing.T) {
	ctx := context.Background()
	alice, bob, carol := newBLSConsensusLoopNodes(t)
	nodes := []*Node{alice, bob, carol}
	for _, node := range nodes {
		startNode(t, node)
		defer node.Stop(ctx)
	}
	loopConfig := ConsensusLoopConfig{Interval: time.Millisecond, RoundTimeout: time.Second, MaxBlockBytes: 1 << 20, CreateEmptyBlocks: true, ExecutionCommitMode: ExecutionCommitModeFinalized}
	for _, node := range nodes {
		if err := node.StartConsensusLoop(ctx, loopConfig); err != nil {
			t.Fatal(err)
		}
	}
	waitForNodeHeight(t, alice, 8)
	for _, node := range nodes {
		if err := node.StopConsensusLoop(ctx); err != nil {
			t.Fatal(err)
		}
	}
	runtime, err := alice.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	trustedBlock, err := alice.LightBlock(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := finality.NewTrustedBlock(trustedBlock.Proof.Header, trustedBlock.NextValidatorSet)
	if err != nil {
		t.Fatal(err)
	}
	options := finality.LightClientOptions{TrustingPeriod: time.Hour, MaxClockDrift: time.Second}
	newClient := func(source finality.LightSource) *finality.LightClient {
		// A fresh suite holds no registered key: the client registers the keys of every set it
		// fetches, with their proofs of possession, before it verifies with them.
		suite, err := vexocrypto.NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendBLS})
		if err != nil {
			t.Fatal(err)
		}
		client, err := finality.NewLightClient(trusted, source, options, suite)
		if err != nil {
			t.Fatal(err)
		}
		return client
	}

	verified, err := newClient(nodeLightSource{node: alice}).VerifyTo(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	record, err := runtime.BlockByHeight(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Header != record.Block.Header {
		t.Fatalf("expected the client to trust the committed block 6, got %+v", verified.Header)
	}
	if verified.Header.TimeUnixNano <= trusted.Header.TimeUnixNano || verified.Header.TimeUnixNano > time.Now().UnixNano() {
		t.Fatalf("expected block 6 to carry a real time after the trusted block's, got %d", verified.Header.TimeUnixNano)
	}

	tampered := nodeLightSource{node: alice, tamper: func(block *finality.LightBlock) {
		block.Proof.Header.AppHash[0] ^= 0xff
		block.Proof.BlockHash = finality.HeaderHash(block.Proof.Header)
	}}
	if _, err := newClient(tampered).VerifyTo(ctx, 6); err == nil {
		t.Fatal("expected a tampered header to be refused")
	}
	noChild := nodeLightSource{node: alice, tamper: func(block *finality.LightBlock) { block.Proof.CommitChain = nil }}
	if _, err := newClient(noChild).VerifyTo(ctx, 6); !errors.Is(err, finality.ErrCommitChainTooShort) {
		t.Fatalf("expected a block without its commit child to be refused, got %v", err)
	}
}
