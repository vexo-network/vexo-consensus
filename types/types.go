package types

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
