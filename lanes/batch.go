// Package lanes is the data plane of the consensus: every validator disseminates the transactions
// it is responsible for as signed batches on its own lane, and a batch that validators holding a
// quorum of the voting power acknowledged storing is certified available. A leader then proposes a
// block that names certified batches instead of carrying their transactions, so its upload per
// block is a few bytes per batch whatever the block size, and the dissemination load is spread
// over all validators instead of resting on the leader.
//
// The consensus is unchanged: a block still carries its transactions, its header still commits
// them, and validators still judge and execute exactly that block. Lanes only change how the block
// travels: a validator rebuilds it from the batches it already stores (see consensus.LaneProposal).
package lanes

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const (
	// MaxBatchBytes bounds the transaction bytes of one batch.
	MaxBatchBytes = 1 << 20
	// MaxBatchTxs bounds the transactions of one batch.
	MaxBatchTxs = 8192
)

var (
	ErrInvalidBatch = errors.New("invalid lane batch")
	ErrInvalidAck   = errors.New("invalid lane batch acknowledgement")
	ErrInvalidCert  = errors.New("invalid lane batch certificate")
)

// Batch is a sequence of transactions one validator, its author, disseminates on its lane. Seq
// orders the batches of a lane: a leader proposes the batches of a lane in Seq order, so the
// transactions of one sender, which one lane carries, keep their nonce order.
type Batch struct {
	ChainID   string            `json:"chain_id"`
	Author    types.ValidatorID `json:"author"`
	Seq       uint64            `json:"seq"`
	Txs       []types.Tx        `json:"txs"`
	Signature types.Signature   `json:"signature,omitempty"`
}

// ID returns the hash that names the batch. It covers the chain, the author, the sequence number
// and every transaction, so an acknowledgement of the id vouches for exactly these transactions.
func (batch Batch) ID() types.Hash {
	hasher := sha256.New()
	hasher.Write([]byte("vexo/lanes/batch/v1"))
	writeBytes(hasher, []byte(batch.ChainID))
	writeBytes(hasher, []byte(batch.Author))
	writeUint64(hasher, batch.Seq)
	writeUint64(hasher, uint64(len(batch.Txs)))
	for _, tx := range batch.Txs {
		writeBytes(hasher, tx)
	}
	var id types.Hash
	copy(id[:], hasher.Sum(nil))
	return id
}

// Size returns the transaction bytes of the batch.
func (batch Batch) Size() int {
	size := 0
	for _, tx := range batch.Txs {
		size += len(tx)
	}
	return size
}

// checkShape refuses a batch no honest author makes: without a chain, an author or a sequence
// number, empty, with an empty transaction, or over the limits.
func (batch Batch) checkShape(chainID string) error {
	if batch.ChainID != chainID {
		return fmt.Errorf("%w: chain %q", ErrInvalidBatch, batch.ChainID)
	}
	if batch.Author == "" || batch.Seq == 0 {
		return fmt.Errorf("%w: no author or sequence number", ErrInvalidBatch)
	}
	if len(batch.Txs) == 0 || len(batch.Txs) > MaxBatchTxs {
		return fmt.Errorf("%w: %d transactions", ErrInvalidBatch, len(batch.Txs))
	}
	for _, tx := range batch.Txs {
		if len(tx) == 0 {
			return fmt.Errorf("%w: empty transaction", ErrInvalidBatch)
		}
	}
	if batch.Size() > MaxBatchBytes {
		return fmt.Errorf("%w: %d bytes", ErrInvalidBatch, batch.Size())
	}
	return nil
}

// Ack is a validator's statement that it stores the batch of BatchID and serves it to the
// validators that ask.
type Ack struct {
	BatchID   types.Hash        `json:"batch_id"`
	Voter     types.ValidatorID `json:"voter"`
	Signature types.Signature   `json:"signature"`
}

// Cert certifies a batch available: validators holding a quorum of the voting power acknowledged
// storing it, so at most a third of the power does not hold it, and at least one honest validator
// does. Signature aggregates their acknowledgements in the order Signers lists them.
type Cert struct {
	BatchID   types.Hash               `json:"batch_id"`
	Author    types.ValidatorID        `json:"author"`
	Signers   types.Bitmap             `json:"signers"`
	Signature types.AggregateSignature `json:"signature"`
}

