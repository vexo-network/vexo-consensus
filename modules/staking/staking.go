package staking

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	vexovalidator "github.com/vexo-network/vexo-consensus/validator"
)

const ModuleName = "staking"

const (
	delegateGasCost         uint64 = 50
	createValidatorGasCost  uint64 = 200
	undelegateGasCost       uint64 = 40
	unjailGasCost           uint64 = 20
	claimRewardsGasCost     uint64 = 20
	withdrawUnbondedGasCost uint64 = 25
	setCommissionGasCost    uint64 = 20
)

// Result codes of staking transactions. A transaction that fails because of its content returns one of
// them; nothing in this module panics or aborts the block for transaction content.
const (
	codeNoStore      uint32 = 1
	codeInvalidTx    uint32 = 2
	codeBadArguments uint32 = 3
	codeFailed       uint32 = 4
	codeOutOfGas     uint32 = 5
	codeUnauthorized uint32 = 6
)

const bankNamespace = "bank"
const defaultFeeCollector types.Address = "fee_collector"
const defaultUnbondingDelay types.Height = 1209600
const commissionDenominatorBPS uint64 = 10000

var (
	ErrInvalidStakingTx           = errors.New("invalid staking transaction")
	ErrInvalidIdentity            = errors.New("invalid staking identifier")
	ErrInsufficientBalance        = errors.New("insufficient balance for delegation")
	ErrInsufficientStake          = errors.New("insufficient delegated stake")
	ErrInvalidStakeRecord         = errors.New("invalid stake record")
	ErrMissingValidatorKey        = errors.New("validator public key is required")
	ErrInvalidConsensusKey        = errors.New("invalid validator consensus key")
	ErrDuplicateConsensusKey      = errors.New("validator consensus key is already registered")
	ErrValidatorNotFound          = errors.New("validator is not registered")
	ErrValidatorExists            = errors.New("validator is already registered")
	ErrValidatorNotJailed         = errors.New("validator is not jailed")
	ErrJailNotExpired             = errors.New("validator jail has not expired")
	ErrNoBondedStake              = errors.New("validator has no bonded stake")
	ErrTooManyUnbondingEntries    = errors.New("too many unbonding entries")
	ErrInvalidSlash               = errors.New("invalid slash request")
	ErrStakingStoreRequired       = errors.New("missing staking store")
	ErrStakeOverflow              = errors.New("staking amount overflow")
	ErrNoRewards                  = errors.New("no staking rewards available")
	ErrNoUnbonding                = errors.New("no matured unbonding balance")
	ErrUnbondingNotMature         = errors.New("unbonding balance is not mature")
	ErrInvalidCommission          = errors.New("invalid validator commission")
	ErrUnauthorizedStaking        = errors.New("unauthorized staking transaction")
	ErrValidatorTombstoned        = errors.New("validator is tombstoned")
	ErrStakingSnapshot            = errors.New("staking snapshot export is required")
	ErrStakingAtomicStoreRequired = errors.New("staking multi-key writes require atomic batch store")
	ErrDuplicateGenesisStake      = errors.New("genesis stake names a delegation twice")
)

type Module struct {
	unbondingDelay      types.Height
	feeCollector        types.Address
	maxCommissionBPS    uint64
	maxUnbondingEntries int
	cryptoBackend       config.CryptoBackend
	admission           vexovalidator.AdmissionPolicy
	// touched holds the validators a transaction of the current block changed. Their updates are built
	// from the state the block ends in, never from the state a transaction left behind.
	touched map[types.ValidatorID]struct{}
}

type Policy struct {
	UnbondingDelay   types.Height
	FeeCollector     types.Address
	MaxCommissionBPS uint64
	// MaxUnbondingEntries caps the live unbonding entries of one delegator/validator pair.
	MaxUnbondingEntries int
	// CryptoBackend selects the consensus key format validators must register.
	CryptoBackend config.CryptoBackend
	// Admission, when set, decides which validators may enter the validator set.
	Admission vexovalidator.AdmissionPolicy
}

func NewModule() *Module {
	return NewModuleWithPolicy(Policy{})
}

func NewModuleWithUnbondingDelay(delay types.Height) *Module {
	return NewModuleWithPolicy(Policy{UnbondingDelay: delay})
}

func NewModuleWithFeeCollector(collector types.Address) *Module {
	return NewModuleWithPolicy(Policy{FeeCollector: collector})
}

func NewModuleWithPolicy(policy Policy) *Module {
	if policy.UnbondingDelay == 0 {
		policy.UnbondingDelay = defaultUnbondingDelay
	}
	if policy.FeeCollector == "" {
		policy.FeeCollector = defaultFeeCollector
	}
	if policy.MaxCommissionBPS == 0 {
		policy.MaxCommissionBPS = commissionDenominatorBPS
	}
	if policy.MaxCommissionBPS > commissionDenominatorBPS {
		policy.MaxCommissionBPS = commissionDenominatorBPS
	}
	if policy.MaxUnbondingEntries <= 0 {
		policy.MaxUnbondingEntries = defaultMaxUnbondingEntries
	}
	if policy.CryptoBackend == "" {
		policy.CryptoBackend = config.CryptoBackendDeterministic
	}
	return &Module{
		unbondingDelay:      policy.UnbondingDelay,
		feeCollector:        policy.FeeCollector,
		maxCommissionBPS:    policy.MaxCommissionBPS,
		maxUnbondingEntries: policy.MaxUnbondingEntries,
		cryptoBackend:       policy.CryptoBackend,
		admission:           policy.Admission,
	}
}

