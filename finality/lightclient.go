package finality

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrTrustExpired        = errors.New("trusted block is older than the trusting period")
	ErrNotEnoughTrust      = errors.New("less than one third of the trusted validator set signed the block")
	ErrBlockNotAfterTrust  = errors.New("block does not follow the trusted block")
	ErrBlockFromTheFuture  = errors.New("block time is ahead of the local clock")
	ErrLightClientOptions  = errors.New("invalid light client options")
	ErrLightBlockSetHeight = errors.New("light block validator set is not the one its header names")
)

// LightClientOptions are the trust assumptions of a light client.
//
// TrustingPeriod is how long the validators of a trusted block are assumed to keep less than one
// third of its voting power faulty. It must be shorter than the time a validator stays slashable
// after it leaves the set, so that the validators who could sign a fake block are still
// accountable for it. MaxClockDrift is how far ahead of the local clock a block time may be.
type LightClientOptions struct {
	TrustingPeriod time.Duration
	MaxClockDrift  time.Duration
}

func (options LightClientOptions) validate() error {
	if options.TrustingPeriod <= 0 {
		return fmt.Errorf("%w: trusting period must be positive", ErrLightClientOptions)
	}
	if options.MaxClockDrift < 0 {
		return fmt.Errorf("%w: clock drift must not be negative", ErrLightClientOptions)
	}
	return nil
}

// TrustedBlock is a final block a light client trusts, with the validator set its header names as
// next: the set that certifies its children, which the client measures later blocks against.
type TrustedBlock struct {
	Header           types.Header
	NextValidatorSet validator.Set
}

// NewTrustedBlock returns header as a trusted block, with its next validator set, which must be the
// set the header names.
func NewTrustedBlock(header types.Header, nextValidatorSet validator.Set) (TrustedBlock, error) {
	if nextValidatorSet == nil || nextValidatorSet.Hash() != header.NextValidatorSetHash {
		return TrustedBlock{}, fmt.Errorf("%w: next validator set of block %d", ErrValidatorSetMismatch, header.Height)
	}
	return TrustedBlock{Header: header, NextValidatorSet: nextValidatorSet}, nil
}

// LightBlock is what a light client fetches to move its trust to a later block: the finality proof
// of the block, the validator set its header names, and the next set its header names, which
// certifies its commit child.
type LightBlock struct {
	Proof            Proof
	ValidatorSet     validator.Set
	NextValidatorSet validator.Set
}

// ValidatorKeyRegistrar is implemented by signature verifiers that verify only registered keys, as
// BLS does with proofs of possession: a light client registers the sets it fetched before it
// verifies a certificate with them, and a set whose keys do not register is refused.
type ValidatorKeyRegistrar interface {
	RegisterValidators(validators []validator.Validator) error
}

// LightSource serves light blocks from an untrusted full node. Nothing it serves is trusted: every
// light block is verified against the client's trusted block.
type LightSource interface {
	LightBlock(ctx context.Context, height types.Height) (LightBlock, error)
}

