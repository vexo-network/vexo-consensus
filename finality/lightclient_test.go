package finality

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var lightGenesisTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// lightChain builds the finality proofs of a chain whose blocks are each proposed in the view after
// their parent's, so every block commits its grandparent and the set of a block at height h > 1 is
// the registry's at h-1. members names the validators of each registry height; each validator's
// key derives from its name and key.
type lightChain struct {
	t       *testing.T
	members func(registryHeight types.Height) []string
	key     func(id string) string
	signers map[string]vexocrypto.DeterministicSigner
}

func newLightChain(t *testing.T, members func(types.Height) []string) *lightChain {
	return &lightChain{t: t, members: members, key: func(id string) string { return id }, signers: map[string]vexocrypto.DeterministicSigner{}}
}

func (chain *lightChain) signer(id string) vexocrypto.DeterministicSigner {
	seed := chain.key(id)
	signer, found := chain.signers[seed]
	if !found {
		var err error
		signer, err = vexocrypto.NewDeterministicSigner([]byte(seed))
		if err != nil {
			chain.t.Fatal(err)
		}
		chain.signers[seed] = signer
	}
	return signer
}

func (chain *lightChain) set(registryHeight types.Height) validator.Set {
	validators := make([]validator.Validator, 0)
	for _, id := range chain.members(registryHeight) {
		validators = append(validators, validator.Validator{ID: types.ValidatorID(id), VotingPower: 1, PublicKey: chain.signer(id).PublicKey()})
	}
	return testValidatorSet(chain.t, validators)
}

// Every block of a lightChain proves the commit of its grandparent, so a block at height h >= 2
// commits the state of h-2 and is certified by the set after its parent's state.
func lightStateHeight(height types.Height) types.Height {
	if height < 2 {
		return 0
	}
	return height - 2
}

func lightSetHeight(height types.Height) types.Height {
	if height <= 2 {
		return FirstValidatorSetHeight
	}
	return lightStateHeight(height-1) + 1
}

func lightBlockTime(height types.Height) int64 {
	return lightGenesisTime.Add(time.Duration(height) * time.Second).UnixNano()
}

// cert is the certificate of the block with header, signed by every member of its set.
func (chain *lightChain) cert(header types.Header) QuorumCert {
	ids := chain.members(header.ValidatorSetHeight)
	slices.Sort(ids)
	signers := make([]types.ValidatorID, 0, len(ids))
	keys := make([]vexocrypto.Signer, 0, len(ids))
	for _, id := range ids {
		signers = append(signers, types.ValidatorID(id))
		keys = append(keys, chain.signer(id))
	}
	cert := QuorumCert{Height: header.Height, Round: header.Round, BlockHash: HeaderHash(header), Signers: EncodeSigners(signers), VotingPower: types.VotingPower(len(ids))}
	cert.Signature = signCert(chain.t, cert, vexocrypto.DeterministicAggregateSigner{}.Aggregate, keys...)
	return cert
}

func (chain *lightChain) header(height types.Height, parent types.Hash) types.Header {
	var justifyRound types.Round
	if height > 1 {
		justifyRound = types.Round(height - 2)
	}
	setHeight := lightSetHeight(height)
	return types.Header{
		ChainID:              "vexo-test",
		Height:               height,
		TimeUnixNano:         lightBlockTime(height),
		PreviousBlockHash:    parent,
		Round:                types.Round(height - 1),
		JustifyRound:         justifyRound,
		ValidatorSetHeight:   setHeight,
		StateHeight:          lightStateHeight(height),
		ValidatorSetHash:     chain.set(setHeight).Hash(),
		NextValidatorSetHash: chain.set(lightStateHeight(height) + 1).Hash(),
	}
}

func (chain *lightChain) proof(height types.Height) Proof {
	header := chain.header(height, types.Hash{byte(height)})
	proof := NewProof(header, chain.cert(header))
	child := chain.header(height+1, proof.BlockHash)
	if child.ValidatorSetHeight != ChildValidatorSetHeight(header) || child.StateHeight != ChildStateHeight(header) {
		chain.t.Fatalf("child of %d names set %d and state %d, the rule fixes %d and %d", height, child.ValidatorSetHeight, child.StateHeight, ChildValidatorSetHeight(header), ChildStateHeight(header))
	}
	return proof.WithChild(child, chain.cert(child))
}

func (chain *lightChain) trusted(height types.Height) TrustedBlock {
	header := chain.header(height, types.Hash{byte(height)})
	return TrustedBlock{Header: header, NextValidatorSet: chain.set(header.StateHeight + 1)}
}

// lightChainSource serves a chain as a full node would, and counts what it serves.
type lightChainSource struct {
	chain   *lightChain
	fetched []types.Height
}

func (source *lightChainSource) LightBlock(_ context.Context, height types.Height) (LightBlock, error) {
	source.fetched = append(source.fetched, height)
	return source.chain.lightBlock(height), nil
}

