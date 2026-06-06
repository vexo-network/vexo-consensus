package staking

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/slashing"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "staking"

const (
	delegateGasCost      uint64 = 50
	undelegateGasCost    uint64 = 40
	unjailGasCost        uint64 = 20
	claimRewardsGasCost  uint64 = 20
	setCommissionGasCost uint64 = 20
)
const bankNamespace = "bank"
const defaultFeeCollector types.Address = "fee_collector"
const defaultUnbondingDelay types.Height = 1209600
const commissionDenominatorBPS uint64 = 10000

var (
	ErrInvalidStakingTx     = errors.New("invalid staking transaction")
	ErrInsufficientBalance  = errors.New("insufficient balance for delegation")
	ErrInsufficientStake    = errors.New("insufficient delegated stake")
	ErrInvalidStakeRecord   = errors.New("invalid stake record")
	ErrMissingValidatorKey  = errors.New("validator public key is required")
	ErrStakingStoreRequired = errors.New("missing staking store")
	ErrStakeOverflow        = errors.New("staking amount overflow")
	ErrNoRewards            = errors.New("no staking rewards available")
	ErrInvalidCommission    = errors.New("invalid validator commission")
	ErrUnauthorizedStaking  = errors.New("unauthorized staking transaction")
	ErrInvalidSlashReceipt  = errors.New("invalid staking slash receipt")
)

type Module struct {
	unbondingDelay   types.Height
	feeCollector     types.Address
	maxCommissionBPS uint64
	pending          []types.ValidatorUpdate
}

type Policy struct {
	UnbondingDelay   types.Height
	FeeCollector     types.Address
	MaxCommissionBPS uint64
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
	return &Module{
		unbondingDelay:   policy.UnbondingDelay,
		feeCollector:     policy.FeeCollector,
		maxCommissionBPS: policy.MaxCommissionBPS,
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
		UnbondingDelay:   module.unbondingDelay,
		FeeCollector:     module.feeCollector,
		MaxCommissionBPS: module.maxCommissionBPS,
	}
}

func (module *Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	if ctx.Store == nil {
		return nil
	}
	for rawKey, rawValue := range genesis {
		if !strings.HasPrefix(rawKey, ModuleName+":stake:") {
			continue
		}
		parts := strings.Split(rawKey, ":")
		if len(parts) != 4 {
			return fmt.Errorf("%w: %s", ErrInvalidStakeRecord, rawKey)
		}
		amount, err := strconv.ParseUint(string(rawValue), 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidStakeRecord, rawKey)
		}
		if err := setStake(ctx.GoContext(), ctx.Store, types.Address(parts[2]), types.ValidatorID(parts[3]), amount); err != nil {
			return err
		}
	}
	return nil
}

func (module *Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	module.pending = module.pending[:0]
	return nil
}

