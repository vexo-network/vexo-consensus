package finality

import (
	"encoding/binary"

	"github.com/vexo-network/vexo-consensus/types"
)

// The certificates and the proof have the canonical binary encoding of package types: a value has
// one encoding, fields follow the order of the struct that declares them, and a decoder refuses
// anything else. The decoders only bound what they read; whether a certificate certifies anything
// is for the verifiers.

const (
	// MaxSignersBytes bounds the signer list of a certificate.
	MaxSignersBytes = 1 << 20
	// MaxAggregateSignatureBytes bounds the aggregate signature of a certificate.
	MaxAggregateSignatureBytes = 1 << 20
	// MaxCommitLinks bounds the links of the commit chain of a proof. The 2-chain rule uses one.
	MaxCommitLinks = 16
)

// AppendQuorumCert appends the encoding of qc.
func AppendQuorumCert(dst []byte, qc QuorumCert) []byte {
	dst = binary.AppendUvarint(dst, uint64(qc.Height))
	dst = binary.AppendUvarint(dst, uint64(qc.Round))
	dst = append(dst, qc.BlockHash[:]...)
	dst = types.AppendBytes(dst, qc.Signers)
	dst = types.AppendBytes(dst, qc.Signature)
	return binary.AppendUvarint(dst, uint64(qc.VotingPower))
}

// ReadQuorumCert reads a quorum certificate.
func ReadQuorumCert(reader *types.Reader) QuorumCert {
	var qc QuorumCert
	qc.Height = types.Height(reader.Uvarint())
	qc.Round = types.Round(reader.Uvarint())
	qc.BlockHash = reader.Hash()
	qc.Signers = types.Bitmap(reader.Bytes(MaxSignersBytes))
	qc.Signature = types.AggregateSignature(reader.Bytes(MaxAggregateSignatureBytes))
	qc.VotingPower = types.VotingPower(reader.Uvarint())
	return qc
}

// EncodeQuorumCert returns the encoding of qc.
func EncodeQuorumCert(qc QuorumCert) []byte {
	return AppendQuorumCert(nil, qc)
}

// DecodeQuorumCert decodes a quorum certificate that EncodeQuorumCert encoded, and refuses anything
// else.
func DecodeQuorumCert(data []byte) (QuorumCert, error) {
	reader := types.NewReader(data)
	qc := ReadQuorumCert(reader)
	if err := reader.Done(); err != nil {
		return QuorumCert{}, err
	}
	return qc, nil
}

// AppendTimeoutCert appends the encoding of timeoutCert.
func AppendTimeoutCert(dst []byte, timeoutCert TimeoutCert) []byte {
	dst = binary.AppendUvarint(dst, uint64(timeoutCert.Height))
	dst = binary.AppendUvarint(dst, uint64(timeoutCert.Round))
	dst = AppendQuorumCert(dst, timeoutCert.HighQC)
	dst = types.AppendBytes(dst, timeoutCert.Signers)
	return types.AppendBytes(dst, timeoutCert.Signature)
}

// ReadTimeoutCert reads a timeout certificate.
func ReadTimeoutCert(reader *types.Reader) TimeoutCert {
	var timeoutCert TimeoutCert
	timeoutCert.Height = types.Height(reader.Uvarint())
	timeoutCert.Round = types.Round(reader.Uvarint())
	timeoutCert.HighQC = ReadQuorumCert(reader)
	timeoutCert.Signers = types.Bitmap(reader.Bytes(MaxSignersBytes))
	timeoutCert.Signature = types.AggregateSignature(reader.Bytes(MaxAggregateSignatureBytes))
	return timeoutCert
}

// EncodeTimeoutCert returns the encoding of timeoutCert.
func EncodeTimeoutCert(timeoutCert TimeoutCert) []byte {
	return AppendTimeoutCert(nil, timeoutCert)
}

// DecodeTimeoutCert decodes a timeout certificate that EncodeTimeoutCert encoded, and refuses
// anything else.
func DecodeTimeoutCert(data []byte) (TimeoutCert, error) {
	reader := types.NewReader(data)
	timeoutCert := ReadTimeoutCert(reader)
	if err := reader.Done(); err != nil {
		return TimeoutCert{}, err
	}
	return timeoutCert, nil
}

// minCommitLinkBytes is the length of the encoding of the smallest link, which bounds the links an
// input can hold.
var minCommitLinkBytes = len(appendCommitLink(nil, CommitLink{}))

func appendCommitLink(dst []byte, link CommitLink) []byte {
	dst = types.AppendHeader(dst, link.Header)
	dst = append(dst, link.BlockHash[:]...)
	return AppendQuorumCert(dst, link.QuorumCert)
}

func readCommitLink(reader *types.Reader) CommitLink {
	var link CommitLink
	link.Header = reader.Header()
	link.BlockHash = reader.Hash()
	link.QuorumCert = ReadQuorumCert(reader)
	return link
}

// AppendProof appends the encoding of proof.
func AppendProof(dst []byte, proof Proof) []byte {
	dst = types.AppendHeader(dst, proof.Header)
	dst = append(dst, proof.BlockHash[:]...)
	dst = AppendQuorumCert(dst, proof.QuorumCert)
	dst = binary.AppendUvarint(dst, uint64(len(proof.CommitChain)))
	for _, link := range proof.CommitChain {
		dst = appendCommitLink(dst, link)
	}
	dst = binary.AppendUvarint(dst, uint64(proof.ValidatorSetHeight))
	return append(dst, proof.ValidatorSetHash[:]...)
}

// ReadProof reads a finality proof.
func ReadProof(reader *types.Reader) Proof {
	var proof Proof
	proof.Header = reader.Header()
	proof.BlockHash = reader.Hash()
	proof.QuorumCert = ReadQuorumCert(reader)
	if count := reader.Count(MaxCommitLinks, minCommitLinkBytes); count > 0 {
		proof.CommitChain = make([]CommitLink, count)
		for index := range proof.CommitChain {
			proof.CommitChain[index] = readCommitLink(reader)
		}
	}
	proof.ValidatorSetHeight = types.Height(reader.Uvarint())
	proof.ValidatorSetHash = reader.Hash()
	return proof
}

// EncodeProof returns the encoding of proof.
func EncodeProof(proof Proof) []byte {
	return AppendProof(nil, proof)
}

// DecodeProof decodes a finality proof that EncodeProof encoded, and refuses anything else.
func DecodeProof(data []byte) (Proof, error) {
	reader := types.NewReader(data)
	proof := ReadProof(reader)
	if err := reader.Done(); err != nil {
		return Proof{}, err
	}
	return proof, nil
}
