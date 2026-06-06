package consensus

import (
	"context"
	"errors"
	"testing"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestApplicationBlockExecutorExecutesAcceptedProposal(t *testing.T) {
	application := &executorApplication{accepted: true}
	block := types.Block{Header: types.Header{Height: 1}, Txs: []types.Tx{[]byte("tx")}}

	response, err := ApplicationBlockExecutor{}.Execute(context.Background(), application, block)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected one result")
	}
	if !application.finalized {
		t.Fatal("expected app finalized")
	}
}

func TestApplicationBlockExecutorRejectsProposal(t *testing.T) {
	application := &executorApplication{accepted: false}
	_, err := ApplicationBlockExecutor{}.Execute(context.Background(), application, types.Block{})
	if !errors.Is(err, app.ErrProposalRejected) {
		t.Fatalf("expected proposal rejected, got %v", err)
	}
}

func TestApplicationBlockExecutorPropagatesFinalizeError(t *testing.T) {
	expectedErr := errors.New("finalize failed")
	application := &executorApplication{accepted: true, finalizeErr: expectedErr}
	_, err := ApplicationBlockExecutor{}.Execute(context.Background(), application, types.Block{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected finalize error, got %v", err)
	}
}

func TestApplicationBlockExecutorContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ApplicationBlockExecutor{}.Execute(ctx, &executorApplication{accepted: true}, types.Block{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestApplicationBlockExecutorUsesContextAwareApplication(t *testing.T) {
	application := &contextExecutorApplication{executorApplication: executorApplication{accepted: true}}
	_, err := ApplicationBlockExecutor{}.Execute(context.Background(), application, types.Block{Header: types.Header{Height: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !application.processContext || !application.finalizeContext {
		t.Fatalf("expected context-aware process/finalize path, got process=%t finalize=%t", application.processContext, application.finalizeContext)
	}
}

type executorApplication struct {
	accepted    bool
	finalized   bool
	finalizeErr error
}

func (application *executorApplication) InitChain(req app.InitChainRequest) (app.InitChainResponse, error) {
	return app.InitChainResponse{}, nil
}

func (application *executorApplication) CheckTx(tx types.Tx) app.CheckTxResponse {
	return app.CheckTxResponse{}
}

func (application *executorApplication) PrepareProposal(req app.PrepareProposalRequest) (app.PrepareProposalResponse, error) {
	return app.PrepareProposalResponse{}, nil
}

func (application *executorApplication) ProcessProposal(req app.ProcessProposalRequest) app.ProcessProposalResponse {
	return app.ProcessProposalResponse{Accepted: application.accepted}
}

func (application *executorApplication) FinalizeBlock(req app.FinalizeBlockRequest) (app.FinalizeBlockResponse, error) {
	application.finalized = true
	if application.finalizeErr != nil {
		return app.FinalizeBlockResponse{}, application.finalizeErr
	}
	return app.FinalizeBlockResponse{Results: []types.Result{{}}}, nil
}

func (application *executorApplication) Commit() (app.CommitResponse, error) {
	return app.CommitResponse{}, nil
}

func (application *executorApplication) Query(req app.QueryRequest) app.QueryResponse {
	return app.QueryResponse{}
}

type contextExecutorApplication struct {
	executorApplication
	processContext  bool
	finalizeContext bool
}

func (application *contextExecutorApplication) ProcessProposalContext(ctx context.Context, req app.ProcessProposalRequest) app.ProcessProposalResponse {
	application.processContext = true
	return application.executorApplication.ProcessProposal(req)
}

func (application *contextExecutorApplication) FinalizeBlockContext(ctx context.Context, req app.FinalizeBlockRequest) (app.FinalizeBlockResponse, error) {
	application.finalizeContext = true
	return application.executorApplication.FinalizeBlock(req)
}
