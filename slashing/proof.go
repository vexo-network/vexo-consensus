package slashing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrMalformedProof rejects a proof that is not exactly one JSON document of its type: it does
	// not parse, names a field the type does not have or is followed by more data.
	ErrMalformedProof = errors.New("evidence proof is malformed")
	// ErrNonCanonicalProof rejects a proof that decodes but is not spelled the way json.Marshal
	// spells what it decodes to.
	ErrNonCanonicalProof = errors.New("evidence proof is not in canonical form")
)

// DecodeProof decodes the proof of evidence, a JSON document of type T, and accepts nothing but its
// canonical spelling: the bytes json.Marshal gives the decoded value. JSON has endless spellings of
// one value (whitespace, the order of fields, the case of field names, escapes, line breaks in
// base64, trailing padding, repeated fields), and a proof is part of a transaction, so every
// spelling would be a different transaction proving the same offence. Each of them would cost
// every node a full verification and a place in a block.
//
// The decoder rejects unknown fields and trailing data, then compares the proof with its own
// re-encoding, which also rejects whatever else a decoder is lenient about. A proof is therefore
// what a constructor produced, and the same bytes on every node.
func DecodeProof[T any](proof []byte) (T, error) {
	return decodeCanonical[T](proof, ErrMalformedProof, ErrNonCanonicalProof)
}

// decodeCanonical decodes document, one JSON value of type T, and accepts nothing but the bytes
// json.Marshal gives the decoded value. It fails with malformed when document is no single value of
// T and with nonCanonical when it is one that is spelled differently. It returns the zero T on
// failure.
func decodeCanonical[T any](document []byte, malformed error, nonCanonical error) (T, error) {
	var none T
	var decoded T
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return none, fmt.Errorf("%w: %v", malformed, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return none, fmt.Errorf("%w: trailing data", malformed)
	}
	canonical, err := json.Marshal(decoded)
	if err != nil {
		return none, fmt.Errorf("%w: %v", malformed, err)
	}
	if !bytes.Equal(canonical, document) {
		return none, nonCanonical
	}
	return decoded, nil
}
