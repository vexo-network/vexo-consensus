package finality

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/vexo-network/vexo-consensus/signbytes"
	"github.com/vexo-network/vexo-consensus/types"
)

var ErrEmptySigner = errors.New("empty signer in quorum certificate")

// HeaderHash is the hash of the block that owns header. It is the same hash as consensus.HashHeader,
// so a quorum certificate for a block certifies HeaderHash of that block's header.
func HeaderHash(header types.Header) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte(header.ChainID))
	writeUint64(hasher, uint64(header.Height))
	writeUint64(hasher, uint64(header.TimeUnixNano))
	hasher.Write(header.PreviousBlockHash[:])
	hasher.Write(header.AppHash[:])
	hasher.Write(header.ValidatorSetHash[:])
	hasher.Write(header.ConsensusHash[:])
	hasher.Write(header.TxRoot[:])
	writeUint64(hasher, uint64(header.Round))
	writeUint64(hasher, uint64(header.JustifyRound))
	writeUint64(hasher, uint64(header.ValidatorSetHeight))
	writeUint64(hasher, uint64(header.StateHeight))
	hasher.Write(header.NextValidatorSetHash[:])

	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

// FirstValidatorSetHeight is the registry height of the validator set of the first block: the
// genesis set.
const FirstValidatorSetHeight types.Height = 1

// AppHashHeight returns the height of the state whose app hash a header commits, its StateHeight
// (the genesis state for 0). Every validator that verifies the block has executed that block
// (ChildStateHeight), so each checks the hash against its own state before it votes, and a
// certificate of the header vouches for the replicated state of that height (see HeaderAppHash).
// It reports false for a header that names no validator set, which no block can be.
func AppHashHeight(header types.Header) (types.Height, bool) {
	if header.ValidatorSetHeight == 0 {
		return 0, false
	}
	return header.StateHeight, true
}

// HeaderAppHash returns the hash a block header carries as its AppHash for a state: the
// application's app hash of the state together with the root of the validator registry after it.
// The application writes no validator set; the runtime records the sets the application's updates
// produce in the same atomic commit, after the application hashed its state. Committing both
// covers the whole replicated state, so the validator sets of past heights, which evidence is
// judged against, are as certified as the application state they came from.
func HeaderAppHash(appHash types.Hash, validatorRegistryRoot types.Hash) types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo/header-app-hash/v1"))
	hasher.Write(appHash[:])
	hasher.Write(validatorRegistryRoot[:])
	var hash types.Hash
	copy(hash[:], hasher.Sum(nil))
	return hash
}

// ChildStateHeight returns the state height a child of parent commits. The state advances only
// where the chain itself proves a commit: a child carries QC(parent), and when the parent was
// proposed in the view directly after its own justify certificate QC(G), the pair QC(G),
// QC(parent) is in consecutive views and commits the grandparent G under the 2-chain rule. Every
// validator that verifies the child has then executed G, at height parent.Height-1. Otherwise the
// child commits the state of its parent.
func ChildStateHeight(parent types.Header) types.Height {
	if parent.Height >= 2 && parent.JustifyRound != ^types.Round(0) && parent.Round == parent.JustifyRound+1 {
		return parent.Height - 1
	}
	return parent.StateHeight
}

// ChildValidatorSetHeight returns the registry height of the validator set that certifies a child
// of parent: the set after the state the parent commits, parent.StateHeight+1. The parent's
// AppHash commits that state and its NextValidatorSetHash names the set, so a certified header
// fixes the set of its children before that set signs anything: a light client that trusts a
// header verifies its child with full quorum, whatever the set change. A validator set update so
// applies one commit after the state that produced it, and never to a height that may already be
// certified by the previous set. The rule depends only on the parent header, so every node, a
// light client included, derives the same set for the same block.
func ChildValidatorSetHeight(parent types.Header) types.Height {
	return parent.StateHeight + 1
}

// SignBytes returns the bytes validators sign when they vote for the certified block in the
// certified view. The consensus vote domain is applied on top by the signature verifier.
func (cert QuorumCert) SignBytes() []byte {
	return signbytes.Vote(cert.Height, cert.Round, cert.BlockHash)
}

// SignBytes returns the vote sign bytes of QC(B), the certificate of the proven block.
func (proof Proof) SignBytes() []byte {
	return proof.QuorumCert.SignBytes()
}

// HasCommitProof reports whether the proof carries the single child link of the 2-chain commit
// rule. It does not verify the link; Verifier does.
func (proof Proof) HasCommitProof() bool {
	return len(proof.CommitChain) == 1
}

// WithChild returns the proof extended with the child C of its block: the header of C and QC(C).
// The child's hash is derived from its header.
func (proof Proof) WithChild(header types.Header, quorumCert QuorumCert) Proof {
	proof.CommitChain = []CommitLink{{
		Header:     header,
		BlockHash:  HeaderHash(header),
		QuorumCert: quorumCert,
	}}
	return proof
}

func EncodeSigners(signers []types.ValidatorID) types.Bitmap {
	parts := make([]string, 0, len(signers))
	for _, signer := range signers {
		parts = append(parts, string(signer))
	}
	return types.Bitmap(strings.Join(parts, ","))
}

func ParseSigners(signers types.Bitmap) ([]types.ValidatorID, error) {
	if len(signers) == 0 {
		return nil, nil
	}

	parts := strings.Split(string(signers), ",")
	parsed := make([]types.ValidatorID, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, ErrEmptySigner
		}
		parsed = append(parsed, types.ValidatorID(part))
	}
	return parsed, nil
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeUint64(writer byteWriter, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	writer.Write(buffer[:])
}
