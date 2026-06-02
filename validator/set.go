package validator

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/types"
)

type setSnapshot struct {
	validators []Validator
	byID       map[types.ValidatorID]Validator
	totalPower types.VotingPower
	hash       types.Hash
}

func newSetSnapshot(validators []Validator) setSnapshot {
	byID := make(map[types.ValidatorID]Validator, len(validators))
	copiedValidators := make([]Validator, len(validators))
	var totalPower types.VotingPower
	for index, validatorInfo := range validators {
		validatorInfo = cloneValidator(validatorInfo)
		copiedValidators[index] = validatorInfo
		byID[validatorInfo.ID] = cloneValidator(validatorInfo)
		totalPower += validatorInfo.VotingPower
	}
	return setSnapshot{
		validators: copiedValidators,
		byID:       byID,
		totalPower: totalPower,
		hash:       hashValidators(copiedValidators),
	}
}

func (set setSnapshot) Hash() types.Hash {
	return set.hash
}

func (set setSnapshot) TotalVotingPower() types.VotingPower {
	return set.totalPower
}

func (set setSnapshot) Get(id types.ValidatorID) (Validator, bool) {
	validatorInfo, found := set.byID[id]
	if !found {
		return Validator{}, false
	}
	return cloneValidator(validatorInfo), true
}

func (set setSnapshot) List() []Validator {
	validators := make([]Validator, len(set.validators))
	for index, validatorInfo := range set.validators {
		validators[index] = cloneValidator(validatorInfo)
	}
	return validators
}

func cloneValidator(validatorInfo Validator) Validator {
	validatorInfo.PublicKey = append(types.PublicKey(nil), validatorInfo.PublicKey...)
	if validatorInfo.Metadata != nil {
		metadata := make(map[string]string, len(validatorInfo.Metadata))
		for key, value := range validatorInfo.Metadata {
			metadata[key] = value
		}
		validatorInfo.Metadata = metadata
	}
	return validatorInfo
}

func hashValidators(validators []Validator) types.Hash {
	hasher := sha256.New()
	for _, validatorInfo := range validators {
		hasher.Write([]byte(validatorInfo.ID))
		writeUint64(hasher, uint64(validatorInfo.VotingPower))
		writeUint64(hasher, validatorInfo.Stake)
		hasher.Write(validatorInfo.PublicKey)
	}

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}
