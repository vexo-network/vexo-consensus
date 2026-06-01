package node

import (
	"context"
	"errors"
	"time"
)

const (
	defaultConsensusLoopInterval = 50 * time.Millisecond
	defaultConsensusRoundTimeout = 500 * time.Millisecond
	defaultConsensusMaxBytes     = 1024 * 1024
)

type ConsensusLoopConfig struct {
	Interval      time.Duration
	RoundTimeout  time.Duration
	MaxBlockBytes int64
}

func DefaultConsensusLoopConfig() ConsensusLoopConfig {
	return ConsensusLoopConfig{
		Interval:      defaultConsensusLoopInterval,
		RoundTimeout:  defaultConsensusRoundTimeout,
		MaxBlockBytes: defaultConsensusMaxBytes,
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
	node.mu.Unlock()

	go node.runConsensusLoop(runCtx, cfg, done)
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
	for {
		result, err := node.StepConsensus(ctx, cfg.MaxBlockBytes)
		if err != nil {
			if errors.Is(err, ErrNodeNotRunning) {
				return
			}
		}
		if result.Committed || result.Proposed {
			lastTimeout = time.Now()
		}
		if !result.Committed && !result.Proposed && time.Since(lastTimeout) >= cfg.RoundTimeout {
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
}

func normalizeConsensusLoopConfig(cfg ConsensusLoopConfig) ConsensusLoopConfig {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultConsensusLoopInterval
	}
	if cfg.RoundTimeout <= 0 {
		cfg.RoundTimeout = defaultConsensusRoundTimeout
	}
	if cfg.MaxBlockBytes <= 0 {
		cfg.MaxBlockBytes = defaultConsensusMaxBytes
	}
	return cfg
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
