package crypto

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/vexo-network/vexo-consensus/signbytes"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestDomainMessageEncoding(t *testing.T) {
	message, err := DomainMessage(DomainConsensusVote, []byte("xy"))
	if err != nil {
		t.Fatal(err)
	}
	expected := "VEXO-DOMAIN\x00vexo.consensus.vote.v1\x00\x00\x00\x00\x00\x00\x00\x02xy"
	if string(message) != expected {
		t.Fatalf("unexpected domain message encoding %q", message)
	}
}

func TestParseSignMessageReadsKindHeightAndView(t *testing.T) {
	cases := []struct {
		name     string
		message  []byte
		expected signStatement
	}{
		{
			name:     "vote",
			message:  voteMessage(t, 41, 77, 'v'),
			expected: signStatement{signType: SignTypeConsensusVote, guarded: true, height: 41, view: 77},
		},
		{
			name:     "timeout vote",
			message:  timeoutVoteMessage(t, 42, 78, 't'),
			expected: signStatement{signType: SignTypeConsensusTimeoutVote, guarded: true, height: 42, view: 78},
		},
		{
			name:     "proposal without proposer id",
			message:  proposalMessage(t, 43, 79, "", 'p'),
			expected: signStatement{signType: SignTypeConsensusProposal, guarded: true, height: 43, view: 79},
		},
		{
			name:     "proposal with proposer id",
			message:  proposalMessage(t, 44, 80, "validator-with-a-long-identifier", 'p'),
			expected: signStatement{signType: SignTypeConsensusProposal, guarded: true, height: 44, view: 80},
		},
		{
			name:     "maximal height and view",
			message:  voteMessage(t, types.Height(^uint64(0)), types.Round(^uint64(0)), 'v'),
			expected: signStatement{signType: SignTypeConsensusVote, guarded: true, height: types.Height(^uint64(0)), view: types.Round(^uint64(0))},
		},
		{
			name:     "probe",
			message:  wrap(t, DomainRemoteSignerProbe, []byte("any challenge")),
			expected: signStatement{signType: SignTypeRemoteSignerProbe},
		},
		{
			name:     "raw bytes",
			message:  []byte("vexo:signed-tx:v1\nchain\npayload"),
			expected: signStatement{},
		},
		{
			name:     "empty",
			message:  nil,
			expected: signStatement{},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			statement, err := parseSignMessage(testCase.message)
			if err != nil {
				t.Fatal(err)
			}
			if statement != testCase.expected {
				t.Fatalf("expected %+v, got %+v", testCase.expected, statement)
			}
		})
	}
}

