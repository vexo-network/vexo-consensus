package consensus

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrVotesDoNotConflict = errors.New("votes do not conflict")
	ErrVotePairMismatch   = errors.New("votes are not from the same validator height and round")
)

type ConflictingVoteProof struct {
	First  Vote `json:"first"`
	Second Vote `json:"second"`
}

func NewConflictingVoteEvidence(first Vote, second Vote) (slashing.Evidence, error) {
	if first.ValidatorID != second.ValidatorID || first.Height != second.Height || first.Round != second.Round {
		return slashing.Evidence{}, ErrVotePairMismatch
	}
	if first.BlockHash == second.BlockHash {
		return slashing.Evidence{}, ErrVotesDoNotConflict
	}

	proof, err := json.Marshal(ConflictingVoteProof{
		First:  first,
		Second: second,
	})
	if err != nil {
		return slashing.Evidence{}, err
	}

	return slashing.Evidence{
		Type:      slashing.EvidenceConflictingVote,
		Validator: first.ValidatorID,
		Height:    first.Height,
		Round:     first.Round,
		Proof:     proof,
	}, nil
}

func DecodeConflictingVoteProof(proof []byte) (ConflictingVoteProof, error) {
	var decoded ConflictingVoteProof
	if err := json.Unmarshal(proof, &decoded); err != nil {
		return ConflictingVoteProof{}, err
	}
	return decoded, nil
}

func VerifyConflictingVoteEvidence(evidence slashing.Evidence) error {
	if evidence.Type != slashing.EvidenceConflictingVote {
		return slashing.ErrUnknownEvidenceType
	}
	decoded, err := DecodeConflictingVoteProof(evidence.Proof)
	if err != nil {
		return err
	}
	if decoded.First.ValidatorID != evidence.Validator ||
		decoded.First.Height != evidence.Height ||
		decoded.First.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	if decoded.Second.ValidatorID != evidence.Validator ||
		decoded.Second.Height != evidence.Height ||
		decoded.Second.Round != evidence.Round {
		return ErrVotePairMismatch
	}
	if bytes.Equal(decoded.First.BlockHash[:], decoded.Second.BlockHash[:]) {
		return ErrVotesDoNotConflict
	}
	return nil
}

func VoteConflictFromPrevious(previousBlock types.Hash, vote Vote) (slashing.Evidence, error) {
	previous := Vote{
		Height:      vote.Height,
		Round:       vote.Round,
		BlockHash:   previousBlock,
		ValidatorID: vote.ValidatorID,
	}
	return NewConflictingVoteEvidence(previous, vote)
}