func (chain *lightChain) lightBlock(height types.Height) LightBlock {
	proof := chain.proof(height)
	return LightBlock{Proof: proof, ValidatorSet: chain.set(proof.Header.ValidatorSetHeight), NextValidatorSet: chain.set(proof.Header.StateHeight + 1)}
}

var lightOptions = LightClientOptions{TrustingPeriod: 24 * time.Hour, MaxClockDrift: 10 * time.Second}

func newTestLightClient(t *testing.T, chain *lightChain, trustedHeight types.Height, source LightSource) *LightClient {
	t.Helper()
	client, err := NewLightClient(chain.trusted(trustedHeight), source, lightOptions, vexocrypto.DeterministicAggregateSigner{})
	if err != nil {
		t.Fatal(err)
	}
	client.now = func() time.Time { return lightGenesisTime.Add(time.Hour) }
	return client
}

func fixedMembers(ids ...string) func(types.Height) []string {
	return func(types.Height) []string { return slices.Clone(ids) }
}

// With an unchanged set, the client skips straight to the target.
func TestLightClientSkipsToTheTargetWhenTheSetIsUnchanged(t *testing.T) {
	chain := newLightChain(t, fixedMembers("a", "b", "c", "d"))
	source := &lightChainSource{chain: chain}
	client := newTestLightClient(t, chain, 2, source)
	trusted, err := client.VerifyTo(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if trusted.Header.Height != 1000 || !slices.Equal(source.fetched, []types.Height{1000}) {
		t.Fatalf("expected one step to block 1000, trusted %d after fetching %v", trusted.Header.Height, source.fetched)
	}
}

// A set that rotates one validator every ten heights shares too little with the trusted set far
// ahead; the client bisects through blocks the trusted set still vouches for and reaches the target.
func TestLightClientBisectsAcrossARotatingSet(t *testing.T) {
	chain := newLightChain(t, func(height types.Height) []string {
		shift := int(height / 10)
		return []string{fmt.Sprint("v", shift), fmt.Sprint("v", shift+1), fmt.Sprint("v", shift+2), fmt.Sprint("v", shift+3)}
	})
	source := &lightChainSource{chain: chain}
	client := newTestLightClient(t, chain, 2, source)
	if _, err := VerifyLightBlock(context.Background(), chain.trusted(2), mustFetch(t, client, 200), client.now(), lightOptions, vexocrypto.DeterministicAggregateSigner{}); !errors.Is(err, ErrNotEnoughTrust) {
		t.Fatalf("expected the trusted set not to vouch for block 200 directly, got %v", err)
	}
	trusted, err := client.VerifyTo(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	if trusted.Header.Height != 200 || trusted.NextValidatorSet.Hash() != chain.set(199).Hash() {
		t.Fatalf("expected to trust block 200 and its next set, got block %d", trusted.Header.Height)
	}
	if len(source.fetched) < 3 {
		t.Fatalf("expected the client to bisect, it fetched %v", source.fetched)
	}
}

func mustFetch(t *testing.T, client *LightClient, height types.Height) LightBlock {
	t.Helper()
	block, err := client.fetch(context.Background(), height)
	if err != nil {
		t.Fatal(err)
	}
	return block
}

// A source that answers with another chain, signed by a quorum of validators the client does not
// trust, or by trusted validator names under other keys, is refused however valid the proof is.
func TestLightClientRefusesBlocksTheTrustedSetDoesNotVouchFor(t *testing.T) {
	honest := newLightChain(t, fixedMembers("a", "b", "c", "d"))
	trusted := honest.trusted(2)
	now := lightGenesisTime.Add(time.Hour)
	verify := func(block LightBlock) error {
		_, err := VerifyLightBlock(context.Background(), trusted, block, now, lightOptions, vexocrypto.DeterministicAggregateSigner{})
		return err
	}

	strangers := newLightChain(t, fixedMembers("w", "x", "y", "z"))
	if err := verify(strangers.lightBlock(50)); !errors.Is(err, ErrNotEnoughTrust) {
		t.Fatalf("expected a chain of strangers to be refused, got %v", err)
	}
	impostors := newLightChain(t, fixedMembers("a", "b", "c", "d"))
	impostors.key = func(id string) string { return "stolen-name-" + id }
	if err := verify(impostors.lightBlock(50)); !errors.Is(err, ErrNotEnoughTrust) {
		t.Fatalf("expected trusted names under other keys to be refused, got %v", err)
	}
	// One trusted validator is not more than a third of four.
	mostlyStrangers := newLightChain(t, fixedMembers("a", "x", "y", "z"))
	if err := verify(mostlyStrangers.lightBlock(50)); !errors.Is(err, ErrNotEnoughTrust) {
		t.Fatalf("expected a quarter of the trusted power to be refused, got %v", err)
	}
	block := honest.lightBlock(50)
	if err := verify(block); err != nil {
		t.Fatalf("expected the honest block to verify, got %v", err)
	}
	// The child's set is the next set the block names: a child certified by strangers, even with
	// a next set the source swaps in to match, breaks the binding the block's quorum signed.
	forgedChild := honest.lightBlock(50)
	fakeSet := newLightChain(t, fixedMembers("w", "x", "y", "z"))
	childHeader := forgedChild.Proof.CommitChain[0].Header
	childHeader.ValidatorSetHash = fakeSet.set(childHeader.ValidatorSetHeight).Hash()
	forgedChild.Proof = NewProof(forgedChild.Proof.Header, forgedChild.Proof.QuorumCert).WithChild(childHeader, fakeSet.cert(childHeader))
	forgedChild.NextValidatorSet = fakeSet.set(childHeader.ValidatorSetHeight)
	if err := verify(forgedChild); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected a child certified by strangers to be refused, got %v", err)
	}
	// Strangers cannot fake the block right after the trusted one either: it must be signed by the
	// set the trusted header names.
	if err := verify(strangers.lightBlock(3)); !errors.Is(err, ErrNotEnoughTrust) {
		t.Fatalf("expected strangers' next block to be refused, got %v", err)
	}
	// A set other than the one the header names is refused before any trust is counted.
	wrongSet := honest.lightBlock(50)
	wrongSet.ValidatorSet = strangers.set(49)
	if err := verify(wrongSet); !errors.Is(err, ErrValidatorSetMismatch) {
		t.Fatalf("expected a set the header does not name to be refused, got %v", err)
	}
}

// The whole set is replaced at once, twice. No later block shares a validator with the trusted
// set, yet every header names the set of its children, so the client walks across each
// replacement with full quorum and reaches the target.
func TestLightClientCrossesAFullSetReplacement(t *testing.T) {
	chain := newLightChain(t, func(height types.Height) []string {
		switch {
		case height < 30:
			return []string{"a", "b", "c", "d"}
		case height < 70:
			return []string{"w", "x", "y", "z"}
		default:
			return []string{"p", "q", "r", "s", "t"}
		}
	})
	source := &lightChainSource{chain: chain}
	client := newTestLightClient(t, chain, 2, source)
	trusted, err := client.VerifyTo(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if trusted.Header.Height != 100 || trusted.NextValidatorSet.Hash() != chain.set(99).Hash() {
		t.Fatalf("expected to trust block 100 and the last set, trusts %d", trusted.Header.Height)
	}
	t.Logf("crossed two full set replacements with %d light blocks", len(source.fetched))
}

func TestLightClientRefusesExpiredTrustAndBlocksFromTheFuture(t *testing.T) {
	chain := newLightChain(t, fixedMembers("a", "b", "c", "d"))
	trusted := chain.trusted(2)
	target := mustFetch(t, newTestLightClient(t, chain, 2, &lightChainSource{chain: chain}), 50)
	signatures := vexocrypto.DeterministicAggregateSigner{}

	expired := time.Unix(0, trusted.Header.TimeUnixNano).Add(lightOptions.TrustingPeriod)
	if _, err := VerifyLightBlock(context.Background(), trusted, target, expired, lightOptions, signatures); !errors.Is(err, ErrTrustExpired) {
		t.Fatalf("expected trust to expire after the trusting period, got %v", err)
	}
	early := time.Unix(0, target.Proof.Header.TimeUnixNano).Add(-lightOptions.MaxClockDrift - time.Nanosecond)
	if _, err := VerifyLightBlock(context.Background(), trusted, target, early, lightOptions, signatures); !errors.Is(err, ErrBlockFromTheFuture) {
		t.Fatalf("expected a block beyond the clock drift to be refused, got %v", err)
	}
	if _, err := VerifyLightBlock(context.Background(), trusted, target, early.Add(time.Nanosecond), lightOptions, signatures); err != nil {
		t.Fatalf("expected a block within the clock drift to verify, got %v", err)
	}
	if _, err := VerifyLightBlock(context.Background(), chain.trusted(60), target, early, lightOptions, signatures); !errors.Is(err, ErrBlockNotAfterTrust) {
		t.Fatalf("expected a block below the trusted one to be refused, got %v", err)
	}
}

func TestMoreThanOneThird(t *testing.T) {
	for _, test := range []struct {
		power, total types.VotingPower
		want         bool
	}{{1, 3, false}, {2, 3, true}, {1, 4, false}, {2, 4, true}, {1, 2, true}, {0, 0, false}, {^types.VotingPower(0)/3 + 1, ^types.VotingPower(0), true}, {^types.VotingPower(0) / 3, ^types.VotingPower(0), false}} {
		if got := moreThanOneThird(test.power, test.total); got != test.want {
			t.Fatalf("moreThanOneThird(%d, %d) = %v", test.power, test.total, got)
		}
	}
}
