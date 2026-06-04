package node

import (
	"context"
	"errors"
	"testing"
	"time"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/dataavailability"
	"github.com/vexo-network/vexo-consensus/finality"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestNodeStartStopLifecycle(t *testing.T) {
	node := newTestNode(t)

	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := node.Status(context.Background())
	if !status.Running || status.ChainID != "vexo-test" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err := node.Runtime(); err != nil {
		t.Fatal(err)
	}
	if err := node.Start(context.Background()); !errors.Is(err, ErrNodeAlreadyRunning) {
		t.Fatalf("expected already running, got %v", err)
	}
	if err := node.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := node.Runtime(); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running, got %v", err)
	}
	if err := node.Stop(context.Background()); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running on second stop, got %v", err)
	}
}

func TestNodeRejectsUnsafeConfigWhenNetworkSafetyRequired(t *testing.T) {
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.RequireNetworkSafety = true
	if _, err := New(cfg, validGenesis(), newTestApplication(t)); !errors.Is(err, config.ErrUnsafeNetworkConfig) {
		t.Fatalf("expected unsafe network config error, got %v", err)
	}
}

func TestNodeExecutesBlockThroughRuntime(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}
	status := node.Status(context.Background())
	if status.LatestHeight != 1 || status.LatestAppHash == (types.Hash{}) {
		t.Fatalf("unexpected status after block: %+v", status)
	}
}

func TestNodePersistsRuntimeStore(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := node.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	restarted := newTestNodeWithDataDir(t, node.cfg.DataDir)
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop(context.Background())
	restartedRuntime, err := restarted.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	record, err := restartedRuntime.BlockByHeight(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if record.Block.Header.Height != 2 {
		t.Fatalf("expected stored block height 2, got %+v", record)
	}
	status := restarted.Status(context.Background())
	if status.LatestHeight != 2 || status.LatestAppHash == (types.Hash{}) {
		t.Fatalf("expected recovered status at height 2, got %+v", status)
	}
	machine, err := restarted.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	consensusStatus := machine.Status(context.Background())
	if consensusStatus.Height != 3 {
		t.Fatalf("expected consensus to resume at height 3, got %+v", consensusStatus)
	}
}

func TestNodeConsensusWALPreventsDoubleSignAfterRestart(t *testing.T) {
	dataDir := t.TempDir()
	node := newTestNodeWithDataDir(t, dataDir)
	node.cfg.ValidatorID = "alice"
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	firstHash := types.Hash{1}
	if err := node.recordConsensusVote(consensus.Vote{Height: 1, Round: 0, BlockHash: firstHash, ValidatorID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := node.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	restarted := newTestNodeWithDataDir(t, dataDir)
	restarted.cfg.ValidatorID = "alice"
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop(context.Background())
	err := restarted.recordConsensusVote(consensus.Vote{Height: 1, Round: 0, BlockHash: types.Hash{2}, ValidatorID: "alice"})
	if !errors.Is(err, consensus.ErrDoubleSignDetected) {
		t.Fatalf("expected wal double-sign guard, got %v", err)
	}
}

func TestNodeQueriesBlocks(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("bank:first")},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs:    []types.Tx{[]byte("bank:second")},
	}); err != nil {
		t.Fatal(err)
	}

	first, err := node.BlockByHeight(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Block.Header.Height != 1 || len(first.Block.Txs) != 1 {
		t.Fatalf("unexpected first block: %+v", first)
	}
	latest, err := node.LatestBlock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if latest.Block.Header.Height != 2 || len(latest.Block.Txs) != 1 {
		t.Fatalf("unexpected latest block: %+v", latest)
	}
	index, err := node.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 1 || index.LatestHeight != 2 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected block index: %+v", index)
	}
	state, err := node.LatestState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Height != 2 || state.AppHash == (types.Hash{}) {
		t.Fatalf("unexpected latest state: %+v", state)
	}
	root, err := node.StateRoot(context.Background(), 2, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if root.Height != 2 || root.Namespace != "bank" || root.Root == (types.Hash{}) {
		t.Fatalf("unexpected state root: %+v", root)
	}
	validatorSet, err := node.ValidatorSet(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	validators := validatorSet.List()
	if len(validators) != 1 || validators[0].ID != "alice" {
		t.Fatalf("unexpected validators: %+v", validators)
	}
	committeeResult, err := node.Committee(context.Background(), 2, 0, types.Hash{9})
	if err != nil {
		t.Fatal(err)
	}
	if len(committeeResult.Members) != 1 || committeeResult.Members[0].Validator.ID != "alice" {
		t.Fatalf("unexpected committee: %+v", committeeResult)
	}
}

func TestNodeMetricsReportsRuntimeSnapshot(t *testing.T) {
	node := newTestNode(t)
	node.cfg.Chain.P2P.WindowResetInterval = time.Hour
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.P2PScore.ObserveMessage(context.Background(), "peer-a", true); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 1},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}

	metrics, err := node.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !metrics.Running || metrics.ChainID != "vexo-test" || metrics.LatestHeight != 1 || metrics.LatestAppHash == (types.Hash{}) {
		t.Fatalf("unexpected metrics identity: %+v", metrics)
	}
	if metrics.StartedAtUnix == 0 {
		t.Fatalf("expected started_at metric, got %+v", metrics)
	}
	if metrics.EarliestBlockHeight != 1 || metrics.LatestBlockHeight != 1 || metrics.TotalBlocks != 1 {
		t.Fatalf("unexpected block metrics: %+v", metrics)
	}
	if metrics.ValidatorCount != 1 || metrics.TotalVotingPower != 1 || metrics.ValidatorSetHash == (types.Hash{}) {
		t.Fatalf("unexpected validator metrics: %+v", metrics)
	}
	if metrics.PeerCount != 1 || metrics.PeerWindowMessages != 1 {
		t.Fatalf("unexpected peer metrics: %+v", metrics)
	}
}

