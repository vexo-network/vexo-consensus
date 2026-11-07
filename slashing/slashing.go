// Package slashing defines accountable-fault evidence, the penalties it carries and the key that
// makes an offence punishable once.
//
// The package holds no state. A penalty only takes effect when a block that carries the evidence
// executes: the slashing module verifies the proof against the validator set of the evidence
// height and punishes the validator through the staking module, so every node reaches the same
// state.
package slashing

import (
	"errors"
	"math/bits"
	"strconv"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrUnknownEvidenceType  = errors.New("unknown evidence type")
	ErrEmptyProof           = errors.New("evidence proof is empty")
	ErrMissingValidator     = errors.New("evidence validator is required")
	ErrMissingHeight        = errors.New("evidence height is required")
	ErrPenaltyNotConfigured = errors.New("penalty is not configured")
	ErrInvalidSlashFraction = errors.New("invalid slash fraction")
)

// BasisPoints is the denominator of slash fractions: 10000 basis points slash everything.
const BasisPoints uint64 = 10000

type EvidenceType string

const (
	// EvidenceDoubleSign is the legacy label of EvidenceConflictingVote. Normalize maps it, so both
	// labels describe one offence.
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

// Normalize returns evidence in its canonical form: the legacy double_sign label becomes
// conflicting_vote. Every dedup key, penalty lookup and proof check works on normalized evidence.
func (evidence Evidence) Normalize() Evidence {
	if evidence.Type == EvidenceDoubleSign {
		evidence.Type = EvidenceConflictingVote
	}
	return evidence
}

// Clone returns evidence that shares no memory with the receiver.
func (evidence Evidence) Clone() Evidence {
	evidence.Proof = append([]byte(nil), evidence.Proof...)
	return evidence
}

// Validate checks the fields every kind of evidence carries.
func (evidence Evidence) Validate() error {
	if evidence.Validator == "" {
		return ErrMissingValidator
	}
	if evidence.Height == 0 {
		return ErrMissingHeight
	}
	if len(evidence.Proof) == 0 {
		return ErrEmptyProof
	}
	if _, err := offenceKind(evidence.Normalize().Type); err != nil {
		return err
	}
	return nil
}

// offenceKind groups the evidence types that punish the same misbehaviour, so that two proofs of
// one offence cannot be slashed twice.
func offenceKind(evidenceType EvidenceType) (string, error) {
	switch evidenceType {
	case EvidenceDoubleSign, EvidenceConflictingVote:
		return "vote", nil
	case EvidenceConflictingTimeoutVote:
		return "timeout_vote", nil
	case EvidenceInvalidProposal, EvidenceUnavailableData:
		return "proposal", nil
	case EvidenceFinalityConflict:
		return "finality", nil
	default:
		return "", ErrUnknownEvidenceType
	}
}

// OffenceKey identifies the offence evidence proves: the kind of misbehaviour, the validator, the
// height and the round. Two proofs of one offence, however they are labelled or assembled, have
// the same key and are punished once. The proof of a finality conflict names no single round, so
// that offence is keyed by validator and height alone.
//
// The key is only meaningful for evidence whose height and round the proof verification bound to
// the proof itself, which the slashing module's verification does.
func OffenceKey(evidence Evidence) (string, error) {
	evidence = evidence.Normalize()
	kind, err := offenceKind(evidence.Type)
	if err != nil {
		return "", err
	}
	if evidence.Validator == "" {
		return "", ErrMissingValidator
	}
	if evidence.Height == 0 {
		return "", ErrMissingHeight
	}
	round := evidence.Round
	if evidence.Type == EvidenceFinalityConflict {
		round = 0
	}
	validator := string(evidence.Validator)
	return kind + "/" + strconv.Itoa(len(validator)) + ":" + validator + "/" +
		strconv.FormatUint(uint64(evidence.Height), 10) + "/" +
		strconv.FormatUint(uint64(round), 10), nil
}

// Penalty is the punishment of one offence. The slash fraction is an integer number of basis
// points, so a penalty is exact and cannot be NaN, infinite or negative.
type Penalty struct {
	SlashBPS uint64
	// JailDuration is the number of blocks the validator stays out of the validator set.
	JailDuration uint64
}

// Validate rejects a slash larger than the whole stake.
func (penalty Penalty) Validate() error {
	if penalty.SlashBPS > BasisPoints {
		return ErrInvalidSlashFraction
	}
	return nil
}

type PenaltyPolicy map[EvidenceType]Penalty

func DefaultPenaltyPolicy() PenaltyPolicy {
	return PenaltyPolicy{
		EvidenceConflictingVote:        {SlashBPS: 500, JailDuration: 1209600},
		EvidenceConflictingTimeoutVote: {SlashBPS: 500, JailDuration: 1209600},
		EvidenceInvalidProposal:        {SlashBPS: 100, JailDuration: 86400},
		EvidenceUnavailableData:        {SlashBPS: 200, JailDuration: 604800},
		EvidenceFinalityConflict:       {SlashBPS: 500, JailDuration: 1209600},
	}
}

// Penalty returns the penalty configured for evidence, whichever label it carries.
func (policy PenaltyPolicy) Penalty(evidenceType EvidenceType) (Penalty, error) {
	penalty, found := policy[(Evidence{Type: evidenceType}).Normalize().Type]
	if !found {
		return Penalty{}, ErrPenaltyNotConfigured
	}
	if err := penalty.Validate(); err != nil {
		return Penalty{}, err
	}
	return penalty, nil
}

// Validate checks every penalty of the policy.
func (policy PenaltyPolicy) Validate() error {
	for evidenceType, penalty := range policy {
		if _, err := offenceKind(evidenceType); err != nil {
			return err
		}
		if (Evidence{Type: evidenceType}).Normalize().Type != evidenceType {
			// A policy keyed by the legacy label would never be consulted.
			return ErrUnknownEvidenceType
		}
		if err := penalty.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type LifecyclePolicy struct {
	// EvidenceMaxAge is how many blocks after its height evidence stays punishable.
	EvidenceMaxAge types.Height
}

func DefaultLifecyclePolicy() LifecyclePolicy {
	return LifecyclePolicy{
		EvidenceMaxAge: 1209600,
	}
}

// ApplySlash returns the voting power that remains after penalty, rounding the remainder down so
// the slash rounds up. The multiplication is 128 bits wide, so no power overflows.
func ApplySlash(power types.VotingPower, penalty Penalty) (types.VotingPower, error) {
	if err := penalty.Validate(); err != nil {
		return 0, err
	}
	high, low := bits.Mul64(uint64(power), BasisPoints-penalty.SlashBPS)
	remaining, _ := bits.Div64(high, low, BasisPoints)
	return types.VotingPower(remaining), nil
}
