package node

import (
	"context"
	"time"

	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

type Metrics struct {
	ChainID              string
	Running              bool
	StartedAtUnix        int64
	UptimeSeconds        uint64
	DataDir              string
	LatestHeight         types.Height
	LatestAppHash        types.Hash
	EarliestBlockHeight  types.Height
	LatestBlockHeight    types.Height
	TotalBlocks          uint64
	ValidatorCount       int
	TotalVotingPower     uint64
	ValidatorSetHash     types.Hash
	PeerCount            int
	BannedPeers          int
	PeerWindowMessages   uint64
	ConsensusLoopRunning bool
	HeightRatePerMinute  float64
	RoundTimeouts        uint64
	ProposalLatencyNanos uint64
	VoteLatencyNanos     uint64
	MempoolSize          uint64
	CommitLatencyNanos   uint64
	SnapshotHealthy      bool
	ReplayHealthy        bool
	SigningFailures      uint64
}

func (node *Node) Metrics(ctx context.Context) (Metrics, error) {
	status := node.Status(ctx)
	metrics := Metrics{
		ChainID:            status.ChainID,
		Running:            status.Running,
		DataDir:            status.DataDir,
		LatestHeight:       status.LatestHeight,
		LatestAppHash:      status.LatestAppHash,
		PeerCount:          status.PeerCount,
		BannedPeers:        status.BannedPeers,
		PeerWindowMessages: peerWindowMessages(status.Peers),
	}
	if !status.StartedAt.IsZero() {
		metrics.StartedAtUnix = status.StartedAt.Unix()
		metrics.UptimeSeconds = uint64(time.Since(status.StartedAt).Seconds())
	}

	node.mu.Lock()
	metrics.ConsensusLoopRunning = node.loopCancel != nil
	node.mu.Unlock()

	if !status.Running {
		return metrics, nil
	}
	runtime, err := node.Runtime()
	if err != nil {
		return Metrics{}, err
	}
	if runtime.Mempool != nil {
		metrics.MempoolSize = uint64(runtime.Mempool.Len())
	}
	index, err := runtime.BlockIndex(ctx)
	if err == nil {
		metrics.EarliestBlockHeight = index.EarliestHeight
		metrics.LatestBlockHeight = index.LatestHeight
		metrics.TotalBlocks = index.TotalBlocks
	} else if !ignoreMissingMetricsError(err) {
		return Metrics{}, err
	}

	validatorHeight := metrics.LatestHeight
	if validatorHeight == 0 {
		validatorHeight = 1
	}
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, validatorHeight)
	if err != nil {
		return Metrics{}, err
	}
	hash := validatorSet.Hash()
	metrics.ValidatorSetHash = hash
	for _, validatorInfo := range validatorSet.List() {
		metrics.ValidatorCount++
		metrics.TotalVotingPower = types.MustAddUint64Saturating(metrics.TotalVotingPower, uint64(validatorInfo.VotingPower))
	}
	if runtime.P2PScore != nil {
		totalMessages, err := runtime.P2PScore.TotalWindowMessages(ctx)
		if err == nil {
			metrics.PeerWindowMessages = totalMessages
		}
	}
	return metrics, nil
}

func peerWindowMessages(peers []p2p.PeerSnapshot) uint64 {
	var total uint64
	for _, peer := range peers {
		total = types.MustAddUint64Saturating(total, peer.WindowMessages)
	}
	return total
}
