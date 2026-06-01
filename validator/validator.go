package validator

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
)

type Validator struct {
	ID          types.ValidatorID
	Address     types.Address
	PublicKey   types.PublicKey
	VotingPower types.VotingPower
	Stake       uint64
	Metadata    map[string]string
}

type Candidate struct {
	Address   types.Address
	PublicKey types.PublicKey
	Stake     uint64
	Metadata  map[string]string
}

type AdmissionPolicy interface {
	CanJoin(ctx context.Context, candidate Candidate, currentSet Set) error
}

type Set interface {
	Hash() types.Hash
	TotalVotingPower() types.VotingPower
	Get(id types.ValidatorID) (Validator, bool)
	List() []Validator
}

type Registry interface {
	ValidatorSet(ctx context.Context, height types.Height) (Set, error)
	ApplyJoin(ctx context.Context, candidate Candidate) (Validator, error)
	ApplyLeave(ctx context.Context, id types.ValidatorID) error
	UpdateVotingPower(ctx context.Context, id types.ValidatorID, power types.VotingPower) error
}
