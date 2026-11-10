package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

// TestConsensusThroughput measures how fast in-process validators commit a stream of transactions
// in the finalized execution mode. Clients submit to every validator, at once or at VEXO_RATE, and
// back off while a mempool is full. It runs only with VEXO_THROUGHPUT=<txs>, so the numbers it logs are a
// measurement, not a gate:
//
//	VEXO_THROUGHPUT=200000 go test ./node -run TestConsensusThroughput -v -cpuprofile cpu.out
//
// The treatment is set by the environment:
//
//	VEXO_VALIDATORS=<n>          validators, 3 by default
//	VEXO_UPLINK_MBIT=<mbit/s>    each validator's upload, unlimited by default
//	VEXO_LATENCY_MS=<ms>         one-way link latency, 0 by default
//	VEXO_DISSEMINATION=proposals the baseline: transaction gossip and whole proposals, no lanes
//	VEXO_ROUND_TIMEOUT_MS=<ms>   the round timeout, 1s plus eight one-way latencies by default
//	VEXO_RATE=<tx/s>             submit open loop at this offered load instead of all at once
//
// Submit-to-commit latency runs from a transaction's acceptance to the first observation of its
// committed block at the first validator; at saturation it measures the backlog, so latency is
// measured at an offered load below capacity (VEXO_RATE).
func TestConsensusThroughput(t *testing.T) {
	total, err := strconv.Atoi(os.Getenv("VEXO_THROUGHPUT"))
	if err != nil || total <= 0 {
		t.Skip("set VEXO_THROUGHPUT=<txs> to measure throughput")
	}
	count := envInt(t, "VEXO_VALIDATORS", 3)
	uplink := envInt(t, "VEXO_UPLINK_MBIT", 0)
	latency := time.Duration(envInt(t, "VEXO_LATENCY_MS", 0)) * time.Millisecond
	byProposal := os.Getenv("VEXO_DISSEMINATION") == "proposals"
	rate := envInt(t, "VEXO_RATE", 0)
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	nodes, links := newThroughputValidators(t, count, float64(uplink)*1e6/8, latency, byProposal)
	for _, node := range nodes {
		startNode(t, node)
		defer node.Stop(context.Background())
	}
	for _, link := range links {
		defer link.close()
	}
	roundTimeout := time.Duration(envInt(t, "VEXO_ROUND_TIMEOUT_MS", int((time.Second+8*latency)/time.Millisecond))) * time.Millisecond
	loopConfig := ConsensusLoopConfig{
		Interval:            time.Millisecond,
		RoundTimeout:        roundTimeout,
		MaxBlockBytes:       int64(envInt(t, "VEXO_BLOCK_BYTES", 1<<20)),
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
			if height := nodes[0].Status(ctx).LatestHeight; height != last {
				last, since = height, time.Now()
				continue
			}
			if time.Since(since) > 20*time.Second+20*roundTimeout {
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
	// admitted records when each transaction was accepted, for its submit-to-commit latency.
	admitted := make([]atomic.Int64, total)
	var clients sync.WaitGroup
	failures := make(chan error, len(nodes))
	for client, node := range nodes {
		clients.Add(1)
		go func() {
			defer clients.Done()
			for index := client; index < total; index += len(nodes) {
				if rate > 0 {
					time.Sleep(time.Until(started.Add(time.Duration(index) * time.Second / time.Duration(rate))))
				}
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
					admitted[index].Store(time.Now().UnixNano())
					break
				}
			}
		}()
	}
	// Commits are counted while clients submit. The run ends once every transaction is committed.
	// A mempool may evict a transaction under load; the run then ends once every client is done, the
	// pools stay empty and nothing commits for drainGrace.
	var submitted atomic.Int64
	go func() {
		clients.Wait()
		submitted.Store(int64(time.Since(started)))
		close(failures)
	}()
	runtime, err := nodes[0].Runtime()
	if err != nil {
		t.Fatal(err)
	}
	const drainGrace = 10 * time.Second
	deadline := time.Now().Add(10 * time.Minute)
	// committed counts the distinct transactions committed, duplicates the commits of a
	// transaction a block committed before: the workload's transactions carry no nonce, so only the
	// dissemination keeps a block from committing one again.
	committed, duplicates, counted := 0, 0, types.Height(0)
	done := make([]bool, total)
	lastProgress := time.Now()
	latencies := make([]time.Duration, 0, total)
	for committed < total && ctx.Err() == nil {
		for height := nodes[0].Status(ctx).LatestHeight; counted < height; {
			counted++
			record, err := runtime.BlockByHeight(ctx, counted)
			if err != nil {
				t.Fatal(err)
			}
			if len(record.Block.Txs) > 0 {
				lastProgress = time.Now()
				for _, tx := range record.Block.Txs {
					index := throughputTxIndex(t, tx)
					if done[index] {
						duplicates++
						continue
					}
					done[index] = true
					committed++
					if at := admitted[index].Load(); at != 0 {
						latencies = append(latencies, time.Duration(lastProgress.UnixNano()-at))
					}
				}
			}
		}
		pending := 0
		for _, node := range nodes {
			nodeRuntime, err := node.Runtime()
			if err != nil {
				t.Fatal(err)
			}
			pending += nodeRuntime.Mempool.Len()
		}
		if submitted.Load() != 0 && pending == 0 && time.Since(lastProgress) > drainGrace {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d txs committed after %s, %d pending, status %+v", committed, total, time.Since(started), pending, nodes[0].Status(ctx))
		}
		time.Sleep(time.Millisecond)
	}
	if ctx.Err() != nil {
		t.FailNow()
	}
	// The run ended with every transaction submitted.
	clients.Wait()
	for err := range failures {
		t.Fatal(err)
	}
	elapsed := lastProgress.Sub(started)
	height := counted
	slices.Sort(latencies)
	percentile := func(fraction float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		return latencies[int(fraction*float64(len(latencies)-1))]
	}
	metrics, err := nodes[0].Metrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var proposals consensus.LaneProposalStats
	for _, node := range nodes {
		reactor, err := node.ConsensusReactor()
		if err != nil {
			t.Fatal(err)
		}
		stats := reactor.LaneProposalStats()
		proposals.Rebuilt += stats.Rebuilt
		proposals.Completed += stats.Completed
		proposals.Fetched += stats.Fetched
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
	// Upload per transaction: the busiest validator, and the proposal topic of the busiest leader.
	var maxUpload, maxProposal float64
	byTopic := map[p2p.Topic]float64{}
	for _, link := range links {
		upload := 0.0
		for _, topic := range []p2p.Topic{p2p.TopicTx, p2p.TopicBatch, p2p.TopicProposal, p2p.TopicVote, p2p.TopicTimeout, p2p.TopicCommit, p2p.TopicEvidence} {
			upload += float64(link.Sent(topic))
			byTopic[topic] = max(byTopic[topic], float64(link.Sent(topic))/float64(committed))
		}
		maxUpload = max(maxUpload, upload/float64(committed))
		maxProposal = max(maxProposal, float64(link.Sent(p2p.TopicProposal))/float64(committed))
	}
	t.Logf("busiest upload by topic, bytes/tx: %v", byTopic)
	mode := "lanes"
	if byProposal {
		mode = "proposals"
	}
	t.Logf("%s, %d validators, uplink %d Mbit/s, latency %s, round timeout %s: committed %d of %d txs (all submitted after %s) in %d blocks (%d empty) in %s: %.0f tx/s, %.1f blocks/s, "+
		"submit-to-commit latency p50 %s p95 %s p99 %s, last commit execution %s (p95 %s), round timeouts %d, busiest validator upload %.1f bytes/tx, busiest proposal upload %.2f bytes/tx, lane proposals rebuilt %d completed %d fetched whole %d",
		mode, count, uplink, latency, roundTimeout, committed, total, time.Duration(submitted.Load()), height, empty, elapsed, float64(committed)/elapsed.Seconds(), float64(height)/elapsed.Seconds(),
		percentile(0.50), percentile(0.95), percentile(0.99), time.Duration(metrics.CommitLatencyNanos), time.Duration(metrics.CommitLatencyP95Nanos), metrics.RoundTimeouts,
		maxUpload, maxProposal, proposals.Rebuilt, proposals.Completed, proposals.Fetched)
	if duplicates > 0 {
		t.Errorf("%d commits of transactions committed before", duplicates)
	}
}

// throughputTxIndex returns the index of a transaction of the throughput workload.
func throughputTxIndex(t *testing.T, tx types.Tx) int {
	t.Helper()
	const prefix = "bank:throughput-"
	if len(tx) < len(prefix)+8 || string(tx[:len(prefix)]) != prefix {
		t.Fatalf("not a throughput transaction: %q", tx)
	}
	index, err := strconv.Atoi(string(tx[len(prefix) : len(prefix)+8]))
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func envInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		t.Fatalf("%s=%q is not a count", name, value)
	}
	return parsed
}

// newThroughputValidators returns count validators of equal power on shaped links of one bus.
func newThroughputValidators(t *testing.T, count int, rate float64, latency time.Duration, byProposal bool) ([]*Node, []*shapedTransport) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	ids := make([]types.ValidatorID, count)
	genesis := Genesis{ChainID: "vexo-test", Governance: map[types.Address]types.VotingPower{}}
	for index := range ids {
		ids[index] = types.ValidatorID(fmt.Sprintf("validator-%02d", index))
		genesis.Validators = append(genesis.Validators, validator.Validator{ID: ids[index], Address: types.Address(ids[index]), VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID(ids[index])})
		genesis.Governance[types.Address(ids[index])] = 1
	}
	nodes := make([]*Node, 0, count)
	links := make([]*shapedTransport, 0, count)
	for _, id := range ids {
		inner, err := bus.NewPeer(p2p.PeerID(id))
		if err != nil {
			t.Fatal(err)
		}
		link := newShapedTransport(inner, rate, latency, count-1)
		cfg := UnsafeTestConfig("vexo-test", t.TempDir())
		cfg.ValidatorID = id
		cfg.DisseminateByProposal = byProposal
		// As the network-safe template: a relayed copy of a committed transaction is refused.
		cfg.Chain.Mempool.SeenTTL = time.Hour
		node, err := New(cfg, genesis, newTestApplication(t))
		if err != nil {
			t.Fatal(err)
		}
		node.WithTransport(link)
		node.WithSigner(deterministicSignerForID(id))
		nodes = append(nodes, node)
		links = append(links, link)
	}
	return nodes, links
}
