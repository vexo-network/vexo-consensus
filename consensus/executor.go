package consensus

import (
	"context"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/types"
)

// ApplicationBlockExecutor executes a certified block against the application.
//
// ProcessProposal is the vote gate: the proposal handler runs it before a
// validator votes. A block that reached finality must execute on every node, so
// Execute never re-applies the vote gate; a tx that fails is a non-zero Result
// inside the FinalizeBlock response, not an error.
type ApplicationBlockExecutor struct{}

func (ApplicationBlockExecutor) Execute(ctx context.Context, application app.Application, block types.Block) (app.FinalizeBlockResponse, error) {
	select {
	case <-ctx.Done():
		return app.FinalizeBlockResponse{}, ctx.Err()
	default:
	}

	finalizeRequest := app.FinalizeBlockRequest{Block: block}
	if finalizer, ok := application.(app.ContextFinalizeBlockApplication); ok {
		return finalizer.FinalizeBlockContext(ctx, finalizeRequest)
	}
	return application.FinalizeBlock(finalizeRequest)
}
