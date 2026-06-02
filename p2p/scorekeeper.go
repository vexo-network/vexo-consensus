package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const ScoreKeeperVersionV1 = "v1"

var (
	ErrPeerBanned        = errors.New("peer is banned")
	ErrRateLimitExceeded = errors.New("peer rate limit exceeded")
)

type ScoreConfig struct {
	InitialScore              int64
	ValidMessageReward        int64
	InvalidMessageCost        int64
	RateLimitCost             int64
	BanThreshold              int64
	MaxMessagesPerWindow      uint64
	MaxTotalMessagesPerWindow uint64
	WindowResetInterval       time.Duration
	ScoreRecovery             int64
	BanDuration               time.Duration
}

type PeerState struct {
	Score          int64
	Banned         bool
	BannedUntil    time.Time
	WindowMessages uint64
}

type PeerSnapshot struct {
	Peer           PeerID
	Score          int64
	Banned         bool
	BannedUntil    time.Time
	WindowMessages uint64
}

type ScoreDocument struct {
	SchemaVersion       string            `json:"schema_version"`
	TotalWindowMessages uint64            `json:"total_window_messages,omitempty"`
	Peers               []PeerScoreRecord `json:"peers"`
}

type PeerScoreRecord struct {
	Peer           PeerID `json:"peer"`
	Score          int64  `json:"score"`
	Banned         bool   `json:"banned,omitempty"`
	BannedUntil    string `json:"banned_until,omitempty"`
	WindowMessages uint64 `json:"window_messages,omitempty"`
}

type ScoreKeeper struct {
	mu                  sync.Mutex
	config              ScoreConfig
	peers               map[PeerID]PeerState
	totalWindowMessages uint64
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
	keeper.totalWindowMessages++
	if keeper.config.MaxTotalMessagesPerWindow > 0 && keeper.totalWindowMessages > keeper.config.MaxTotalMessagesPerWindow {
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
	keeper.totalWindowMessages = 0
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

func (keeper *ScoreKeeper) TotalWindowMessages(ctx context.Context) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()
	return keeper.totalWindowMessages, nil
}

func (keeper *ScoreKeeper) Snapshot(ctx context.Context) ([]PeerSnapshot, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	peers := make([]PeerID, 0, len(keeper.peers))
	for peer := range keeper.peers {
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i] < peers[j]
	})

	snapshot := make([]PeerSnapshot, 0, len(peers))
	for _, peer := range peers {
		state := keeper.expireBan(keeper.peers[peer])
		keeper.peers[peer] = state
		snapshot = append(snapshot, PeerSnapshot{
			Peer:           peer,
			Score:          state.Score,
			Banned:         state.Banned,
			BannedUntil:    state.BannedUntil,
			WindowMessages: state.WindowMessages,
		})
	}
	return snapshot, nil
}

func (keeper *ScoreKeeper) SaveFile(path string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	document := keeper.document()
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (keeper *ScoreKeeper) LoadFile(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var document ScoreDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	if document.SchemaVersion != "" && document.SchemaVersion != ScoreKeeperVersionV1 {
		return nil
	}
	keeper.restore(document)
	return nil
}

func (keeper *ScoreKeeper) document() ScoreDocument {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	peers := make([]PeerID, 0, len(keeper.peers))
	for peer := range keeper.peers {
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i] < peers[j]
	})
	records := make([]PeerScoreRecord, 0, len(peers))
	for _, peer := range peers {
		state := keeper.expireBan(keeper.peers[peer])
		keeper.peers[peer] = state
		record := PeerScoreRecord{
			Peer:           peer,
			Score:          state.Score,
			Banned:         state.Banned,
			WindowMessages: state.WindowMessages,
		}
		if !state.BannedUntil.IsZero() {
			record.BannedUntil = state.BannedUntil.UTC().Format(time.RFC3339Nano)
		}
		records = append(records, record)
	}
	return ScoreDocument{
		SchemaVersion:       ScoreKeeperVersionV1,
		TotalWindowMessages: keeper.totalWindowMessages,
		Peers:               records,
	}
}

func (keeper *ScoreKeeper) restore(document ScoreDocument) {
	keeper.mu.Lock()
	defer keeper.mu.Unlock()

	peers := make(map[PeerID]PeerState, len(document.Peers))
	for _, record := range document.Peers {
		if record.Peer == "" {
			continue
		}
		state := PeerState{
			Score:          record.Score,
			Banned:         record.Banned,
			WindowMessages: record.WindowMessages,
		}
		if record.BannedUntil != "" {
			if bannedUntil, err := time.Parse(time.RFC3339Nano, record.BannedUntil); err == nil {
				state.BannedUntil = bannedUntil
			}
		}
		state = keeper.expireBan(state)
		peers[record.Peer] = state
	}
	keeper.peers = peers
	keeper.totalWindowMessages = document.TotalWindowMessages
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
