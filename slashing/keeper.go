package slashing

import (
	"context"
	"errors"
	"strconv"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrUnknownEvidenceType  = errors.New("unknown evidence type")
	ErrEmptyProof           = errors.New("evidence proof is empty")
	ErrMissingValidator     = errors.New("evidence validator is required")
	ErrMissingHeight        = errors.New("evidence height is required")
	ErrDuplicateEvidence    = errors.New("duplicate evidence")
	ErrPenaltyNotConfigured = errors.New("penalty is not configured")
	ErrInvalidSlashFraction = errors.New("invalid slash fraction")
	ErrEvidenceExpired      = errors.New("evidence expired")
	ErrEvidenceAppealed     = errors.New("evidence is under appeal")
)

type PenaltyPolicy map[EvidenceType]Penalty

type InMemoryKeeper struct {
	policy          PenaltyPolicy
	lifecyclePolicy LifecyclePolicy
	evidence        map[string]Evidence
	evidenceStatus  map[string]EvidenceStatus
	evidenceExpires map[string]types.Height
	penalties       map[string]PenaltyReceipt
	jails           map[types.ValidatorID]types.Height
	unbonding       map[types.ValidatorID]types.Height
}

func NewInMemoryKeeper(policy PenaltyPolicy) *InMemoryKeeper {
	return NewInMemoryKeeperWithLifecycle(policy, DefaultLifecyclePolicy())
}

func NewInMemoryKeeperWithLifecycle(policy PenaltyPolicy, lifecyclePolicy LifecyclePolicy) *InMemoryKeeper {
	if policy == nil {
		policy = DefaultPenaltyPolicy()
	}
	lifecyclePolicy = normalizeLifecyclePolicy(lifecyclePolicy)
	return &InMemoryKeeper{
		policy:          policy,
		lifecyclePolicy: lifecyclePolicy,
		evidence:        make(map[string]Evidence),
		evidenceStatus:  make(map[string]EvidenceStatus),
		evidenceExpires: make(map[string]types.Height),
		penalties:       make(map[string]PenaltyReceipt),
		jails:           make(map[types.ValidatorID]types.Height),
		unbonding:       make(map[types.ValidatorID]types.Height),
	}
}

func DefaultPenaltyPolicy() PenaltyPolicy {
	return PenaltyPolicy{
		EvidenceDoubleSign:             {SlashFraction: "0.05", JailDuration: 1209600},
		EvidenceConflictingVote:        {SlashFraction: "0.05", JailDuration: 1209600},
		EvidenceConflictingTimeoutVote: {SlashFraction: "0.05", JailDuration: 1209600},
		EvidenceInvalidProposal:        {SlashFraction: "0.01", JailDuration: 86400},
		EvidenceUnavailableData:        {SlashFraction: "0.02", JailDuration: 604800},
	}
}

func DefaultLifecyclePolicy() LifecyclePolicy {
	return LifecyclePolicy{
		EvidenceMaxAge: 1209600,
		AppealWindow:   100,
		UnbondingDelay: 1209600,
	}
}

func (keeper *InMemoryKeeper) SubmitEvidence(ctx context.Context, evidence Evidence) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := keeper.ValidateEvidence(ctx, evidence); err != nil {
		return err
	}
	key := evidenceKey(evidence)
	if _, found := keeper.evidence[key]; found {
		return ErrDuplicateEvidence
	}
	keeper.evidence[key] = cloneEvidence(evidence)
	keeper.evidenceStatus[key] = EvidenceStatusSubmitted
	if keeper.lifecyclePolicy.EvidenceMaxAge > 0 {
		keeper.evidenceExpires[key] = evidence.Height + keeper.lifecyclePolicy.EvidenceMaxAge
	}
	return nil
}

func (keeper *InMemoryKeeper) ValidateEvidence(ctx context.Context, evidence Evidence) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if evidence.Validator == "" {
		return ErrMissingValidator
	}
	if evidence.Height == 0 {
		return ErrMissingHeight
	}
	if len(evidence.Proof) == 0 {
		return ErrEmptyProof
	}
	if _, found := keeper.policy[evidence.Type]; !found {
		return ErrUnknownEvidenceType
	}
	return nil
}

func (keeper *InMemoryKeeper) ApplyPenalty(ctx context.Context, evidence Evidence) (Penalty, error) {
	select {
	case <-ctx.Done():
		return Penalty{}, ctx.Err()
	default:
	}

	if err := keeper.ValidateEvidence(ctx, evidence); err != nil {
		return Penalty{}, err
	}
	penalty, found := keeper.policy[evidence.Type]
	if !found {
		return Penalty{}, ErrPenaltyNotConfigured
	}
	return penalty, nil
}

func (keeper *InMemoryKeeper) SubmitAndApply(ctx context.Context, evidence Evidence) (PenaltyReceipt, error) {
	if err := keeper.SubmitEvidence(ctx, evidence); err != nil {
		return PenaltyReceipt{}, err
	}
	penalty, err := keeper.ApplyPenalty(ctx, evidence)
	if err != nil {
		return PenaltyReceipt{}, err
	}
	receipt := PenaltyReceipt{
		Evidence: cloneEvidence(evidence),
		Penalty:  penalty,
	}
	key := evidenceKey(evidence)
	keeper.penalties[key] = receipt
	keeper.evidenceStatus[key] = EvidenceStatusApplied
	return receipt, nil
}

func (keeper *InMemoryKeeper) SubmitWithExpiration(ctx context.Context, evidence Evidence, expiresAt types.Height) error {
	if expiresAt != 0 && evidence.Height >= expiresAt {
		return ErrEvidenceExpired
	}
	if err := keeper.SubmitEvidence(ctx, evidence); err != nil {
		return err
	}
	if expiresAt != 0 {
		keeper.evidenceExpires[evidenceKey(evidence)] = expiresAt
	}
	return nil
}

