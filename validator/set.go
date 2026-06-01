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
	var totalPower types.VotingPower
	for _, validatorInfo := range validators {
		byID[validatorInfo.ID] = validatorInfo
		totalPower += validatorInfo.VotingPower
	}
	return setSnapshot{
		validators: append([]Validator(nil), validators...),
		byID:       byID,
		totalPower: totalPower,
		hash:       hashValidators(validators),
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
	return validatorInfo, found
}

func (set setSnapshot) List() []Validator {
	return append([]Validator(nil), set.validators...)
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
