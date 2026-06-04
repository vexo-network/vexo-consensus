package finality

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

var (
	ErrNoFinalityConflict     = errors.New("no finality conflict")
	ErrFinalitySetUnavailable = errors.New("finality validator set is unavailable")
)

type proofVerifier interface {
	VerifyFinalityProofWithContext(ctx context.Context, proof Proof) error
}

type validatorSetProvider interface {
	ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error)
}

type staticValidatorSetProvider struct {
	validatorSet validator.Set
}

func (provider staticValidatorSetProvider) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if provider.validatorSet == nil {
		return nil, ErrFinalitySetUnavailable
	}
	return provider.validatorSet, nil
}

type AccountableSafetyViolation struct {
	ChainID               string
	Height                types.Height
	ValidatorSetHeight    types.Height
	ValidatorSetHash      types.Hash
	FirstBlockHash        types.Hash
	SecondBlockHash       types.Hash
	FirstAppHash          types.Hash
	SecondAppHash         types.Hash
	FirstRound            types.Round
	SecondRound           types.Round
	FirstSigners          []types.ValidatorID
	SecondSigners         []types.ValidatorID
	DoubleSigners         []types.ValidatorID
	DoubleSignVotingPower types.VotingPower
	TotalVotingPower      types.VotingPower
	FaultPowerThreshold   types.VotingPower
}

func (violation AccountableSafetyViolation) MeetsFaultThreshold() bool {
	return violation.DoubleSignVotingPower >= violation.FaultPowerThreshold && violation.FaultPowerThreshold > 0
}

type AttackDetector struct {
	verifier proofVerifier
	sets     validatorSetProvider
	proofs   map[types.Height]Proof
}

func NewAttackDetector(validatorSet validator.Set, signatures SignatureVerifier) *AttackDetector {
	return &AttackDetector{
		verifier: NewVerifier(validatorSet, signatures),
		sets:     staticValidatorSetProvider{validatorSet: validatorSet},
		proofs:   make(map[types.Height]Proof),
	}
}

func NewRegistryAttackDetector(registry validator.Registry, signatures SignatureVerifier) *AttackDetector {
	return &AttackDetector{
		verifier: NewRegistryVerifier(registry, signatures),
		sets:     registry,
		proofs:   make(map[types.Height]Proof),
	}
}

func (detector *AttackDetector) Observe(proof Proof) (*AccountableSafetyViolation, error) {
	return detector.ObserveWithContext(context.Background(), proof)
}

func (detector *AttackDetector) ObserveWithContext(ctx context.Context, proof Proof) (*AccountableSafetyViolation, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if detector == nil || detector.verifier == nil || detector.sets == nil {
		return nil, ErrFinalitySetUnavailable
	}
	if err := detector.verifier.VerifyFinalityProofWithContext(ctx, proof); err != nil {
		return nil, err
	}
	previous, found := detector.proofs[proof.Header.Height]
	if !found {
		detector.proofs[proof.Header.Height] = proof
		return nil, nil
	}
	violation, err := DetectAccountableSafetyViolationWithContext(ctx, detector.sets, previous, proof)
	if errors.Is(err, ErrNoFinalityConflict) {
		return nil, nil
	}
	return violation, err
}

func DetectAccountableSafetyViolation(validatorSet validator.Set, first Proof, second Proof) (*AccountableSafetyViolation, error) {
	return DetectAccountableSafetyViolationWithContext(context.Background(), staticValidatorSetProvider{validatorSet: validatorSet}, first, second)
}

