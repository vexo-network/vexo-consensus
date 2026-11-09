// Package slashing is the application module that punishes accountable faults.
//
// Evidence reaches the chain as a transaction, `slashing:evidence:<payload>`, and the penalty is
// applied only while the block that carries it executes: the module re-verifies the proof against
// the validator set in effect at the evidence height, punishes the validator through the staking
// module's SlashValidator and reports the validator's new standing as a validator update, which the
// runtime applies at the next height. Gossip and RPC never change state; they verify evidence and
// queue it.
//
// Everything a node needs to judge evidence is part of the replicated state: the evidence itself,
// the validator-set snapshot of its height and the offence markers the module writes. Evidence
// whose truth depends on node-local data is therefore not accepted (see consensus.VerifyEvidence).
package slashing

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	vexoslashing "github.com/vexo-network/vexo-consensus/slashing"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/txcodec"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

const ModuleName = "slashing"

// ActionEvidence is the only transaction action: `slashing:evidence:<payload>`.
const ActionEvidence = "evidence"

const (
	// evidenceBaseGasCost and evidenceGasBytes meter an evidence transaction: verifying a proof costs
	// time in proportion to its size.
	evidenceBaseGasCost uint64 = 300
	evidenceGasBytes    uint64 = 32

	// MaxProofBytes bounds the proof of one piece of evidence.
	MaxProofBytes = 512 * 1024
)

// Result codes of evidence transactions. A transaction that fails because of its content returns
// one of them; nothing in this module panics or aborts the block for transaction content.
const (
	codeNoStore      uint32 = 1
	codeInvalidTx    uint32 = 2
	codeRejected     uint32 = 3
	codeOutOfGas     uint32 = 4
	codeApplyFailure uint32 = 5
)

var (
	ErrInvalidEvidenceTx      = errors.New("invalid evidence transaction")
	ErrEvidenceTooLarge       = errors.New("evidence is too large")
	ErrEvidenceFromFuture     = errors.New("evidence is from a height that has not been reached")
	ErrEvidenceExpired        = errors.New("evidence is too old to punish")
	ErrEvidenceAlreadyApplied = errors.New("offence was already punished")
	ErrSlashingStoreRequired  = errors.New("missing slashing store")
	ErrSlashingPolicyInvalid  = errors.New("invalid slashing policy")
	ErrValidatorSetMissing    = errors.New("validator set of the evidence height is unavailable")
	errMarkerCorrupt          = errors.New("corrupt offence marker")
)

// Policy configures the module. The zero value selects the defaults.
type Policy struct {
	// Penalties maps an evidence type to its punishment; nil selects DefaultPenaltyPolicy.
	Penalties vexoslashing.PenaltyPolicy
	// Lifecycle bounds how old evidence may be; a zero EvidenceMaxAge selects the default.
	Lifecycle vexoslashing.LifecyclePolicy
}

type Module struct {
	policy Policy
	suite  crypto.RuntimeSuite

	mu sync.Mutex
	// slashed holds the offences punished in the current block, in execution order.
	slashed []appliedSlash
}

// appliedSlash is what the module needs to report the standing of a punished validator at the end
// of the block.
type appliedSlash struct {
	offence   string
	validator types.ValidatorID
	infracted types.Height
	slashBPS  uint64
	jailUntil types.Height
}

// NewModule returns a module with the default policy and the deterministic crypto suite.
func NewModule() *Module {
	suite, err := crypto.NewRuntimeSuite(config.CryptoConfig{Backend: config.CryptoBackendDeterministic})
	if err != nil {
		// The deterministic backend has no failure mode; reaching this is a programming error.
		panic(err)
	}
	module, err := NewModuleWithPolicy(Policy{}, suite)
	if err != nil {
		panic(err)
	}
	return module
}