func (module *Module) CloneModule() vexoapp.Module {
	return NewModuleWithPolicy(module.Policy())
}

func (module *Module) Name() string {
	return ModuleName
}

func (module *Module) FeeCollector() types.Address {
	return module.feeCollector
}

func (module *Module) Policy() Policy {
	return Policy{
		UnbondingDelay:      module.unbondingDelay,
		FeeCollector:        module.feeCollector,
		MaxCommissionBPS:    module.maxCommissionBPS,
		MaxUnbondingEntries: module.maxUnbondingEntries,
		CryptoBackend:       module.cryptoBackend,
		Admission:           module.admission,
	}
}

// InitGenesis seeds the staking state of the genesis validator set. Explicit
// "staking:stake:<delegator>:<validator>" entries of the app state add delegators to genesis
// validators; every genesis validator is self-bonded by its operator for the rest of its power.
func (module *Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	if ctx.Store == nil {
		return nil
	}
	stakes, err := genesisStakes(genesis)
	if err != nil {
		return err
	}
	if len(ctx.GenesisValidators) == 0 && len(stakes) == 0 {
		return nil
	}
	return seedGenesisValidators(ctx.GoContext(), ctx.Store, ctx.GenesisValidators, stakes)
}

func genesisStakes(genesis vexoapp.GenesisState) ([]genesisStake, error) {
	keys := make([]string, 0, len(genesis))
	for rawKey := range genesis {
		if strings.HasPrefix(rawKey, ModuleName+":stake:") {
			keys = append(keys, rawKey)
		}
	}
	sort.Strings(keys)
	stakes := make([]genesisStake, 0, len(keys))
	for _, rawKey := range keys {
		parts := strings.Split(rawKey, ":")
		if len(parts) != 4 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidStakeRecord, rawKey)
		}
		amount, err := strconv.ParseUint(string(genesis[rawKey]), 10, 64)
		if err != nil || amount == 0 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidStakeRecord, rawKey)
		}
		stake := genesisStake{delegator: types.Address(parts[2]), validatorID: types.ValidatorID(parts[3]), amount: amount}
		if validateDelegator(stake.delegator) != nil || validateValidatorID(stake.validatorID) != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidStakeRecord, rawKey)
		}
		stakes = append(stakes, stake)
	}
	return stakes, nil
}

func (module *Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	module.touched = nil
	return nil
}

func failure(code uint32, err error) types.Result {
	return types.Result{Code: code, Log: err.Error()}
}

// executionFailure classifies the error of a state transition: running out of gas and missing
// authorization have their own codes, everything else is a failed execution.
func executionFailure(err error) types.Result {
	switch {
	case errors.Is(err, vexoapp.ErrOutOfGas):
		return failure(codeOutOfGas, err)
	case errors.Is(err, ErrUnauthorizedStaking):
		return failure(codeUnauthorized, err)
	default:
		return failure(codeFailed, err)
	}
}

// authorize requires the transaction signer to be actor. Every action on behalf of an account named in
// the transaction arguments must pass it before touching state.
func authorize(tx types.Tx, actor types.Address) error {
	if err := vexoapp.AuthorizeActor(tx, actor); err != nil {
		return fmt.Errorf("%w: %w", ErrUnauthorizedStaking, err)
	}
	return nil
}

func (module *Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return failure(codeNoStore, ErrStakingStoreRequired)
	}
	parts := stakingTxParts(tx)
	if len(parts) < 2 || parts[0] != ModuleName {
		return failure(codeInvalidTx, ErrInvalidStakingTx)
	}
	switch {
	case len(parts) == 5 && parts[1] == "delegate":
		return module.deliverDelegate(ctx, tx, parts)
	case (len(parts) == 6 || len(parts) == 7) && parts[1] == "create-validator":
		return module.deliverCreateValidator(ctx, tx, parts)
	case len(parts) == 5 && parts[1] == "undelegate":
		return module.deliverUndelegate(ctx, tx, parts)
	case len(parts) == 3 && parts[1] == "unjail":
		return module.deliverUnjail(ctx, tx, parts)
	case len(parts) == 4 && parts[1] == "claim-rewards":
		return module.deliverClaimRewards(ctx, tx, parts)
	case len(parts) == 4 && parts[1] == "withdraw-unbonded":
		return module.deliverWithdrawUnbonded(ctx, tx, parts)
	case len(parts) == 4 && parts[1] == "set-commission":
		return module.deliverSetCommission(ctx, tx, parts)
	default:
		return failure(codeInvalidTx, ErrInvalidStakingTx)
	}
}

func parseDelegation(delegatorPart string, validatorPart string) (types.Address, types.ValidatorID, error) {
	delegator := types.Address(delegatorPart)
	validatorID := types.ValidatorID(validatorPart)
	if err := validateDelegator(delegator); err != nil {
		return "", "", err
	}
	if err := validateValidatorID(validatorID); err != nil {
		return "", "", err
	}
	return delegator, validatorID, nil
}

