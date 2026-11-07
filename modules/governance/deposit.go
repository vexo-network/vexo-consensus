package governance

import (
	"math/big"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/economics"
	vexogov "github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	"github.com/vexo-network/vexo-consensus/types"
)

// refundDeposit returns the deposit of an executed proposal to its submitter.
func (module *Module) refundDeposit(ctx vexoapp.Context, state vexogov.ProposalState) error {
	return module.moveDeposit(ctx, state, module.depositEscrowAddress(), state.Proposal.Submitter)
}

// slashDeposit forfeits the deposit of a rejected proposal.
func (module *Module) slashDeposit(ctx vexoapp.Context, state vexogov.ProposalState) error {
	return module.moveDeposit(ctx, state, module.depositEscrowAddress(), module.rejectedDepositAddress())
}

func (module *Module) moveDeposit(ctx vexoapp.Context, state vexogov.ProposalState, from types.Address, to types.Address) error {
	if strings.TrimSpace(state.Proposal.Deposit) == "" {
		return nil
	}
	amount, err := module.validateProposalDeposit(state.Proposal.Deposit)
	if err != nil {
		return err
	}
	return bank.TransferBig(ctx.GoContext(), ctx.Store, from, to, amount)
}

func (module *Module) validateProposalDeposit(value string) (*big.Int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if module.policy.RequireDeposit {
			return nil, ErrInvalidProposalDeposit
		}
		return nil, nil
	}
	if suffix := amountDenomSuffix(value); suffix != "" && module.policy.DepositDenom != "" && suffix != module.policy.DepositDenom {
		return nil, ErrInvalidProposalDeposit
	}
	amount, err := bank.ParseAmountBig(value)
	if err != nil || amount == nil || amount.Sign() <= 0 {
		return nil, ErrInvalidProposalDeposit
	}
	if strings.TrimSpace(module.policy.MinDeposit) != "" {
		minimum, err := bank.ParseAmountBig(module.policy.MinDeposit)
		if err != nil || minimum == nil || minimum.Sign() <= 0 {
			return nil, ErrInvalidProposalDeposit
		}
		if amount.Cmp(minimum) < 0 {
			return nil, ErrInvalidProposalDeposit
		}
	}
	return amount, nil
}

// normalizeGovernancePolicy fills the thresholds and addresses a policy leaves unset. A policy
// without a quorum or a yes threshold would let a single unit of power decide a proposal, so those
// default instead of being disabled; the veto threshold stays optional.
func normalizeGovernancePolicy(policy vexogov.TallyPolicy) vexogov.TallyPolicy {
	if policy.QuorumBPS == 0 {
		policy.QuorumBPS = vexogov.DefaultQuorumBPS
	}
	if policy.YesThresholdBPS == 0 {
		policy.YesThresholdBPS = vexogov.DefaultYesThresholdBPS
	}
	if policy.VotingPeriod == 0 {
		policy.VotingPeriod = 1
	}
	if policy.DepositDenom == "" {
		policy.DepositDenom = economics.AtomicDenom
	}
	if policy.DepositEscrow == "" {
		policy.DepositEscrow = governanceEscrowAddress
	}
	if policy.RejectedDeposits == "" {
		policy.RejectedDeposits = governanceRejectedDepositAddress
	}
	return policy
}

func (module *Module) depositEscrowAddress() types.Address {
	if module.policy.DepositEscrow != "" {
		return module.policy.DepositEscrow
	}
	return governanceEscrowAddress
}

func (module *Module) rejectedDepositAddress() types.Address {
	if module.policy.RejectedDeposits != "" {
		return module.policy.RejectedDeposits
	}
	return governanceRejectedDepositAddress
}

func amountDenomSuffix(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	index := len(value)
	for index > 0 {
		char := value[index-1]
		if char < 'a' || char > 'z' {
			break
		}
		index--
	}
	return value[index:]
}