func TestParseSignMessageRefusesMalformedStatements(t *testing.T) {
	vote := signbytes.Vote(1, 2, types.Hash(filled(32, 'a')))
	validTimeout := statementPayload("timeout_vote", 1, 2, filled(signQuorumCertSize, 'a'))
	validProposal := statementPayload("proposal", 1, 2, filled(signHashSize, 'a'), filled(signQuorumCertSize, 1), filled(signTimeoutCertSize, 2))
	withLength := func(domain Domain, declared byte, payload []byte) []byte {
		message := append([]byte(domainMessageTag), domain...)
		message = append(message, 0, 0, 0, 0, 0, 0, 0, declared)
		return append(message, payload...)
	}
	cases := map[string][]byte{
		"unknown domain":            []byte(domainMessageTag + "vexo.finality.proof.v1\x00\x00\x00\x00\x00\x00\x00\x00"),
		"unrelated domain":          wrap(t, Domain("vexo.other.v1"), vote),
		"domain only":               []byte(domainMessageTag + string(DomainConsensusVote)),
		"truncated length":          []byte(domainMessageTag + string(DomainConsensusVote) + "\x00\x00\x00"),
		"length beyond payload":     withLength(DomainConsensusVote, byte(len(vote)+1), vote),
		"length short of payload":   withLength(DomainConsensusVote, byte(len(vote)-1), vote),
		"vote with trailing bytes":  wrap(t, DomainConsensusVote, append(append([]byte(nil), vote...), 0)),
		"vote missing bytes":        wrap(t, DomainConsensusVote, vote[:len(vote)-1]),
		"vote tag under proposal":   wrap(t, DomainConsensusProposal, vote),
		"proposal tag under vote":   wrap(t, DomainConsensusVote, validProposal),
		"timeout under vote":        wrap(t, DomainConsensusVote, validTimeout),
		"vote under timeout":        wrap(t, DomainConsensusTimeoutVote, vote),
		"timeout with extra bytes":  wrap(t, DomainConsensusTimeoutVote, append(append([]byte(nil), validTimeout...), 0)),
		"timeout missing bytes":     wrap(t, DomainConsensusTimeoutVote, validTimeout[:len(validTimeout)-1]),
		"proposal below minimum":    wrap(t, DomainConsensusProposal, validProposal[:len(validProposal)-1]),
		"empty vote payload":        wrap(t, DomainConsensusVote, nil),
		"header without view":       wrap(t, DomainConsensusVote, vote[:len("vote")+8]),
		"tag prefix collision":      wrap(t, DomainConsensusVote, append([]byte("voted"), vote[len("vote"):]...)),
		"probe domain with garbage": []byte(domainMessageTag + string(DomainRemoteSignerProbe) + "\x00\x00"),
	}
	for name, message := range cases {
		if _, err := parseSignMessage(message); !errors.Is(err, ErrInvalidSignMessage) {
			t.Fatalf("%s: expected invalid sign message, got %v", name, err)
		}
	}
}

func TestSignKindDomainsAreUnambiguous(t *testing.T) {
	for index, kind := range signKinds {
		if expected, ok := signKindOf(kind.signType); !ok || expected.domain != kind.domain {
			t.Fatalf("sign type %q is not resolvable", kind.signType)
		}
		if kind.guarded() && kind.payloadSize < len(kind.tag)+signStatementHeaderSize {
			t.Fatalf("sign type %q cannot hold its height and view", kind.signType)
		}
		for other, candidate := range signKinds {
			if index == other {
				continue
			}
			if kind.signType == candidate.signType || strings.HasPrefix(string(kind.domain), string(candidate.domain)) {
				t.Fatalf("sign kinds %q and %q are ambiguous", kind.signType, candidate.signType)
			}
		}
	}
}

func TestSignPolicyValidateAcceptsOnlyKnownTypeDomainPairs(t *testing.T) {
	for _, kind := range signKinds {
		policy := SignPolicy{ChainID: "vexo-test", Height: 1, Type: kind.signType, Domain: kind.domain}
		if err := policy.Validate(); err != nil {
			t.Fatalf("%s: %v", kind.signType, err)
		}
	}
	retired := SignPolicy{ChainID: "vexo-test", Height: 1, Type: SignType("finality_proof"), Domain: Domain("vexo.finality.proof.v1")}
	if err := retired.Validate(); !errors.Is(err, ErrInvalidSignPolicy) {
		t.Fatalf("expected the retired finality proof type to be refused, got %v", err)
	}
}

func TestPolicyMatchesDeclaresTheSignedStatement(t *testing.T) {
	vote := voteMessage(t, 9, 4, 'a')
	if err := votePolicy(9, 4).Matches(vote); err != nil {
		t.Fatal(err)
	}
	for name, policy := range map[string]SignPolicy{
		"height": votePolicy(10, 4),
		"round":  votePolicy(9, 5),
		"type":   {ChainID: "vexo-test", Height: 9, Round: 4, Type: SignTypeConsensusProposal, Domain: DomainConsensusProposal},
	} {
		if err := policy.Matches(vote); !errors.Is(err, ErrInvalidSignPolicy) {
			t.Fatalf("%s: expected mismatch, got %v", name, err)
		}
	}
	if err := votePolicy(9, 4).Matches(bytes.Repeat([]byte{1}, 8)); !errors.Is(err, ErrInvalidSignPolicy) {
		t.Fatalf("expected raw bytes to mismatch a consensus policy, got %v", err)
	}
}
