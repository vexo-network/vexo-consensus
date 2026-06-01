package node

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/p2p"
)

func (node *Node) observePeerMessage(ctx context.Context, peer p2p.PeerID, valid bool) bool {
	if peer == "" {
		return true
	}
	runtime, err := node.Runtime()
	if err != nil || runtime.P2PScore == nil {
		return true
	}
	if err := runtime.P2PScore.ObserveMessage(ctx, peer, valid); errors.Is(err, p2p.ErrPeerBanned) {
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
