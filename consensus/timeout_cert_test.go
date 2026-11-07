package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/types"
)

// reencodeTimeoutCert rewrites the signature groups of a timeout certificate.
func reencodeTimeoutCert(t *testing.T, timeoutCert finality.TimeoutCert, mutate func([]timeoutSignatureGroup) []timeoutSignatureGroup) finality.TimeoutCert {
	t.Helper()
	groups, err := decodeTimeoutSignatureGroups(timeoutCert.Signature)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(mutate(groups))
	if err != nil {
		t.Fatal(err)
	}
	timeoutCert.Signature = types.AggregateSignature(encoded)
	return timeoutCert
}

func TestVerifyTimeoutCertAcceptsGenuineCertificates(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	qc := cluster.qc(1, 0, types.Hash{1})

	if err := machine.VerifyTimeoutCert(cluster.timeoutCert(2, 4, qc, "a", "b", "c")); err != nil {
		t.Fatalf("a unanimous timeout certificate must verify: %v", err)
	}
	if err := machine.VerifyTimeoutCert(cluster.timeoutCert(2, 4, qc, "a", "b")); err != nil {
		t.Fatalf("a quorum timeout certificate must verify: %v", err)
	}

	// Voters may quote different certificates; each group of identical votes is aggregated
	// and verified on its own, and the best quoted certificate is the one the cert claims.
	better := cluster.qc(2, 2, types.Hash{2})
	mixed := cluster.timeoutCertFromVotes(4,
		cluster.timeoutVote("a", 3, 4, qc),
		cluster.timeoutVote("b", 3, 4, better),
		cluster.timeoutVote("c", 3, 4, finality.QuorumCert{}),
	)
	if err := machine.VerifyTimeoutCert(mixed); err != nil {
		t.Fatalf("a timeout certificate with mixed high qcs must verify: %v", err)
	}
	if mixed.HighQC.Round != 2 {
		t.Fatalf("expected the best quoted certificate, got %+v", mixed.HighQC)
	}
}

func TestVerifyTimeoutCertRejectsMalformedCertificates(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	qc := cluster.qc(1, 0, types.Hash{1})
	genuine := cluster.timeoutCert(2, 4, qc, "a", "b", "c")
	outsider := newEqualTestCluster(t, "a", "b", "c", "d")

	cases := []struct {
		name string
		cert func() finality.TimeoutCert
	}{
		{"missing signature groups", func() finality.TimeoutCert {
			cert := genuine
			cert.Signature = nil
			return cert
		}},
		{"signature groups that are not json", func() finality.TimeoutCert {
			cert := genuine
			cert.Signature = types.AggregateSignature("not-json")
			return cert
		}},
		{"no signature groups", func() finality.TimeoutCert {
			cert := genuine
			cert.Signature = types.AggregateSignature("[]")
			return cert
		}},
		{"garbage group signature", func() finality.TimeoutCert {
			return reencodeTimeoutCert(t, genuine, func(groups []timeoutSignatureGroup) []timeoutSignatureGroup {
				groups[0].Signature = types.AggregateSignature("garbage")
				return groups
			})
		}},
		{"certificate height below the votes", func() finality.TimeoutCert {
			cert := genuine
			cert.Height = 1
			return cert
		}},
		{"certificate height above the votes", func() finality.TimeoutCert {
			cert := genuine
			cert.Height = 9
			return cert
		}},
		{"certificate without a height", func() finality.TimeoutCert {
			cert := genuine
			cert.Height = 0
			return cert
		}},
		{"view beyond the supported range", func() finality.TimeoutCert {
			cert := genuine
			cert.Round = math.MaxUint64
			return cert
		}},
		{"a different view than the votes signed", func() finality.TimeoutCert {
			cert := genuine
			cert.Round = 5
			return cert
		}},
		{"sub-quorum signers", func() finality.TimeoutCert {
			return newEqualTestCluster(t, "a").timeoutCert(2, 4, qc, "a")
		}},
		{"a signer outside the validator set", func() finality.TimeoutCert {
			return outsider.timeoutCert(2, 4, qc, "a", "b", "d")
		}},
		{"the same signer in two groups", func() finality.TimeoutCert {
			return reencodeTimeoutCert(t, genuine, func(groups []timeoutSignatureGroup) []timeoutSignatureGroup {
				duplicate := groups[0]
				duplicate.HighQC = finality.QuorumCert{}
				return append(groups, duplicate)
			})
		}},
		{"signer list that differs from the groups", func() finality.TimeoutCert {
			cert := genuine
			cert.Signers = types.Bitmap("a,b")
			return cert
		}},
		{"claimed high qc better than any vote quoted", func() finality.TimeoutCert {
			cert := genuine
			cert.HighQC = cluster.qc(2, 3, types.Hash{9})
			return cert
		}},
		{"claimed high qc worse than the votes quoted", func() finality.TimeoutCert {
			cert := genuine
			cert.HighQC = finality.QuorumCert{}
			return cert
		}},
		{"votes quoting a certificate from their own view", func() finality.TimeoutCert {
			return cluster.timeoutCert(2, 4, cluster.qc(2, 4, types.Hash{2}), "a", "b", "c")
		}},
		{"votes quoting a certificate from a later view", func() finality.TimeoutCert {
			return cluster.timeoutCert(2, 4, cluster.qc(2, 9, types.Hash{2}), "a", "b", "c")
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := machine.VerifyTimeoutCert(testCase.cert()); !errors.Is(err, ErrInvalidTimeoutCert) {
				t.Fatalf("expected an invalid timeout certificate, got %v", err)
			}
		})
	}
}

// The collector trusts what it is given, so it can assemble a certificate whose votes quote
// a certificate from their own view. The machine refuses such votes one by one and refuses
// the assembled certificate as well.
func TestTimeoutVotesMustQuoteAnEarlierView(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	machine.StartRound(1, 0)

	sameView := cluster.qc(1, 3, types.Hash{1})
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 2, 3, sameView)); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected a vote quoting its own view to be rejected, got %v", err)
	}
	later := cluster.qc(1, 7, types.Hash{1})
	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", 2, 3, later)); !errors.Is(err, ErrInvalidVote) {
		t.Fatalf("expected a vote quoting a later view to be rejected, got %v", err)
	}
	if len(machine.timeouts.votes) != 0 {
		t.Fatal("a rejected timeout vote was stored")
	}

	assembled := cluster.timeoutCert(2, 3, sameView, "a", "b", "c")
	if err := machine.VerifyTimeoutCert(assembled); !errors.Is(err, ErrInvalidTimeoutCert) {
		t.Fatalf("expected the assembled certificate to be rejected, got %v", err)
	}
}

func TestTimeoutQuorumBuildsAVerifiableCertificate(t *testing.T) {
	cluster := newThreeValidatorCluster(t)
	machine := cluster.newMachine()
	ctx := context.Background()
	blocks := cluster.extendChain(machine, 2)
	status := machine.Status(ctx)

	if _, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("a", status.Height, status.Round, blocks[1].qc)); !errors.Is(err, ErrNoQuorum) {
		t.Fatalf("expected no quorum after one vote, got %v", err)
	}
	timeoutCert, err := machine.OnTimeoutVote(ctx, cluster.timeoutVote("b", status.Height, status.Round, blocks[1].qc))
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.VerifyTimeoutCert(timeoutCert); err != nil {
		t.Fatalf("the certificate the machine built must verify: %v", err)
	}
	if after := machine.Status(ctx); after.Round != status.Round+1 || after.Height != status.Height {
		t.Fatalf("a timeout moves the view and never the height: before %+v after %+v", status, after)
	}
}