func TestNodeMetricsReportsStoppedSnapshot(t *testing.T) {
	node := newTestNode(t)

	metrics, err := node.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Running || metrics.ChainID != "vexo-test" || metrics.DataDir == "" {
		t.Fatalf("unexpected stopped metrics: %+v", metrics)
	}
}

func TestNodeStateSnapshotReportsLatestStateRoots(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := node.StateSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Height != 2 || snapshot.AppHash == (types.Hash{}) || snapshot.LastBlockHash == (types.Hash{}) {
		t.Fatalf("unexpected snapshot identity: %+v", snapshot)
	}
	if len(snapshot.StateRoots) != 1 || snapshot.StateRoots[0].Namespace != "bank" || snapshot.StateRoots[0].Root == (types.Hash{}) {
		t.Fatalf("unexpected snapshot state roots: %+v", snapshot.StateRoots)
	}
}

func TestNodeStateSnapshotRequiresRunningNode(t *testing.T) {
	node := newTestNode(t)

	if _, err := node.StateSnapshot(context.Background()); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running snapshot error, got %v", err)
	}
}

func TestNodeRecoveryReportSummarizesStoreAndRepair(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
		Header: types.Header{ChainID: "vexo-test", Height: 2},
		Txs:    []types.Tx{[]byte("bank:send")},
	}); err != nil {
		t.Fatal(err)
	}

	report, err := node.RecoveryReport(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || !report.Running || !report.SnapshotAvailable || !report.Repaired {
		t.Fatalf("unexpected recovery status: %+v", report)
	}
	if report.LatestStateHeight != 2 || report.LatestBlock != 2 || report.SafeHeight != 2 || report.TotalBlocks != 1 || report.RecoverResult.BlockIndexKeys != 1 {
		t.Fatalf("unexpected recovery heights: %+v", report)
	}
}

func TestSafeRecoveryHeightUsesLastConsistentState(t *testing.T) {
	if got := safeRecoveryHeight(10, 12); got != 10 {
		t.Fatalf("expected safe state height 10, got %d", got)
	}
	if got := safeRecoveryHeight(12, 10); got != 10 {
		t.Fatalf("expected safe block height 10, got %d", got)
	}
	if got := safeRecoveryHeight(0, 10); got != 0 {
		t.Fatalf("expected zero without state, got %d", got)
	}
}