func (module *Module) deliverDelegate(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(delegateGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	delegator, validatorID, err := parseDelegation(parts[2], parts[3])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	amount, err := parseAmount(parts[4])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	if err := authorize(tx, delegator); err != nil {
		return executionFailure(err)
	}
	if err := module.delegate(ctx, delegator, validatorID, amount); err != nil {
		return executionFailure(err)
	}
	return types.Result{}
}

func (module *Module) deliverCreateValidator(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(createValidatorGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	operator, validatorID, err := parseDelegation(parts[2], parts[3])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	amount, err := parseAmount(parts[4])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	publicKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(publicKey) == 0 {
		return failure(codeBadArguments, ErrMissingValidatorKey)
	}
	proof := ""
	if len(parts) == 7 {
		proof = parts[6]
	}
	if err := module.createValidator(ctx, tx, operator, validatorID, amount, publicKey, proof); err != nil {
		return executionFailure(err)
	}
	return types.Result{}
}

func (module *Module) deliverUndelegate(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(undelegateGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	delegator, validatorID, err := parseDelegation(parts[2], parts[3])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	amount, err := parseAmount(parts[4])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	if err := authorize(tx, delegator); err != nil {
		return executionFailure(err)
	}
	if err := module.undelegate(ctx, delegator, validatorID, amount); err != nil {
		return executionFailure(err)
	}
	return types.Result{}
}

func (module *Module) deliverUnjail(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(unjailGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	validatorID := types.ValidatorID(parts[2])
	if err := validateValidatorID(validatorID); err != nil {
		return failure(codeBadArguments, err)
	}
	if err := module.unjail(ctx, tx, validatorID); err != nil {
		return executionFailure(err)
	}
	return types.Result{}
}

func (module *Module) deliverClaimRewards(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(claimRewardsGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	delegator, validatorID, err := parseDelegation(parts[2], parts[3])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	if err := authorize(tx, delegator); err != nil {
		return executionFailure(err)
	}
	if err := module.claimRewards(ctx.GoContext(), ctx.Store, delegator, validatorID); err != nil {
		return executionFailure(err)
	}
	return types.Result{}
}

func (module *Module) deliverWithdrawUnbonded(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(withdrawUnbondedGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	delegator, validatorID, err := parseDelegation(parts[2], parts[3])
	if err != nil {
		return failure(codeBadArguments, err)
	}
	if err := authorize(tx, delegator); err != nil {
		return executionFailure(err)
	}
	if err := module.withdrawUnbonded(ctx, delegator, validatorID); err != nil {
		return executionFailure(err)
	}
	return types.Result{}
}

func (module *Module) deliverSetCommission(ctx vexoapp.Context, tx types.Tx, parts []string) types.Result {
	if err := ctx.ConsumeGas(setCommissionGasCost); err != nil {
		return failure(codeOutOfGas, err)
	}
	validatorID := types.ValidatorID(parts[2])
	if err := validateValidatorID(validatorID); err != nil {
		return failure(codeBadArguments, err)
	}
	commissionBPS, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		return failure(codeBadArguments, ErrInvalidCommission)
	}
	if err := module.setCommission(ctx, tx, validatorID, commissionBPS); err != nil {
		return executionFailure(err)
	}
	return types.Result{}
}

func (module *Module) EndBlock(ctx vexoapp.Context) error {
	if ctx.Store == nil {
		return nil
	}
	if err := module.distributeFees(ctx.GoContext(), ctx.Store); err != nil {
		return err
	}
	// ValidatorUpdates has no error to return, so a state that cannot be read must fail the block here,
	// where every node sees it, instead of silently dropping an update on one of them.
	_, err := module.touchedUpdates(ctx.GoContext(), ctx.Store)
	return err
}

func (module *Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	parts := stakingTxParts(tx)
	if len(parts) < 2 || parts[0] != ModuleName {
		return 0, ErrInvalidStakingTx
	}
	switch {
	case len(parts) == 5 && parts[1] == "delegate":
		return delegateGasCost, nil
	case (len(parts) == 6 || len(parts) == 7) && parts[1] == "create-validator":
		return createValidatorGasCost, nil
	case len(parts) == 5 && parts[1] == "undelegate":
		return undelegateGasCost + unbondingEntryGasCost*module.estimateUnbondingEntries(ctx, parts[2], parts[3], 1), nil
	case len(parts) == 3 && parts[1] == "unjail":
		return unjailGasCost, nil
	case len(parts) == 4 && parts[1] == "claim-rewards":
		return claimRewardsGasCost, nil
	case len(parts) == 4 && parts[1] == "withdraw-unbonded":
		return withdrawUnbondedGasCost + unbondingEntryGasCost*module.estimateUnbondingEntries(ctx, parts[2], parts[3], 0), nil
	case len(parts) == 4 && parts[1] == "set-commission":
		return setCommissionGasCost, nil
	default:
		return 0, ErrInvalidStakingTx
	}
}

// estimateUnbondingEntries counts the entries a transaction will touch: the live entries of the pair
// plus extra new ones. Without readable state the estimate assumes an empty pair.
func (module *Module) estimateUnbondingEntries(ctx vexoapp.Context, delegatorPart string, validatorPart string, extra uint64) uint64 {
	if ctx.Store == nil {
		return extra
	}
	entries, err := loadUnbondingEntries(ctx.GoContext(), ctx.Store, types.Address(delegatorPart), types.ValidatorID(validatorPart))
	if err != nil {
		return extra
	}
	return uint64(len(entries)) + extra
}

// ValidatorUpdates returns the validator set updates of the block: one update per validator touched by
// a transaction, ordered by validator id. Each update is read from the state the block ended in,
// after every module ran. The runtime keeps the last update of a validator whatever the order of the
// modules, so an update frozen at the time of a transaction could undo a jail or slash that a module
// registered before this one applied later in the block.
func (module *Module) ValidatorUpdates(ctx vexoapp.Context) []types.ValidatorUpdate {
	if len(module.touched) == 0 || ctx.Store == nil {
		return nil
	}
	// EndBlock proved that the updates can be built; a failure now is a store fault, and reporting no
	// update leaves the validator set as it was.
	updates, err := module.touchedUpdates(ctx.GoContext(), ctx.Store)
	if err != nil {
		return nil
	}
	return updates
}

// touchedUpdates builds the current update of every touched validator, ordered by validator id. A
// touched validator without a registration has nothing to report: its transaction did not survive.
func (module *Module) touchedUpdates(ctx context.Context, store vexoapp.StateStore) ([]types.ValidatorUpdate, error) {
	ids := make([]types.ValidatorID, 0, len(module.touched))
	for validatorID := range module.touched {
		ids = append(ids, validatorID)
	}
	sort.Slice(ids, func(left int, right int) bool { return ids[left] < ids[right] })
	updates := make([]types.ValidatorUpdate, 0, len(ids))
	for _, validatorID := range ids {
		update, err := validatorUpdate(ctx, store, validatorID)
		if errors.Is(err, ErrValidatorNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		updates = append(updates, update)
	}
	return updates, nil
}

func queryFailure(code uint32, err error) vexoapp.QueryResponse {
	return vexoapp.QueryResponse{Code: code, Log: err.Error()}
}

func uintQueryResponse(value uint64, err error) vexoapp.QueryResponse {
	if err != nil {
		return queryFailure(3, err)
	}
	return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(value, 10))}
}

func (module *Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if ctx.Store == nil {
		return queryFailure(1, ErrStakingStoreRequired)
	}
	goCtx := ctx.GoContext()
	store := ctx.Store
	for _, element := range req.Path {
		if element != "" && strings.ContainsAny(element, "/\x00") {
			return queryFailure(3, ErrInvalidIdentity)
		}
	}
	switch {
	case len(req.Path) == 3 && req.Path[0] == "stake":
		return uintQueryResponse(Stake(goCtx, store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2])))
	case len(req.Path) == 2 && req.Path[0] == "validator":
		return uintQueryResponse(ValidatorPower(goCtx, store, types.ValidatorID(req.Path[1])))
	case len(req.Path) == 3 && req.Path[0] == "unbonding":
		releaseHeight, err := UnbondingReleaseHeight(goCtx, store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2]))
		return uintQueryResponse(uint64(releaseHeight), err)
	case len(req.Path) == 3 && req.Path[0] == "unbonding-balance":
		return uintQueryResponse(UnbondingAmount(goCtx, store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2])))
	case len(req.Path) == 3 && req.Path[0] == "rewards":
		return uintQueryResponse(Rewards(goCtx, store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2])))
	case len(req.Path) == 2 && req.Path[0] == "commission":
		return uintQueryResponse(Commission(goCtx, store, types.ValidatorID(req.Path[1])))
	case len(req.Path) == 2 && req.Path[0] == "tombstone":
		tombstoned, err := Tombstoned(goCtx, store, types.ValidatorID(req.Path[1]))
		if err != nil {
			return queryFailure(3, err)
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatBool(tombstoned))}
	case len(req.Path) == 2 && req.Path[0] == "jail":
		jailEnd, jailed, err := JailedUntil(goCtx, store, types.ValidatorID(req.Path[1]))
		if err != nil {
			return queryFailure(3, err)
		}
		if !jailed {
			return vexoapp.QueryResponse{Value: []byte("0")}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(uint64(jailEnd), 10))}
	case len(req.Path) == 2 && req.Path[0] == "operator":
		record, found, err := loadValidator(goCtx, store, types.ValidatorID(req.Path[1]))
		if err != nil {
			return queryFailure(3, err)
		}
		if !found {
			return queryFailure(3, ErrValidatorNotFound)
		}
		return vexoapp.QueryResponse{Value: []byte(record.Operator)}
	default:
		return vexoapp.QueryResponse{Code: 2, Log: "invalid staking query"}
	}
}

