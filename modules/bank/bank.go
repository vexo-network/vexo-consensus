package bank

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
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
		balance, err := strconv.ParseUint(string(rawBalance), 10, 64)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidGenesisBalance, rawAddress)
		}
		if err := setBalance(context.Background(), ctx.Store, types.Address(address), balance); err != nil {
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
		amount, err := parseAmount(parts[3])
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
		amount, err := parseAmount(parts[4])
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
	balance, err := Balance(ctx.GoContext(), ctx.Store, types.Address(req.Path[1]))
	if err != nil {
		return vexoapp.QueryResponse{Code: 3, Log: err.Error()}
	}
	return vexoapp.QueryResponse{Value: []byte(strconv.FormatUint(balance, 10))}
}

func Balance(ctx context.Context, store vexoapp.StateStore, address types.Address) (uint64, error) {
	if store == nil {
		return 0, errors.New("missing bank store")
	}
	value, err := store.Get(ctx, ModuleName, balanceKey(address))
	if errors.Is(err, vexostore.ErrKeyNotFound) {
		value, err = store.Get(ctx, ModuleName, []byte(address))
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
		return 0, ErrInvalidGenesisBalance
	}
	return binary.BigEndian.Uint64(value), nil
}

func mint(ctx context.Context, store vexoapp.StateStore, to types.Address, amount uint64) error {
	if to == "" || amount == 0 {
		return ErrInvalidBankTx
	}
	balance, err := Balance(ctx, store, to)
	if err != nil {
		return err
	}
	if balance > ^uint64(0)-amount {
		return ErrBalanceOverflow
	}
	return setBalance(ctx, store, to, balance+amount)
}

func send(ctx context.Context, store vexoapp.StateStore, from types.Address, to types.Address, amount uint64) error {
	if from == "" || to == "" || amount == 0 {
		return ErrInvalidBankTx
	}
	fromBalance, err := Balance(ctx, store, from)
	if err != nil {
		return err
	}
	if fromBalance < amount {
		return ErrInsufficientFunds
	}
	toBalance, err := Balance(ctx, store, to)
	if err != nil {
		return err
	}
	if toBalance > ^uint64(0)-amount {
		return ErrBalanceOverflow
	}
	if batchStore, ok := store.(kvbatch.BatchKVStore); ok {
		return batchStore.SetBatch(ctx, []kvbatch.KVWrite{
			{Namespace: ModuleName, Key: balanceKey(from), Value: encodeBalance(fromBalance - amount)},
			{Namespace: ModuleName, Key: balanceKey(to), Value: encodeBalance(toBalance + amount)},
		})
	}
	if err := setBalance(ctx, store, from, fromBalance-amount); err != nil {
		return err
	}
	return setBalance(ctx, store, to, toBalance+amount)
}

func setBalance(ctx context.Context, store vexoapp.StateStore, address types.Address, balance uint64) error {
	return store.Set(ctx, ModuleName, balanceKey(address), encodeBalance(balance))
}

func encodeBalance(balance uint64) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, balance)
	return encoded
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
	amount, err := strconv.ParseUint(value, 10, 64)
	if err != nil || amount == 0 {
		return 0, ErrInvalidBankTx
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