func TestNodePruneBelowRemovesOldBlocks(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 4; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := node.PruneBelow(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.RetainFromHeight != 3 || result.PrunedBlocks != 2 {
		t.Fatalf("unexpected prune result: %+v", result)
	}
	if _, err := node.BlockByHeight(context.Background(), 1); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected pruned block not found, got %v", err)
	}
	index, err := node.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 3 || index.LatestHeight != 4 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected index after prune: %+v", index)
	}
}

func TestNodePruneBelowRequiresRunningNode(t *testing.T) {
	node := newTestNode(t)

	if _, err := node.PruneBelow(context.Background(), 1); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running prune error, got %v", err)
	}
	if _, err := node.PruneByRetention(context.Background(), store.RetentionPolicy{RetainRecent: 1}); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running retention prune error, got %v", err)
	}
}

func TestNodePruneByRetentionRecoverIndexesAndCompact(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 5; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := node.PruneByRetention(context.Background(), store.RetentionPolicy{RetainRecent: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.RetainFromHeight != 4 || result.PrunedBlocks != 3 {
		t.Fatalf("unexpected retention prune result: %+v", result)
	}
	recoverResult, err := node.RecoverIndexes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if recoverResult.BlockIndexKeys != 2 || recoverResult.LatestHeight != 5 {
		t.Fatalf("unexpected recover result: %+v", recoverResult)
	}
	if err := node.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNodeReplayRangeRestoresLatestState(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 3; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := node.Replay(context.Background(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.FromHeight != 1 || result.ToHeight != 1 || result.Blocks != 1 || result.LastHash == (types.Hash{}) {
		t.Fatalf("unexpected replay result: %+v", result)
	}
	status := node.Status(context.Background())
	if status.LatestHeight != 3 || status.LatestAppHash == (types.Hash{}) {
		t.Fatalf("expected replay to restore latest status, got %+v", status)
	}
}

func TestNodeReplayAllAfterPrune(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 4; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := node.PruneBelow(context.Background(), 3); err != nil {
		t.Fatal(err)
	}

	result, err := node.ReplayAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.FromHeight != 3 || result.ToHeight != 4 || result.Blocks != 2 || result.LastHash == (types.Hash{}) {
		t.Fatalf("unexpected replay all result after prune: %+v", result)
	}
}

func TestNodeReplayRequiresRunningNode(t *testing.T) {
	node := newTestNode(t)

	if _, err := node.Replay(context.Background(), 1, 1); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running replay error, got %v", err)
	}
	if _, err := node.ReplayAll(context.Background()); !errors.Is(err, ErrNodeNotRunning) {
		t.Fatalf("expected not running replay all error, got %v", err)
	}
}

func TestNodeRecoversAfterPruneAndRestart(t *testing.T) {
	node := newTestNode(t)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	for height := types.Height(1); height <= 4; height++ {
		if _, err := runtime.ExecuteBlock(context.Background(), types.Block{
			Header: types.Header{ChainID: "vexo-test", Height: height},
			Txs:    []types.Tx{[]byte("bank:send")},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := node.PruneBelow(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	dataDir := node.cfg.DataDir
	if err := node.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	restarted := newTestNodeWithDataDir(t, dataDir)
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop(context.Background())

	status := restarted.Status(context.Background())
	if status.LatestHeight != 4 || status.LatestAppHash == (types.Hash{}) {
		t.Fatalf("unexpected recovered status after prune: %+v", status)
	}
	if _, err := restarted.BlockByHeight(context.Background(), 1); !errors.Is(err, store.ErrBlockNotFound) {
		t.Fatalf("expected pruned block not found after restart, got %v", err)
	}
	index, err := restarted.BlockIndex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if index.EarliestHeight != 3 || index.LatestHeight != 4 || index.TotalBlocks != 2 {
		t.Fatalf("unexpected recovered index after prune: %+v", index)
	}
	machine, err := restarted.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	if consensusStatus := machine.Status(context.Background()); consensusStatus.Height != 5 {
		t.Fatalf("expected consensus to resume at height 5, got %+v", consensusStatus)
	}
}

func TestNodeValidation(t *testing.T) {
	application := newTestApplication(t)
	_, err := New(DefaultConfig("vexo-test", ""), validGenesis(), application)
	if !errors.Is(err, ErrMissingDataDir) {
		t.Fatalf("expected missing data dir, got %v", err)
	}
	_, err = New(DefaultConfig("vexo-test", t.TempDir()), Genesis{ChainID: "other"}, application)
	if !errors.Is(err, ErrGenesisChainID) {
		t.Fatalf("expected genesis chain id mismatch, got %v", err)
	}
	_, err = New(DefaultConfig("vexo-test", t.TempDir()), Genesis{ChainID: "vexo-test"}, application)
	if !errors.Is(err, ErrMissingValidators) {
		t.Fatalf("expected missing validators, got %v", err)
	}
	_, err = New(DefaultConfig("vexo-test", t.TempDir()), validGenesis(), nil)
	if !errors.Is(err, ErrMissingApplication) {
		t.Fatalf("expected missing application, got %v", err)
	}
}

func TestNodeValidatorModeRequiresSigner(t *testing.T) {
	cfg := DefaultConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = "alice"
	node, err := New(cfg, validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := node.Start(context.Background()); !errors.Is(err, ErrMissingSigner) {
		t.Fatalf("expected missing signer rejection, got %v", err)
	}
}

func TestNodeSignsConsensusMessagesWithDomains(t *testing.T) {
	signer, err := vexocrypto.NewDeterministicSigner([]byte("alice-key"))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	node := newTestNode(t).WithSigner(signer)

	proposal := consensus.Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}},
		Round:    0,
		Proposer: "alice",
	}
	if err := node.signConsensusProposal(&proposal); err != nil {
		t.Fatalf("sign proposal: %v", err)
	}
	proposalVerifier, err := vexocrypto.NewDomainSigner(signer, vexocrypto.DomainConsensusProposal)
	if err != nil {
		t.Fatalf("new proposal verifier: %v", err)
	}
	if !proposalVerifier.Verify(signer.PublicKey(), consensus.ProposalSignBytes(proposal), proposal.Signature) {
		t.Fatal("expected proposal signature to verify")
	}

	vote := consensus.Vote{Height: 1, Round: 0, BlockHash: types.Hash{1}, ValidatorID: "alice"}
	if err := node.signConsensusVote(&vote); err != nil {
		t.Fatalf("sign vote: %v", err)
	}
	voteVerifier, err := vexocrypto.NewDomainSigner(signer, vexocrypto.DomainConsensusVote)
	if err != nil {
		t.Fatalf("new vote verifier: %v", err)
	}
	if !voteVerifier.Verify(signer.PublicKey(), consensus.VoteSignBytes(vote), vote.Signature) {
		t.Fatal("expected vote signature to verify")
	}

	timeoutVote := consensus.TimeoutVote{Height: 1, Round: 0, ValidatorID: "alice"}
	if err := node.signConsensusTimeoutVote(&timeoutVote); err != nil {
		t.Fatalf("sign timeout vote: %v", err)
	}
	timeoutVerifier, err := vexocrypto.NewDomainSigner(signer, vexocrypto.DomainConsensusTimeoutVote)
	if err != nil {
		t.Fatalf("new timeout verifier: %v", err)
	}
	if !timeoutVerifier.Verify(signer.PublicKey(), consensus.TimeoutVoteSignBytes(timeoutVote), timeoutVote.Signature) {
		t.Fatal("expected timeout vote signature to verify")
	}
}

func TestNodeRejectsCommitGossipWithInvalidAggregateSignature(t *testing.T) {
	signer, err := vexocrypto.NewDeterministicSigner([]byte("alice-key"))
	if err != nil {
		t.Fatal(err)
	}
	node := newTestNodeWithSigner(t, signer)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	block := testCommitBlock(t, node, 1)
	blockHash := consensus.HashBlock(block)
	qc := finality.QuorumCert{
		Height:      block.Header.Height,
		Round:       0,
		BlockHash:   blockHash,
		Signers:     finality.EncodeSigners([]types.ValidatorID{"alice"}),
		Signature:   types.AggregateSignature("bad-signature"),
		VotingPower: 1,
	}
	err = node.verifyCommitCertificate(context.Background(), block, qc)
	if !errors.Is(err, finality.ErrMissingQCSignature) {
		t.Fatalf("expected invalid aggregate signature rejection, got %v", err)
	}
}

func TestNodeAcceptsCommitGossipWithConsensusVoteQC(t *testing.T) {
	signer, err := vexocrypto.NewDeterministicSigner([]byte("alice-key"))
	if err != nil {
		t.Fatal(err)
	}
	node := newTestNodeWithSigner(t, signer)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	block := testCommitBlock(t, node, 1)
	blockHash := consensus.HashBlock(block)
	vote := consensus.Vote{Height: block.Header.Height, Round: 0, BlockHash: blockHash, ValidatorID: "alice"}
	domainSigner, err := vexocrypto.NewDomainSigner(signer, vexocrypto.DomainConsensusVote)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := domainSigner.Sign(consensus.VoteSignBytes(vote))
	if err != nil {
		t.Fatal(err)
	}
	aggregate, err := (vexocrypto.DeterministicAggregateSigner{}).Aggregate([]types.Signature{signature})
	if err != nil {
		t.Fatal(err)
	}
	qc := finality.QuorumCert{
		Height:      block.Header.Height,
		Round:       0,
		BlockHash:   blockHash,
		Signers:     finality.EncodeSigners([]types.ValidatorID{"alice"}),
		Signature:   aggregate,
		VotingPower: 1,
	}
	if err := node.verifyCommitCertificate(context.Background(), block, qc); err != nil {
		t.Fatalf("expected consensus vote QC to verify, got %v", err)
	}
}

func TestNodeUsesPolicySignerForConsensusMessages(t *testing.T) {
	baseSigner, err := vexocrypto.NewDeterministicSigner([]byte("alice-key"))
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	policySigner := &recordingPolicySigner{Signer: baseSigner}
	node := newTestNode(t).WithSigner(policySigner)

	proposal := consensus.Proposal{
		Block:    types.Block{Header: types.Header{ChainID: "vexo-test", Height: 3}},
		Round:    2,
		Proposer: "alice",
	}
	if err := node.signConsensusProposal(&proposal); err != nil {
		t.Fatalf("sign proposal: %v", err)
	}
	vote := consensus.Vote{Height: 3, Round: 2, BlockHash: types.Hash{1}, ValidatorID: "alice"}
	if err := node.signConsensusVote(&vote); err != nil {
		t.Fatalf("sign vote: %v", err)
	}
	timeoutVote := consensus.TimeoutVote{Height: 3, Round: 2, ValidatorID: "alice"}
	if err := node.signConsensusTimeoutVote(&timeoutVote); err != nil {
		t.Fatalf("sign timeout vote: %v", err)
	}

	if len(policySigner.policies) != 3 {
		t.Fatalf("expected 3 policy signatures, got %+v", policySigner.policies)
	}
	expectedTypes := []vexocrypto.SignType{
		vexocrypto.SignTypeConsensusProposal,
		vexocrypto.SignTypeConsensusVote,
		vexocrypto.SignTypeConsensusTimeoutVote,
	}
	for index, expectedType := range expectedTypes {
		policy := policySigner.policies[index]
		if policy.ChainID != "vexo-test" || policy.Height != 3 || policy.Round != 2 || policy.Type != expectedType {
			t.Fatalf("unexpected policy[%d]: %+v", index, policy)
		}
	}
}

func TestNodeReconcilesPendingEvidenceAfterRestart(t *testing.T) {
	ctx := context.Background()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	signer := deterministicSignerForID("alice")
	validators := []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	}
	runtime, err := vexoruntime.NewWithStore(config.Default("vexo-test"), newTestApplication(t), validators, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := consensus.NewConflictingVoteEvidence(
		signedNodeTestVote(t, signer, "alice", 1, 0, types.Hash{1}),
		signedNodeTestVote(t, signer, "alice", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveState(ctx, store.StateRecord{Height: 10}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveEvidence(ctx, store.EvidenceRecord{Evidence: evidence, Applied: false, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	if err := (&Node{}).reconcileEvidence(ctx, runtime); err != nil {
		t.Fatal(err)
	}
	key := store.EvidenceKey(evidence)
	record, err := storage.EvidenceByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !record.Applied {
		t.Fatal("expected pending evidence to be marked applied")
	}
	set1, err := runtime.Validators.ValidatorSet(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	aliceAtEvidenceHeight, found := set1.Get("alice")
	if !found || aliceAtEvidenceHeight.VotingPower != 100 {
		t.Fatalf("evidence height set must stay intact, got %+v found=%t", aliceAtEvidenceHeight, found)
	}
	set10, err := runtime.Validators.ValidatorSet(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	aliceAtApplyHeight, found := set10.Get("alice")
	if !found || aliceAtApplyHeight.VotingPower != 95 {
		t.Fatalf("expected slash at apply height, got %+v found=%t", aliceAtApplyHeight, found)
	}
}

func TestNodeReconcilesLegacyAppliedEvidenceAfterRestart(t *testing.T) {
	ctx := context.Background()
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	signer := deterministicSignerForID("alice")
	validators := []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 100, Stake: 100, PublicKey: signer.PublicKey()},
	}
	runtime, err := vexoruntime.NewWithStore(config.Default("vexo-test"), newTestApplication(t), validators, nil, storage)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := consensus.NewConflictingVoteEvidence(
		signedNodeTestVote(t, signer, "alice", 1, 0, types.Hash{1}),
		signedNodeTestVote(t, signer, "alice", 1, 0, types.Hash{2}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveState(ctx, store.StateRecord{Height: 10}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveEvidence(ctx, store.EvidenceRecord{Evidence: evidence, Applied: true, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}

	if err := (&Node{}).reconcileEvidence(ctx, runtime); err != nil {
		t.Fatal(err)
	}
	receipt, found, err := runtime.Slashing.(*slashing.StoreKeeper).PenaltyReceipt(ctx, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !found || receipt.RemainingPower != 95 {
		t.Fatalf("expected legacy applied evidence repair receipt, found=%t receipt=%+v", found, receipt)
	}
	setAtApplyHeight, err := runtime.Validators.ValidatorSet(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	alice, found := setAtApplyHeight.Get("alice")
	if !found || alice.VotingPower != 95 {
		t.Fatalf("expected repaired validator power, got %+v found=%t", alice, found)
	}
}

func TestNodeAppliesInvalidProposalEvidence(t *testing.T) {
	signer := deterministicSignerForID("alice")
	node := newTestNodeWithSigner(t, signer)
	if err := node.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer node.Stop(context.Background())

	proposal := signedNodeTestProposal(t, signer, consensus.Proposal{
		Block: types.Block{
			Header: types.Header{
				ChainID:       "vexo-test",
				Height:        1,
				ConsensusHash: dataavailability.Commitment([]types.Tx{[]byte("other")}),
			},
			Txs: []types.Tx{[]byte("tx")},
		},
		Round:    0,
		Proposer: "alice",
	})
	evidence, err := consensus.NewInvalidProposalEvidence(proposal, string(consensus.InvalidProposalReasonDAMismatch))
	if err != nil {
		t.Fatal(err)
	}

	result, applied, err := node.SubmitEvidence(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !applied || result.RemainingPower != 1 {
		t.Fatalf("expected invalid proposal evidence to apply with minimum remaining power, applied=%t result=%+v", applied, result)
	}
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	receipt, found, err := runtime.Slashing.(*slashing.StoreKeeper).PenaltyReceipt(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !found || receipt.Evidence.Type != slashing.EvidenceInvalidProposal {
		t.Fatalf("expected invalid proposal receipt, found=%t receipt=%+v", found, receipt)
	}
}

func signedNodeTestVote(t *testing.T, signer vexocrypto.Signer, validatorID types.ValidatorID, height types.Height, round types.Round, blockHash types.Hash) consensus.Vote {
	t.Helper()
	vote := consensus.Vote{Height: height, Round: round, BlockHash: blockHash, ValidatorID: validatorID}
	domainSigner, err := vexocrypto.NewDomainSigner(signer, vexocrypto.DomainConsensusVote)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := domainSigner.Sign(consensus.VoteSignBytes(vote))
	if err != nil {
		t.Fatal(err)
	}
	vote.Signature = signature
	return vote
}

func signedNodeTestProposal(t *testing.T, signer vexocrypto.Signer, proposal consensus.Proposal) consensus.Proposal {
	t.Helper()
	signature, err := vexocrypto.SignWithDomain(signer, vexocrypto.DomainConsensusProposal, consensus.ProposalSignBytes(proposal))
	if err != nil {
		t.Fatal(err)
	}
	proposal.Signature = signature
	return proposal
}

func newTestNodeWithSigner(t *testing.T, signer vexocrypto.Signer) *Node {
	t.Helper()
	genesis := Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: signer.PublicKey()},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
	node, err := New(DefaultConfig("vexo-test", t.TempDir()), genesis, newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	node.cfg.ValidatorID = "alice"
	return node.WithSigner(signer)
}

func testCommitBlock(t *testing.T, node *Node, height types.Height) types.Block {
	t.Helper()
	runtime, err := node.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	validatorSet, err := runtime.Validators.ValidatorSet(context.Background(), height)
	if err != nil {
		t.Fatal(err)
	}
	return types.Block{Header: types.Header{
		ChainID:          "vexo-test",
		Height:           height,
		ValidatorSetHash: validatorSet.Hash(),
	}}
}

type recordingPolicySigner struct {
	vexocrypto.Signer
	policies []vexocrypto.SignPolicy
}

func (signer *recordingPolicySigner) SignWithPolicy(policy vexocrypto.SignPolicy, message []byte) (types.Signature, error) {
	signer.policies = append(signer.policies, policy)
	return signer.Signer.Sign(message)
}

func newTestNode(t *testing.T) *Node {
	t.Helper()
	return newTestNodeWithDataDir(t, t.TempDir())
}

func newTestNodeWithDataDir(t *testing.T, dataDir string) *Node {
	t.Helper()
	node, err := New(DefaultConfig("vexo-test", dataDir), validGenesis(), newTestApplication(t))
	if err != nil {
		t.Fatal(err)
	}
	return node.WithSigner(deterministicSignerForID("alice"))
}

func validGenesis() Genesis {
	return Genesis{
		ChainID: "vexo-test",
		Validators: []validator.Validator{
			{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: deterministicPublicKeyForID("alice")},
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
}

func deterministicSignerForID(validatorID types.ValidatorID) vexocrypto.DeterministicSigner {
	signer, err := vexocrypto.NewDeterministicSigner([]byte(string(validatorID) + "-key"))
	if err != nil {
		panic(err)
	}
	return signer
}

func deterministicPublicKeyForID(validatorID types.ValidatorID) types.PublicKey {
	return deterministicSignerForID(validatorID).PublicKey()
}

func newTestApplication(t *testing.T) vexoapp.Application {
	t.Helper()
	application, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{testModule{name: "bank"}}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	return application
}

type testModule struct {
	name string
}

func (module testModule) Name() string {
	return module.name
}

func (module testModule) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module testModule) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module testModule) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	return types.Result{}
}

func (module testModule) EndBlock(ctx vexoapp.Context) error {
	return nil
}
