package types

import (
	"errors"
	"math/bits"
)

var ErrVotingPowerOverflow = errors.New("voting power overflow")

type Hash [32]byte

type Address string

type ValidatorID string

type Tx []byte

type Height uint64

type Round uint64

type VotingPower uint64

type Signature []byte

type PublicKey []byte

type AggregateSignature []byte

type Bitmap []byte

type ValidatorUpdate struct {
	ID          ValidatorID
	Address     Address
	PublicKey   PublicKey
	VotingPower VotingPower
	Stake       uint64
	Metadata    map[string]string
}

func AddVotingPower(left VotingPower, right VotingPower) (VotingPower, error) {
	sum, carry := bits.Add64(uint64(left), uint64(right), 0)
	if carry != 0 {
		return 0, ErrVotingPowerOverflow
	}
	return VotingPower(sum), nil
}

func AddUint64(left uint64, right uint64) (uint64, error) {
	sum, carry := bits.Add64(left, right, 0)
	if carry != 0 {
		return 0, ErrVotingPowerOverflow
	}
	return sum, nil
}

func MustAddUint64Saturating(left uint64, right uint64) uint64 {
	sum, err := AddUint64(left, right)
	if err != nil {
		return ^uint64(0)
	}
	return sum
}

func MustAddVotingPowerSaturating(left VotingPower, right VotingPower) VotingPower {
	sum, err := AddVotingPower(left, right)
	if err != nil {
		return ^VotingPower(0)
	}
	return sum
}

func TwoThirdsQuorumThreshold(total VotingPower) VotingPower {
	if total == 0 {
		return 0
	}
	base := total / 3 * 2
	remainder := total % 3
	extra := VotingPower(0)
	if remainder > 0 {
		extra = VotingPower((uint64(remainder)*2 + 2) / 3)
	}
	return MustAddVotingPowerSaturating(base, extra)
}

func HasTwoThirdsQuorum(power VotingPower, total VotingPower) bool {
	if total == 0 {
		return false
	}
	return power >= TwoThirdsQuorumThreshold(total)
}

type Header struct {
	ChainID           string
	Height            Height
	TimeUnixNano      int64
	PreviousBlockHash Hash
	// AppHash commits the replicated state after executing block StateHeight, the genesis state
	// when that is height 0: the application's app hash together with the root of the validator
	// registry (see finality.HeaderAppHash).
	AppHash          Hash
	ValidatorSetHash Hash
	// NextValidatorSetHash is the hash of the validator set that certifies the children of the
	// block: the set the registry holds after executing block StateHeight, which is part of the
	// state AppHash commits. A certified header so names the set of its children before that set
	// signs anything (see finality.ChildValidatorSetHeight).
	NextValidatorSetHash Hash
	ConsensusHash        Hash
	TxRoot               Hash
	// Round is the view the block is proposed in, and JustifyRound the view of the certificate
	// of its parent the proposal carries (zero for the first block).
	Round        Round
	JustifyRound Round
	// ValidatorSetHeight is the registry height of the validator set that certifies the block:
	// the set the registry holds after executing block ValidatorSetHeight-1.
	ValidatorSetHeight Height
	// StateHeight is the height of the executed block whose resulting state AppHash commits.
	StateHeight Height
}

type Block struct {
	Header Header
	Txs    []Tx
}

type Result struct {
	Code    uint32
	Log     string
	Data    []byte
	GasUsed uint64
	FeePaid uint64
}