// NewModuleWithPolicy returns a module that verifies proofs with suite.
func NewModuleWithPolicy(policy Policy, suite crypto.RuntimeSuite) (*Module, error) {
	if policy.Penalties == nil {
		policy.Penalties = vexoslashing.DefaultPenaltyPolicy()
	}
	if err := policy.Penalties.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSlashingPolicyInvalid, err)
	}
	if policy.Lifecycle.EvidenceMaxAge == 0 {
		policy.Lifecycle.EvidenceMaxAge = vexoslashing.DefaultLifecyclePolicy().EvidenceMaxAge
	}
	if suite.ConsensusVerifier == nil || suite.FinalityVerifier == nil {
		return nil, fmt.Errorf("%w: crypto suite is required", ErrSlashingPolicyInvalid)
	}
	return &Module{policy: policy, suite: suite}, nil
}

func (module *Module) CloneModule() vexoapp.Module {
	return &Module{policy: module.policy, suite: module.suite}
}

func (module *Module) Name() string {
	return ModuleName
}

func (module *Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	return nil
}

// BeginBlock records the app hash the header of the executing block commits, which a quorum
// certified, for app-hash evidence to be judged against (see recordCertifiedAppHash).
func (module *Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	module.mu.Lock()
	module.slashed = nil
	module.mu.Unlock()
	if ctx.Store == nil {
		return nil
	}
	return module.recordCertifiedAppHash(ctx.GoContext(), ctx.Store, header)
}

func (module *Module) EndBlock(ctx vexoapp.Context) error {
	return nil
}

// EncodeEvidenceTx returns the transaction payload that carries evidence. The payload is unsigned
// and fee-less: the ante handler admits it through AuthenticateTx, because the evidence itself is
// signed by the offender. The proof is encoded without padding so the payload
// contains neither ':' nor '=', which the canonical transaction grammar reserves.
//
// This is the only spelling of the evidence that DecodeEvidenceTx accepts.
func EncodeEvidenceTx(evidence vexoslashing.Evidence) (types.Tx, error) {
	argument, err := encodeEvidenceArgument(evidence)
	if err != nil {
		return nil, err
	}
	return types.Tx(ModuleName + ":" + ActionEvidence + ":" + argument), nil
}

