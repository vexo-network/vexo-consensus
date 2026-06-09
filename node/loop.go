package node

import (
	"context"
	"errors"
	goruntime "runtime"
	"time"
)

const (
	defaultConsensusLoopInterval     = 50 * time.Millisecond
	defaultConsensusTimeoutPropose   = 3 * time.Second
	defaultConsensusTimeoutPrevote   = time.Second
	defaultConsensusTimeoutPrecommit = time.Second
	defaultConsensusTimeoutCommit    = time.Second
	defaultConsensusMaxBytes         = 1024 * 1024
)

type ExecutionCommitMode string

const (
	ExecutionCommitModeQC        ExecutionCommitMode = "qc"
	ExecutionCommitModeFinalized ExecutionCommitMode = "finalized"
)

type ConsensusLoopConfig struct {
	Interval            time.Duration
	TimeoutPropose      time.Duration
	TimeoutPrevote      time.Duration
	TimeoutPrecommit    time.Duration
	TimeoutCommit       time.Duration
	RoundTimeout        time.Duration
	MaxBlockBytes       int64
	CreateEmptyBlocks   bool
	ExecutionCommitMode ExecutionCommitMode
}

func DefaultConsensusLoopConfig() ConsensusLoopConfig {
	return ConsensusLoopConfig{
		Interval:            defaultConsensusLoopInterval,
		TimeoutPropose:      defaultConsensusTimeoutPropose,
		TimeoutPrevote:      defaultConsensusTimeoutPrevote,
		TimeoutPrecommit:    defaultConsensusTimeoutPrecommit,
		TimeoutCommit:       defaultConsensusTimeoutCommit,
		MaxBlockBytes:       defaultConsensusMaxBytes,
		CreateEmptyBlocks:   false,
		ExecutionCommitMode: ExecutionCommitModeFinalized,
	}
}

func (node *Node) StartConsensusLoop(ctx context.Context, cfg ConsensusLoopConfig) error {
	cfg = normalizeConsensusLoopConfig(cfg)

	node.mu.Lock()
	if !node.running {
		node.mu.Unlock()
		return ErrNodeNotRunning
	}
	if node.loopCancel != nil {
		node.mu.Unlock()
		return ErrLoopAlreadyRunning
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	node.loopCancel = cancel
	node.loopDone = done
	node.loopConfig = cfg
	node.mu.Unlock()

	if err := node.runConsensusStartupBurst(ctx, cfg); err != nil {
		cancel()
		node.mu.Lock()
		if node.loopDone == done {
			node.loopCancel = nil
			node.loopDone = nil
			node.loopConfig = ConsensusLoopConfig{}
		}
		node.mu.Unlock()
		close(done)
		return err
	}

	go node.runConsensusLoop(runCtx, cfg, done)
	return nil
}

func (node *Node) runConsensusStartupBurst(ctx context.Context, cfg ConsensusLoopConfig) error {
	for attempt := 0; attempt < 4; attempt++ {
		if _, err := node.StepConsensusWithConfig(ctx, cfg); err != nil {
			return err
		}
		goruntime.Gosched()
	}
	return nil
}

func (node *Node) StopConsensusLoop(ctx context.Context) error {
	node.mu.Lock()
	cancel := node.loopCancel
	done := node.loopDone
	if cancel == nil {
		node.mu.Unlock()
		return ErrLoopNotRunning
	}
	node.loopCancel = nil
	node.loopDone = nil
	node.loopConfig = ConsensusLoopConfig{}
	node.mu.Unlock()

	cancel()
	return waitLoopDone(ctx, done)
}

func (node *Node) ConsensusLoopRunning() bool {
	node.mu.Lock()
	defer node.mu.Unlock()
	return node.loopCancel != nil
}

func (node *Node) runConsensusLoop(ctx context.Context, cfg ConsensusLoopConfig, done chan struct{}) {
	defer node.clearConsensusLoop(done)
	defer close(done)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	lastTimeout := time.Now()
	lastCommit := time.Time{}
	for {
		if !lastCommit.IsZero() && cfg.TimeoutCommit > 0 {
			if remaining := cfg.TimeoutCommit - time.Since(lastCommit); remaining > 0 {
				if !sleepConsensusLoop(ctx, remaining) {
					return
				}
			}
		}
		result, err := node.StepConsensusWithConfig(ctx, cfg)
		if err != nil {
			if errors.Is(err, ErrNodeNotRunning) {
				return
			}
			node.logEvent("consensus_step_failed", map[string]any{
				"error": err.Error(),
			})
		}
		if result.Committed || result.Proposed {
			lastTimeout = time.Now()
		}
		if result.Committed {
			lastCommit = time.Now()
		}
		if !result.Committed && !result.Proposed && time.Since(lastTimeout) >= cfg.roundTimeout() {
			node.metrics.observeRoundTimeout()
			if _, _, err := node.TimeoutRound(ctx); err != nil && errors.Is(err, ErrNodeNotRunning) {
				return
			}
			lastTimeout = time.Now()
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (node *Node) clearConsensusLoop(done chan struct{}) {
	node.mu.Lock()
	defer node.mu.Unlock()
	if node.loopDone != done {
		return
	}
	node.loopCancel = nil
	node.loopDone = nil
	node.loopConfig = ConsensusLoopConfig{}
}

func normalizeConsensusLoopConfig(cfg ConsensusLoopConfig) ConsensusLoopConfig {
	defaults := DefaultConsensusLoopConfig()
	if cfg.Interval <= 0 {
		cfg.Interval = defaults.Interval
	}
	if cfg.TimeoutPropose <= 0 {
		cfg.TimeoutPropose = defaults.TimeoutPropose
	}
	if cfg.TimeoutPrevote <= 0 {
		cfg.TimeoutPrevote = defaults.TimeoutPrevote
	}
	if cfg.TimeoutPrecommit <= 0 {
		cfg.TimeoutPrecommit = defaults.TimeoutPrecommit
	}
	if cfg.TimeoutCommit <= 0 {
		cfg.TimeoutCommit = defaults.TimeoutCommit
	}
	if cfg.RoundTimeout <= 0 {
		cfg.RoundTimeout = cfg.roundTimeout()
	}
	if cfg.MaxBlockBytes <= 0 {
		cfg.MaxBlockBytes = defaults.MaxBlockBytes
	}
	if cfg.ExecutionCommitMode == "" {
		cfg.ExecutionCommitMode = defaults.ExecutionCommitMode
	}
	return cfg
}

func (cfg ConsensusLoopConfig) roundTimeout() time.Duration {
	if cfg.RoundTimeout > 0 {
		return cfg.RoundTimeout
	}
	return cfg.TimeoutPropose + cfg.TimeoutPrevote + cfg.TimeoutPrecommit
}

func sleepConsensusLoop(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func waitLoopDone(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