// delegationWrites moves amount from the delegator's balance into stake with the validator and
// records the height of the stake increase for governance snapshots.
func delegationWrites(ctx context.Context, store vexoapp.StateStore, height types.Height, delegator types.Address, validatorID types.ValidatorID, amount uint64) ([]kvbatch.KVWrite, error) {
	balance, err := bankBalanceBig(ctx, store, delegator)
	if err != nil {
		return nil, err
	}
	amountBig := new(big.Int).SetUint64(amount)
	if balance.Cmp(amountBig) < 0 {
		return nil, ErrInsufficientBalance
	}
	currentStake, err := Stake(ctx, store, delegator, validatorID)
	if err != nil {
		return nil, err
	}
	currentPower, err := ValidatorPower(ctx, store, validatorID)
	if err != nil {
		return nil, err
	}
	if currentPower > ^uint64(0)-amount || currentStake > ^uint64(0)-amount {
		return nil, ErrStakeOverflow
	}
	writes := []kvbatch.KVWrite{
		{Namespace: bankNamespace, Key: bankBalanceKey(delegator), Value: encodeBankBalanceBig(new(big.Int).Sub(balance, amountBig))},
	}
	writes = append(writes, stakeWrites(delegator, validatorID, currentStake+amount)...)
	return append(writes,
		kvbatch.KVWrite{Namespace: ModuleName, Key: validatorPowerKey(validatorID), Value: encodeUint64(currentPower + amount)},
		kvbatch.KVWrite{Namespace: ModuleName, Key: delegationHeightKey(delegator), Value: encodeUint64(uint64(height))},
	), nil
}

