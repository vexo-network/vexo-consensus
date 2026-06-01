package p2p

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type PeerID string

type Topic string

const (
	TopicTx       Topic = "tx"
	TopicBatch    Topic = "batch"
	TopicProposal Topic = "proposal"
	TopicVote     Topic = "vote"
	TopicTimeout  Topic = "timeout_vote"
	TopicCommit   Topic = "commit"
	TopicEvidence Topic = "evidence"
)

type Message struct {
	Topic Topic
	From  PeerID
	Data  []byte
}

type Network interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Broadcast(ctx context.Context, topic Topic, data []byte) error
	Subscribe(ctx context.Context, topic Topic) (<-chan Message, error)
	PeerScore(ctx context.Context, peer PeerID) int64
}

type PeerInfo struct {
	ID          PeerID
	Address     string
	ValidatorID types.ValidatorID
	Score       int64
}