func DetectAccountableSafetyViolationWithContext(ctx context.Context, sets validatorSetProvider, first Proof, second Proof) (*AccountableSafetyViolation, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if first.Header.Height != second.Header.Height {
		return nil, ErrHeightMismatch
	}
	firstBlockHash := proofBlockHash(first)
	secondBlockHash := proofBlockHash(second)
	if firstBlockHash == secondBlockHash {
		return nil, ErrNoFinalityConflict
	}
	if first.ValidatorSetHeight == 0 || second.ValidatorSetHeight == 0 || first.ValidatorSetHeight != second.ValidatorSetHeight {
		return nil, ErrHeightMismatch
	}
	if first.ValidatorSetHash != second.ValidatorSetHash || first.Header.ValidatorSetHash != second.Header.ValidatorSetHash {
		return nil, ErrValidatorSetMismatch
	}
	if sets == nil {
		return nil, ErrFinalitySetUnavailable
	}
	validatorSet, err := sets.ValidatorSet(ctx, first.ValidatorSetHeight)
	if err != nil {
		return nil, err
	}
	if validatorSet == nil {
		return nil, ErrFinalitySetUnavailable
	}
	if first.ValidatorSetHash != validatorSet.Hash() {
		return nil, ErrValidatorSetMismatch
	}
	firstSigners, err := ParseSigners(first.QuorumCert.Signers)
	if err != nil {
		return nil, err
	}
	secondSigners, err := ParseSigners(second.QuorumCert.Signers)
	if err != nil {
		return nil, err
	}
	doubleSigners, doubleSignPower, err := overlappingVotingPower(validatorSet, firstSigners, secondSigners)
	if err != nil {
		return nil, err
	}
	return &AccountableSafetyViolation{
		ChainID:               first.Header.ChainID,
		Height:                first.Header.Height,
		ValidatorSetHeight:    first.ValidatorSetHeight,
		ValidatorSetHash:      first.ValidatorSetHash,
		FirstBlockHash:        firstBlockHash,
		SecondBlockHash:       secondBlockHash,
		FirstAppHash:          first.Header.AppHash,
		SecondAppHash:         second.Header.AppHash,
		FirstRound:            first.QuorumCert.Round,
		SecondRound:           second.QuorumCert.Round,
		FirstSigners:          sortedSigners(firstSigners),
		SecondSigners:         sortedSigners(secondSigners),
		DoubleSigners:         doubleSigners,
		DoubleSignVotingPower: doubleSignPower,
		TotalVotingPower:      validatorSet.TotalVotingPower(),
		FaultPowerThreshold:   faultPowerThreshold(validatorSet.TotalVotingPower()),
	}, nil
}

func proofBlockHash(proof Proof) types.Hash {
	if proof.BlockHash != (types.Hash{}) {
		return proof.BlockHash
	}
	if proof.QuorumCert.BlockHash != (types.Hash{}) {
		return proof.QuorumCert.BlockHash
	}
	return proof.HeaderHash()
}

func overlappingVotingPower(validatorSet validator.Set, firstSigners []types.ValidatorID, secondSigners []types.ValidatorID) ([]types.ValidatorID, types.VotingPower, error) {
	secondSet := make(map[types.ValidatorID]struct{}, len(secondSigners))
	for _, signer := range secondSigners {
		secondSet[signer] = struct{}{}
	}
	seen := make(map[types.ValidatorID]struct{}, len(firstSigners))
	overlapping := make([]types.ValidatorID, 0)
	var power types.VotingPower
	for _, signer := range firstSigners {
		if _, duplicate := seen[signer]; duplicate {
			continue
		}
		seen[signer] = struct{}{}
		if _, found := secondSet[signer]; !found {
			continue
		}
		validatorInfo, found := validatorSet.Get(signer)
		if !found {
			return nil, 0, ErrUnknownSigner
		}
		overlapping = append(overlapping, signer)
		power += validatorInfo.VotingPower
	}
	sort.Slice(overlapping, func(left int, right int) bool {
		return overlapping[left] < overlapping[right]
	})
	return overlapping, power, nil
}

func sortedSigners(signers []types.ValidatorID) []types.ValidatorID {
	copied := append([]types.ValidatorID(nil), signers...)
	sort.Slice(copied, func(left int, right int) bool {
		return copied[left] < copied[right]
	})
	return copied
}

func faultPowerThreshold(total types.VotingPower) types.VotingPower {
	if total == 0 {
		return 0
	}
	threshold := total / 3
	if total%3 != 0 {
		threshold++
	}
	return threshold
}