// encodeEvidenceArgument is the canonical payload argument of evidence: unpadded URL-safe base64 of
// the JSON document of the normalized evidence.
func encodeEvidenceArgument(evidence vexoslashing.Evidence) (string, error) {
	encoded, err := json.Marshal(evidence.Normalize())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

// DecodeEvidenceTx extracts the evidence of an evidence transaction. It is strict: the payload
// argument must be canonical base64 of one JSON document without unknown fields or trailing data,
// carrying no more than MaxProofBytes of proof, and it must be byte for byte the argument
// EncodeEvidenceTx writes for that evidence. Evidence has no other spelling: JSON whitespace, the
// order or the case of fields, line breaks inside base64, the legacy double_sign label and padding
// would each be a different transaction for one offence, and every one of them would cost every
// node a verification. The proof inside the evidence is held to the same standard by the proof
// decoders (see slashing.DecodeProof).
//
// A reporter that signs the transaction appends its signer, fee and nonce tags after the argument;
// they are no part of the evidence and the ante handler owns them.
func DecodeEvidenceTx(tx types.Tx) (vexoslashing.Evidence, error) {
	if len(tx) > maxEvidenceTxBytes {
		return vexoslashing.Evidence{}, ErrEvidenceTooLarge
	}
	argument, err := evidenceArgument(tx)
	if err != nil {
		return vexoslashing.Evidence{}, err
	}
	if base64.RawURLEncoding.DecodedLen(len(argument)) > base64.StdEncoding.EncodedLen(MaxProofBytes)+maxEnvelopeOverhead {
		return vexoslashing.Evidence{}, ErrEvidenceTooLarge
	}
	document, err := base64.RawURLEncoding.Strict().DecodeString(argument)
	if err != nil {
		return vexoslashing.Evidence{}, fmt.Errorf("%w: payload is not base64", ErrInvalidEvidenceTx)
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	var evidence vexoslashing.Evidence
	if err := decoder.Decode(&evidence); err != nil {
		return vexoslashing.Evidence{}, fmt.Errorf("%w: %v", ErrInvalidEvidenceTx, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return vexoslashing.Evidence{}, fmt.Errorf("%w: trailing data", ErrInvalidEvidenceTx)
	}
	canonical, err := encodeEvidenceArgument(evidence)
	if err != nil {
		return vexoslashing.Evidence{}, fmt.Errorf("%w: %v", ErrInvalidEvidenceTx, err)
	}
	if argument != canonical {
		return vexoslashing.Evidence{}, fmt.Errorf("%w: payload is not in canonical form", ErrInvalidEvidenceTx)
	}
	return evidence, nil
}

// maxEnvelopeOverhead covers the JSON field names around the proof, which the JSON document carries
// as standard base64, so a proof of exactly the limit still fits.
const maxEnvelopeOverhead = 4096

// maxEvidenceTxBytes bounds a whole evidence transaction before it is parsed. The proof travels as
// base64 inside a JSON document that is itself base64 encoded, which is below twice its size.
const maxEvidenceTxBytes = 2 * (MaxProofBytes + maxEnvelopeOverhead)

// evidenceArgument returns the single payload argument of an evidence transaction.
func evidenceArgument(tx types.Tx) (string, error) {
	canonical, err := txcodec.ParseCanonical(tx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidEvidenceTx, err)
	}
	if canonical.Module != ModuleName || canonical.Action != ActionEvidence || len(canonical.Args) != 1 || canonical.Args[0] == "" {
		return "", ErrInvalidEvidenceTx
	}
	return canonical.Args[0], nil
}

func evidenceGas(argument string) uint64 {
	return evidenceBaseGasCost + uint64(len(argument))/evidenceGasBytes
}

func (module *Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	argument, err := evidenceArgument(tx)
	if err != nil {
		return 0, err
	}
	return evidenceGas(argument), nil
}

// AuthenticateTx lets the ante handler admit an unsigned, fee-less evidence
// transaction: the evidence is authenticated by the offender's own signatures,
// which ValidateTx verifies against the validator set.
func (module *Module) AuthenticateTx(ctx vexoapp.Context, tx types.Tx) error {
	return module.ValidateTx(ctx, tx)
}

// ValidateTx admits an evidence transaction only if its evidence verifies against the state of ctx
// and the offence is not yet punished.
func (module *Module) ValidateTx(ctx vexoapp.Context, tx types.Tx) error {
	if ctx.Store == nil {
		return ErrSlashingStoreRequired
	}
	evidence, err := DecodeEvidenceTx(tx)
	if err != nil {
		return err
	}
	_, err = module.CheckEvidence(ctx.GoContext(), ctx.Store, ctx.Height, evidence)
	return err
}

// ExclusiveTxKey is the offence the evidence of an evidence transaction proves: a block carries at
// most one evidence transaction per offence (see vexoapp.BlockExclusiveTx). Block validation judges
// every transaction against the committed state, in which two proofs of one offence are both valid
// although only the first takes effect; the others would be failed transactions that cost every
// node a verification and a place in the block. The runtime asks only for a transaction that passed
// ValidateTx, whose verification bound the validator, height and round of the evidence to its proof,
// so the key is the offence the proof shows.
func (module *Module) ExclusiveTxKey(tx types.Tx) (string, error) {
	evidence, err := DecodeEvidenceTx(tx)
	if err != nil {
		return "", err
	}
	return vexoslashing.OffenceKey(evidence)
}

func (module *Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return failure(codeNoStore, ErrSlashingStoreRequired)
	}
	argument, err := evidenceArgument(tx)
	if err != nil {
		return failure(codeInvalidTx, err)
	}
	if err := ctx.ConsumeGas(evidenceGas(argument)); err != nil {
		return failure(codeOutOfGas, err)
	}
	evidence, err := DecodeEvidenceTx(tx)
	if err != nil {
		return failure(codeInvalidTx, err)
	}
	checked, err := module.verify(ctx.GoContext(), ctx.Store, ctx.Height, evidence)
	if err != nil {
		return failure(codeRejected, err)
	}
	slash := appliedSlash{
		offence:   checked.offence,
		validator: checked.evidence.Validator,
		infracted: checked.evidence.Height,
		slashBPS:  checked.penalty.SlashBPS,
		jailUntil: jailEnd(ctx.Height, checked.penalty),
	}
	if _, applied, err := staking.SlashValidator(ctx.GoContext(), ctx.Store, slash.offence, slash.validator, slash.infracted, ctx.Height, slash.slashBPS, slash.jailUntil); err != nil {
		return failure(codeApplyFailure, err)
	} else if !applied {
		return failure(codeRejected, ErrEvidenceAlreadyApplied)
	}
	if err := ctx.Store.Set(ctx.GoContext(), ModuleName, markerKey(slash.offence), encodeHeight(ctx.Height)); err != nil {
		return failure(codeApplyFailure, err)
	}
	module.mu.Lock()
	module.slashed = append(module.slashed, slash)
	module.mu.Unlock()
	return types.Result{Data: []byte(slash.offence)}
}

// jailEnd is the height until which the penalty jails a validator punished at height.
func jailEnd(height types.Height, penalty vexoslashing.Penalty) types.Height {
	if penalty.JailDuration == 0 {
		return 0
	}
	end := uint64(height) + penalty.JailDuration
	if end < uint64(height) {
		return types.Height(^uint64(0))
	}
	return types.Height(end)
}

type verifiedEvidence struct {
	evidence vexoslashing.Evidence
	offence  string
	penalty  vexoslashing.Penalty
}

// CheckEvidence verifies evidence exactly as the execution of a block at height does, against the
// state in store, and fails with ErrEvidenceAlreadyApplied when the offence was punished before.
// It reads state and never writes it, so gossip and RPC use it to decide whether evidence is worth
// queueing. The returned evidence is normalized.
func (module *Module) CheckEvidence(ctx context.Context, store vexoapp.StateStore, height types.Height, evidence vexoslashing.Evidence) (vexoslashing.Evidence, error) {
	checked, err := module.verify(ctx, store, height, evidence)
	if err != nil {
		return vexoslashing.Evidence{}, err
	}
	return checked.evidence, nil
}

// verify is the single judgement of evidence, shared by admission and execution so that the two
// cannot disagree. Everything it reads is replicated state.
func (module *Module) verify(ctx context.Context, store vexoapp.StateStore, height types.Height, evidence vexoslashing.Evidence) (verifiedEvidence, error) {
	if store == nil {
		return verifiedEvidence{}, ErrSlashingStoreRequired
	}
	evidence = evidence.Normalize()
	if err := evidence.Validate(); err != nil {
		return verifiedEvidence{}, err
	}
	if len(evidence.Proof) > MaxProofBytes {
		return verifiedEvidence{}, ErrEvidenceTooLarge
	}
	if evidence.Height > height {
		return verifiedEvidence{}, ErrEvidenceFromFuture
	}
	if uint64(height-evidence.Height) > uint64(module.policy.Lifecycle.EvidenceMaxAge) {
		return verifiedEvidence{}, ErrEvidenceExpired
	}
	penalty, err := module.policy.Penalties.Penalty(evidence.Type)
	if err != nil {
		return verifiedEvidence{}, err
	}
	offence, err := vexoslashing.OffenceKey(evidence)
	if err != nil {
		return verifiedEvidence{}, err
	}
	if applied, err := offenceApplied(ctx, store, offence); err != nil {
		return verifiedEvidence{}, err
	} else if applied {
		return verifiedEvidence{}, ErrEvidenceAlreadyApplied
	}
	set, err := validator.LoadValidatorSet(ctx, store, evidence.Height)
	if err != nil {
		return verifiedEvidence{}, fmt.Errorf("%w: %v", ErrValidatorSetMissing, err)
	}
	if err := module.suite.RegisterValidators(set.List()); err != nil {
		return verifiedEvidence{}, err
	}
	bundle := consensus.NewEvidenceVerifier(module.suite.ConsensusVerifier, module.suite.FinalityVerifier)
	bundle.FinalitySets = storedValidatorSets{store: store, module: module, height: evidence.Height}
	bundle.CertifiedAppHashes = certifiedAppHashes{ctx: ctx, store: store}
	if err := consensus.VerifyEvidence(evidence, set, bundle); err != nil {
		return verifiedEvidence{}, err
	}
	return verifiedEvidence{evidence: evidence, offence: offence, penalty: penalty}, nil
}

// Settled reports whether evidence needs no further handling at height: its offence was already
// punished, it is too old to punish, or it names no offence at all. It reads two values and verifies
// no signature, so it is cheap enough to run for every pooled record after every block.
func (module *Module) Settled(ctx context.Context, store vexoapp.StateStore, height types.Height, evidence vexoslashing.Evidence) (bool, error) {
	if store == nil {
		return false, ErrSlashingStoreRequired
	}
	evidence = evidence.Normalize()
	if evidence.Height < height && uint64(height-evidence.Height) > uint64(module.policy.Lifecycle.EvidenceMaxAge) {
		return true, nil
	}
	offence, err := vexoslashing.OffenceKey(evidence)
	if err != nil {
		return true, nil
	}
	return offenceApplied(ctx, store, offence)
}

func offenceApplied(ctx context.Context, store vexoapp.StateStore, offence string) (bool, error) {
	value, err := store.Get(ctx, ModuleName, markerKey(offence))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(value) != 8 {
		return false, errMarkerCorrupt
	}
	return true, nil
}

// ValidatorUpdates reports the standing of every validator punished in the block, ordered by
// validator id. The update is read from the staking state after the whole block executed, so it
// reflects the slash together with everything else the block did to the validator. The runtime
// applies it at the next height.
func (module *Module) ValidatorUpdates(ctx vexoapp.Context) []types.ValidatorUpdate {
	module.mu.Lock()
	slashed := append([]appliedSlash(nil), module.slashed...)
	module.mu.Unlock()
	if len(slashed) == 0 || ctx.Store == nil {
		return nil
	}
	sort.SliceStable(slashed, func(left int, right int) bool { return slashed[left].validator < slashed[right].validator })
	updates := make([]types.ValidatorUpdate, 0, len(slashed))
	var previous types.ValidatorID
	for _, slash := range slashed {
		if len(updates) > 0 && slash.validator == previous {
			continue
		}
		previous = slash.validator
		// The offence is already marked, so staking changes nothing and reports the current standing.
		update, applied, err := staking.SlashValidator(ctx.GoContext(), ctx.Store, slash.offence, slash.validator, slash.infracted, ctx.Height, slash.slashBPS, slash.jailUntil)
		if err != nil || applied {
			// A read failure here leaves the validator set unchanged for one block; the staking state
			// already holds the slash and the next update of the validator reports it.
			continue
		}
		updates = append(updates, update)
	}
	return updates
}

func markerKey(offence string) []byte {
	return []byte("offence/" + offence)
}

func encodeHeight(height types.Height) []byte {
	var buffer [8]byte
	binary.BigEndian.PutUint64(buffer[:], uint64(height))
	return buffer[:]
}

func failure(code uint32, err error) types.Result {
	return types.Result{Code: code, Log: err.Error()}
}

var (
	_ vexoapp.Module                  = (*Module)(nil)
	_ vexoapp.ModuleCloner            = (*Module)(nil)
	_ vexoapp.TxValidator             = (*Module)(nil)
	_ vexoapp.BlockExclusiveTx        = (*Module)(nil)
	_ vexoapp.GasEstimator            = (*Module)(nil)
	_ vexoapp.ValidatorUpdateProvider = (*Module)(nil)
)

// storedValidatorSets serves the replicated validator sets the headers of a finality conflict name,
// with their keys admitted to the crypto suite. A header of the evidence height names a registry
// height below it, so a later height is refused.
type storedValidatorSets struct {
	store  vexoapp.StateStore
	module *Module
	height types.Height
}

func (sets storedValidatorSets) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	if height == 0 || height > sets.height {
		return nil, fmt.Errorf("%w: registry height %d for evidence of height %d", ErrValidatorSetMissing, height, sets.height)
	}
	set, err := validator.LoadValidatorSet(ctx, sets.store, height)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidatorSetMissing, err)
	}
	if err := sets.module.suite.RegisterValidators(set.List()); err != nil {
		return nil, err
	}
	return set, nil
}

