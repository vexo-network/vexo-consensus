package crypto

import "github.com/vexo-network/vexo-consensus/types"

type Signer interface {
	PublicKey() types.PublicKey
	Sign(message []byte) (types.Signature, error)
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

type AggregateSigner interface {
	Aggregate(signatures []types.Signature) (types.AggregateSignature, error)
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

type BLSAdapter interface {
	Signer
	AggregateSigner
	ValidatePublicKey(publicKey types.PublicKey) error
	VerifyProofOfPossession(publicKey types.PublicKey, proof types.Signature) bool
	Metadata() BLSAdapterMetadata
}

type BLSAdapterMetadata struct {
	Name                  string
	Version               string
	Audited               bool
	AuditReport           string
	DomainSeparation      bool
	PublicKeyValidation   bool
	SubgroupChecks        bool
	RogueKeyDefense       bool
	DeterministicEncoding bool
	MalformedInputFuzzed  bool
	ProofOfPossession     bool
}

type VRF interface {
	Prove(seed []byte) (output []byte, proof []byte, err error)
	Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool
}

type Suite struct {
	Signer          Signer
	AggregateSigner AggregateSigner
	VRF             VRF
}
