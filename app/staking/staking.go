package staking

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "staking"
const bankNamespace = "bank"
const defaultUnbondingDelay types.Height = 1209600

var (
	ErrInvalidStakingTx     = errors.New("invalid staking transaction")
	ErrInsufficientBalance  = errors.New("insufficient balance for delegation")
	ErrInsufficientStake    = errors.New("insufficient delegated stake")
	ErrInvalidStakeRecord   = errors.New("invalid stake record")
	ErrMissingValidatorKey  = errors.New("validator public key is required")
	ErrStakingStoreRequired = errors.New("missing staking store")
)

type Module struct {
	unbondingDelay types.Height
	pending        []types.ValidatorUpdate
}

func NewModule() *Module {
	return &Module{unbondingDelay: defaultUnbondingDelay}
}

func NewModuleWithUnbondingDelay(delay types.Height) *Module {
	if delay == 0 {
		delay = defaultUnbondingDelay
	}
	return &Module{unbondingDelay: delay}
}

func (module *Module) Name() string {
	return ModuleName
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
		if err := setStake(context.Background(), ctx.Store, types.Address(parts[2]), types.ValidatorID(parts[3]), amount); err != nil {
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
		amount, err := parseAmount(parts[4])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		publicKey, err := base64.StdEncoding.DecodeString(parts[5])
		if err != nil || len(publicKey) == 0 {
			return types.Result{Code: 3, Log: ErrMissingValidatorKey.Error()}
		}
		update, err := module.delegate(context.Background(), ctx.Store, types.Address(parts[2]), types.ValidatorID(parts[3]), amount, types.PublicKey(publicKey))
		if err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		module.pending = append(module.pending, update)
		return types.Result{}
	case len(parts) == 5 && parts[1] == "undelegate":
		amount, err := parseAmount(parts[4])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		update, err := module.undelegate(context.Background(), ctx.Store, ctx.Height, types.Address(parts[2]), types.ValidatorID(parts[3]), amount)
		if err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		module.pending = append(module.pending, update)
		return types.Result{}
	case len(parts) == 3 && parts[1] == "unjail":
		if err := ctx.Store.Delete(context.Background(), ModuleName, jailKey(types.ValidatorID(parts[2]))); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		return types.Result{}
	default:
		return types.Result{Code: 2, Log: ErrInvalidStakingTx.Error()}
	}
}

func (module *Module) EndBlock(ctx vexoapp.Context) error {
	return nil
}

func (module *Module) ValidatorUpdates(ctx vexoapp.Context) []types.ValidatorUpdate {
	return append([]types.ValidatorUpdate(nil), module.pending...)
}

func (module *Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrStakingStoreRequired.Error()}
	}
	if len(req.Path) == 3 && req.Path[0] == "stake" {
		amount, err := Stake(context.Background(), ctx.Store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(amount, 10))}
	}
	if len(req.Path) == 2 && req.Path[0] == "validator" {
		power, err := ValidatorPower(context.Background(), ctx.Store, types.ValidatorID(req.Path[1]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(power, 10))}
	}
	if len(req.Path) == 3 && req.Path[0] == "unbonding" {
		releaseHeight, err := UnbondingReleaseHeight(context.Background(), ctx.Store, types.Address(req.Path[1]), types.ValidatorID(req.Path[2]))
		if err != nil {
			return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(uint64(releaseHeight), 10))}
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
	if err := setBankBalance(ctx, store, delegator, balance-amount); err != nil {
		return types.ValidatorUpdate{}, err
	}
	currentStake, err := Stake(ctx, store, delegator, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	if err := setStake(ctx, store, delegator, validatorID, currentStake+amount); err != nil {
		return types.ValidatorUpdate{}, err
	}
	currentPower, err := ValidatorPower(ctx, store, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	newPower := currentPower + amount
	if err := setValidatorPower(ctx, store, validatorID, newPower); err != nil {
		return types.ValidatorUpdate{}, err
	}
	if err := store.Set(ctx, ModuleName, validatorKeyKey(validatorID), publicKey); err != nil {
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
	if err := setStake(ctx, store, delegator, validatorID, currentStake-amount); err != nil {
		return types.ValidatorUpdate{}, err
	}
	currentPower, err := ValidatorPower(ctx, store, validatorID)
	if err != nil {
		return types.ValidatorUpdate{}, err
	}
	newPower := currentPower - amount
	if err := setValidatorPower(ctx, store, validatorID, newPower); err != nil {
		return types.ValidatorUpdate{}, err
	}
	if err := setUnbondingReleaseHeight(ctx, store, delegator, validatorID, height+module.unbondingDelay); err != nil {
		return types.ValidatorUpdate{}, err
	}
	publicKey, _ := store.Get(ctx, ModuleName, validatorKeyKey(validatorID))
	return types.ValidatorUpdate{
		ID:          validatorID,
		Address:     types.Address(validatorID),
		PublicKey:   append(types.PublicKey(nil), publicKey...),
		VotingPower: types.VotingPower(newPower),
		Stake:       newPower,
		Metadata:    map[string]string{"source": "staking"},
	}, nil
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

func bankBalance(ctx context.Context, store vexoapp.StateStore, address types.Address) (uint64, error) {
	value, err := store.Get(ctx, bankNamespace, []byte(address))
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

func setBankBalance(ctx context.Context, store vexoapp.StateStore, address types.Address, amount uint64) error {
	return setUint64(ctx, store, bankNamespace, []byte(address), amount)
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

func setUint64(ctx context.Context, store vexoapp.StateStore, namespace string, key []byte, amount uint64) error {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], amount)
	return store.Set(ctx, namespace, key, encoded[:])
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