// VerifyLightBlock moves trust from trusted to the block of target, and returns the new trusted
// block. It is skipping verification under the 2-chain commit rule:
//
//   - The trusted block is within the trusting period, and the target block follows it in height
//     and time, and is not ahead of the local clock by more than the drift.
//   - The target is final: its proof verifies with the set its header names and, for its commit
//     child, the next set it names (the 2-chain rule of Verifier). Each fetched set hashes to what
//     the header names.
//   - More than one third of the trusted block's next set's voting power, with the keys that set
//     holds, signed QC(B) of the target. Under the trust assumption one of them is honest, and an
//     honest validator votes only for a header naming the sets its own replicated state holds: the
//     target's sets are genuine, so its quorum, and the next set it names, are the chain's.
//
// A header commits the set of its children (types.Header.NextValidatorSetHash), so the block right
// after the trusted one is certified by the trusted next set itself and always passes: whatever
// the set changes, a client that cannot skip reaches the target by bisection (LightClient.VerifyTo).
func VerifyLightBlock(ctx context.Context, trusted TrustedBlock, target LightBlock, now time.Time, options LightClientOptions, signatures SignatureVerifier) (TrustedBlock, error) {
	if err := options.validate(); err != nil {
		return TrustedBlock{}, err
	}
	if trusted.NextValidatorSet == nil || target.ValidatorSet == nil || target.NextValidatorSet == nil {
		return TrustedBlock{}, ErrValidatorSetMismatch
	}
	if trusted.NextValidatorSet.Hash() != trusted.Header.NextValidatorSetHash {
		return TrustedBlock{}, ErrValidatorSetMismatch
	}
	trustedTime := time.Unix(0, trusted.Header.TimeUnixNano)
	if !now.Before(trustedTime.Add(options.TrustingPeriod)) {
		return TrustedBlock{}, fmt.Errorf("%w: trusted block %d at %s, period %s", ErrTrustExpired, trusted.Header.Height, trustedTime.UTC().Format(time.RFC3339), options.TrustingPeriod)
	}
	header := target.Proof.Header
	if header.ChainID != trusted.Header.ChainID || header.Height <= trusted.Header.Height || header.TimeUnixNano <= trusted.Header.TimeUnixNano {
		return TrustedBlock{}, fmt.Errorf("%w: block %d at %d, trusted block %d at %d", ErrBlockNotAfterTrust, header.Height, header.TimeUnixNano, trusted.Header.Height, trusted.Header.TimeUnixNano)
	}
	if header.TimeUnixNano > now.Add(options.MaxClockDrift).UnixNano() {
		return TrustedBlock{}, fmt.Errorf("%w: block %d", ErrBlockFromTheFuture, header.Height)
	}
	child, err := checkProofStructure(target.Proof)
	if err != nil {
		return TrustedBlock{}, err
	}
	if target.NextValidatorSet.Hash() != header.NextValidatorSetHash {
		return TrustedBlock{}, ErrValidatorSetMismatch
	}
	sets := lightBlockSets{
		header.ValidatorSetHeight:       target.ValidatorSet,
		child.Header.ValidatorSetHeight: target.NextValidatorSet,
	}
	if len(sets) == 1 && target.ValidatorSet.Hash() != target.NextValidatorSet.Hash() {
		return TrustedBlock{}, ErrLightBlockSetHeight
	}
	if registrar, ok := signatures.(ValidatorKeyRegistrar); ok {
		for _, set := range sets {
			if err := registrar.RegisterValidators(set.List()); err != nil {
				return TrustedBlock{}, fmt.Errorf("validator keys of block %d: %w", header.Height, err)
			}
		}
	}
	if err := NewRegistryVerifier(sets, signatures).VerifyFinalityProofWithContext(ctx, target.Proof); err != nil {
		return TrustedBlock{}, err
	}
	if err := checkTrustedSigners(trusted.NextValidatorSet, target.ValidatorSet, target.Proof.QuorumCert); err != nil {
		return TrustedBlock{}, err
	}
	return TrustedBlock{Header: header, NextValidatorSet: target.NextValidatorSet}, nil
}

// lightBlockSets serves the two sets of a light block by the registry heights its headers name.
type lightBlockSets map[types.Height]validator.Set

func (sets lightBlockSets) ValidatorSet(_ context.Context, height types.Height) (validator.Set, error) {
	set, found := sets[height]
	if !found {
		return nil, fmt.Errorf("%w: no set for registry height %d", ErrLightBlockSetHeight, height)
	}
	return set, nil
}

