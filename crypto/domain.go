package crypto

import (
	"encoding/binary"
	"errors"

	"github.com/vexo-network/vexo-consensus/types"
)

type Domain string

const (
	DomainConsensusProposal    Domain = "vexo.consensus.proposal.v1"
	DomainConsensusVote        Domain = "vexo.consensus.vote.v1"
	DomainConsensusTimeoutVote Domain = "vexo.consensus.timeout_vote.v1"
	// DomainRemoteSignerProbe wraps operator challenges that prove a remote signer holds
	// its key. No verifier accepts it, so a probe signature never authorizes anything.
	DomainRemoteSignerProbe Domain = "vexo.remote_signer.probe.v1"

	// DomainLaneBatch signs a batch of transactions a validator disseminates on its lane, and
	// DomainLaneBatchAck the acknowledgement of a validator that stores it (see package lanes).
	// Neither is a consensus statement: they order nothing and are not subject to the double-sign
	// guard.
	DomainLaneBatch    Domain = "vexo.lanes.batch.v1"
	DomainLaneBatchAck Domain = "vexo.lanes.batch_ack.v1"
)

// domainMessageTag opens every domain-separated message: tag, NUL, domain, big-endian
// payload length, payload. DomainMessage builds it and the remote signer guard parses it.
const domainMessageTag = "VEXO-DOMAIN\x00"

var ErrEmptyDomain = errors.New("signature domain is empty")

type DomainSigner struct {
	signer Signer
	domain Domain
}

func NewDomainSigner(signer Signer, domain Domain) (DomainSigner, error) {
	if signer == nil {
		return DomainSigner{}, ErrKeyNotFound
	}
	if domain == "" {
		return DomainSigner{}, ErrEmptyDomain
	}
	return DomainSigner{signer: signer, domain: domain}, nil
}

func (signer DomainSigner) PublicKey() types.PublicKey {
	return signer.signer.PublicKey()
}

func (signer DomainSigner) Sign(message []byte) (types.Signature, error) {
	domainMessage, err := DomainMessage(signer.domain, message)
	if err != nil {
		return nil, err
	}
	return signer.signer.Sign(domainMessage)
}

func (signer DomainSigner) Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool {
	domainMessage, err := DomainMessage(signer.domain, message)
	if err != nil {
		return false
	}
	return signer.signer.Verify(publicKey, domainMessage, signature)
}

type DomainAggregateSigner struct {
	signer AggregateSigner
	domain Domain
}

type DomainAggregateVerifier struct {
	verifier interface {
		VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
	}
	domain Domain
}

func NewDomainAggregateSigner(signer AggregateSigner, domain Domain) (DomainAggregateSigner, error) {
	if signer == nil {
		return DomainAggregateSigner{}, ErrKeyNotFound
	}
	if domain == "" {
		return DomainAggregateSigner{}, ErrEmptyDomain
	}
	return DomainAggregateSigner{signer: signer, domain: domain}, nil
}

func NewDomainAggregateVerifier(verifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}, domain Domain) (DomainAggregateVerifier, error) {
	if verifier == nil {
		return DomainAggregateVerifier{}, ErrKeyNotFound
	}
	if domain == "" {
		return DomainAggregateVerifier{}, ErrEmptyDomain
	}
	return DomainAggregateVerifier{verifier: verifier, domain: domain}, nil
}

func (signer DomainAggregateSigner) Aggregate(signatures []types.Signature) (types.AggregateSignature, error) {
	return signer.signer.Aggregate(signatures)
}

func (signer DomainAggregateSigner) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	domainMessage, err := DomainMessage(signer.domain, message)
	if err != nil {
		return false
	}
	return signer.signer.VerifyAggregate(publicKeys, domainMessage, signature)
}

func (verifier DomainAggregateVerifier) VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool {
	domainMessage, err := DomainMessage(verifier.domain, message)
	if err != nil {
		return false
	}
	return verifier.verifier.VerifyAggregate(publicKeys, domainMessage, signature)
}

func SignWithDomain(signer Signer, domain Domain, message []byte) (types.Signature, error) {
	domainSigner, err := NewDomainSigner(signer, domain)
	if err != nil {
		return nil, err
	}
	return domainSigner.Sign(message)
}

func DomainMessage(domain Domain, message []byte) ([]byte, error) {
	if domain == "" {
		return nil, ErrEmptyDomain
	}

	encoded := make([]byte, 0, len(domainMessageTag)+len(domain)+8+len(message))
	encoded = append(encoded, domainMessageTag...)
	encoded = append(encoded, domain...)
	encoded = binary.BigEndian.AppendUint64(encoded, uint64(len(message)))
	encoded = append(encoded, message...)
	return encoded, nil
}
