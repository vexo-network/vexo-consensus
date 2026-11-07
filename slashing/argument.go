package slashing

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// TxModule and TxAction address the transaction that carries evidence: `slashing:evidence:<argument>`,
// where the argument is what EncodeArgument returns.
const (
	TxModule = "slashing"
	TxAction = "evidence"
)

var (
	// ErrMalformedEvidence rejects an argument that is not base64 of exactly one JSON document of
	// evidence: it does not decode, names a field evidence does not have or is followed by more data.
	ErrMalformedEvidence = errors.New("evidence is malformed")
	// ErrNonCanonicalEvidence rejects an argument that decodes to evidence but is not the argument
	// EncodeArgument writes for it.
	ErrNonCanonicalEvidence = errors.New("evidence is not in canonical form")
)

// EncodeArgument returns the argument that carries evidence in an evidence transaction: unpadded
// URL-safe base64 of the JSON document of the normalized evidence. Neither ':' nor '=' occurs in it,
// which the canonical transaction grammar reserves for arguments and tags.
func EncodeArgument(evidence Evidence) (string, error) {
	document, err := json.Marshal(evidence.Normalize())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(document), nil
}

// DecodeArgument returns the evidence an argument carries and accepts nothing but the argument
// EncodeArgument writes for it. Evidence has one spelling: JSON whitespace, the order or the case of
// fields, line breaks inside base64 (which base64 decoders skip), padding and the legacy double_sign
// label would each be another transaction proving the same offence, and each would cost every node a
// verification and a place in a block. The proof inside the evidence is held to the same standard by
// DecodeProof.
//
// The argument is not bounded here; callers that read it from the network bound it first.
func DecodeArgument(argument string) (Evidence, error) {
	document, err := base64.RawURLEncoding.Strict().DecodeString(argument)
	if err != nil {
		return Evidence{}, fmt.Errorf("%w: %v", ErrMalformedEvidence, err)
	}
	evidence, err := decodeCanonical[Evidence](document, ErrMalformedEvidence, ErrNonCanonicalEvidence)
	if err != nil {
		return Evidence{}, err
	}
	canonical, err := EncodeArgument(evidence)
	if err != nil {
		return Evidence{}, fmt.Errorf("%w: %v", ErrMalformedEvidence, err)
	}
	if canonical != argument {
		return Evidence{}, ErrNonCanonicalEvidence
	}
	return evidence, nil
}