// Certified app hashes.
//
// Every block header commits the header app hash of the state of finality.AppHashHeight, and a
// quorum certified that header before the block executes. The module records that hash per state
// height, so a proposal that names another app hash for the same height is evidence of an invalid
// proposal every node judges alike, from replicated state. A record is kept while a block that
// named its height is young enough for evidence about it (Lifecycle.EvidenceMaxAge): each block
// also records which state height it named, and the block that falls out of the window drops the
// record unless a later block named the same height.

const (
	certifiedAppHashPrefix = "certified_app_hash/"
	certifiedNamingPrefix  = "certified_app_hash_named/"
)

func certifiedAppHashKey(stateHeight types.Height) []byte {
	return append([]byte(certifiedAppHashPrefix), encodeHeight(stateHeight)...)
}

func certifiedNamingKey(blockHeight types.Height) []byte {
	return append([]byte(certifiedNamingPrefix), encodeHeight(blockHeight)...)
}

// encodeCertifiedAppHash encodes the record of a state height: its header app hash and the
// latest block height whose header named it.
func encodeCertifiedAppHash(hash types.Hash, namedAt types.Height) []byte {
	return append(append([]byte(nil), hash[:]...), encodeHeight(namedAt)...)
}

func decodeCertifiedAppHash(encoded []byte) (types.Hash, types.Height, error) {
	var hash types.Hash
	if len(encoded) != len(hash)+8 {
		return types.Hash{}, 0, errMarkerCorrupt
	}
	copy(hash[:], encoded[:len(hash)])
	return hash, types.Height(binary.BigEndian.Uint64(encoded[len(hash):])), nil
}

