package committee

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var errTestVRFKey = errors.New("invalid test vrf key")

func TestNewVRFSelectorRejectsMissingVRF(t *testing.T) {
	_, err := NewVRFSelector(RotationPolicy{EpochLength: 1, CommitteeSize: 1}, nil)
	if !errors.Is(err, ErrMissingVRF) {
		t.Fatalf("expected missing vrf, got %v", err)
	}
}

func TestVRFSelectorSelectsAndVerifiesMembers(t *testing.T) {
	keys := map[string][]byte{
		"a-pub": []byte("a-secret"),
		"b-pub": []byte("b-secret"),
		"c-pub": []byte("c-secret"),
	}
	selector := mustVRFSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 2}, testVRF{keys: keys})
	committee, err := selector.Select(context.Background(), 1, 2, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(committee.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(committee.Members))
	}
	for _, member := range committee.Members {
		if len(member.Proof) == 0 {
			t.Fatal("expected member proof")
		}
		if len(member.Output) == 0 {
			t.Fatal("expected member output")
		}
		if !selector.VerifyMember(committee.Epoch, committee.Round, committee.Seed, member) {
			t.Fatal("expected member proof to verify")
		}
	}
}

func TestVRFSelectorIsDeterministic(t *testing.T) {
	keys := map[string][]byte{
		"a-pub": []byte("a-secret"),
		"b-pub": []byte("b-secret"),
		"c-pub": []byte("c-secret"),
	}
	selector := mustVRFSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 2}, testVRF{keys: keys})
	set := testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
		{ID: "b", VotingPower: 1, PublicKey: []byte("b-pub")},
		{ID: "c", VotingPower: 1, PublicKey: []byte("c-pub")},
	})

	first, err := selector.Select(context.Background(), 1, 2, testSeed(1), set)
	if err != nil {
		t.Fatal(err)
	}
	second, err := selector.Select(context.Background(), 1, 2, testSeed(1), set)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(memberIDs(first), memberIDs(second)) {
		t.Fatalf("expected deterministic members, got %v and %v", memberIDs(first), memberIDs(second))
	}
}

func TestVRFSelectorRejectsMissingValidatorKey(t *testing.T) {
	selector := mustVRFSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 1}, testVRF{})
	_, err := selector.Select(context.Background(), 1, 2, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
	}))
	if !errors.Is(err, errTestVRFKey) {
		t.Fatalf("expected invalid vrf key, got %v", err)
	}
}

func TestVRFSelectorPropagatesContextToVRF(t *testing.T) {
	vrf := &contextAwareTestVRF{testVRF: testVRF{keys: map[string][]byte{"a-pub": []byte("a-secret")}}}
	selector := mustVRFSelector(t, RotationPolicy{EpochLength: 10, CommitteeSize: 1}, vrf)
	ctx := context.WithValue(context.Background(), contextKey("vrf-test"), "expected")
	committee, err := selector.Select(ctx, 1, 2, testSeed(1), testSet(t, []validator.Validator{
		{ID: "a", VotingPower: 1, PublicKey: []byte("a-pub")},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !vrf.proveSawContext || !vrf.verifySawContext {
		t.Fatalf("expected selector to pass context to VRF prove/verify")
	}
	if !selector.VerifyMemberWithContext(ctx, committee.Epoch, committee.Round, committee.Seed, committee.Members[0]) || !vrf.memberVerifySawContext {
		t.Fatalf("expected member verification to pass context to VRF")
	}
}

func mustVRFSelector(t *testing.T, policy RotationPolicy, vrf VRF) VRFSelector {
	t.Helper()
	selector, err := NewVRFSelector(policy, vrf)
	if err != nil {
		t.Fatal(err)
	}
	return selector
}

type testVRF struct {
	keys map[string][]byte
}

type contextKey string

type contextAwareTestVRF struct {
	testVRF
	proveSawContext        bool
	verifySawContext       bool
	memberVerifySawContext bool
}

func (vrf *contextAwareTestVRF) ProveWithContext(ctx context.Context, publicKey types.PublicKey, seed []byte) (output []byte, proof []byte, err error) {
	if ctx.Value(contextKey("vrf-test")) == "expected" {
		vrf.proveSawContext = true
	}
	return vrf.testVRF.Prove(publicKey, seed)
}

func (vrf *contextAwareTestVRF) VerifyWithContext(ctx context.Context, publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	if ctx.Value(contextKey("vrf-test")) == "expected" {
		if vrf.verifySawContext {
			vrf.memberVerifySawContext = true
		}
		vrf.verifySawContext = true
	}
	return vrf.testVRF.Verify(publicKey, seed, output, proof)
}

func (vrf testVRF) Prove(publicKey types.PublicKey, seed []byte) (output []byte, proof []byte, err error) {
	privateKey, found := vrf.keys[string(publicKey)]
	if !found {
		return nil, nil, errTestVRFKey
	}
	output = testVRFOutput(privateKey, seed)
	return output, append([]byte(nil), output...), nil
}

func (vrf testVRF) Verify(publicKey types.PublicKey, seed []byte, output []byte, proof []byte) bool {
	privateKey, found := vrf.keys[string(publicKey)]
	if !found {
		return false
	}
	expected := testVRFOutput(privateKey, seed)
	return hmac.Equal(expected, output) && hmac.Equal(expected, proof)
}

func testVRFOutput(privateKey []byte, seed []byte) []byte {
	mac := hmac.New(sha256.New, privateKey)
	mac.Write(seed)
	return mac.Sum(nil)
}
