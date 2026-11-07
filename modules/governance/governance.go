package governance

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexogov "github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "governance"

const (
	submitGasCost  uint64 = 60
	voteGasCost    uint64 = 25
	executeGasCost uint64 = 80
)

// Result codes of a governance transaction or query.
const (
	codeStore        uint32 = 1
	codeInvalidTx    uint32 = 2
	codeRefused      uint32 = 3
	codeExecution    uint32 = 4
	codeOutOfGas     uint32 = 5
	codeUnauthorized uint32 = 6
)

var (
	ErrInvalidGovernanceTx     = errors.New("invalid governance transaction")
	ErrGovernanceRequired      = errors.New("missing governance state store")
	ErrNoGovernanceVotingPower = errors.New("voter has no governance voting power for this proposal")
	ErrInvalidProposalDeposit  = errors.New("invalid governance proposal deposit")
	ErrUnauthorizedGovernance  = errors.New("unauthorized governance action")
)

const (
	governanceEscrowAddress          types.Address = "module:governance:deposit_escrow"
	governanceRejectedDepositAddress types.Address = "module:governance:rejected_deposits"
)

// Module is the governance module. It holds only the tally policy: every transaction and query
// opens a keeper over the store of its own execution context, so a transaction's governance writes
// are staged, committed and discarded together with the rest of the transaction.
type Module struct {
	policy vexogov.TallyPolicy
}

func NewModule() *Module {
	return NewModuleWithPolicy(vexogov.TallyPolicy{})
}

func NewModuleWithPolicy(policy vexogov.TallyPolicy) *Module {
	return &Module{policy: normalizeGovernancePolicy(policy)}
}

func (module *Module) CloneModule() vexoapp.Module {
	return &Module{policy: module.policy}
}

func (module *Module) Name() string {
	return ModuleName
}

func (module *Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

func (module *Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module *Module) EndBlock(ctx vexoapp.Context) error {
	return nil
}

func failure(code uint32, err error) types.Result {
	return types.Result{Code: code, Log: err.Error()}
}

// authorize requires the transaction signer to be actor: every governance action taken for an account
// named in the transaction arguments must pass it before touching state.
func authorize(tx types.Tx, actor types.Address) error {
	if err := vexoapp.AuthorizeActor(tx, actor); err != nil {
		return fmt.Errorf("%w: %w", ErrUnauthorizedGovernance, err)
	}
	return nil
}

// canonicalActor is the identity an account is recorded under, so that every spelling of the same
// address is one voter, one submitter and one depositor.
func canonicalActor(actor types.Address) types.Address {
	return types.Address(vexoapp.NormalizeActor(actor))
}

func (module *Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	parts := governanceTxParts(tx)
	if len(parts) < 2 || parts[0] != ModuleName {
		return failure(codeInvalidTx, ErrInvalidGovernanceTx)
	}
	if ctx.Store == nil {
		return failure(codeStore, ErrGovernanceRequired)
	}
	switch {
	case (len(parts) == 7 || len(parts) == 8) && parts[1] == "submit":
		return module.deliverSubmit(ctx, tx, parts)
	case len(parts) == 3 && parts[1] == "submit-json":
		return module.deliverSubmitJSON(ctx, tx, parts[2])
	case len(parts) == 5 && parts[1] == "vote":
		return module.deliverVote(ctx, tx, parts)
	case len(parts) == 3 && parts[1] == "execute":
		return module.deliverExecute(ctx, parts[2])
	default:
		return failure(codeInvalidTx, ErrInvalidGovernanceTx)
	}
}

func (module *Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	parts := governanceTxParts(tx)
	switch {
	case (len(parts) == 7 || len(parts) == 8) && parts[1] == "submit":
		return submitGasCost, nil
	case len(parts) == 3 && parts[1] == "submit-json":
		return submitGasCost, nil
	case len(parts) == 5 && parts[1] == "vote":
		return voteGasCost, nil
	case len(parts) == 3 && parts[1] == "execute":
		return executeGasCost, nil
	default:
		return 0, ErrInvalidGovernanceTx
	}
}

func (module *Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: codeStore, Log: ErrGovernanceRequired.Error()}
	}
	keeper, err := module.keeper(ctx)
	if err != nil {
		return vexoapp.QueryResponse{Code: codeExecution, Log: err.Error()}
	}
	if len(req.Path) == 2 && req.Path[0] == "proposal" {
		proposalID, err := parseProposalID(req.Path[1])
		if err != nil {
			return vexoapp.QueryResponse{Code: codeInvalidTx, Log: err.Error()}
		}
		state, found, err := keeper.ProposalContext(ctx.GoContext(), proposalID)
		if err != nil {
			return vexoapp.QueryResponse{Code: codeExecution, Log: err.Error()}
		}
		if !found {
			return vexoapp.QueryResponse{Code: codeRefused, Log: vexogov.ErrProposalNotFound.Error()}
		}
		return jsonQueryResponse(proposalView{State: state})
	}
	if len(req.Path) == 2 && req.Path[0] == "tally" {
		proposalID, err := parseProposalID(req.Path[1])
		if err != nil {
			return vexoapp.QueryResponse{Code: codeInvalidTx, Log: err.Error()}
		}
		tally, found, err := keeper.TallyContext(ctx.GoContext(), proposalID)
		if err != nil {
			return vexoapp.QueryResponse{Code: codeExecution, Log: err.Error()}
		}
		if !found {
			return vexoapp.QueryResponse{Code: codeRefused, Log: vexogov.ErrProposalNotFound.Error()}
		}
		return jsonQueryResponse(tally)
	}
	if len(req.Path) == 1 && req.Path[0] == "applied" {
		changes, err := keeper.AppliedChangesContext(ctx.GoContext())
		if err != nil {
			return vexoapp.QueryResponse{Code: codeExecution, Log: err.Error()}
		}
		return jsonQueryResponse(changes)
	}
	return vexoapp.QueryResponse{Code: codeInvalidTx, Log: "invalid governance query"}
}

// keeper opens a keeper over the store of ctx, with the clock set to the height ctx executes at.
func (module *Module) keeper(ctx vexoapp.Context) (*vexogov.StoreKeeper, error) {
	keeper, err := vexogov.NewStoreKeeperContext(ctx.GoContext(), ctx.Store, module.policy, nil)
	if err != nil {
		return nil, err
	}
	keeper.SetTime(uint64(ctx.Height))
	return keeper, nil
}

type proposalView struct {
	State vexogov.ProposalState `json:"state"`
}

func jsonQueryResponse(value any) vexoapp.QueryResponse {
	encoded, err := json.Marshal(value)
	if err != nil {
		return vexoapp.QueryResponse{Code: codeExecution, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: encoded}
}

func parseProposalID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, ErrInvalidGovernanceTx
	}
	return id, nil
}

func governanceTxParts(tx types.Tx) []string {
	rawParts := strings.Split(string(tx), ":")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if isExecutionTagPart(part) {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func isExecutionTagPart(part string) bool {
	return strings.HasPrefix(part, "fee=") ||
		strings.HasPrefix(part, "gas=") ||
		strings.HasPrefix(part, "signer=") ||
		strings.HasPrefix(part, "nonce=")
}