func (module *Module) recordCertifiedAppHash(ctx context.Context, store vexoapp.StateStore, header types.Header) error {
	stateHeight, ok := finality.AppHashHeight(header)
	if !ok || header.Height == 0 {
		return nil
	}
	if err := store.Set(ctx, ModuleName, certifiedAppHashKey(stateHeight), encodeCertifiedAppHash(header.AppHash, header.Height)); err != nil {
		return err
	}
	if err := store.Set(ctx, ModuleName, certifiedNamingKey(header.Height), encodeHeight(stateHeight)); err != nil {
		return err
	}
	window := uint64(module.policy.Lifecycle.EvidenceMaxAge) + 1
	if uint64(header.Height) <= window {
		return nil
	}
	expired := header.Height - types.Height(window)
	named, err := store.Get(ctx, ModuleName, certifiedNamingKey(expired))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(named) != 8 {
		return errMarkerCorrupt
	}
	if err := store.Delete(ctx, ModuleName, certifiedNamingKey(expired)); err != nil {
		return err
	}
	expiredState := types.Height(binary.BigEndian.Uint64(named))
	encoded, err := store.Get(ctx, ModuleName, certifiedAppHashKey(expiredState))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, namedAt, err := decodeCertifiedAppHash(encoded)
	if err != nil {
		return err
	}
	if namedAt != expired {
		return nil
	}
	return store.Delete(ctx, ModuleName, certifiedAppHashKey(expiredState))
}

// certifiedAppHashes serves the recorded certified app hashes to evidence verification.
type certifiedAppHashes struct {
	ctx   context.Context
	store vexoapp.StateStore
}

// CertifiedAppHash returns the header app hash certified blocks committed for the state of
// stateHeight, as the replicated state in store records it; found is false when no block in the
// evidence window named that height.
func CertifiedAppHash(ctx context.Context, store vexoapp.StateStore, stateHeight types.Height) (types.Hash, bool, error) {
	return certifiedAppHashes{ctx: ctx, store: store}.CertifiedAppHash(stateHeight)
}

func (hashes certifiedAppHashes) CertifiedAppHash(stateHeight types.Height) (types.Hash, bool, error) {
	encoded, err := hashes.store.Get(hashes.ctx, ModuleName, certifiedAppHashKey(stateHeight))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return types.Hash{}, false, nil
	}
	if err != nil {
		return types.Hash{}, false, err
	}
	hash, _, err := decodeCertifiedAppHash(encoded)
	if err != nil {
		return types.Hash{}, false, err
	}
	return hash, true, nil
}