// checkTrustedSigners requires the signers of cert, a certificate of the set signing, that trusted
// holds with the key they signed with to have more than one third of trusted's voting power. The
// certificate's signature is verified separately, with the keys of signing.
func checkTrustedSigners(trusted validator.Set, signing validator.Set, cert QuorumCert) error {
	signers, err := ParseSigners(cert.Signers)
	if err != nil {
		return err
	}
	var power types.VotingPower
	for _, signer := range signers {
		known, found := trusted.Get(signer)
		if !found {
			continue
		}
		signed, found := signing.Get(signer)
		if !found || !bytes.Equal(known.PublicKey, signed.PublicKey) {
			continue
		}
		power = types.MustAddVotingPowerSaturating(power, known.VotingPower)
	}
	if !moreThanOneThird(power, trusted.TotalVotingPower()) {
		return fmt.Errorf("%w: %d of %d", ErrNotEnoughTrust, power, trusted.TotalVotingPower())
	}
	return nil
}

// moreThanOneThird reports whether 3*power > total, without overflow: with total = 3q+r and r < 3,
// that holds exactly when power > q.
func moreThanOneThird(power types.VotingPower, total types.VotingPower) bool {
	return total > 0 && power > total/3
}

// LightClient keeps a trusted block and moves it forward with blocks from an untrusted source.
type LightClient struct {
	trusted    TrustedBlock
	source     LightSource
	options    LightClientOptions
	signatures SignatureVerifier
	now        func() time.Time
}

// NewLightClient returns a client that trusts trusted, a final block learned outside the source.
func NewLightClient(trusted TrustedBlock, source LightSource, options LightClientOptions, signatures SignatureVerifier) (*LightClient, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	if source == nil || signatures == nil {
		return nil, ErrMissingSignatureVerifier
	}
	if trusted.NextValidatorSet == nil || trusted.NextValidatorSet.Hash() != trusted.Header.NextValidatorSetHash {
		return nil, ErrValidatorSetMismatch
	}
	return &LightClient{trusted: trusted, source: source, options: options, signatures: signatures, now: time.Now}, nil
}

// Trusted returns the block the client trusts.
func (client *LightClient) Trusted() TrustedBlock {
	return client.trusted
}

// VerifyTo moves the client's trust to the final block at height and returns it. It tries the
// target first and, while the trusted set does not vouch for a block, bisects: it trusts the
// block halfway first, which needs less overlap with the trusted set, and retries the target from
// there. The block right after a trusted one is certified by the set the trusted header names, so
// against an honest source bisection always ends; any other failure, a source serving a block
// that does not verify included, stops it.
func (client *LightClient) VerifyTo(ctx context.Context, height types.Height) (TrustedBlock, error) {
	if height <= client.trusted.Header.Height {
		return TrustedBlock{}, fmt.Errorf("%w: height %d, trusted block %d", ErrBlockNotAfterTrust, height, client.trusted.Header.Height)
	}
	pivots := []types.Height{height}
	for len(pivots) > 0 {
		pivot := pivots[len(pivots)-1]
		target, err := client.fetch(ctx, pivot)
		if err != nil {
			return TrustedBlock{}, err
		}
		next, err := VerifyLightBlock(ctx, client.trusted, target, client.now(), client.options, client.signatures)
		if errors.Is(err, ErrNotEnoughTrust) {
			middle := client.trusted.Header.Height + (pivot-client.trusted.Header.Height)/2
			if middle == client.trusted.Header.Height {
				return TrustedBlock{}, fmt.Errorf("block %d directly after the trusted block: %w", pivot, err)
			}
			pivots = append(pivots, middle)
			continue
		}
		if err != nil {
			return TrustedBlock{}, fmt.Errorf("block %d: %w", pivot, err)
		}
		client.trusted = next
		pivots = pivots[:len(pivots)-1]
	}
	return client.trusted, nil
}

// fetch loads the light block at height from the source.
func (client *LightClient) fetch(ctx context.Context, height types.Height) (LightBlock, error) {
	block, err := client.source.LightBlock(ctx, height)
	if err != nil {
		return LightBlock{}, fmt.Errorf("light block %d: %w", height, err)
	}
	if block.Proof.Header.Height != height {
		return LightBlock{}, fmt.Errorf("%w: source served block %d for height %d", ErrHeightMismatch, block.Proof.Header.Height, height)
	}
	return block, nil
}
