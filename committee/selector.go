package committee

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrEmptyValidatorSet    = errors.New("validator set is empty")
	ErrInvalidCommitteeSize = errors.New("committee size must be greater than zero")
	ErrInvalidEpochLength   = errors.New("epoch length must be greater than zero")
)

type DeterministicSelector struct {
	policy RotationPolicy
}

func NewDeterministicSelector(policy RotationPolicy) (DeterministicSelector, error) {
	if policy.EpochLength == 0 {
		return DeterministicSelector{}, ErrInvalidEpochLength
	}
	if policy.CommitteeSize == 0 {
		return DeterministicSelector{}, ErrInvalidCommitteeSize
	}
	return DeterministicSelector{policy: policy}, nil
}

func (selector DeterministicSelector) EpochForHeight(height types.Height) uint64 {
	if height == 0 {
		return 0
	}
	return (uint64(height) - 1) / selector.policy.EpochLength
}

func (selector DeterministicSelector) Select(ctx context.Context, epoch uint64, round types.Round, seed types.Hash, set validator.Set) (Committee, error) {
	select {
	case <-ctx.Done():
		return Committee{}, ctx.Err()
	default:
	}

	validators := eligibleValidators(set.List(), selector.policy.MinVotingPower)
	if len(validators) == 0 {
		return Committee{}, ErrEmptyValidatorSet
	}

	candidates := make([]scoredValidator, 0, len(validators))
	for _, validatorInfo := range validators {
		score, proof := selectionScore(seed, epoch, round, validatorInfo)
		candidates = append(candidates, scoredValidator{
			validator: validatorInfo,
			score:     score,
			proof:     proof,
		})
	}

	sort.Slice(candidates, func(left, right int) bool {
		comparison := bytes.Compare(candidates[left].score[:], candidates[right].score[:])
		if comparison == 0 {
			return candidates[left].validator.ID < candidates[right].validator.ID
		}
		return comparison < 0
	})

	size := int(selector.policy.CommitteeSize)
	if size > len(candidates) {
		size = len(candidates)
	}

	members := make([]Member, 0, size)
	for _, candidate := range candidates[:size] {
		members = append(members, Member{
			Validator: candidate.validator,
			Weight:    candidate.validator.VotingPower,
			Proof:     candidate.proof,
		})
	}

	return Committee{
		Epoch:   epoch,
		Round:   round,
		Seed:    seed,
		Members: members,
	}, nil
}

type scoredValidator struct {
	validator validator.Validator
	score     types.Hash
	proof     []byte
}

func eligibleValidators(validators []validator.Validator, minPower types.VotingPower) []validator.Validator {
	eligible := make([]validator.Validator, 0, len(validators))
	for _, validatorInfo := range validators {
		if validatorInfo.VotingPower >= minPower {
			eligible = append(eligible, validatorInfo)
		}
	}
	return eligible
}

func selectionScore(seed types.Hash, epoch uint64, round types.Round, validatorInfo validator.Validator) (types.Hash, []byte) {
	hasher := sha256.New()
	hasher.Write(seed[:])
	writeUint64(hasher, epoch)
	writeUint64(hasher, uint64(round))
	hasher.Write([]byte(validatorInfo.ID))
	writeUint64(hasher, uint64(validatorInfo.VotingPower))

	sum := hasher.Sum(nil)
	var score types.Hash
	copy(score[:], sum)
	return score, append([]byte(nil), sum...)
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}
