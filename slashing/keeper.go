package slashing

import (
	"context"
	"errors"
	"strconv"
)

var (
	ErrUnknownEvidenceType  = errors.New("unknown evidence type")
	ErrEmptyProof           = errors.New("evidence proof is empty")
	ErrMissingValidator     = errors.New("evidence validator is required")
	ErrMissingHeight        = errors.New("evidence height is required")
	ErrDuplicateEvidence    = errors.New("duplicate evidence")
	ErrPenaltyNotConfigured = errors.New("penalty is not configured")
)

type PenaltyPolicy map[EvidenceType]Penalty

type InMemoryKeeper struct {
	policy   PenaltyPolicy
	evidence map[string]Evidence
}

func NewInMemoryKeeper(policy PenaltyPolicy) *InMemoryKeeper {
	if policy == nil {
		policy = DefaultPenaltyPolicy()
	}
	return &InMemoryKeeper{
		policy:   policy,
		evidence: make(map[string]Evidence),
	}
}

func DefaultPenaltyPolicy() PenaltyPolicy {
	return PenaltyPolicy{
		EvidenceDoubleSign:      {SlashFraction: "0.05", JailDuration: 1209600},
		EvidenceConflictingVote: {SlashFraction: "0.05", JailDuration: 1209600},
		EvidenceInvalidProposal: {SlashFraction: "0.01", JailDuration: 86400},
		EvidenceUnavailableData: {SlashFraction: "0.02", JailDuration: 604800},
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

func (keeper *InMemoryKeeper) EvidenceCount() int {
	return len(keeper.evidence)
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