// BatchSignBytes returns what the author of the batch of id signs, in DomainLaneBatch.
func BatchSignBytes(id types.Hash) []byte {
	return append([]byte("lane_batch"), id[:]...)
}

// AckSignBytes returns what a validator storing the batch of id signs, in DomainLaneBatchAck.
func AckSignBytes(id types.Hash) []byte {
	return append([]byte("lane_batch_ack"), id[:]...)
}

// SignatureVerifier verifies one signature over a domain-separated message.
type SignatureVerifier interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

// AggregateVerifier verifies an aggregate of signatures over one message.
type AggregateVerifier interface {
	VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}

// VerifyBatch checks that batch is well formed and signed by its author, a validator of set.
func VerifyBatch(chainID string, batch Batch, set validator.Set, signatures SignatureVerifier) error {
	if err := batch.checkShape(chainID); err != nil {
		return err
	}
	author, found := set.Get(batch.Author)
	if !found {
		return fmt.Errorf("%w: author %q is not a validator", ErrInvalidBatch, batch.Author)
	}
	if !verifyDomain(signatures, author.PublicKey, vexocrypto.DomainLaneBatch, BatchSignBytes(batch.ID()), batch.Signature) {
		return fmt.Errorf("%w: author signature", ErrInvalidBatch)
	}
	return nil
}

// VerifyAck checks that ack is signed by its voter, a validator of set.
func VerifyAck(ack Ack, set validator.Set, signatures SignatureVerifier) error {
	voter, found := set.Get(ack.Voter)
	if !found {
		return fmt.Errorf("%w: voter %q is not a validator", ErrInvalidAck, ack.Voter)
	}
	if !verifyDomain(signatures, voter.PublicKey, vexocrypto.DomainLaneBatchAck, AckSignBytes(ack.BatchID), ack.Signature) {
		return fmt.Errorf("%w: voter signature", ErrInvalidAck)
	}
	return nil
}

// VerifyCert checks that cert aggregates the acknowledgements of distinct validators of set that
// hold a quorum of its voting power.
func VerifyCert(cert Cert, set validator.Set, aggregates AggregateVerifier) error {
	if _, found := set.Get(cert.Author); !found {
		return fmt.Errorf("%w: author %q is not a validator", ErrInvalidCert, cert.Author)
	}
	signers, err := finality.ParseSigners(cert.Signers)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidCert, err)
	}
	var power types.VotingPower
	keys := make([]types.PublicKey, 0, len(signers))
	seen := make(map[types.ValidatorID]struct{}, len(signers))
	for _, signer := range signers {
		if _, duplicate := seen[signer]; duplicate {
			return fmt.Errorf("%w: duplicate signer %q", ErrInvalidCert, signer)
		}
		seen[signer] = struct{}{}
		member, found := set.Get(signer)
		if !found {
			return fmt.Errorf("%w: signer %q is not a validator", ErrInvalidCert, signer)
		}
		if power, err = types.AddVotingPower(power, member.VotingPower); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCert, err)
		}
		keys = append(keys, member.PublicKey)
	}
	if !types.HasTwoThirdsQuorum(power, set.TotalVotingPower()) {
		return fmt.Errorf("%w: signers hold no quorum", ErrInvalidCert)
	}
	verifier, err := vexocrypto.NewDomainAggregateVerifier(aggregates, vexocrypto.DomainLaneBatchAck)
	if err != nil {
		return err
	}
	if len(cert.Signature) == 0 || !verifier.VerifyAggregate(keys, AckSignBytes(cert.BatchID), cert.Signature) {
		return fmt.Errorf("%w: aggregate signature", ErrInvalidCert)
	}
	return nil
}

func verifyDomain(signatures SignatureVerifier, key types.PublicKey, domain vexocrypto.Domain, message []byte, signature types.Signature) bool {
	if signatures == nil || len(signature) == 0 {
		return false
	}
	domainMessage, err := vexocrypto.DomainMessage(domain, message)
	if err != nil {
		return false
	}
	return signatures.Verify(key, domainMessage, signature)
}

func writeBytes(hasher hash.Hash, value []byte) {
	writeUint64(hasher, uint64(len(value)))
	hasher.Write(value)
}

func writeUint64(hasher hash.Hash, value uint64) {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], value)
	hasher.Write(buffer[:])
}
