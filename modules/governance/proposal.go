package governance

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexogov "github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/types"
)

func (module *Module) deliverSubmit(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(submitGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	proposal := vexogov.Proposal{
		Submitter: types.Address(parts[2]),
		Title:     parts[3],
		Changes: []vexogov.ParameterChange{
			{Module: parts[4], Key: parts[5], Value: []byte(parts[6])},
		},
	}
	if len(parts) == 8 {
		proposal.Deposit = strings.TrimSpace(parts[7])
	}
	return module.submitProposal(ctx, tx, proposal)
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

func (module *Module) deliverSubmitJSON(ctx vexoapp.Context, tx types.Tx, encodedPayload string) types.Result {
	if err := ctx.ConsumeGas(submitGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	rawPayload, err := base64.RawStdEncoding.DecodeString(encodedPayload)
	if err != nil {
		rawPayload, err = base64.StdEncoding.DecodeString(encodedPayload)
	}
	if err != nil {
		return failure(codeRefused, ErrInvalidGovernanceTx)
	}
	var payload proposalJSONPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return failure(codeRefused, ErrInvalidGovernanceTx)
	}
	if payload.Submitter == "" || payload.Title == "" || len(payload.Changes) == 0 {
		return failure(codeRefused, ErrInvalidGovernanceTx)
	}
	if payload.Type != "" && payload.Type != "parameter_change" {
		return failure(codeRefused, ErrInvalidGovernanceTx)
	}
	changes := make([]vexogov.ParameterChange, 0, len(payload.Changes))
	for _, change := range payload.Changes {
		if change.Module == "" || change.Key == "" {
			return failure(codeRefused, ErrInvalidGovernanceTx)
		}
		changes = append(changes, vexogov.ParameterChange{
			Module: change.Module,
			Key:    change.Key,
			Value:  []byte(change.Value),
		})
	}
	return module.submitProposal(ctx, tx, vexogov.Proposal{
		Submitter:   types.Address(payload.Submitter),
		Title:       payload.Title,
		Description: proposalDescription(payload),
		Deposit:     strings.TrimSpace(payload.Deposit),
		Changes:     changes,
	})
}

// submitProposal opens a proposal for the signer. The proposal snapshots the stake bonded now as the
// base of its thresholds, and its deposit is taken from the submitter only after the submitter was
// authenticated.
func (module *Module) submitProposal(ctx vexoapp.Context, tx types.Tx, proposal vexogov.Proposal) types.Result {
	if err := authorize(tx, proposal.Submitter); err != nil {
		return failure(codeUnauthorized, err)
	}
	proposal.Submitter = canonicalActor(proposal.Submitter)
	if err := proposal.Validate(); err != nil {
		return failure(codeRefused, err)
	}
	amount, err := module.validateProposalDeposit(proposal.Deposit)
	if err != nil {
		return failure(codeRefused, err)
	}
	goCtx := ctx.GoContext()
	keeper, err := module.keeper(ctx)
	if err != nil {
		return failure(codeStore, err)
	}
	bonded, err := staking.TotalBondedPower(goCtx, ctx.Store)
	if err != nil {
		return failure(codeStore, err)
	}
	keeper.SetTotalPower(bonded)
	if amount != nil {
		if err := bank.TransferBig(goCtx, ctx.Store, proposal.Submitter, module.depositEscrowAddress(), amount); err != nil {
			return failure(codeRefused, err)
		}
	}
	id, err := keeper.SubmitProposal(goCtx, proposal)
	if err != nil {
		return failure(codeRefused, err)
	}
	return types.Result{Data: []byte(strconv.FormatUint(id, 10))}
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

// deliverVote records the signer's vote. The voter's power is the stake it had bonded when the
// proposal was submitted, so stake delegated after a proposal exists cannot vote on it.
func (module *Module) deliverVote(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(voteGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	proposalID, err := parseProposalID(parts[2])
	if err != nil {
		return failure(codeRefused, err)
	}
	if err := authorize(tx, types.Address(parts[3])); err != nil {
		return failure(codeUnauthorized, err)
	}
	voter := canonicalActor(types.Address(parts[3]))
	goCtx := ctx.GoContext()
	keeper, err := module.keeper(ctx)
	if err != nil {
		return failure(codeStore, err)
	}
	state, found, err := keeper.ProposalContext(goCtx, proposalID)
	if err != nil {
		return failure(codeStore, err)
	}
	if !found {
		return failure(codeRefused, vexogov.ErrProposalNotFound)
	}
	power, err := staking.VotingPowerAt(goCtx, ctx.Store, voter, types.Height(state.SnapshotHeight))
	if err != nil {
		return failure(codeStore, err)
	}
	if power == 0 {
		return failure(codeRefused, ErrNoGovernanceVotingPower)
	}
	keeper.SetVotingPower(voter, power)
	if err := keeper.Vote(goCtx, proposalID, voter, vexogov.VoteOption(parts[4])); err != nil {
		return failure(codeRefused, err)
	}
	return types.Result{}
}

// deliverExecute executes a proposal once its voting period and timelock are over. Anyone can
// trigger it: it acts for the proposal, not for an account. A proposal that does not pass is rejected
// and its deposit is forfeited; that is a successful transaction so the rejection is committed.
func (module *Module) deliverExecute(ctx vexoapp.Context, encodedID string) types.Result {
	if err := ctx.ConsumeGas(executeGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	proposalID, err := parseProposalID(encodedID)
	if err != nil {
		return failure(codeRefused, err)
	}
	goCtx := ctx.GoContext()
	keeper, err := module.keeper(ctx)
	if err != nil {
		return failure(codeStore, err)
	}
	state, found, err := keeper.ProposalContext(goCtx, proposalID)
	if err != nil {
		return failure(codeStore, err)
	}
	if !found {
		return failure(codeExecution, vexogov.ErrProposalNotFound)
	}
	if state.Rejected {
		return failure(codeExecution, vexogov.ErrProposalRejected)
	}
	if err := keeper.Execute(goCtx, proposalID); err != nil {
		if errors.Is(err, vexogov.ErrProposalRejected) {
			if slashErr := module.slashDeposit(ctx, state); slashErr != nil {
				return failure(codeExecution, slashErr)
			}
			return types.Result{Data: []byte("proposal_rejected")}
		}
		return failure(codeExecution, err)
	}
	if err := module.refundDeposit(ctx, state); err != nil {
		return failure(codeExecution, err)
	}
	return types.Result{}
}
