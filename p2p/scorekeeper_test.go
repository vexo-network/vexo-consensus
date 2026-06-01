package p2p

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestScoreKeeperRewardsValidMessages(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:       10,
		ValidMessageReward: 2,
		BanThreshold:       0,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatal(err)
	}
	score, err := keeper.Score(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if score != 12 {
		t.Fatalf("expected score 12, got %d", score)
	}
}

func TestScoreKeeperPenalizesInvalidMessagesAndBans(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:       3,
		InvalidMessageCost: 2,
		BanThreshold:       0,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", false); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", false); !errors.Is(err, ErrPeerBanned) {
		t.Fatalf("expected peer banned, got %v", err)
	}
	banned, err := keeper.IsBanned(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if !banned {
		t.Fatal("expected peer banned")
	}
}

func TestScoreKeeperRejectsAlreadyBannedPeer(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:       1,
		InvalidMessageCost: 1,
		BanThreshold:       0,
	})
	if err := keeper.ObserveMessage(context.Background(), "peer-a", false); !errors.Is(err, ErrPeerBanned) {
		t.Fatalf("expected peer banned, got %v", err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); !errors.Is(err, ErrPeerBanned) {
		t.Fatalf("expected already banned peer rejected, got %v", err)
	}
}

func TestScoreKeeperUnbansAfterBanDuration(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:       5,
		InvalidMessageCost: 5,
		BanThreshold:       0,
		BanDuration:        time.Nanosecond,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", false); !errors.Is(err, ErrPeerBanned) {
		t.Fatalf("expected peer banned, got %v", err)
	}
	waitForUnbanned(t, keeper, "peer-a")
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatalf("expected unbanned peer accepted, got %v", err)
	}
	score, err := keeper.Score(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if score != 5 {
		t.Fatalf("expected score reset to 5 after unban and valid reward, got %d", score)
	}
}

func TestScoreKeeperRateLimitsPeer(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:         10,
		RateLimitCost:        3,
		BanThreshold:         0,
		MaxMessagesPerWindow: 2,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected rate limit, got %v", err)
	}
	score, err := keeper.Score(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if score != 7 {
		t.Fatalf("expected score 7 after rate limit cost, got %d", score)
	}
}

func TestScoreKeeperResetWindowAllowsMessagesAgain(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:         10,
		MaxMessagesPerWindow: 1,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected rate limit, got %v", err)
	}
	if err := keeper.ResetWindow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatalf("expected message after reset, got %v", err)
	}
}

func TestScoreKeeperResetWindowRecoversScore(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:       10,
		InvalidMessageCost: 4,
		BanThreshold:       0,
		ScoreRecovery:      3,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", false); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ResetWindow(context.Background()); err != nil {
		t.Fatal(err)
	}
	score, err := keeper.Score(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if score != 9 {
		t.Fatalf("expected recovered score 9, got %d", score)
	}
	if err := keeper.ResetWindow(context.Background()); err != nil {
		t.Fatal(err)
	}
	score, err = keeper.Score(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("expected recovered score capped at 10, got %d", score)
	}
}

func TestScoreKeeperDoesNotRecoverActiveBan(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:       5,
		InvalidMessageCost: 5,
		BanThreshold:       0,
		ScoreRecovery:      10,
		BanDuration:        time.Hour,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", false); !errors.Is(err, ErrPeerBanned) {
		t.Fatalf("expected peer banned, got %v", err)
	}
	if err := keeper.ResetWindow(context.Background()); err != nil {
		t.Fatal(err)
	}
	score, err := keeper.Score(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("expected active banned score unchanged, got %d", score)
	}
}

func TestScoreKeeperSnapshotReturnsSortedPeerStates(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:       10,
		InvalidMessageCost: 3,
		ValidMessageReward: 2,
		BanThreshold:       0,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-c", false); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-b", false); err != nil {
		t.Fatal(err)
	}

	snapshot, err := keeper.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(snapshot))
	}
	expected := []struct {
		peer  PeerID
		score int64
	}{
		{peer: "peer-a", score: 12},
		{peer: "peer-b", score: 7},
		{peer: "peer-c", score: 7},
	}
	for i, item := range expected {
		if snapshot[i].Peer != item.peer || snapshot[i].Score != item.score {
			t.Fatalf("expected snapshot[%d] peer=%s score=%d, got %+v", i, item.peer, item.score, snapshot[i])
		}
		if snapshot[i].Banned {
			t.Fatalf("expected peer %s not banned", snapshot[i].Peer)
		}
		if snapshot[i].WindowMessages != 1 {
			t.Fatalf("expected peer %s window messages 1, got %d", snapshot[i].Peer, snapshot[i].WindowMessages)
		}
	}
}

func TestScoreKeeperRateLimitCanBan(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{
		InitialScore:         1,
		RateLimitCost:        2,
		BanThreshold:         0,
		MaxMessagesPerWindow: 1,
	})

	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatal(err)
	}
	if err := keeper.ObserveMessage(context.Background(), "peer-a", true); !errors.Is(err, ErrRateLimitExceeded) {
		t.Fatalf("expected rate limit exceeded, got %v", err)
	}
	banned, err := keeper.IsBanned(context.Background(), "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	if !banned {
		t.Fatal("expected peer banned by rate limit")
	}
}

func TestScoreKeeperUnknownPeerHasInitialScore(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{InitialScore: 5})
	score, err := keeper.Score(context.Background(), "unknown")
	if err != nil {
		t.Fatal(err)
	}
	if score != 5 {
		t.Fatalf("expected initial score 5, got %d", score)
	}
}

func TestScoreKeeperContextCancellation(t *testing.T) {
	keeper := NewScoreKeeper(ScoreConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := keeper.ObserveMessage(ctx, "peer-a", true); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected observe canceled, got %v", err)
	}
	if err := keeper.ResetWindow(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected reset canceled, got %v", err)
	}
	if _, err := keeper.Score(ctx, "peer-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected score canceled, got %v", err)
	}
	if _, err := keeper.IsBanned(ctx, "peer-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected banned canceled, got %v", err)
	}
	if _, err := keeper.WindowMessages(ctx, "peer-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected window messages canceled, got %v", err)
	}
	if _, err := keeper.Snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected snapshot canceled, got %v", err)
	}
}

func waitForUnbanned(t *testing.T, keeper *ScoreKeeper, peer PeerID) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		banned, err := keeper.IsBanned(context.Background(), peer)
		if err == nil && !banned {
			return
		}
		time.Sleep(time.Millisecond)
	}
	banned, err := keeper.IsBanned(context.Background(), peer)
	if err != nil {
		t.Fatal(err)
	}
	if banned {
		t.Fatalf("expected peer %s unbanned", peer)
	}
}