func (module *Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: ErrStakingStoreRequired.Error()}
	}
	parts := stakingTxParts(tx)
	if len(parts) == 0 || parts[0] != ModuleName {
		return types.Result{Code: 2, Log: ErrInvalidStakingTx.Error()}
	}
	switch {
	case len(parts) == 6 && parts[1] == "delegate":
		if err := ctx.ConsumeGas(delegateGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		amount, err := parseAmount(parts[4])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		publicKey, err := base64.StdEncoding.DecodeString(parts[5])
		if err != nil || len(publicKey) == 0 {
			return types.Result{Code: 3, Log: ErrMissingValidatorKey.Error()}
		}
		update, err := module.delegate(ctx.GoContext(), ctx.Store, types.Address(parts[2]), types.ValidatorID(parts[3]), amount, types.PublicKey(publicKey))
		if err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		module.pending = append(module.pending, update)
		return types.Result{}
	case len(parts) == 5 && parts[1] == "undelegate":
		if err := ctx.ConsumeGas(undelegateGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		amount, err := parseAmount(parts[4])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		update, err := module.undelegate(ctx.GoContext(), ctx.Store, ctx.Height, types.Address(parts[2]), types.ValidatorID(parts[3]), amount)
		if err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		module.pending = append(module.pending, update)
		return types.Result{}
	case len(parts) == 3 && parts[1] == "unjail":
		if err := ctx.ConsumeGas(unjailGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		if err := ctx.Store.Delete(ctx.GoContext(), ModuleName, jailKey(types.ValidatorID(parts[2]))); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		return types.Result{}
	case len(parts) == 4 && parts[1] == "claim-rewards":
		if err := ctx.ConsumeGas(claimRewardsGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		if err := module.claimRewards(ctx.GoContext(), ctx.Store, types.Address(parts[2]), types.ValidatorID(parts[3])); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		return types.Result{}
	case len(parts) == 4 && parts[1] == "set-commission":
		if err := ctx.ConsumeGas(setCommissionGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		commissionBPS, err := strconv.ParseUint(parts[3], 10, 64)
		if err != nil {
			return types.Result{Code: 3, Log: ErrInvalidCommission.Error()}
		}
		if err := module.setCommission(ctx.GoContext(), ctx.Store, tx, types.ValidatorID(parts[2]), commissionBPS); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		return types.Result{}
	default:
		return types.Result{Code: 2, Log: ErrInvalidStakingTx.Error()}
	}
}

func (module *Module) EndBlock(ctx vexoapp.Context) error {
	if ctx.Store == nil {
		return nil
	}
	return module.distributeFees(ctx.GoContext(), ctx.Store)
}

func (module *Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	parts := stakingTxParts(tx)
	switch {
	case len(parts) == 6 && parts[1] == "delegate":
		return delegateGasCost, nil
	case len(parts) == 5 && parts[1] == "undelegate":
		return undelegateGasCost, nil
	case len(parts) == 3 && parts[1] == "unjail":
		return unjailGasCost, nil
	case len(parts) == 4 && parts[1] == "claim-rewards":
		return claimRewardsGasCost, nil
	case len(parts) == 4 && parts[1] == "set-commission":
		return setCommissionGasCost, nil
	default:
		return 0, ErrInvalidStakingTx
	}
}

func (module *Module) ValidatorUpdates(ctx vexoapp.Context) []types.ValidatorUpdate {
	return append([]types.ValidatorUpdate(nil), module.pending...)
}

func (module *Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrStakingStoreRequired.Error()}
	}
	if len(req.Path) == 3 && req.Path[0] == "stake" {
		amount, err := Stake(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(amount, 10))}
	}
	if len(req.Path) == 2 && req.Path[0] == "validator" {
		power, err := ValidatorPower(ctx.GoContext(), ctx.Store, types.ValidatorID(req.Path[1]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(power, 10))}
	}
	if len(req.Path) == 3 && req.Path[0] == "unbonding" {
		releaseHeight, err := UnbondingReleaseHeight(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(uint64(releaseHeight), 10))}
	}
	if len(req.Path) == 3 && req.Path[0] == "rewards" {
		amount, err := Rewards(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(amount, 10))}
	}
	if len(req.Path) == 2 && req.Path[0] == "commission" {
		commissionBPS, err := Commission(ctx.GoContext(), ctx.Store, types.ValidatorID(req.Path[1]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(commissionBPS, 10))}
	}
	return vexoapp.QueryResponse{Code: 2, Log: "invalid staking query"}
}

