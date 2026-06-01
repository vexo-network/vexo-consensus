package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestRuntimeNewWiresModules(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, map[types.Address]types.VotingPower{"alice": 1})
	if err != nil {
		t.Fatal(err)
	}

	if runtime.Config.ChainID != "vexo-test" {
		t.Fatalf("unexpected chain id: %s", runtime.Config.ChainID)
	}
	if runtime.Validators == nil || runtime.Mempool == nil || runtime.Slashing == nil || runtime.Governance == nil || runtime.P2PScore == nil {
		t.Fatal("expected modules to be wired")
	}
}

func TestRuntimeRejectsInvalidConfig(t *testing.T) {
	_, err := New(config.Default(""), noopApp{}, nil, nil)
	if !errors.Is(err, config.ErrMissingChainID) {
		t.Fatalf("expected missing chain id, got %v", err)
	}
}

func TestRuntimeRejectsUnsupportedCryptoBackend(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Crypto.Backend = "unknown"
	_, err := New(cfg, noopApp{}, nil, nil)
	if !errors.Is(err, vexocrypto.ErrUnsupportedCryptoBackend) {
		t.Fatalf("expected unsupported crypto backend, got %v", err)
	}
}

func TestRuntimeBuildsConsensusStateMachine(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	stateMachine, err := runtime.NewConsensusStateMachine(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	stateMachine.StartRound(1, 0)
	status := stateMachine.Status(context.Background())
	if status.ChainID != "vexo-test" || status.Height != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestRuntimeBuildsFinalityVerifier(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("pub")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.NewFinalityVerifier(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeBuildsEd25519FinalityVerifier(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Crypto.Backend = config.CryptoBackendEd25519
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: []byte("pub")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.NewFinalityVerifier(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeContextCancellation(t *testing.T) {
	cfg := config.Default("vexo-test")
	runtime, err := New(cfg, noopApp{}, []validator.Validator{
		{ID: "alice", Address: "alice", VotingPower: 1, Stake: 1},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := runtime.NewConsensusStateMachine(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected consensus context canceled, got %v", err)
	}
	if _, err := runtime.NewFinalityVerifier(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected finality context canceled, got %v", err)
	}
}

type noopApp struct{}

func (noopApp) InitChain(req app.InitChainRequest) (app.InitChainResponse, error) {
	return app.InitChainResponse{}, nil
}

func (noopApp) CheckTx(tx types.Tx) app.CheckTxResponse {
	return app.CheckTxResponse{}
}

func (noopApp) PrepareProposal(req app.PrepareProposalRequest) (app.PrepareProposalResponse, error) {
	return app.PrepareProposalResponse{Txs: req.Txs}, nil
}

func (noopApp) ProcessProposal(req app.ProcessProposalRequest) app.ProcessProposalResponse {
	return app.ProcessProposalResponse{Accepted: true}
}

func (noopApp) FinalizeBlock(req app.FinalizeBlockRequest) (app.FinalizeBlockResponse, error) {
	return app.FinalizeBlockResponse{}, nil
}

func (noopApp) Commit() (app.CommitResponse, error) {
	return app.CommitResponse{}, nil
}

func (noopApp) Query(req app.QueryRequest) app.QueryResponse {
	return app.QueryResponse{}
}
