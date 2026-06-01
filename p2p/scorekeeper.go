package p2p

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrPeerBanned        = errors.New("peer is banned")
	ErrRateLimitExceeded = errors.New("peer rate limit exceeded")
)

type ScoreConfig struct {
	InitialScore         int64
	ValidMessageReward   int64
	InvalidMessageCost   int64
	RateLimitCost        int64
	BanThreshold         int64
	MaxMessagesPerWindow uint64
}

type PeerState struct {
	Score          int64
	Banned         bool
	WindowMessages uint64
}

type ScoreKeeper struct {
	mu     sync.Mutex
	config ScoreConfig
	peers  map[PeerID]PeerState
}

func NewScoreKeeper(config ScoreConfig) *ScoreKeeper {
	if config.InvalidMessageCost == 0 {
		config.InvalidMessageCost = 1
	}
	if config.RateLimitCost == 0 {
		config.RateLimitCost = 1
	}
	return &ScoreKeeper{
		config: config,
		peers:  make(map[PeerID]PeerState),
	}
}

func (keeper *ScoreKeeper) ObserveMessage(ctx context.Context, peer PeerID, valid bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	state := keeper.state(peer)
	if state.Banned {
		return ErrPeerBanned
	}

	state.WindowMessages++
	if keeper.config.MaxMessagesPerWindow > 0 && state.WindowMessages > keeper.config.MaxMessagesPerWindow {
		state.Score -= keeper.config.RateLimitCost
		state.Banned = keeper.shouldBan(state.Score)
		keeper.peers[peer] = state
		return ErrRateLimitExceeded
	}

	if valid {
		state.Score += keeper.config.ValidMessageReward
	} else {
		state.Score -= keeper.config.InvalidMessageCost
	}
	state.Banned = keeper.shouldBan(state.Score)
	keeper.peers[peer] = state

	if state.Banned {
		return ErrPeerBanned
	}
	return nil
}

func (keeper *ScoreKeeper) ResetWindow(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	for peer, state := range keeper.peers {
		state.WindowMessages = 0
		keeper.peers[peer] = state
	}
	return nil
}

func (keeper *ScoreKeeper) Score(ctx context.Context, peer PeerID) (int64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	return keeper.state(peer).Score, nil
}

func (keeper *ScoreKeeper) IsBanned(ctx context.Context, peer PeerID) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	return keeper.state(peer).Banned, nil
}

func (keeper *ScoreKeeper) state(peer PeerID) PeerState {
	state, found := keeper.peers[peer]
	if !found {
		return PeerState{Score: keeper.config.InitialScore}
	}
	return state
}

func (keeper *ScoreKeeper) shouldBan(score int64) bool {
	return score <= keeper.config.BanThreshold
}
