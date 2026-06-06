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

	processRequest := app.ProcessProposalRequest{Block: block}
	var processResponse app.ProcessProposalResponse
	if processor, ok := application.(app.ContextProcessProposalApplication); ok {
		processResponse = processor.ProcessProposalContext(ctx, processRequest)
	} else {
		processResponse = application.ProcessProposal(processRequest)
	}
	if !processResponse.Accepted {
		return app.FinalizeBlockResponse{}, app.ErrProposalRejected
	}
	finalizeRequest := app.FinalizeBlockRequest{Block: block}
	if finalizer, ok := application.(app.ContextFinalizeBlockApplication); ok {
		return finalizer.FinalizeBlockContext(ctx, finalizeRequest)
	}
	return application.FinalizeBlock(finalizeRequest)
}
