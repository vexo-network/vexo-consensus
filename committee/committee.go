package committee

import (
	"context"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

type Backend string

const (
	BackendDeterministic Backend = "deterministic"
	BackendVRF           Backend = "vrf"
)

type Member struct {
	Validator validator.Validator
	Weight    types.VotingPower
	Proof     []byte
}

type Committee struct {
	Epoch   uint64
	Round   types.Round
	Seed    types.Hash
	Members []Member
}

type Selector interface {
	Select(ctx context.Context, epoch uint64, round types.Round, seed types.Hash, set validator.Set) (Committee, error)
}

type RotationPolicy struct {
	EpochLength    uint64
	CommitteeSize  uint64
	VRFThreshold   []byte
	MinVotingPower types.VotingPower
	Backend        Backend
}