// stakeWrites sets the stake of one delegation, or removes the delegation for a stake of zero. The
// stake lives under two keys, one per way it is read (see stakeKeyPrefix and validatorStakeKeyPrefix),
// and every writer goes through here so that the two never disagree.
func stakeWrites(delegator types.Address, validatorID types.ValidatorID, amount uint64) []kvbatch.KVWrite {
	byDelegator := kvbatch.KVWrite{Namespace: ModuleName, Key: stakeKey(delegator, validatorID)}
	byValidator := kvbatch.KVWrite{Namespace: ModuleName, Key: validatorStakeKey(validatorID, delegator)}
	if amount == 0 {
		byDelegator.Delete = true
		byValidator.Delete = true
	} else {
		byDelegator.Value = encodeUint64(amount)
		byValidator.Value = encodeUint64(amount)
	}
	return []kvbatch.KVWrite{byDelegator, byValidator}
}

// delegate adds stake to a registered validator. It never touches the validator's key or metadata.
//
// A validator without power that is not jailed is outside the validator set. Delegating to it puts it
// back, which is a join like the one of unjail and create-validator, so the admission policy decides
// here. Otherwise the registry would refuse the join when the block's validator updates are applied,
// while staking went on counting the validator's power: it would earn rewards and take a seat of
// MaxValidators without ever being in the set.
func (module *Module) delegate(ctx vexoapp.Context, delegator types.Address, validatorID types.ValidatorID, amount uint64) error {
	goCtx := ctx.GoContext()
	store := ctx.Store
	record, found, err := loadValidator(goCtx, store, validatorID)
	if err != nil {
		return err
	}
	if !found {
		return ErrValidatorNotFound
	}
	status, err := loadValidatorStatus(goCtx, store, validatorID)
	if err != nil {
		return err
	}
	if status.tombstoned {
		return ErrValidatorTombstoned
	}
	writes, err := delegationWrites(goCtx, store, ctx.Height, delegator, validatorID, amount)
	if err != nil {
		return err
	}
	if !status.jailed && status.power == 0 {
		if err := module.admit(goCtx, store, vexovalidator.Candidate{
			Address:   record.Address,
			PublicKey: record.PublicKey,
			Stake:     amount,
			Metadata:  record.Metadata,
		}, validatorID); err != nil {
			return err
		}
	}
	if err := applyAtomicWrites(goCtx, store, writes); err != nil {
		return err
	}
	module.touch(validatorID)
	return nil
}

// undelegate moves stake into one unbonding entry. The transaction reads the pair's live entries,
// pays gas for each of them and writes only the entry it creates.
func (module *Module) undelegate(ctx vexoapp.Context, delegator types.Address, validatorID types.ValidatorID, amount uint64) error {
	goCtx := ctx.GoContext()
	store := ctx.Store
	if _, found, err := loadValidator(goCtx, store, validatorID); err != nil {
		return err
	} else if !found {
		return ErrValidatorNotFound
	}
	currentStake, err := Stake(goCtx, store, delegator, validatorID)
	if err != nil {
		return err
	}
	if currentStake < amount {
		return ErrInsufficientStake
	}
	currentPower, err := ValidatorPower(goCtx, store, validatorID)
	if err != nil {
		return err
	}
	if currentPower < amount {
		return ErrInvalidStakeRecord
	}
	entries, err := loadUnbondingEntries(goCtx, store, delegator, validatorID)
	if err != nil {
		return err
	}
	if err := ctx.ConsumeGas(unbondingEntryGasCost * uint64(len(entries)+1)); err != nil {
		return err
	}
	if uint64(ctx.Height) > ^uint64(0)-uint64(module.unbondingDelay) {
		return ErrStakeOverflow
	}
	created := unbondingEntry{CreatedHeight: ctx.Height, ReleaseHeight: ctx.Height + module.unbondingDelay, Amount: amount}
	merged := false
	for _, entry := range entries {
		if entry.CreatedHeight == ctx.Height {
			if entry.Amount > ^uint64(0)-amount {
				return ErrStakeOverflow
			}
			created.Amount = entry.Amount + amount
			merged = true
		}
	}
	if !merged && len(entries) >= module.maxUnbondingEntries {
		return ErrTooManyUnbondingEntries
	}
	unbonding, err := sumUnbondingEntries(entries)
	if err != nil {
		return err
	}
	if unbonding > ^uint64(0)-amount {
		return ErrStakeOverflow
	}
	writes := stakeWrites(delegator, validatorID, currentStake-amount)
	writes = append(writes,
		kvbatch.KVWrite{Namespace: ModuleName, Key: validatorPowerKey(validatorID), Value: encodeUint64(currentPower - amount)},
		kvbatch.KVWrite{Namespace: ModuleName, Key: unbondingEntryKey(delegator, validatorID, created.CreatedHeight), Value: encodeUnbondingEntry(created)},
	)
	if err := applyAtomicWrites(goCtx, store, writes); err != nil {
		return err
	}
	module.touch(validatorID)
	return nil
}

