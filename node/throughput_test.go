package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/types"
)

// TestConsensusThroughput measures how fast three in-process validators commit a stream of
// transactions in the finalized execution mode. Clients submit to every validator at once and
// back off while a mempool is full. It runs only with VEXO_THROUGHPUT=<txs>, so the numbers it
// logs are a measurement, not a gate:
//
//	VEXO_THROUGHPUT=200000 go test ./node -run TestConsensusThroughput -v -cpuprofile cpu.out
func TestConsensusThroughput(t *testing.T) {
	total, err := strconv.Atoi(os.Getenv("VEXO_THROUGHPUT"))
	if err != nil || total <= 0 {
		t.Skip("set VEXO_THROUGHPUT=<txs> to measure throughput")
	}
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	bus, alice, bob, carol := newConsensusLoopNodesOnBus(t)
	nodes := []*Node{alice, bob, carol}
	// An observer on the proposal topic receives what each validator receives from the leaders:
	// the leader upload per validator.
	observer, err := bus.NewPeer("observer")
	if err != nil {
		t.Fatal(err)
	}
	if err := observer.Start(ctx); err != nil {
		t.Fatal(err)
	}
	proposalEnvelopes, err := observer.Subscribe(ctx, p2p.TopicProposal)
	if err != nil {
		t.Fatal(err)
	}
	var proposalBytes atomic.Int64
	go func() {
		for envelope := range proposalEnvelopes {
			proposalBytes.Add(int64(len(envelope.Data)))
		}
	}()
	for _, node := range nodes {
		startNode(t, node)
		defer node.Stop(context.Background())
	}
	loopConfig := ConsensusLoopConfig{
		Interval:            time.Millisecond,
		RoundTimeout:        time.Second,
		MaxBlockBytes:       1 << 20,
		CreateEmptyBlocks:   true,
		ExecutionCommitMode: ExecutionCommitModeFinalized,
	}
	started := time.Now()
	for _, node := range nodes {
		if err := node.StartConsensusLoop(ctx, loopConfig); err != nil {
			t.Fatal(err)
		}
		defer node.StopConsensusLoop(context.Background())
	}
	stalled := make(chan struct{})
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		last, since := types.Height(0), time.Now()
		for {
			select {
			case <-stalled:
				return
			case <-time.After(100 * time.Millisecond):
			}
			if height := alice.Status(ctx).LatestHeight; height != last {
				last, since = height, time.Now()
				continue
			}
			if time.Since(since) > 20*time.Second {
				for _, node := range nodes {
					t.Errorf("stalled at height %d: %+v", last, node.Status(ctx))
				}
				ctxCancel()
				return
			}
		}
	}()
	defer func() {
		close(stalled)
		<-watched
	}()
	var clients sync.WaitGroup
	failures := make(chan error, len(nodes))
	for client, node := range nodes {
		clients.Add(1)
		go func() {
			defer clients.Done()
			for index := client; index < total; index += len(nodes) {
				tx := []byte(fmt.Sprintf("bank:throughput-%08d-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", index))
				for {
					if ctx.Err() != nil {
						return
					}
					err := node.SubmitTx(ctx, tx)
					if errors.Is(err, mempool.ErrMempoolFull) {
						time.Sleep(time.Millisecond)
						continue
					}
					if err != nil && !errors.Is(err, mempool.ErrDuplicateTx) {
						failures <- err
						return
					}
					break
				}
			}
		}()
	}
	clients.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	submitted := time.Since(started)
	deadline := time.Now().Add(5 * time.Minute)
	for {
		pending := 0
		for _, node := range nodes {
			runtime, err := node.Runtime()
			if err != nil {
				t.Fatal(err)
			}
			pending += runtime.Mempool.Len()
		}
		if pending == 0 || ctx.Err() != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d txs still pending after %s, status %+v", pending, time.Since(started), alice.Status(ctx))
		}
		time.Sleep(time.Millisecond)
	}
	if ctx.Err() != nil {
		t.FailNow()
	}
	elapsed := time.Since(started)
	height := alice.Status(ctx).LatestHeight
	metrics, err := alice.Metrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := alice.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	var compact consensus.CompactProposalStats
	for _, node := range nodes {
		reactor, err := node.ConsensusReactor()
		if err != nil {
			t.Fatal(err)
		}
		stats := reactor.CompactProposalStats()
		compact.Rebuilt += stats.Rebuilt
		compact.Completed += stats.Completed
		compact.Fetched += stats.Fetched
		compact.FetchedTxs += stats.FetchedTxs
		compact.ServedBytes += stats.ServedBytes
	}
	empty := 0
	for blockHeight := types.Height(1); blockHeight <= height; blockHeight++ {
		record, err := runtime.BlockByHeight(ctx, blockHeight)
		if err != nil {
			t.Fatal(err)
		}
		if len(record.Block.Txs) == 0 {
			empty++
		}
	}
	t.Logf("committed %d txs (all submitted after %s) in %d blocks (%d empty) in %s: %.0f tx/s, %.1f blocks/s, commit latency p50 %s p95 %s, round timeouts %d, leader upload %.1f bytes/tx per validator, compact proposals rebuilt %d completed %d fetched whole %d (%.1f%% of txs fetched)",
		total, submitted, height, empty, elapsed, float64(total)/elapsed.Seconds(), float64(height)/elapsed.Seconds(),
		time.Duration(metrics.CommitLatencyNanos), time.Duration(metrics.CommitLatencyP95Nanos), metrics.RoundTimeouts,
		(float64(proposalBytes.Load())+float64(compact.ServedBytes)/float64(len(nodes)-1))/float64(total), compact.Rebuilt, compact.Completed, compact.Fetched,
		100*float64(compact.FetchedTxs)/float64(2*total))
}
