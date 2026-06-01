package p2p

import (
	"context"
	"errors"
	"sync"
	"time"
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
	WindowResetInterval  time.Duration
	ScoreRecovery        int64
	BanDuration          time.Duration
}

type PeerState struct {
	Score          int64
	Banned         bool
	BannedUntil    time.Time
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
	if err := keeper.AdmitMessage(ctx, peer); err != nil {
		return err
	}
	return keeper.ScoreMessage(ctx, peer, valid)
}

func (keeper *ScoreKeeper) AdmitMessage(ctx context.Context, peer PeerID) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	state := keeper.state(peer)
	state = keeper.expireBan(state)
	if state.Banned {
		keeper.peers[peer] = state
		return ErrPeerBanned
	}
	state.WindowMessages++
	if keeper.config.MaxMessagesPerWindow > 0 && state.WindowMessages > keeper.config.MaxMessagesPerWindow {
		state.Score -= keeper.config.RateLimitCost
		state = keeper.applyBan(state)
		keeper.peers[peer] = state
		return ErrRateLimitExceeded
	}
	keeper.peers[peer] = state
	return nil
}

func (keeper *ScoreKeeper) ScoreMessage(ctx context.Context, peer PeerID, valid bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	state := keeper.state(peer)
	state = keeper.expireBan(state)
	if state.Banned {
		keeper.peers[peer] = state
		return ErrPeerBanned
	}
	if valid {
		state.Score += keeper.config.ValidMessageReward
	} else {
		state.Score -= keeper.config.InvalidMessageCost
	}
	state = keeper.applyBan(state)
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
		state = keeper.expireBan(state)
		state.WindowMessages = 0
		state = keeper.recoverScore(state)
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
	state := keeper.expireBan(keeper.state(peer))
	keeper.peers[peer] = state
	return state.Banned, nil
}

func (keeper *ScoreKeeper) WindowMessages(ctx context.Context, peer PeerID) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	return keeper.state(peer).WindowMessages, nil
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

func (keeper *ScoreKeeper) applyBan(state PeerState) PeerState {
	if !keeper.shouldBan(state.Score) {
		state.Banned = false
		state.BannedUntil = time.Time{}
		return state
	}
	state.Banned = true
	if keeper.config.BanDuration > 0 {
		state.BannedUntil = time.Now().Add(keeper.config.BanDuration)
	}
	return state
}

func (keeper *ScoreKeeper) expireBan(state PeerState) PeerState {
	if !state.Banned || state.BannedUntil.IsZero() || time.Now().Before(state.BannedUntil) {
		return state
	}
	state.Banned = false
	state.BannedUntil = time.Time{}
	state.Score = keeper.config.InitialScore
	state.WindowMessages = 0
	return state
}

func (keeper *ScoreKeeper) recoverScore(state PeerState) PeerState {
	if state.Banned || keeper.config.ScoreRecovery <= 0 || state.Score >= keeper.config.InitialScore {
		return state
	}
	state.Score += keeper.config.ScoreRecovery
	if state.Score > keeper.config.InitialScore {
		state.Score = keeper.config.InitialScore
	}
	return state
}
