package finality

import (
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var ErrValidatorNotInFinalityConflict = errors.New("validator is not accountable for finality conflict")

type ConflictProof struct {
	First  Proof `json:"first"`
	Second Proof `json:"second"`
}

func NewConflictEvidence(validatorSet validator.Set, first Proof, second Proof, validatorID types.ValidatorID) (slashing.Evidence, error) {
	if validatorID == "" {
		return slashing.Evidence{}, slashing.ErrMissingValidator
	}
	violation, err := DetectAccountableSafetyViolation(validatorSet, first, second)
	if err != nil {
		return slashing.Evidence{}, err
	}
	if !containsSigner(violation.DoubleSigners, validatorID) {
		return slashing.Evidence{}, ErrValidatorNotInFinalityConflict
	}
	proof, err := json.Marshal(ConflictProof{First: first, Second: second})
	if err != nil {
		return slashing.Evidence{}, err
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceFinalityConflict,
		Validator: validatorID,
		Height:    first.Header.Height,
		Round:     first.QuorumCert.Round,
		Proof:     proof,
	}, nil
}

func DecodeConflictProof(proof []byte) (ConflictProof, error) {
	var decoded ConflictProof
	if err := json.Unmarshal(proof, &decoded); err != nil {
		return ConflictProof{}, err
	}
	return decoded, nil
}

func VerifyConflictEvidence(validatorSet validator.Set, signatures SignatureVerifier, evidence slashing.Evidence) (*AccountableSafetyViolation, error) {
	if evidence.Type != slashing.EvidenceFinalityConflict {
		return nil, slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeConflictProof(evidence.Proof)
	if err != nil {
		return nil, err
	}
	if decoded.First.Header.Height != evidence.Height ||
		decoded.Second.Header.Height != evidence.Height ||
		decoded.First.QuorumCert.Round != evidence.Round {
		return nil, ErrHeightMismatch
	}
	verifier := NewVerifier(validatorSet, signatures)
	if err := verifier.VerifyFinalityProof(decoded.First); err != nil {
		return nil, err
	}
	if err := verifier.VerifyFinalityProof(decoded.Second); err != nil {
		return nil, err
	}
	violation, err := DetectAccountableSafetyViolation(validatorSet, decoded.First, decoded.Second)
	if err != nil {
		return nil, err
	}
	if !containsSigner(violation.DoubleSigners, evidence.Validator) {
		return nil, ErrValidatorNotInFinalityConflict
	}
	return violation, nil
}

func containsSigner(signers []types.ValidatorID, target types.ValidatorID) bool {
	for _, signer := range signers {
		if signer == target {
			return true
		}
	}
	return false
}
