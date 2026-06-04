package committee

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var ErrMissingVRF = errors.New("vrf is required")

const ValidatorMetadataVRFPublicKey = "vrf_public_key"

type VRF interface {
	Prove(publicKey types.PublicKey, seed []byte) (output []byte, proof []byte, err error)
	Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool
}

type VRFSelector struct {
	policy RotationPolicy
	vrf    VRF
}

func NewVRFSelector(policy RotationPolicy, vrf VRF) (VRFSelector, error) {
	if vrf == nil {
		return VRFSelector{}, ErrMissingVRF
	}
	if policy.EpochLength == 0 {
		return VRFSelector{}, ErrInvalidEpochLength
	}
	if policy.CommitteeSize == 0 {
		return VRFSelector{}, ErrInvalidCommitteeSize
	}
	return VRFSelector{policy: policy, vrf: vrf}, nil
}

func (selector VRFSelector) Select(ctx context.Context, epoch uint64, round types.Round, seed types.Hash, set validator.Set) (Committee, error) {
	select {
	case <-ctx.Done():
		return Committee{}, ctx.Err()
	default:
	}

	validators := eligibleValidators(set.List(), selector.policy.MinVotingPower)
	if len(validators) == 0 {
		return Committee{}, ErrEmptyValidatorSet
	}

	selectionSeed := vrfSeed(seed, epoch, round)
	candidates := make([]vrfCandidate, 0, len(validators))
	for _, validatorInfo := range validators {
		vrfPublicKey := validatorVRFPublicKey(validatorInfo)
		output, proof, err := selector.vrf.Prove(vrfPublicKey, selectionSeed)
		if err != nil {
			return Committee{}, err
		}
		if !selector.vrf.Verify(vrfPublicKey, selectionSeed, output, proof) {
			return Committee{}, ErrMissingVRF
		}
		candidates = append(candidates, vrfCandidate{
			validator: validatorInfo,
			output:    append([]byte(nil), output...),
			proof:     append([]byte(nil), proof...),
		})
	}

	sort.Slice(candidates, func(left, right int) bool {
		comparison := bytes.Compare(candidates[left].output, candidates[right].output)
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
			Output:    candidate.output,
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

func (selector VRFSelector) VerifyMember(epoch uint64, round types.Round, seed types.Hash, member Member) bool {
	selectionSeed := vrfSeed(seed, epoch, round)
	output := member.Output
	if len(output) == 0 {
		output = member.Proof
	}
	return selector.vrf.Verify(validatorVRFPublicKey(member.Validator), selectionSeed, output, member.Proof)
}

type vrfCandidate struct {
	validator validator.Validator
	output    []byte
	proof     []byte
}

func vrfSeed(seed types.Hash, epoch uint64, round types.Round) []byte {
	buffer := make([]byte, 0, len(seed)+16)
	buffer = append(buffer, seed[:]...)
	var encoded [16]byte
	binary.BigEndian.PutUint64(encoded[:8], epoch)
	binary.BigEndian.PutUint64(encoded[8:], uint64(round))
	return append(buffer, encoded[:]...)
}

func validatorVRFPublicKey(validatorInfo validator.Validator) types.PublicKey {
	if validatorInfo.Metadata != nil {
		if encoded := validatorInfo.Metadata[ValidatorMetadataVRFPublicKey]; encoded != "" {
			if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
				return types.PublicKey(decoded)
			}
			return types.PublicKey(encoded)
		}
	}
	return validatorInfo.PublicKey
}
