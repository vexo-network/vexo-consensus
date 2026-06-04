package node

import (
	"context"
	"time"
)

func (node *Node) startPeerScoreWindowReset(ctx context.Context) {
	interval := node.cfg.Chain.P2P.WindowResetInterval
	if interval <= 0 {
		return
	}
	runtime := node.runtime
	if runtime == nil || runtime.P2PScore == nil {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	node.scoreCancel = cancel
	node.scoreDone = done
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		lastSaved := time.Time{}
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := runtime.P2PScore.ResetWindow(runCtx); err != nil {
					return
				}
				now := time.Now()
				if lastSaved.IsZero() || now.Sub(lastSaved) >= time.Second {
					_ = runtime.P2PScore.SaveFile(node.cfg.PeerScorePath())
					lastSaved = now
				}
			}
		}
	}()
}
