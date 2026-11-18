package node

import (
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
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
//	VEXO_BLOCK_BYTES=<bytes>     the block size limit, 1 MiB by default
//	VEXO_CRYPTO=bls              sign and verify consensus messages with the production BLS backend
//	VEXO_APP=runtime             run the production application stack on signed transfers, not the test application
//	VEXO_ACCOUNTS=<n>            with VEXO_APP=runtime, the accounts that sign, 10000 or VEXO_THROUGHPUT if fewer
//	VEXO_SUBMITTERS=<n>          concurrent submitters per validator, 64 by default, as many RPC clients would
//
// The test application accepts unsigned transactions of 89 bytes without authenticating, pricing or
// sequencing them. VEXO_APP=runtime builds every validator as cmd/vexod does, with the modules, the
// ante handler (signature verification, nonces, fees, fee market), the LevelDB store and a mempool
// write-ahead log, and submits transfers signed before the run starts. An account submits its next
// transaction once the validator it submits to committed the previous one, so VEXO_ACCOUNTS bounds
// the backlog. Genesis funds every account on every validator, which takes seconds per thousand
// accounts before the run starts.
//
// The run fails with "stalled" when the slowest validator commits nothing for a while, and says what
// that validator logged meanwhile.
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

	// The workload signs its transactions here, before the validators start the clock.
	workload := newThroughputWorkload(t, total, count)
	nodes, links := newWorkloadValidators(t, count, float64(uplink)*1e6/8, latency, byProposal, workload)
	events := make([]*throughputEvents, len(nodes))
	initialized := time.Now()
	for index, node := range nodes {
		events[index] = &throughputEvents{counts: make(map[string]int)}
		node.WithEventLogger(events[index].log)
		startNode(t, node)
		defer node.Stop(context.Background())
	}
	t.Logf("started %d validators and their genesis state in %s", count, time.Since(initialized))
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
	// stall is how long the slowest validator may go without committing before the run gives up on
	// it: the others keep committing without it, but what was submitted to it never drains.
	stall := 20*time.Second + 20*roundTimeout
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
			slowest := nodes[0].Status(ctx).LatestHeight
			for _, node := range nodes[1:] {
				slowest = min(slowest, node.Status(ctx).LatestHeight)
			}
			if slowest != last {
				last, since = slowest, time.Now()
				continue
			}
			if time.Since(since) > stall {
				for index, node := range nodes {
					t.Errorf("stalled at height %d: validator %d %+v\nevents it logged: %s", last, index, node.Status(ctx), events[index].summary())
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
	// admitted records when each transaction was accepted, for its submit-to-commit latency, and
	// committedAt the height of the block it was first seen committed in, which is zero until then.
	// progress is when a block last carried transactions.
	admitted := make([]atomic.Int64, total)
	committedAt := make([]atomic.Uint64, total)
	var progress atomic.Int64
	progress.Store(started.UnixNano())
	// Each validator has submitters that submit its transactions concurrently, as the many clients of a
	// node's RPC do: a submission returns once its admission is durable, so one sequential submitter
	// per validator would measure the latency of the write-ahead log, not the node.
	submitters := envInt(t, "VEXO_SUBMITTERS", 64)
	var clients sync.WaitGroup
	failures := make(chan error, len(nodes)*submitters)
	for client, node := range nodes {
		for submitter := range submitters {
			clients.Add(1)
			go func() {
				defer clients.Done()
				for index := client + submitter*len(nodes); index < total; index += len(nodes) * submitters {
					if rate > 0 {
						time.Sleep(time.Until(started.Add(time.Duration(index) * time.Second / time.Duration(rate))))
					}
					if before, gated := workload.predecessor(index); gated && !awaitCommit(ctx, node, &committedAt[before], &progress, stall) {
						if ctx.Err() == nil {
							failures <- fmt.Errorf("transaction %d did not commit, so transaction %d, the next of its sender, was not submitted", before, index)
						}
						return
					}
					tx := workload.tx(index)
					for {
						if ctx.Err() != nil {
							return
						}
						err := node.SubmitTx(ctx, tx)
						if errors.Is(err, mempool.ErrMempoolFull) || errors.Is(err, ErrAdmissionStateChanged) {
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
	// transaction a block committed before, and failed the commits that executed with an error
	// code. The test workload's transactions carry no nonce, so only the dissemination keeps a block
	// from committing one again.
	committed, duplicates, failed, counted := 0, 0, 0, types.Height(0)
	lastProgress := time.Now()
	latencies := make([]time.Duration, 0, total)
	for committed < total && ctx.Err() == nil {
		for height := nodes[0].Status(ctx).LatestHeight; counted < height; {
			counted++
			record, err := runtime.BlockByHeight(ctx, counted)
			if err != nil {
				t.Fatal(err)
			}
			if len(record.TxResults) != len(record.Block.Txs) {
				t.Fatalf("block %d holds %d transactions and %d results", counted, len(record.Block.Txs), len(record.TxResults))
			}
			if len(record.Block.Txs) > 0 {
				lastProgress = time.Now()
				progress.Store(lastProgress.UnixNano())
				for position, tx := range record.Block.Txs {
					index := workload.index(t, tx)
					if result := record.TxResults[position]; result.Code != 0 {
						if failed++; failed <= 3 {
							t.Errorf("transaction %d failed in block %d with code %d: %s", index, counted, result.Code, result.Log)
						}
					}
					if !committedAt[index].CompareAndSwap(0, uint64(counted)) {
						duplicates++
						continue
					}
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
	t.Logf("%s, app=%s, %d validators, uplink %d Mbit/s, latency %s, round timeout %s: committed %d of %d txs of %.0f bytes (all submitted after %s) in %d blocks (%d empty) in %s: %.0f tx/s, %.1f blocks/s, "+
		"submit-to-commit latency p50 %s p95 %s p99 %s, last commit execution %s (p95 %s), round timeouts %d, busiest validator upload %.1f bytes/tx, busiest proposal upload %.2f bytes/tx, lane proposals rebuilt %d completed %d fetched whole %d",
		mode, workload.name(), count, uplink, latency, roundTimeout, committed, total, workload.txBytes(), time.Duration(submitted.Load()), height, empty, elapsed, float64(committed)/elapsed.Seconds(), float64(height)/elapsed.Seconds(),
		percentile(0.50), percentile(0.95), percentile(0.99), time.Duration(metrics.CommitLatencyNanos), time.Duration(metrics.CommitLatencyP95Nanos), metrics.RoundTimeouts,
		maxUpload, maxProposal, proposals.Rebuilt, proposals.Completed, proposals.Fetched)
	if duplicates > 0 {
		t.Errorf("%d commits of transactions committed before", duplicates)
	}
	if failed > 0 {
		t.Errorf("%d committed transactions failed to execute", failed)
	}
	if committed < total && workload.lossless() {
		t.Errorf("%d of %d transactions never committed", total-committed, total)
	}
}

// awaitCommit waits until node has committed the block at the height committed holds, which is
// the height a transaction was seen committed at: from then on node checks a transaction of the
// same sender against the state that transaction produced. It returns false when ctx is done or
// no block carried a transaction for stall, which means the transaction is lost.
func awaitCommit(ctx context.Context, node *Node, committed *atomic.Uint64, progress *atomic.Int64, stall time.Duration) bool {
	for {
		if height := committed.Load(); height != 0 && node.hasCommitted(ctx, types.Height(height)) {
			return true
		}
		if ctx.Err() != nil || time.Since(time.Unix(0, progress.Load())) > stall {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

// maxThroughputEvents bounds the distinct events throughputEvents keeps a count of.
const maxThroughputEvents = 64

// throughputEvents counts what a validator logged apart from the blocks it proposed and committed,
// by what the event says. A validator that cannot judge what it receives logs the same failure for
// as long as it does not, so a run that stalls can say why.
type throughputEvents struct {
	mu     sync.Mutex
	counts map[string]int
}

func (events *throughputEvents) log(event string, fields map[string]any) {
	if event == "block_committed" || event == "block_proposed" {
		return
	}
	if reason, failed := fields["error"]; failed {
		event += ": " + fmt.Sprint(reason)
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	if _, counted := events.counts[event]; !counted && len(events.counts) >= maxThroughputEvents {
		event = "other events"
	}
	events.counts[event]++
}

// summary lists the five events counted most often, the most frequent first.
func (events *throughputEvents) summary() string {
	events.mu.Lock()
	defer events.mu.Unlock()
	type tally struct {
		event string
		count int
	}
	tallies := make([]tally, 0, len(events.counts))
	for event, count := range events.counts {
		tallies = append(tallies, tally{event: event, count: count})
	}
	slices.SortFunc(tallies, func(left, right tally) int {
		return cmp.Or(cmp.Compare(right.count, left.count), cmp.Compare(left.event, right.event))
	})
	if len(tallies) == 0 {
		return "none"
	}
	lines := make([]string, 0, 5)
	for _, entry := range tallies[:min(len(tallies), 5)] {
		lines = append(lines, fmt.Sprintf("%dx %s", entry.count, entry.event))
	}
	return strings.Join(lines, "; ")
}

// throughputChainID is the chain the throughput validators run.
const throughputChainID = "vexo-test"

// throughputWorkload is the transactions a throughput run submits and the application that
// executes them.
type throughputWorkload interface {
	// name says which application executes the workload.
	name() string
	// appState is the application state the genesis block starts from.
	appState() map[string][]byte
	// configure sets the configuration every validator runs the application with.
	configure(cfg *Config)
	// newApplication builds the application of a validator configured as cfg.
	newApplication(t *testing.T, cfg Config) vexoapp.Application
	// tx returns the transaction at index of the workload.
	tx(index int) types.Tx
	// index returns the position in the workload of a transaction a block committed.
	index(t *testing.T, tx types.Tx) int
	// predecessor returns the transaction that has to commit before transaction index can be
	// submitted, if there is one.
	predecessor(index int) (int, bool)
	// txBytes is the mean size of a transaction.
	txBytes() float64
	// lossless reports whether a transaction that is not committed fails the run. The test
	// application's pool may evict one under load, which the run then measures.
	lossless() bool
}

// newThroughputWorkload returns the workload VEXO_APP selects for total transactions submitted
// through clients validators, by default the test application's.
func newThroughputWorkload(t *testing.T, total int, clients int) throughputWorkload {
	t.Helper()
	switch app := os.Getenv("VEXO_APP"); app {
	case "", "test":
		return testThroughputWorkload{}
	case "runtime":
		return newRuntimeThroughputWorkload(t, total, clients, envInt(t, "VEXO_ACCOUNTS", defaultThroughputAccounts))
	default:
		t.Fatalf("VEXO_APP=%q is neither test nor runtime", app)
		return nil
	}
}

// testThroughputWorkload submits transactions the test application executes: it takes any
// transaction of the bank module, without authenticating, pricing or sequencing it.
type testThroughputWorkload struct{}

func (testThroughputWorkload) name() string {
	return "test"
}

func (testThroughputWorkload) appState() map[string][]byte {
	return nil
}

func (testThroughputWorkload) configure(cfg *Config) {
	// As the network-safe template: a relayed copy of a committed transaction is refused.
	cfg.Chain.Mempool.SeenTTL = time.Hour
}

func (testThroughputWorkload) newApplication(t *testing.T, _ Config) vexoapp.Application {
	return newTestApplication(t)
}

func (testThroughputWorkload) tx(index int) types.Tx {
	return []byte(fmt.Sprintf("bank:throughput-%08d-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", index))
}

// index reads the position out of the transaction.
func (testThroughputWorkload) index(t *testing.T, tx types.Tx) int {
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

func (testThroughputWorkload) predecessor(int) (int, bool) {
	return 0, false
}

func (workload testThroughputWorkload) txBytes() float64 {
	return float64(len(workload.tx(0)))
}

func (testThroughputWorkload) lossless() bool {
	return false
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

// newThroughputValidators returns count validators of equal power on shaped links of one bus, which
// run the test application.
func newThroughputValidators(t *testing.T, count int, rate float64, latency time.Duration, byProposal bool) ([]*Node, []*shapedTransport) {
	t.Helper()
	return newWorkloadValidators(t, count, rate, latency, byProposal, testThroughputWorkload{})
}

// newWorkloadValidators returns count validators of equal power on shaped links of one bus, which
// run the application of workload from the state it starts.
func newWorkloadValidators(t *testing.T, count int, rate float64, latency time.Duration, byProposal bool, workload throughputWorkload) ([]*Node, []*shapedTransport) {
	t.Helper()
	bus := transport.NewInMemoryBus()
	ids := make([]types.ValidatorID, count)
	genesis := Genesis{ChainID: throughputChainID, AppState: workload.appState(), Governance: map[types.Address]types.VotingPower{}}
	for index := range ids {
		ids[index] = types.ValidatorID(fmt.Sprintf("validator-%02d", index))
		genesis.Validators = append(genesis.Validators, validator.Validator{ID: ids[index], Address: types.Address(ids[index]), VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID(ids[index])})
		genesis.Governance[types.Address(ids[index])] = 1
	}
	// VEXO_CRYPTO=bls signs and verifies with the production BLS backend (blst) instead of the
	// deterministic test signatures.
	bls := os.Getenv("VEXO_CRYPTO") == "bls"
	signers := make([]vexocrypto.Signer, count)
	for index, id := range ids {
		signers[index] = deterministicSignerForID(id)
		if !bls {
			continue
		}
		adapter, err := vexocrypto.NewBLSTBLSAdapterFromSeed([]byte(fmt.Sprintf("vexo-throughput-bls-seed-%s-padding-32", id)))
		if err != nil {
			t.Fatal(err)
		}
		proof, err := adapter.ProofOfPossession()
		if err != nil {
			t.Fatal(err)
		}
		signers[index] = adapter
		genesis.Validators[index].PublicKey = adapter.PublicKey()
		genesis.Validators[index].Metadata = map[string]string{vexocrypto.BLSProofOfPossessionMetadataKey: base64.StdEncoding.EncodeToString(proof)}
	}
	nodes := make([]*Node, 0, count)
	links := make([]*shapedTransport, 0, count)
	for index, id := range ids {
		inner, err := bus.NewPeer(p2p.PeerID(id))
		if err != nil {
			t.Fatal(err)
		}
		link := newShapedTransport(inner, rate, latency, count-1)
		cfg := UnsafeTestConfig(throughputChainID, t.TempDir())
		cfg.ValidatorID = id
		cfg.DisseminateByProposal = byProposal
		if bls {
			cfg.Chain.Crypto.Backend = config.CryptoBackendBLS
		}
		workload.configure(&cfg)
		node, err := New(cfg, genesis, workload.newApplication(t, cfg))
		if err != nil {
			t.Fatal(err)
		}
		node.WithTransport(link)
		node.WithSigner(signers[index])
		nodes = append(nodes, node)
		links = append(links, link)
	}
	return nodes, links
}
