package governance

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexogov "github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "governance"

const (
	submitGasCost  uint64 = 60
	voteGasCost    uint64 = 25
	executeGasCost uint64 = 80
)

var (
	ErrInvalidGovernanceTx     = errors.New("invalid governance transaction")
	ErrGovernanceRequired      = errors.New("missing governance keeper")
	ErrNoGovernanceVotingPower = errors.New("governance voting power is not available from staking state")
)

type Module struct {
	keeper       contextKeeper
	policy       vexogov.TallyPolicy
	useStorePath bool
}

type contextKeeper interface {
	vexogov.Keeper
	SetTimeContext(ctx context.Context, now uint64) error
	SetVotingPowerContext(ctx context.Context, voter types.Address, power types.VotingPower) error
	ProposalContext(ctx context.Context, proposalID uint64) (vexogov.ProposalState, bool, error)
	AppliedChangesContext(ctx context.Context) ([]vexogov.ParameterChange, error)
	TallyContext(ctx context.Context, proposalID uint64) (vexogov.TallyResult, bool, error)
}

func NewModule() *Module {
	policy := vexogov.TallyPolicy{
		QuorumPower:       1,
		YesThresholdPower: 1,
		VotingPeriod:      1,
	}
	return &Module{keeper: vexogov.NewInMemoryKeeper(policy, nil), policy: policy, useStorePath: true}
}

func NewModuleWithKeeper(keeper vexogov.OperationalKeeper) *Module {
	return &Module{keeper: adaptGovernanceKeeper(keeper)}
}

func (module *Module) CloneModule() vexoapp.Module {
	clone := &Module{
		policy:       module.policy,
		useStorePath: module.useStorePath,
	}
	if !module.useStorePath {
		clone.keeper = module.keeper
	}
	return clone
}

func (module *Module) Name() string {
	return ModuleName
}

func (module *Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return module.BindStore(ctx)
}

func (module *Module) BindStore(ctx vexoapp.Context) error {
	if module.useStorePath && ctx.Store != nil {
		keeper, err := vexogov.NewStoreKeeper(ctx.Store, module.policy, nil)
		if err != nil {
			return err
		}
		module.keeper = keeper
	}
	return nil
}

func (module *Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	if module.keeper == nil {
		return ErrGovernanceRequired
	}
	return module.setTime(ctx.GoContext(), uint64(header.Height))
}