func Stake(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (uint64, error) {
	if err := validateDelegator(delegator); err != nil {
		return 0, err
	}
	if err := validateValidatorID(validatorID); err != nil {
		return 0, err
	}
	return getUint64(ctx, store, stakeKey(delegator, validatorID))
}

func ValidatorPower(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (uint64, error) {
	if err := validateValidatorID(validatorID); err != nil {
		return 0, err
	}
	return getUint64(ctx, store, validatorPowerKey(validatorID))
}

func Rewards(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (uint64, error) {
	amount, err := RewardsBig(ctx, store, delegator, validatorID)
	if err != nil {
		return 0, err
	}
	if !amount.IsUint64() {
		return 0, ErrStakeOverflow
	}
	return amount.Uint64(), nil
}

func RewardsBig(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (*big.Int, error) {
	if err := validateDelegator(delegator); err != nil {
		return nil, err
	}
	if err := validateValidatorID(validatorID); err != nil {
		return nil, err
	}
	return getModuleAmountBig(ctx, store, rewardKey(delegator, validatorID))
}

func Commission(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (uint64, error) {
	if err := validateValidatorID(validatorID); err != nil {
		return 0, err
	}
	return getUint64(ctx, store, commissionKey(validatorID))
}

func Tombstoned(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (bool, error) {
	if err := validateValidatorID(validatorID); err != nil {
		return false, err
	}
	_, err := store.Get(ctx, ModuleName, tombstoneKey(validatorID))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// JailedUntil returns the height a validator's jail ends and whether the validator is jailed. A jailed
// validator stays jailed until its operator unjails it, which is only possible once the jail ended.
func JailedUntil(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (types.Height, bool, error) {
	if err := validateValidatorID(validatorID); err != nil {
		return 0, false, err
	}
	status, err := loadValidatorStatus(ctx, store, validatorID)
	if err != nil {
		return 0, false, err
	}
	return status.jailEnd, status.jailed, nil
}

func (module *Module) claimRewards(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) error {
	reward, err := RewardsBig(ctx, store, delegator, validatorID)
	if err != nil {
		return err
	}
	if reward.Sign() == 0 {
		return ErrNoRewards
	}
	balance, err := bankBalanceBig(ctx, store, delegator)
	if err != nil {
		return err
	}
	newBalance := new(big.Int).Add(balance, reward)
	if err := validateBankBalanceBig(newBalance); err != nil {
		return err
	}
	return applyAtomicWrites(ctx, store, []kvbatch.KVWrite{
		{Namespace: bankNamespace, Key: bankBalanceKey(delegator), Value: encodeBankBalanceBig(newBalance)},
		{Namespace: ModuleName, Key: rewardKey(delegator, validatorID), Value: encodeBankBalanceBig(new(big.Int))},
	})
}

func (module *Module) withdrawUnbonded(ctx vexoapp.Context, delegator types.Address, validatorID types.ValidatorID) error {
	goCtx := ctx.GoContext()
	store := ctx.Store
	entries, err := loadUnbondingEntries(goCtx, store, delegator, validatorID)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return ErrNoUnbonding
	}
	if err := ctx.ConsumeGas(unbondingEntryGasCost * uint64(len(entries))); err != nil {
		return err
	}
	matured := make([]unbondingEntry, 0, len(entries))
	for _, entry := range entries {
		if ctx.Height >= entry.ReleaseHeight {
			matured = append(matured, entry)
		}
	}
	if len(matured) == 0 {
		return ErrUnbondingNotMature
	}
	amount, err := sumUnbondingEntries(matured)
	if err != nil {
		return err
	}
	balance, err := bankBalanceBig(goCtx, store, delegator)
	if err != nil {
		return err
	}
	newBalance := new(big.Int).Add(balance, new(big.Int).SetUint64(amount))
	if err := validateBankBalanceBig(newBalance); err != nil {
		return err
	}
	writes := []kvbatch.KVWrite{{Namespace: bankNamespace, Key: bankBalanceKey(delegator), Value: encodeBankBalanceBig(newBalance)}}
	for _, entry := range matured {
		writes = append(writes, kvbatch.KVWrite{Namespace: ModuleName, Key: unbondingEntryKey(delegator, validatorID, entry.CreatedHeight), Delete: true})
	}
	return applyAtomicWrites(goCtx, store, writes)
}

type delegationRecord struct {
	delegator types.Address
	stake     uint64
}

// loadValidatorDelegations returns the non-empty delegations of one validator ordered by delegator,
// and their total stake. It reads the validator's own prefix of the stake index, so its cost follows
// the delegations of that validator and not the delegations of the chain.
func loadValidatorDelegations(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) ([]delegationRecord, uint64, error) {
	prefixStore, err := prefixStoreOf(store)
	if err != nil {
		return nil, 0, err
	}
	pairs, err := prefixStore.ExportPrefix(ctx, ModuleName, validatorStakePrefix(validatorID))
	if err != nil {
		return nil, 0, err
	}
	delegations := make([]delegationRecord, 0, len(pairs))
	total := uint64(0)
	for _, pair := range pairs {
		indexed, delegator, ok := parseValidatorStakeKey(string(pair.Key))
		if !ok || indexed != validatorID {
			return nil, 0, ErrInvalidStakeRecord
		}
		stake, err := decodeUint64(pair.Value)
		if err != nil {
			return nil, 0, err
		}
		if stake == 0 {
			continue
		}
		if total > ^uint64(0)-stake {
			return nil, 0, ErrStakeOverflow
		}
		total += stake
		delegations = append(delegations, delegationRecord{delegator: delegator, stake: stake})
	}
	return delegations, total, nil
}

// distributeFees pays the fee collector's balance out as staking rewards, in proportion to the
// power of the validators that are in the set. Jailed and tombstoned validators earn nothing.
// Commission and rounding dust go to the validator's operator.
func (module *Module) distributeFees(ctx context.Context, store vexoapp.StateStore) error {
	collectorBalance, err := bankBalanceBig(ctx, store, module.feeCollector)
	if err != nil || collectorBalance.Sign() == 0 {
		return err
	}
	states, err := loadValidatorStates(ctx, store)
	if err != nil {
		return err
	}
	eligible := make([]validatorState, 0, len(states))
	totalPower := uint64(0)
	for _, state := range states {
		if !state.hasRecord || state.status.inactive() || state.status.power == 0 {
			continue
		}
		if totalPower > ^uint64(0)-state.status.power {
			return ErrStakeOverflow
		}
		totalPower += state.status.power
		eligible = append(eligible, state)
	}
	if totalPower == 0 {
		return nil
	}
	rewards := make(map[string]*big.Int)
	distributed := new(big.Int)
	for _, state := range eligible {
		validatorFee := proportionalShareBig(collectorBalance, state.status.power, totalPower)
		if validatorFee.Sign() == 0 {
			continue
		}
		validatorDelegations, totalStake, err := loadValidatorDelegations(ctx, store, state.id)
		if err != nil {
			return err
		}
		operator := state.record.Operator
		commission := proportionalShareBig(validatorFee, state.commissionBPS, commissionDenominatorBPS)
		if commission.Sign() > 0 {
			if err := addReward(rewards, operator, state.id, commission); err != nil {
				return err
			}
		}
		validatorDistributed := new(big.Int).Set(commission)
		distributionPool := new(big.Int).Sub(validatorFee, commission)
		if distributionPool.Sign() > 0 && (len(validatorDelegations) == 0 || totalStake == 0) {
			if err := addReward(rewards, operator, state.id, distributionPool); err != nil {
				return err
			}
			validatorDistributed.Add(validatorDistributed, distributionPool)
		}
		for _, delegation := range validatorDelegations {
			share := proportionalShareBig(distributionPool, delegation.stake, totalStake)
			if share.Sign() == 0 {
				continue
			}
			if err := addReward(rewards, delegation.delegator, state.id, share); err != nil {
				return err
			}
			validatorDistributed.Add(validatorDistributed, share)
		}
		if validatorDistributed.Cmp(validatorFee) < 0 {
			remainder := new(big.Int).Sub(validatorFee, validatorDistributed)
			if err := addReward(rewards, operator, state.id, remainder); err != nil {
				return err
			}
		}
		distributed.Add(distributed, validatorFee)
	}
	if distributed.Sign() == 0 {
		return nil
	}
	writes := []kvbatch.KVWrite{
		{Namespace: bankNamespace, Key: bankBalanceKey(module.feeCollector), Value: encodeBankBalanceBig(new(big.Int).Sub(collectorBalance, distributed))},
	}
	encodedKeys := make([]string, 0, len(rewards))
	for encodedKey := range rewards {
		encodedKeys = append(encodedKeys, encodedKey)
	}
	sort.Strings(encodedKeys)
	for _, encodedKey := range encodedKeys {
		delegator, validatorID := splitRewardMapKey(encodedKey)
		current, err := RewardsBig(ctx, store, delegator, validatorID)
		if err != nil {
			return err
		}
		nextReward := new(big.Int).Add(current, rewards[encodedKey])
		if err := validateBankBalanceBig(nextReward); err != nil {
			return err
		}
		writes = append(writes, kvbatch.KVWrite{
			Namespace: ModuleName,
			Key:       rewardKey(delegator, validatorID),
			Value:     encodeBankBalanceBig(nextReward),
		})
	}
	return applyAtomicWrites(ctx, store, writes)
}

func bankBalance(ctx context.Context, store vexoapp.StateStore, address types.Address) (uint64, error) {
	amount, err := bankBalanceBig(ctx, store, address)
	if err != nil {
		return 0, err
	}
	if !amount.IsUint64() {
		return 0, ErrStakeOverflow
	}
	return amount.Uint64(), nil
}

func bankBalanceBig(ctx context.Context, store vexoapp.StateStore, address types.Address) (*big.Int, error) {
	value, err := store.Get(ctx, bankNamespace, bankBalanceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		value, err = store.Get(ctx, bankNamespace, []byte(address))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return new(big.Int), nil
		}
	}
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return new(big.Int), nil
	}
	if len(value) > 32 {
		return nil, ErrInvalidStakeRecord
	}
	return new(big.Int).SetBytes(value), nil
}

func setBankBalance(ctx context.Context, store vexoapp.StateStore, address types.Address, amount uint64) error {
	return setUint64(ctx, store, bankNamespace, bankBalanceKey(address), amount)
}

func setBankBalanceBig(ctx context.Context, store vexoapp.StateStore, address types.Address, amount *big.Int) error {
	if err := validateBankBalanceBig(amount); err != nil {
		return err
	}
	return store.Set(ctx, bankNamespace, bankBalanceKey(address), encodeBankBalanceBig(amount))
}

func bankBalanceKey(address types.Address) []byte {
	raw := string(address)
	if !strings.HasPrefix(raw, "0x") {
		return []byte(address)
	}
	clean := strings.TrimPrefix(raw, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 20 {
		return []byte(address)
	}
	padded := make([]byte, 20)
	copy(padded[20-len(decoded):], decoded)
	return []byte("0x" + hex.EncodeToString(padded))
}

func applyAtomicWrites(ctx context.Context, store vexoapp.StateStore, writes []kvbatch.KVWrite) error {
	batchStore, ok := store.(kvbatch.BatchKVStore)
	if !ok {
		return ErrStakingAtomicStoreRequired
	}
	return batchStore.SetBatch(ctx, writes)
}

// addReward accumulates a reward. Delegators are keyed by their canonical identity, so one account
// never ends up with two partial rewards that overwrite each other.
func addReward(rewards map[string]*big.Int, delegator types.Address, validatorID types.ValidatorID, amount *big.Int) error {
	if amount == nil || amount.Sign() < 0 {
		return ErrInvalidStakeRecord
	}
	key := rewardMapKey(types.Address(identityKey(delegator)), validatorID)
	current := rewards[key]
	if current == nil {
		current = new(big.Int)
	}
	rewards[key] = new(big.Int).Add(current, amount)
	return nil
}

func rewardMapKey(delegator types.Address, validatorID types.ValidatorID) string {
	return string(delegator) + "\x00" + string(validatorID)
}

func splitRewardMapKey(key string) (types.Address, types.ValidatorID) {
	parts := strings.SplitN(key, "\x00", 2)
	return types.Address(parts[0]), types.ValidatorID(parts[1])
}

func proportionalShareBig(total *big.Int, part uint64, whole uint64) *big.Int {
	if total == nil || total.Sign() == 0 || part == 0 || whole == 0 {
		return new(big.Int)
	}
	share := new(big.Int).Mul(total, new(big.Int).SetUint64(part))
	share.Div(share, new(big.Int).SetUint64(whole))
	return share
}

func getModuleAmountBig(ctx context.Context, store vexoapp.StateStore, key []byte) (*big.Int, error) {
	value, err := store.Get(ctx, ModuleName, key)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return new(big.Int), nil
	}
	if err != nil {
		return nil, err
	}
	if len(value) == 0 {
		return new(big.Int), nil
	}
	if len(value) > 32 {
		return nil, ErrInvalidStakeRecord
	}
	return new(big.Int).SetBytes(value), nil
}

func getUint64(ctx context.Context, store vexoapp.StateStore, key []byte) (uint64, error) {
	value, err := store.Get(ctx, ModuleName, key)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return decodeUint64(value)
}

func decodeUint64(value []byte) (uint64, error) {
	if len(value) == 0 {
		return 0, nil
	}
	if len(value) != 8 {
		return 0, ErrInvalidStakeRecord
	}
	return binary.BigEndian.Uint64(value), nil
}

func setUint64(ctx context.Context, store vexoapp.StateStore, namespace string, key []byte, amount uint64) error {
	return store.Set(ctx, namespace, key, encodeUint64(amount))
}

func encodeUint64(amount uint64) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, amount)
	return encoded
}

func encodeBankBalanceBig(amount *big.Int) []byte {
	if amount == nil || amount.Sign() <= 0 {
		return []byte{0}
	}
	return amount.Bytes()
}

func validateBankBalanceBig(amount *big.Int) error {
	if amount == nil || amount.Sign() < 0 || amount.BitLen() > 256 {
		return ErrStakeOverflow
	}
	return nil
}

func parseAmount(value string) (uint64, error) {
	amount, err := strconv.ParseUint(value, 10, 64)
	if err != nil || amount == 0 {
		return 0, ErrInvalidStakingTx
	}
	return amount, nil
}

func stakingTxParts(tx types.Tx) []string {
	rawParts := strings.Split(string(tx), ":")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if isExecutionTagPart(part) {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func isExecutionTagPart(part string) bool {
	return strings.HasPrefix(part, "fee=") ||
		strings.HasPrefix(part, "gas=") ||
		strings.HasPrefix(part, "signer=") ||
		strings.HasPrefix(part, "nonce=")
}