func (keeper *InMemoryKeeper) ExpireEvidence(currentHeight types.Height) uint64 {
	var expired uint64
	for key, expiresAt := range keeper.evidenceExpires {
		if expiresAt == 0 || currentHeight < expiresAt {
			continue
		}
		if keeper.evidenceStatus[key] == EvidenceStatusApplied {
			continue
		}
		keeper.evidenceStatus[key] = EvidenceStatusExpired
		expired++
	}
	return expired
}

func (keeper *InMemoryKeeper) AppealEvidence(evidence Evidence) bool {
	key := evidenceKey(evidence)
	if _, found := keeper.evidence[key]; !found {
		return false
	}
	if keeper.evidenceStatus[key] == EvidenceStatusApplied {
		return false
	}
	keeper.evidenceStatus[key] = EvidenceStatusAppealed
	return true
}

func (keeper *InMemoryKeeper) EvidenceLifecycle(evidence Evidence) (EvidenceStatus, bool) {
	status, found := keeper.evidenceStatus[evidenceKey(evidence)]
	return status, found
}

func (keeper *InMemoryKeeper) ApplyPenaltyWithStake(ctx context.Context, evidence Evidence, currentPower types.VotingPower) (PenaltyReceipt, error) {
	key := evidenceKey(evidence)
	if status, found := keeper.evidenceStatus[key]; found {
		switch status {
		case EvidenceStatusExpired:
			return PenaltyReceipt{}, ErrEvidenceExpired
		case EvidenceStatusAppealed:
			return PenaltyReceipt{}, ErrEvidenceAppealed
		}
	}
	if expiresAt := keeper.evidenceExpires[key]; expiresAt != 0 && evidence.Height >= expiresAt {
		return PenaltyReceipt{}, ErrEvidenceExpired
	}
	penalty, err := keeper.ApplyPenalty(ctx, evidence)
	if err != nil {
		return PenaltyReceipt{}, err
	}
	remaining, err := ApplySlash(currentPower, penalty)
	if err != nil {
		return PenaltyReceipt{}, err
	}
	receipt := PenaltyReceipt{
		Evidence:       cloneEvidence(evidence),
		Penalty:        penalty,
		PreviousPower:  currentPower,
		RemainingPower: remaining,
	}
	keeper.penalties[key] = receipt
	keeper.evidenceStatus[key] = EvidenceStatusApplied
	if penalty.JailDuration > 0 {
		keeper.jails[evidence.Validator] = evidence.Height + types.Height(penalty.JailDuration)
	}
	if keeper.lifecyclePolicy.UnbondingDelay > 0 {
		keeper.unbonding[evidence.Validator] = evidence.Height + keeper.lifecyclePolicy.UnbondingDelay
	}
	return receipt, nil
}

func (keeper *InMemoryKeeper) JailUntil(validator types.ValidatorID) (types.Height, bool) {
	height, found := keeper.jails[validator]
	return height, found
}

func (keeper *InMemoryKeeper) IsJailed(validator types.ValidatorID, currentHeight types.Height) bool {
	height, found := keeper.JailUntil(validator)
	return found && currentHeight < height
}

func (keeper *InMemoryKeeper) UnbondingReleaseHeight(validator types.ValidatorID) (types.Height, bool) {
	height, found := keeper.unbonding[validator]
	return height, found
}

func (keeper *InMemoryKeeper) CanUnbond(validator types.ValidatorID, currentHeight types.Height) bool {
	height, found := keeper.UnbondingReleaseHeight(validator)
	return !found || currentHeight >= height
}

func (keeper *InMemoryKeeper) EvidenceCount() int {
	return len(keeper.evidence)
}

func (keeper *InMemoryKeeper) PenaltyCount() int {
	return len(keeper.penalties)
}

func (keeper *InMemoryKeeper) PenaltyReceipt(evidence Evidence) (PenaltyReceipt, bool) {
	receipt, found := keeper.penalties[evidenceKey(evidence)]
	if !found {
		return PenaltyReceipt{}, false
	}
	receipt.Evidence = cloneEvidence(receipt.Evidence)
	return receipt, true
}

func ApplySlash(power types.VotingPower, penalty Penalty) (types.VotingPower, error) {
	fraction, err := strconv.ParseFloat(penalty.SlashFraction, 64)
	if err != nil || fraction < 0 || fraction > 1 {
		return 0, ErrInvalidSlashFraction
	}
	remaining := float64(power) * (1 - fraction)
	if remaining < 1 && power > 0 && fraction < 1 {
		return 1, nil
	}
	return types.VotingPower(remaining), nil
}

func evidenceKey(evidence Evidence) string {
	return string(evidence.Type) + "|" +
		string(evidence.Validator) + "|" +
		strconv.FormatUint(uint64(evidence.Height), 10) + "|" +
		strconv.FormatUint(uint64(evidence.Round), 10)
}

func cloneEvidence(evidence Evidence) Evidence {
	evidence.Proof = append([]byte(nil), evidence.Proof...)
	return evidence
}

func normalizeLifecyclePolicy(policy LifecyclePolicy) LifecyclePolicy {
	defaults := DefaultLifecyclePolicy()
	if policy.EvidenceMaxAge == 0 {
		policy.EvidenceMaxAge = defaults.EvidenceMaxAge
	}
	if policy.AppealWindow == 0 {
		policy.AppealWindow = defaults.AppealWindow
	}
	if policy.UnbondingDelay == 0 {
		policy.UnbondingDelay = defaults.UnbondingDelay
	}
	return policy
}
