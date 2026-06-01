package consensus

import (
	"context"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

type ApplicationBlockExecutor struct{}

func (ApplicationBlockExecutor) Execute(ctx context.Context, application app.Application, block types.Block) (app.FinalizeBlockResponse, error) {
	select {
	case <-ctx.Done():
		return app.FinalizeBlockResponse{}, ctx.Err()
	default:
	}

	processResponse := application.ProcessProposal(app.ProcessProposalRequest{Block: block})
	if !processResponse.Accepted {
		return app.FinalizeBlockResponse{}, app.ErrProposalRejected
	}
	return application.FinalizeBlock(app.FinalizeBlockRequest{Block: block})
}
