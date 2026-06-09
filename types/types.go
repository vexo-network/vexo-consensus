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
	AppHash           Hash
	ValidatorSetHash  Hash
	ConsensusHash     Hash
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
