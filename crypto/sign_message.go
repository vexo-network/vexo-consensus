package crypto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/vexo-network/vexo-consensus/types"
)

var ErrInvalidSignMessage = errors.New("sign message is not a recognized signing statement")

// A remote signer never trusts the SignPolicy its caller declares: a buggy or
// compromised node can declare anything. It reads what the bytes it is asked to sign
// commit to instead. A consensus statement is DomainMessage(domain, payload), where
// payload starts with an ASCII tag, the big-endian height and the big-endian view, as
// laid out by consensus.ProposalSignBytes, signbytes.Vote and
// consensus.TimeoutVoteSignBytes. Proposal.Round, Vote.Round and TimeoutVote.Round all
// carry the global view.
const (
	signHashSize = 32
	// signQuorumCertSize is a QC inside a statement: height, round, block hash, voting
	// power and the hash of the signer set.
	signQuorumCertSize = 8 + 8 + signHashSize + 8 + signHashSize
	// signTimeoutCertSize is a timeout certificate inside a proposal: height, round, high
	// QC, hash of the signer set and hash of the aggregate signature.
	signTimeoutCertSize = 8 + 8 + signQuorumCertSize + signHashSize + signHashSize
	// signStatementHeaderSize is the height and the view that follow the tag.
	signStatementHeaderSize = 8 + 8
)

type signKind struct {
	signType SignType
	domain   Domain
	// tag opens the payload of a consensus statement. The probe payload is opaque.
	tag string
	// payloadSize is the exact payload length, or the lower bound when variable is set.
	payloadSize int
	variable    bool
}

// signKinds is the closed set of signable statements. No domain is a prefix of another,
// which keeps the parse of the unterminated domain string unambiguous.
var signKinds = [...]signKind{
	{
		signType:    SignTypeConsensusProposal,
		domain:      DomainConsensusProposal,
		tag:         "proposal",
		payloadSize: len("proposal") + signStatementHeaderSize + signHashSize + signQuorumCertSize + signTimeoutCertSize,
		variable:    true, // the proposer id sits between the view and the block hash
	},
	{
		signType:    SignTypeConsensusVote,
		domain:      DomainConsensusVote,
		tag:         "vote",
		payloadSize: len("vote") + signStatementHeaderSize + signHashSize,
	},
	{
		signType:    SignTypeConsensusTimeoutVote,
		domain:      DomainConsensusTimeoutVote,
		tag:         "timeout_vote",
		payloadSize: len("timeout_vote") + signStatementHeaderSize + signQuorumCertSize,
	},
	{
		signType: SignTypeRemoteSignerProbe,
		domain:   DomainRemoteSignerProbe,
	},
	{
		signType: SignTypeLaneBatch,
		domain:   DomainLaneBatch,
	},
	{
		signType: SignTypeLaneBatchAck,
		domain:   DomainLaneBatchAck,
	},
}

func signKindOf(signType SignType) (signKind, bool) {
	for _, kind := range signKinds {
		if kind.signType == signType {
			return kind, true
		}
	}
	return signKind{}, false
}

// guarded reports whether the kind is a consensus statement, which is subject to the
// double-sign high-water mark. The probe and the lane statements are not.
func (kind signKind) guarded() bool {
	return kind.tag != ""
}

// signStatement is what a message commits to. A message without the domain tag is raw:
// it carries no signType, and no verifier of consensus statements accepts it.
type signStatement struct {
	signType SignType
	// guarded is set for consensus statements, whose height and view are meaningful.
	guarded bool
	height  types.Height
	view    types.Round
}

// parseSignMessage reads the statement out of message. A message that opens with the
// domain tag must be exactly one well-formed statement of a known kind; anything else in
// the domain namespace is refused rather than guessed at.
func parseSignMessage(message []byte) (signStatement, error) {
	rest, tagged := bytes.CutPrefix(message, []byte(domainMessageTag))
	if !tagged {
		return signStatement{}, nil
	}
	for _, kind := range signKinds {
		body, matched := bytes.CutPrefix(rest, []byte(kind.domain))
		if !matched {
			continue
		}
		return kind.parse(body)
	}
	return signStatement{}, fmt.Errorf("%w: unknown domain", ErrInvalidSignMessage)
}

func (kind signKind) parse(body []byte) (signStatement, error) {
	if len(body) < 8 {
		return signStatement{}, fmt.Errorf("%w: truncated %s length", ErrInvalidSignMessage, kind.signType)
	}
	payload := body[8:]
	if declared := binary.BigEndian.Uint64(body[:8]); declared != uint64(len(payload)) {
		return signStatement{}, fmt.Errorf("%w: %s length does not match payload", ErrInvalidSignMessage, kind.signType)
	}
	statement := signStatement{signType: kind.signType, guarded: kind.guarded()}
	if !statement.guarded {
		return statement, nil
	}
	fields, tagged := bytes.CutPrefix(payload, []byte(kind.tag))
	if !tagged {
		return signStatement{}, fmt.Errorf("%w: %s payload has the wrong tag", ErrInvalidSignMessage, kind.signType)
	}
	if len(payload) < kind.payloadSize || (!kind.variable && len(payload) != kind.payloadSize) {
		return signStatement{}, fmt.Errorf("%w: %s payload has the wrong size", ErrInvalidSignMessage, kind.signType)
	}
	statement.height = types.Height(binary.BigEndian.Uint64(fields[:8]))
	statement.view = types.Round(binary.BigEndian.Uint64(fields[8:16]))
	return statement, nil
}

// Matches reports whether the statement the message commits to is the one the policy
// declares, so a declared policy can never describe different bytes than the ones signed.
func (policy SignPolicy) Matches(message []byte) error {
	statement, err := parseSignMessage(message)
	if err != nil {
		return err
	}
	if statement.signType != policy.Type {
		return fmt.Errorf("%w: policy declares %q but the message carries %q", ErrInvalidSignPolicy, policy.Type, statement.signType)
	}
	if statement.guarded && (statement.height != policy.Height || statement.view != policy.Round) {
		return fmt.Errorf("%w: policy declares height %d round %d but the message carries height %d round %d",
			ErrInvalidSignPolicy, policy.Height, policy.Round, statement.height, statement.view)
	}
	return nil
}
