package validator

import (
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

// SelectProposer returns the single active validator scheduled for a height and round.
func SelectProposer(validators []Validator, height types.Height, round types.Round) (types.ValidatorID, bool) {
	if height == 0 {
		height = 1
	}
	activeValidators := make([]Validator, 0, len(validators))
	for _, validatorInfo := range validators {
		if validatorInfo.VotingPower > 0 {
			activeValidators = append(activeValidators, validatorInfo)
		}
	}
	if len(activeValidators) == 0 {
		return "", false
	}
	sort.Slice(activeValidators, func(left, right int) bool {
		return activeValidators[left].ID < activeValidators[right].ID
	})
	index := (uint64(height) - 1 + uint64(round)) % uint64(len(activeValidators))
	return activeValidators[index].ID, true
}
