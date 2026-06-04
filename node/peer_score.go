package node

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/p2p"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/transport"
)

func (node *Node) observePeerMessage(ctx context.Context, peer p2p.PeerID, valid bool) bool {
	if peer == "" {
		return true
	}
	runtime, err := node.Runtime()
	if err != nil || runtime.P2PScore == nil {
		return true
	}
	err = runtime.P2PScore.ScoreMessage(ctx, peer, valid)
	_ = node.persistPeerScores()
	if errors.Is(err, p2p.ErrPeerBanned) {
		node.disconnectPeer(peer)
		return false
	}
	return true
}

func (node *Node) admitPeerMessage(ctx context.Context, peer p2p.PeerID) bool {
	if peer == "" {
		return true
	}
	runtime, err := node.Runtime()
	if err != nil || runtime.P2PScore == nil {
		return true
	}
	err = runtime.P2PScore.AdmitMessage(ctx, peer)
	if errors.Is(err, p2p.ErrPeerBanned) || errors.Is(err, p2p.ErrRateLimitExceeded) {
		_ = node.persistPeerScores()
		if errors.Is(err, p2p.ErrPeerBanned) || node.peerBanned(ctx, peer) {
			node.disconnectPeer(peer)
		}
		return false
	}
	return true
}

func (node *Node) peerBanned(ctx context.Context, peer p2p.PeerID) bool {
	if peer == "" {
		return false
	}
	runtime, err := node.Runtime()
	if err != nil || runtime.P2PScore == nil {
		return false
	}
	banned, err := runtime.P2PScore.IsBanned(ctx, peer)
	return err == nil && banned
}

func (node *Node) PeerScore(ctx context.Context, peer p2p.PeerID) (int64, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return 0, err
	}
	return runtime.P2PScore.Score(ctx, peer)
}

func (node *Node) PeerScores(ctx context.Context) ([]p2p.PeerSnapshot, error) {
	runtime, err := node.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.P2PScore.Snapshot(ctx)
}

func (node *Node) configureTransportPeerGate(runtime *vexoruntime.Runtime) {
	if runtime == nil || runtime.P2PScore == nil || node.wire == nil {
		return
	}
	gate := func(ctx context.Context, peer p2p.PeerID) error {
		banned, err := runtime.P2PScore.IsBanned(ctx, peer)
		if err != nil {
			return err
		}
		if banned {
			return p2p.ErrPeerBanned
		}
		return nil
	}
	if chained, ok := node.wire.(transport.PeerGateChainTransport); ok {
		chained.AddPeerGate(gate)
		return
	}
	gated, ok := node.wire.(transport.PeerGateTransport)
	if !ok {
		return
	}
	gated.SetPeerGate(gate)
}

func (node *Node) disconnectPeer(peer p2p.PeerID) {
	if peer == "" {
		return
	}
	node.mu.Lock()
	wire := node.wire
	node.mu.Unlock()
	disconnecting, ok := wire.(transport.PeerDisconnectTransport)
	if !ok {
		return
	}
	disconnecting.DisconnectPeer(peer)
}

func (node *Node) persistPeerScores() error {
	return nil
}

func (node *Node) persistPeerScoresLocked() error {
	return savePeerScores(node.runtime, node.cfg.PeerScorePath())
}

func savePeerScores(runtime *vexoruntime.Runtime, path string) error {
	if runtime == nil || runtime.P2PScore == nil {
		return nil
	}
	return runtime.P2PScore.SaveFile(path)
}
