package slashing

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type EvidenceType string

const (
	EvidenceDoubleSign             EvidenceType = "double_sign"
	EvidenceConflictingVote        EvidenceType = "conflicting_vote"
	EvidenceConflictingTimeoutVote EvidenceType = "conflicting_timeout_vote"
	EvidenceInvalidProposal        EvidenceType = "invalid_proposal"
	EvidenceUnavailableData        EvidenceType = "unavailable_data"
	EvidenceFinalityConflict       EvidenceType = "finality_conflict"
)

type Evidence struct {
	Type      EvidenceType
	Validator types.ValidatorID
	Height    types.Height
	Round     types.Round
	Proof     []byte
}

type EvidenceStatus string

const (
	EvidenceStatusSubmitted EvidenceStatus = "submitted"
	EvidenceStatusApplied   EvidenceStatus = "applied"
	EvidenceStatusAppealed  EvidenceStatus = "appealed"
	EvidenceStatusExpired   EvidenceStatus = "expired"
)

type Penalty struct {
	SlashFraction string
	JailDuration  uint64
}

type LifecyclePolicy struct {
	EvidenceMaxAge types.Height
	AppealWindow   types.Height
	UnbondingDelay types.Height
}

type PenaltyReceipt struct {
	Evidence       Evidence
	Penalty        Penalty
	PreviousPower  types.VotingPower
	RemainingPower types.VotingPower
}

type Keeper interface {
	SubmitEvidence(ctx context.Context, evidence Evidence) error
	ValidateEvidence(ctx context.Context, evidence Evidence) error
	ApplyPenalty(ctx context.Context, evidence Evidence) (Penalty, error)
}
