package bank

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/economics"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	vexostore "github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const ModuleName = "bank"

const (
	mintGasCost uint64 = 10
	sendGasCost uint64 = 10
)

var (
	ErrInvalidGenesisBalance = errors.New("invalid genesis balance")
	ErrInvalidBankTx         = errors.New("invalid bank transaction")
	ErrInsufficientFunds     = errors.New("insufficient funds")
	ErrBalanceOverflow       = errors.New("balance overflow")
	ErrUnauthorizedMint      = errors.New("unauthorized mint")
)

type Module struct {
	mintAuthority types.Address
}

func NewModule() Module {
	return Module{}
}

func NewModuleWithMintAuthority(authority types.Address) Module {
	return Module{mintAuthority: authority}
}

func (module Module) CloneModule() vexoapp.Module {
	return Module{mintAuthority: module.mintAuthority}
}

func (Module) Name() string {
	return ModuleName
}

func (module Module) MintAuthority() types.Address {
	return module.mintAuthority
}

func (Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	if ctx.Store == nil {
		return nil
	}
	for rawAddress, rawBalance := range genesis {
		if !strings.HasPrefix(rawAddress, ModuleName+":") {
			continue
		}
		address := strings.TrimPrefix(rawAddress, ModuleName+":")
		balance, err := parseAmountBig(string(rawBalance))
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidGenesisBalance, rawAddress)
		}
		if err := setBalanceBig(context.Background(), ctx.Store, types.Address(address), balance); err != nil {
			return err
		}
	}
	return nil
}

func (Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	return nil
}

func (module Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: "missing bank store"}
	}
	parts := bankTxParts(tx)
	if len(parts) == 0 || parts[0] != ModuleName {
		return types.Result{Code: 2, Log: ErrInvalidBankTx.Error()}
	}
	switch {
	case len(parts) == 4 && parts[1] == "mint":
		if err := ctx.ConsumeGas(mintGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		amount, err := parseAmountBig(parts[3])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		if err := module.authorizeMint(tx); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		if err := mint(ctx.GoContext(), ctx.Store, types.Address(parts[2]), amount); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		return types.Result{}
	case len(parts) == 5 && parts[1] == "send":
		if err := ctx.ConsumeGas(sendGasCost); err != nil {
			return types.Result{Code: 5, Log: err.Error()}
		}
		amount, err := parseAmountBig(parts[4])
		if err != nil {
			return types.Result{Code: 3, Log: err.Error()}
		}
		if err := send(ctx.GoContext(), ctx.Store, types.Address(parts[2]), types.Address(parts[3]), amount); err != nil {
			return types.Result{Code: 4, Log: err.Error()}
		}
		return types.Result{}
	default:
		return types.Result{Code: 2, Log: ErrInvalidBankTx.Error()}
	}
}

func (Module) EndBlock(ctx vexoapp.Context) error {
	return nil
}

func (Module) EstimateGas(ctx vexoapp.Context, tx types.Tx) (uint64, error) {
	parts := bankTxParts(tx)
	switch {
	case len(parts) == 4 && parts[1] == "mint":
		return mintGasCost, nil
	case len(parts) == 5 && parts[1] == "send":
		return sendGasCost, nil
	default:
		return 0, ErrInvalidBankTx
	}
}

func (module Module) authorizeMint(tx types.Tx) error {
	if module.mintAuthority == "" {
		return nil
	}
	signer, found := vexoapp.TxTag(tx, "signer")
	if !found || types.Address(signer) != module.mintAuthority {
		return ErrUnauthorizedMint
	}
	return nil
}

func (Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: 1, Log: "missing bank store"}
	}
	if len(req.Path) != 2 || req.Path[0] != "balance" || req.Path[1] == "" {
		return vexoapp.QueryResponse{Code: 2, Log: "invalid bank query"}
	}
	balance, err := BalanceBig(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]))
	if err != nil {
		return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: []byte(balance.String())}
}

func Balance(ctx context.Context, store vexoapp.StateStore, address types.Address) (uint64, error) {
	balance, err := BalanceBig(ctx, store, address)
	if err != nil {
		return 0, err
	}
	if !balance.IsUint64() {
		return 0, ErrBalanceOverflow
	}
	return balance.Uint64(), nil
}