func (module *Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if module.keeper == nil {
		return types.Result{Code: 1, Log: ErrGovernanceRequired.Error()}
	}
	goCtx := ctx.GoContext()
	if err := module.setTime(goCtx, uint64(ctx.Height)); err != nil {
		return types.Result{Code: 1, Log: err.Error()}
	}
	parts := governanceTxParts(tx)
	if len(parts) == 0 || parts[0] != ModuleName {
		return types.Result{Code: 2, Log: ErrInvalidGovernanceTx.Error()}
	}
	switch {
	case len(parts) == 7 && parts[1] == "submit":
		if err := ctx.ConsumeGas(submitGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		id, err := module.submit(goCtx, parts)
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		return types.Result{Data: []byte(strconv.FormatUint(id, 10))}
	case len(parts) == 3 && parts[1] == "submit-json":
		if err := ctx.ConsumeGas(submitGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		id, err := module.submitJSON(goCtx, parts[2])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		return types.Result{Data: []byte(strconv.FormatUint(id, 10))}
	case (len(parts) == 5 || len(parts) == 6) && parts[1] == "vote":
		if err := ctx.ConsumeGas(voteGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		if err := module.vote(ctx, parts); err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		return types.Result{}
	case len(parts) == 3 && parts[1] == "execute":
		if err := ctx.ConsumeGas(executeGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		proposalID, err := parseProposalID(parts[2])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		if err := module.keeper.Execute(goCtx, proposalID); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		return types.Result{}
	default:
		return types.Result{Code: 2, Log: ErrInvalidGovernanceTx.Error()}
	}
}

func (module *Module) EndBlock(ctx vexoapp.Context) error {
	return nil
}

func (module *Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	parts := governanceTxParts(tx)
	switch {
	case len(parts) == 7 && parts[1] == "submit":
		return submitGasCost, nil
	case len(parts) == 3 && parts[1] == "submit-json":
		return submitGasCost, nil
	case (len(parts) == 5 || len(parts) == 6) && parts[1] == "vote":
		return voteGasCost, nil
	case len(parts) == 3 && parts[1] == "execute":
		return executeGasCost, nil
	default:
		return 0, ErrInvalidGovernanceTx
	}
}

func (module *Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if module.keeper == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrGovernanceRequired.Error()}
	}
	if len(req.Path) == 2 && req.Path[0] == "proposal" {
		proposalID, err := parseProposalID(req.Path[1])
		if err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
		state, found, err := module.proposal(ctx.GoContext(), proposalID)
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		if !found {
			return vexoapp.QueryResponse{Code: 3, Log: vexogov.ErrProposalNotFound.Error()}
		}
		return jsonQueryResponse(proposalView{State: state})
	}
	if len(req.Path) == 2 && req.Path[0] == "tally" {
		proposalID, err := parseProposalID(req.Path[1])
		if err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
		tally, found, err := module.tally(ctx.GoContext(), proposalID)
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		if !found {
			return vexoapp.QueryResponse{Code: 3, Log: vexogov.ErrProposalNotFound.Error()}
		}
		return jsonQueryResponse(tally)
	}
	if len(req.Path) == 1 && req.Path[0] == "applied" {
		changes, err := module.appliedChanges(ctx.GoContext())
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		return jsonQueryResponse(changes)
	}
	return vexoapp.QueryResponse{Code: 2, Log: "invalid governance query"}
}

func (module *Module) submit(ctx context.Context, parts []string) (uint64, error) {
	return module.keeper.SubmitProposal(ctx, vexogov.Proposal{
		Submitter: types.Address(parts[2]),
		Title:     parts[3],
		Changes: []vexogov.ParameterChange{
			{Module: parts[4], Key: parts[5], Value: []byte(parts[6])},
		},
	})
}

type proposalJSONPayload struct {
	Submitter   string                        `json:"submitter"`
	Title       string                        `json:"title"`
	Description string                        `json:"description,omitempty"`
	MetadataURI string                        `json:"metadata_uri,omitempty"`
	Type        string                        `json:"type,omitempty"`
	Deposit     string                        `json:"deposit,omitempty"`
	Changes     []proposalJSONParameterChange `json:"changes"`
}

type proposalJSONParameterChange struct {
	Module string `json:"module"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func (module *Module) submitJSON(ctx context.Context, encodedPayload string) (uint64, error) {
	rawPayload, err := base64.RawStdEncoding.DecodeString(encodedPayload)
	if err != nil {
		rawPayload, err = base64.StdEncoding.DecodeString(encodedPayload)
	}
	if err != nil {
		return 0, ErrInvalidGovernanceTx
	}
	var payload proposalJSONPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return 0, ErrInvalidGovernanceTx
	}
	if payload.Submitter == "" || payload.Title == "" || len(payload.Changes) == 0 {
		return 0, ErrInvalidGovernanceTx
	}
	if payload.Type != "" && payload.Type != "parameter_change" {
		return 0, ErrInvalidGovernanceTx
	}
	changes := make([]vexogov.ParameterChange, 0, len(payload.Changes))
	for _, change := range payload.Changes {
		if change.Module == "" || change.Key == "" {
			return 0, ErrInvalidGovernanceTx
		}
		changes = append(changes, vexogov.ParameterChange{
			Module: change.Module,
			Key:    change.Key,
			Value:  []byte(change.Value),
		})
	}
	return module.keeper.SubmitProposal(ctx, vexogov.Proposal{
		Submitter:   types.Address(payload.Submitter),
		Title:       payload.Title,
		Description: proposalDescription(payload),
		Changes:     changes,
	})
}

func proposalDescription(payload proposalJSONPayload) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(payload.Description) != "" {
		parts = append(parts, strings.TrimSpace(payload.Description))
	}
	if strings.TrimSpace(payload.MetadataURI) != "" {
		parts = append(parts, "metadata_uri="+strings.TrimSpace(payload.MetadataURI))
	}
	if strings.TrimSpace(payload.Deposit) != "" {
		parts = append(parts, "deposit="+strings.TrimSpace(payload.Deposit))
	}
	return strings.Join(parts, "\n")
}

func (module *Module) vote(ctx vexoapp.Context, parts []string) error {
	proposalID, err := parseProposalID(parts[2])
	if err != nil {
		return err
	}
	voter := types.Address(parts[3])
	var power types.VotingPower
	if ctx.Store != nil {
		if len(parts) != 5 {
			return ErrInvalidGovernanceTx
		}
		power, err = staking.DelegatedPower(ctx.GoContext(), ctx.Store, voter)
		if err != nil {
			return err
		}
		if power == 0 {
			return ErrNoGovernanceVotingPower
		}
	} else {
		if len(parts) != 6 {
			return ErrInvalidGovernanceTx
		}
		power, err = parseVotingPower(parts[5])
		if err != nil {
			return err
		}
	}
	if err := module.setVotingPower(ctx.GoContext(), voter, power); err != nil {
		return err
	}
	return module.keeper.Vote(ctx.GoContext(), proposalID, voter, vexogov.VoteOption(parts[4]))
}

func (module *Module) setTime(ctx context.Context, now uint64) error {
	return module.keeper.SetTimeContext(ctx, now)
}

func (module *Module) setVotingPower(ctx context.Context, voter types.Address, power types.VotingPower) error {
	return module.keeper.SetVotingPowerContext(ctx, voter, power)
}

func (module *Module) proposal(ctx context.Context, proposalID uint64) (vexogov.ProposalState, bool, error) {
	return module.keeper.ProposalContext(ctx, proposalID)
}

func (module *Module) tally(ctx context.Context, proposalID uint64) (vexogov.TallyResult, bool, error) {
	return module.keeper.TallyContext(ctx, proposalID)
}

func (module *Module) appliedChanges(ctx context.Context) ([]vexogov.ParameterChange, error) {
	return module.keeper.AppliedChangesContext(ctx)
}

func adaptGovernanceKeeper(keeper vexogov.OperationalKeeper) contextKeeper {
	if keeper == nil {
		return nil
	}
	if contextKeeper, ok := keeper.(contextKeeper); ok {
		return contextKeeper
	}
	return legacyContextKeeper{OperationalKeeper: keeper}
}

type legacyContextKeeper struct {
	vexogov.OperationalKeeper
}

func (keeper legacyContextKeeper) SetTimeContext(ctx context.Context, now uint64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.SetTime(now)
	return nil
}

func (keeper legacyContextKeeper) SetVotingPowerContext(ctx context.Context, voter types.Address, power types.VotingPower) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	keeper.SetVotingPower(voter, power)
	return nil
}

func (keeper legacyContextKeeper) ProposalContext(ctx context.Context, proposalID uint64) (vexogov.ProposalState, bool, error) {
	select {
	case <-ctx.Done():
		return vexogov.ProposalState{}, false, ctx.Err()
	default:
	}
	proposal, found := keeper.Proposal(proposalID)
	return proposal, found, nil
}

func (keeper legacyContextKeeper) AppliedChangesContext(ctx context.Context) ([]vexogov.ParameterChange, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return keeper.AppliedChanges(), nil
}

func (keeper legacyContextKeeper) TallyContext(ctx context.Context, proposalID uint64) (vexogov.TallyResult, bool, error) {
	select {
	case <-ctx.Done():
		return vexogov.TallyResult{}, false, ctx.Err()
	default:
	}
	tally, found := keeper.Tally(proposalID)
	return tally, found, nil
}

type proposalView struct {
	State vexogov.ProposalState `json:"state"`
}

func jsonQueryResponse(value any) vexoapp.QueryResponse {
	encoded, err := json.Marshal(value)
	if err != nil {
		return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
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

func parseVotingPower(value string) (types.VotingPower, error) {
	power, err := strconv.ParseUint(value, 10, 64)
	if err != nil || power == 0 {
		return 0, ErrInvalidGovernanceTx
	}
	return types.VotingPower(power), nil
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