func (module *Module) delegate(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID, amount uint64, publicKey types.PublicKey) (types.ValidatorUpdate, error) {
	if delegator == "" || validatorID == "" || amount == 0 {
		return types.ValidatorUpdate{}, ErrInvalidStakingTx
	}
	balance, err := bankBalance(ctx, store, delegator)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	if balance < amount {
		return types.ValidatorUpdate{}, ErrInsufficientBalance
	}
	currentStake, err := Stake(ctx, store, delegator, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	currentPower, err := ValidatorPower(ctx, store, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	if currentPower > ^uint64(0)-amount || currentStake > ^uint64(0)-amount {
		return types.ValidatorUpdate{}, ErrStakeOverflow
	}
	newPower := currentPower + amount
	if batchStore, ok := store.(kvbatch.BatchKVStore); ok {
		if err := batchStore.SetBatch(ctx, []kvbatch.KVWrite{
			{Namespace: bankNamespace, Key: bankBalanceKey(delegator), Value: encodeUint64(balance - amount)},
			{Namespace: ModuleName, Key: stakeKey(delegator, validatorID), Value: encodeUint64(currentStake + amount)},
			{Namespace: ModuleName, Key: validatorPowerKey(validatorID), Value: encodeUint64(newPower)},
			{Namespace: ModuleName, Key: validatorKeyKey(validatorID), Value: append([]byte(nil), publicKey...)},
		}); err != nil {
			return types.ValidatorUpdate{}, err
		}
	} else {
		if err := setBankBalance(ctx, store, delegator, balance-amount); err != nil {
			return types.ValidatorUpdate{}, err
		}
		if err := setStake(ctx, store, delegator, validatorID, currentStake+amount); err != nil {
			return types.ValidatorUpdate{}, err
		}
		if err := setValidatorPower(ctx, store, validatorID, newPower); err != nil {
			return types.ValidatorUpdate{}, err
		}
		if err := store.Set(ctx, ModuleName, validatorKeyKey(validatorID), publicKey); err != nil {
			return types.ValidatorUpdate{}, err
		}
	}
	return types.ValidatorUpdate{
		ID:          validatorID,
		Address:     types.Address(validatorID),
		PublicKey:   append(types.PublicKey(nil), publicKey...),
		VotingPower: types.VotingPower(newPower),
		Stake:       newPower,
		Metadata:    map[string]string{"source": "staking"},
	}, nil
}

func (module *Module) undelegate(ctx context.Context, store vexoapp.StateStore, height types.Height, delegator types.Address, validatorID types.ValidatorID, amount uint64) (types.ValidatorUpdate, error) {
	if delegator == "" || validatorID == "" || amount == 0 {
		return types.ValidatorUpdate{}, ErrInvalidStakingTx
	}
	currentStake, err := Stake(ctx, store, delegator, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	if currentStake < amount {
		return types.ValidatorUpdate{}, ErrInsufficientStake
	}
	currentPower, err := ValidatorPower(ctx, store, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	newPower := currentPower - amount
	if uint64(height) > ^uint64(0)-uint64(module.unbondingDelay) {
		return types.ValidatorUpdate{}, ErrStakeOverflow
	}
	releaseHeight := height + module.unbondingDelay
	if batchStore, ok := store.(kvbatch.BatchKVStore); ok {
		if err := batchStore.SetBatch(ctx, []kvbatch.KVWrite{
			{Namespace: ModuleName, Key: stakeKey(delegator, validatorID), Value: encodeUint64(currentStake - amount)},
			{Namespace: ModuleName, Key: validatorPowerKey(validatorID), Value: encodeUint64(newPower)},
			{Namespace: ModuleName, Key: unbondingKey(delegator, validatorID), Value: encodeUint64(uint64(releaseHeight))},
		}); err != nil {
			return types.ValidatorUpdate{}, err
		}
	} else {
		if err := setStake(ctx, store, delegator, validatorID, currentStake-amount); err != nil {
			return types.ValidatorUpdate{}, err
		}
		if err := setValidatorPower(ctx, store, validatorID, newPower); err != nil {
			return types.ValidatorUpdate{}, err
		}
		if err := setUnbondingReleaseHeight(ctx, store, delegator, validatorID, releaseHeight); err != nil {
			return types.ValidatorUpdate{}, err
		}
	}
	publicKey, err := store.Get(ctx, ModuleName, validatorKeyKey(validatorID))
	if err != nil && !errors.Is(err, vexostore.ErrKeyNotFound) {
		return types.ValidatorUpdate{}, err
	}
	return types.ValidatorUpdate{
		ID:          validatorID,
		Address:     types.Address(validatorID),
		PublicKey:   append(types.PublicKey(nil), publicKey...),
		VotingPower: types.VotingPower(newPower),
		Stake:       newPower,
		Metadata:    map[string]string{"source": "staking"},
	}, nil
}

func (module *Module) ApplySlashingPenalty(ctx context.Context, store vexoapp.StateStore, receipt slashing.PenaltyReceipt) error {
	if store == nil {
		return ErrStakingStoreRequired
	}
	if receipt.Evidence.Validator == "" || receipt.PreviousPower == 0 || receipt.RemainingPower > receipt.PreviousPower {
		return ErrInvalidSlashReceipt
	}
	markerKey := stakingSlashKey(receipt.Evidence)
	if len(markerKey) == 0 {
		return ErrInvalidSlashReceipt
	}
	if _, err := store.Get(ctx, ModuleName, markerKey); err == nil {
		return nil
	} else if err != nil && !errors.Is(err, vexostore.ErrKeyNotFound) {
		return err
	}

	writes := []kvbatch.KVWrite{
		{Namespace: ModuleName, Key: validatorPowerKey(receipt.Evidence.Validator), Value: encodeUint64(uint64(receipt.RemainingPower))},
		{Namespace: ModuleName, Key: markerKey, Value: encodeSlashMarker(receipt)},
	}
	snapshot, ok := store.(vexostore.SnapshotKVStore)
	if ok {
		pairs, err := snapshot.ExportNamespace(ctx, ModuleName)
		if err != nil {
			return err
		}
		for _, delegation := range delegationsForValidator(pairs, receipt.Evidence.Validator) {
			nextStake := proportionalShare(delegation.stake, uint64(receipt.RemainingPower), uint64(receipt.PreviousPower))
			write := kvbatch.KVWrite{
				Namespace: ModuleName,
				Key:       stakeKey(delegation.delegator, delegation.validatorID),
				Value:     encodeUint64(nextStake),
			}
			if nextStake == 0 {
				write.Delete = true
				write.Value = nil
			}
			writes = append(writes, write)
		}
	}
	if batchStore, ok := store.(kvbatch.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if write.Delete {
			if err := store.Delete(ctx, write.Namespace, write.Key); err != nil {
				return err
			}
			continue
		}
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func Stake(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (uint64, error) {
	return getUint64(ctx, store, stakeKey(delegator, validatorID))
}

func ValidatorPower(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (uint64, error) {
	return getUint64(ctx, store, validatorPowerKey(validatorID))
}

func UnbondingReleaseHeight(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (types.Height, error) {
	value, err := getUint64(ctx, store, unbondingKey(delegator, validatorID))
	return types.Height(value), err
}

func Rewards(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) (uint64, error) {
	return getUint64(ctx, store, rewardKey(delegator, validatorID))
}

func Commission(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID) (uint64, error) {
	return getUint64(ctx, store, commissionKey(validatorID))
}

func (module *Module) setCommission(ctx context.Context, store vexoapp.StateStore, tx types.Tx, validatorID types.ValidatorID, commissionBPS uint64) error {
	if validatorID == "" || commissionBPS > module.maxCommissionBPS {
		return ErrInvalidCommission
	}
	signer, found := vexoapp.TxTag(tx, "signer")
	if !found || types.ValidatorID(signer) != validatorID {
		return ErrUnauthorizedStaking
	}
	return setUint64(ctx, store, ModuleName, commissionKey(validatorID), commissionBPS)
}

func (module *Module) claimRewards(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID) error {
	if delegator == "" || validatorID == "" {
		return ErrInvalidStakingTx
	}
	reward, err := Rewards(ctx, store, delegator, validatorID)
	if err != nil {
		return err
	}
	if reward == 0 {
		return ErrNoRewards
	}
	balance, err := bankBalance(ctx, store, delegator)
	if err != nil {
		return err
	}
	if balance > ^uint64(0)-reward {
		return ErrStakeOverflow
	}
	writes := []kvbatch.KVWrite{
		{Namespace: bankNamespace, Key: bankBalanceKey(delegator), Value: encodeUint64(balance + reward)},
		{Namespace: ModuleName, Key: rewardKey(delegator, validatorID), Value: encodeUint64(0)},
	}
	if batchStore, ok := store.(kvbatch.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

type validatorPowerRecord struct {
	validatorID types.ValidatorID
	power       uint64
}

type delegationRecord struct {
	delegator   types.Address
	validatorID types.ValidatorID
	stake       uint64
}

func (module *Module) distributeFees(ctx context.Context, store vexoapp.StateStore) error {
	snapshot, ok := store.(vexostore.SnapshotKVStore)
	if !ok {
		return nil
	}
	collectorBalance, err := bankBalance(ctx, store, module.feeCollector)
	if err != nil || collectorBalance == 0 {
		return err
	}
	pairs, err := snapshot.ExportNamespace(ctx, ModuleName)
	if err != nil {
		return err
	}
	validators, delegations, err := stakingRewardInputs(pairs)
	if err != nil {
		return err
	}
	totalPower, err := totalValidatorPower(validators)
	if err != nil || totalPower == 0 {
		return err
	}
	delegationsByValidator := make(map[types.ValidatorID][]delegationRecord)
	stakeByValidator := make(map[types.ValidatorID]uint64)
	for _, delegation := range delegations {
		delegationsByValidator[delegation.validatorID] = append(delegationsByValidator[delegation.validatorID], delegation)
		if stakeByValidator[delegation.validatorID] > ^uint64(0)-delegation.stake {
			return ErrStakeOverflow
		}
		stakeByValidator[delegation.validatorID] += delegation.stake
	}
	rewards := make(map[string]uint64)
	distributed := uint64(0)
	for _, validator := range validators {
		validatorFee := proportionalShare(collectorBalance, validator.power, totalPower)
		if validatorFee == 0 {
			continue
		}
		commissionBPS, err := Commission(ctx, store, validator.validatorID)
		if err != nil {
			return err
		}
		commission := proportionalShare(validatorFee, commissionBPS, commissionDenominatorBPS)
		if commission > 0 {
			if err := addReward(rewards, types.Address(validator.validatorID), validator.validatorID, commission); err != nil {
				return err
			}
		}
		validatorDistributed := commission
		distributionPool := validatorFee - commission
		validatorDelegations := delegationsByValidator[validator.validatorID]
		totalStake := stakeByValidator[validator.validatorID]
		if distributionPool > 0 && (len(validatorDelegations) == 0 || totalStake == 0) {
			if err := addReward(rewards, types.Address(validator.validatorID), validator.validatorID, distributionPool); err != nil {
				return err
			}
			validatorDistributed += distributionPool
		}
		for _, delegation := range validatorDelegations {
			share := proportionalShare(distributionPool, delegation.stake, totalStake)
			if share == 0 {
				continue
			}
			if err := addReward(rewards, delegation.delegator, delegation.validatorID, share); err != nil {
				return err
			}
			validatorDistributed += share
		}
		if validatorDistributed < validatorFee {
			remainder := validatorFee - validatorDistributed
			if err := addReward(rewards, types.Address(validator.validatorID), validator.validatorID, remainder); err != nil {
				return err
			}
		}
		if distributed > ^uint64(0)-validatorFee {
			return ErrStakeOverflow
		}
		distributed += validatorFee
	}
	if distributed == 0 {
		return nil
	}
	writes := []kvbatch.KVWrite{
		{Namespace: bankNamespace, Key: bankBalanceKey(module.feeCollector), Value: encodeUint64(collectorBalance - distributed)},
	}
	for encodedKey, amount := range rewards {
		delegator, validatorID := splitRewardMapKey(encodedKey)
		current, err := Rewards(ctx, store, delegator, validatorID)
		if err != nil {
			return err
		}
		if current > ^uint64(0)-amount {
			return ErrStakeOverflow
		}
		writes = append(writes, kvbatch.KVWrite{
			Namespace: ModuleName,
			Key:       rewardKey(delegator, validatorID),
			Value:     encodeUint64(current + amount),
		})
	}
	if batchStore, ok := store.(kvbatch.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func bankBalance(ctx context.Context, store vexoapp.StateStore, address types.Address) (uint64, error) {
	value, err := store.Get(ctx, bankNamespace, bankBalanceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		value, err = store.Get(ctx, bankNamespace, []byte(address))
		if errors.Is(err, vexostore.ErrKeyNotFound) {
			return 0, nil
		}
	}
	if err != nil {
		return 0, err
	}
	if len(value) == 0 {
		return 0, nil
	}
	if len(value) != 8 {
		return 0, ErrInvalidStakeRecord
	}
	return binary.BigEndian.Uint64(value), nil
}

func setBankBalance(ctx context.Context, store vexoapp.StateStore, address types.Address, amount uint64) error {
	return setUint64(ctx, store, bankNamespace, bankBalanceKey(address), amount)
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

func setStake(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID, amount uint64) error {
	return setUint64(ctx, store, ModuleName, stakeKey(delegator, validatorID), amount)
}

func setValidatorPower(ctx context.Context, store vexoapp.StateStore, validatorID types.ValidatorID, amount uint64) error {
	return setUint64(ctx, store, ModuleName, validatorPowerKey(validatorID), amount)
}

func setUnbondingReleaseHeight(ctx context.Context, store vexoapp.StateStore, delegator types.Address, validatorID types.ValidatorID, releaseHeight types.Height) error {
	return setUint64(ctx, store, ModuleName, unbondingKey(delegator, validatorID), uint64(releaseHeight))
}

func stakingRewardInputs(pairs []vexostore.KVPair) ([]validatorPowerRecord, []delegationRecord, error) {
	validators := make([]validatorPowerRecord, 0)
	delegations := make([]delegationRecord, 0)
	for _, pair := range pairs {
		key := string(pair.Key)
		if strings.HasPrefix(key, "validator/") && strings.HasSuffix(key, "/power") {
			validatorID, ok := parseValidatorPowerKey(key)
			if !ok {
				continue
			}
			power, err := decodeUint64(pair.Value)
			if err != nil {
				return nil, nil, err
			}
			if power > 0 {
				validators = append(validators, validatorPowerRecord{validatorID: validatorID, power: power})
			}
			continue
		}
		if strings.HasPrefix(key, "stake/") {
			delegator, validatorID, ok := parseStakeKey(key)
			if !ok {
				continue
			}
			stake, err := decodeUint64(pair.Value)
			if err != nil {
				return nil, nil, err
			}
			if stake > 0 {
				delegations = append(delegations, delegationRecord{delegator: delegator, validatorID: validatorID, stake: stake})
			}
		}
	}
	return validators, delegations, nil
}

func delegationsForValidator(pairs []vexostore.KVPair, validatorID types.ValidatorID) []delegationRecord {
	delegations := make([]delegationRecord, 0)
	for _, pair := range pairs {
		key := string(pair.Key)
		if !strings.HasPrefix(key, "stake/") {
			continue
		}
		delegator, parsedValidatorID, ok := parseStakeKey(key)
		if !ok || parsedValidatorID != validatorID {
			continue
		}
		stake, err := decodeUint64(pair.Value)
		if err != nil || stake == 0 {
			continue
		}
		delegations = append(delegations, delegationRecord{
			delegator:   delegator,
			validatorID: parsedValidatorID,
			stake:       stake,
		})
	}
	return delegations
}

func totalValidatorPower(validators []validatorPowerRecord) (uint64, error) {
	var total uint64
	for _, validator := range validators {
		if total > ^uint64(0)-validator.power {
			return 0, ErrStakeOverflow
		}
		total += validator.power
	}
	return total, nil
}

func parseValidatorPowerKey(key string) (types.ValidatorID, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || parts[0] != "validator" || parts[1] == "" || parts[2] != "power" {
		return "", false
	}
	return types.ValidatorID(parts[1]), true
}

func parseStakeKey(key string) (types.Address, types.ValidatorID, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || parts[0] != "stake" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return types.Address(parts[1]), types.ValidatorID(parts[2]), true
}

func addReward(rewards map[string]uint64, delegator types.Address, validatorID types.ValidatorID, amount uint64) error {
	key := rewardMapKey(delegator, validatorID)
	if rewards[key] > ^uint64(0)-amount {
		return ErrStakeOverflow
	}
	rewards[key] += amount
	return nil
}

func rewardMapKey(delegator types.Address, validatorID types.ValidatorID) string {
	return string(delegator) + "\x00" + string(validatorID)
}

func splitRewardMapKey(key string) (types.Address, types.ValidatorID) {
	parts := strings.SplitN(key, "\x00", 2)
	return types.Address(parts[0]), types.ValidatorID(parts[1])
}

func proportionalShare(total uint64, part uint64, whole uint64) uint64 {
	if total == 0 || part == 0 || whole == 0 {
		return 0
	}
	high, low := bits.Mul64(total, part)
	share, _ := bits.Div64(high, low, whole)
	return share
}

func getUint64(ctx context.Context, store vexoapp.StateStore, key []byte) (uint64, error) {
	value, err := store.Get(ctx, ModuleName, key)
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if len(value) == 0 {
		return 0, nil
	}
	if len(value) != 8 {
		return 0, ErrInvalidStakeRecord
	}
	return binary.BigEndian.Uint64(value), nil
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

func encodeSlashMarker(receipt slashing.PenaltyReceipt) []byte {
	encoded := make([]byte, 16)
	binary.BigEndian.PutUint64(encoded[:8], uint64(receipt.PreviousPower))
	binary.BigEndian.PutUint64(encoded[8:], uint64(receipt.RemainingPower))
	return encoded
}

func stakingSlashKey(evidence slashing.Evidence) []byte {
	key := vexostore.EvidenceKey(evidence)
	if key == "" {
		return nil
	}
	return []byte("slash/" + key)
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

func stakeKey(delegator types.Address, validatorID types.ValidatorID) []byte {
	return []byte("stake/" + string(delegator) + "/" + string(validatorID))
}

func validatorPowerKey(validatorID types.ValidatorID) []byte {
	return []byte("validator/" + string(validatorID) + "/power")
}

func validatorKeyKey(validatorID types.ValidatorID) []byte {
	return []byte("validator/" + string(validatorID) + "/public_key")
}

func jailKey(validatorID types.ValidatorID) []byte {
	return []byte("validator/" + string(validatorID) + "/jail")
}

func unbondingKey(delegator types.Address, validatorID types.ValidatorID) []byte {
	return []byte("unbonding/" + string(delegator) + "/" + string(validatorID))
}

func rewardKey(delegator types.Address, validatorID types.ValidatorID) []byte {
	return []byte("rewards/" + string(delegator) + "/" + string(validatorID))
}

func commissionKey(validatorID types.ValidatorID) []byte {
	return []byte("validator/" + string(validatorID) + "/commission_bps")
}
