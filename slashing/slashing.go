package slashing

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type EvidenceType string

const (
	EvidenceDoubleSign      EvidenceType = "double_sign"
	EvidenceConflictingVote EvidenceType = "conflicting_vote"
	EvidenceInvalidProposal EvidenceType = "invalid_proposal"
	EvidenceUnavailableData EvidenceType = "unavailable_data"
)

type Evidence struct {
	Type      EvidenceType
	Validator types.ValidatorID
	Height    types.Height
	Round     types.Round
	Proof     []byte
}

type Penalty struct {
	SlashFraction string
	JailDuration  uint64
}

type Keeper interface {
	SubmitEvidence(ctx context.Context, evidence Evidence) error
	ValidateEvidence(ctx context.Context, evidence Evidence) error
	ApplyPenalty(ctx context.Context, evidence Evidence) (Penalty, error)
}