func BalanceBig(ctx context.Context, store vexoapp.StateStore, address types.Address) (*big.Int, error) {
	if store == nil {
		return nil, errors.New("missing bank store")
	}
	value, err := store.Get(ctx, ModuleName, balanceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		value, err = store.Get(ctx, ModuleName, []byte(address))
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
		return nil, ErrInvalidGenesisBalance
	}
	return new(big.Int).SetBytes(value), nil
}

func mint(ctx context.Context, store vexoapp.StateStore, to types.Address, amount *big.Int) error {
	if to == "" || amount == nil || amount.Sign() <= 0 {
		return ErrInvalidBankTx
	}
	balance, err := BalanceBig(ctx, store, to)
	if err != nil {
		return err
	}
	next := new(big.Int).Add(balance, amount)
	if next.BitLen() > 256 {
		return ErrBalanceOverflow
	}
	return setBalanceBig(ctx, store, to, next)
}

func send(ctx context.Context, store vexoapp.StateStore, from types.Address, to types.Address, amount *big.Int) error {
	if from == "" || to == "" || amount == nil || amount.Sign() <= 0 {
		return ErrInvalidBankTx
	}
	fromBalance, err := BalanceBig(ctx, store, from)
	if err != nil {
		return err
	}
	if fromBalance.Cmp(amount) < 0 {
		return ErrInsufficientFunds
	}
	toBalance, err := BalanceBig(ctx, store, to)
	if err != nil {
		return err
	}
	nextToBalance := new(big.Int).Add(toBalance, amount)
	if nextToBalance.BitLen() > 256 {
		return ErrBalanceOverflow
	}
	nextFromBalance := new(big.Int).Sub(fromBalance, amount)
	if batchStore, ok := store.(kvbatch.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, []kvbatch.KVWrite{
			{Namespace: ModuleName, Key: balanceKey(from), Value: encodeBalanceBig(nextFromBalance)},
			{Namespace: ModuleName, Key: balanceKey(to), Value: encodeBalanceBig(nextToBalance)},
		})
	}
	if err := setBalanceBig(ctx, store, from, nextFromBalance); err != nil {
		return err
	}
	return setBalanceBig(ctx, store, to, nextToBalance)
}

func setBalance(ctx context.Context, store vexoapp.StateStore, address types.Address, balance uint64) error {
	return setBalanceBig(ctx, store, address, new(big.Int).SetUint64(balance))
}

func encodeBalance(balance uint64) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, balance)
	return encoded
}

func setBalanceBig(ctx context.Context, store vexoapp.StateStore, address types.Address, balance *big.Int) error {
	if balance == nil || balance.Sign() < 0 || balance.BitLen() > 256 {
		return ErrBalanceOverflow
	}
	return store.Set(ctx, ModuleName, balanceKey(address), encodeBalanceBig(balance))
}

func encodeBalanceBig(balance *big.Int) []byte {
	if balance == nil || balance.Sign() == 0 {
		return make([]byte, 8)
	}
	if balance.IsUint64() {
		return encodeBalance(balance.Uint64())
	}
	return balance.Bytes()
}

func balanceKey(address types.Address) []byte {
	raw := string(address)
	clean := strings.TrimPrefix(raw, "0x")
	if len(clean)%2 == 1 {
		clean = "0" + clean
	}
	decoded, err := hex.DecodeString(clean)
	if err != nil || len(decoded) > 20 || !strings.HasPrefix(raw, "0x") {
		return []byte(address)
	}
	padded := make([]byte, 20)
	copy(padded[20-len(decoded):], decoded)
	return []byte("0x" + hex.EncodeToString(padded))
}

func parseAmount(value string) (uint64, error) {
	amount, err := parseAmountBig(value)
	if err != nil || !amount.IsUint64() {
		return 0, ErrInvalidBankTx
	}
	return amount.Uint64(), nil
}

func parseAmountBig(value string) (*big.Int, error) {
	amount, err := economics.ParseAmountBig(value)
	if err != nil || amount.Sign() <= 0 || amount.BitLen() > 256 {
		return nil, ErrInvalidBankTx
	}
	return amount, nil
}

func bankTxParts(tx types.Tx) []string {
	rawParts := strings.Split(string(tx), ":")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if strings.Contains(part, "=") {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}
