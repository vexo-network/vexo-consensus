package params

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/events"
	"github.com/vexo-network/vexo-consensus/kvbatch"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

const Namespace = "params"

var (
	ErrInvalidParam = errors.New("invalid parameter")
	ErrInvalidTx    = errors.New("invalid params transaction")
	ErrUnauthorized = errors.New("unauthorized parameter change")
	ErrStoreMissing = errors.New("parameter store is required")
)

type Param struct {
	Module    string `json:"module"`
	Key       string `json:"key"`
	Value     []byte `json:"value"`
	Authority string `json:"authority,omitempty"`
	Version   uint64 `json:"version"`
}

type Change struct {
	Module    string `json:"module"`
	Key       string `json:"key"`
	Value     []byte `json:"value"`
	Authority string `json:"authority,omitempty"`
}

type Keeper struct {
	store vexoapp.StateStore
}

func NewKeeper(store vexoapp.StateStore) *Keeper {
	return &Keeper{store: store}
}

func (keeper *Keeper) Get(ctx context.Context, module string, key string) (Param, bool, error) {
	if keeper == nil || keeper.store == nil {
		return Param{}, false, ErrStoreMissing
	}
	if module == "" || key == "" {
		return Param{}, false, ErrInvalidParam
	}
	encoded, err := keeper.store.Get(ctx, Namespace, paramKey(module, key))
	if errors.Is(err, store.ErrKeyNotFound) {
		return Param{}, false, nil
	}
	if err != nil {
		return Param{}, false, err
	}
	var param Param
	if err := json.Unmarshal(encoded, &param); err != nil {
		return Param{}, false, err
	}
	return param, true, nil
}

func (keeper *Keeper) Set(ctx context.Context, change Change) (Param, error) {
	if keeper == nil || keeper.store == nil {
		return Param{}, ErrStoreMissing
	}
	if change.Module == "" || change.Key == "" {
		return Param{}, ErrInvalidParam
	}
	previous, found, err := keeper.Get(ctx, change.Module, change.Key)
	if err != nil {
		return Param{}, err
	}
	if found && previous.Authority != "" && previous.Authority != change.Authority {
		return Param{}, ErrUnauthorized
	}
	param := Param{
		Module:    change.Module,
		Key:       change.Key,
		Value:     append([]byte(nil), change.Value...),
		Authority: change.Authority,
		Version:   previous.Version + 1,
	}
	encoded, err := json.Marshal(param)
	if err != nil {
		return Param{}, err
	}
	if err := keeper.store.Set(ctx, Namespace, paramKey(param.Module, param.Key), encoded); err != nil {
		return Param{}, err
	}
	return param, nil
}

func (keeper *Keeper) Apply(ctx context.Context, changes []Change) error {
	if keeper == nil || keeper.store == nil {
		return ErrStoreMissing
	}
	writes := make([]kvbatch.KVWrite, 0, len(changes))
	for _, change := range changes {
		if change.Module == "" || change.Key == "" {
			return ErrInvalidParam
		}
		previous, found, err := keeper.Get(ctx, change.Module, change.Key)
		if err != nil {
			return err
		}
		if found && previous.Authority != "" && previous.Authority != change.Authority {
			return ErrUnauthorized
		}
		param := Param{
			Module:    change.Module,
			Key:       change.Key,
			Value:     append([]byte(nil), change.Value...),
			Authority: change.Authority,
			Version:   previous.Version + 1,
		}
		encoded, err := json.Marshal(param)
		if err != nil {
			return err
		}
		writes = append(writes, kvbatch.KVWrite{Namespace: Namespace, Key: paramKey(param.Module, param.Key), Value: encoded})
	}
	if batch, ok := keeper.store.(kvbatch.BatchKVStore); ok {
		return batch.SetBatch(ctx, writes)
	}
	for _, write := range writes {
		if err := keeper.store.Set(ctx, write.Namespace, write.Key, write.Value); err != nil {
			return err
		}
	}
	return nil
}

func (keeper *Keeper) Export(ctx context.Context) ([]Param, error) {
	if keeper == nil || keeper.store == nil {
		return nil, ErrStoreMissing
	}
	snapshot, ok := keeper.store.(store.SnapshotKVStore)
	if !ok {
		return nil, ErrStoreMissing
	}
	pairs, err := snapshot.ExportNamespace(ctx, Namespace)
	if err != nil {
		return nil, err
	}
	params := make([]Param, 0, len(pairs))
	for _, pair := range pairs {
		var param Param
		if err := json.Unmarshal(pair.Value, &param); err != nil {
			return nil, err
		}
		params = append(params, param)
	}
	sort.Slice(params, func(left, right int) bool {
		if params[left].Module != params[right].Module {
			return params[left].Module < params[right].Module
		}
		return params[left].Key < params[right].Key
	})
	return params, nil
}

type Module struct {
	defaults []Change
	pending  []Change
}

func NewModule(defaults []Change) *Module {
	return &Module{defaults: append([]Change(nil), defaults...)}
}

func (module *Module) Name() string { return Namespace }

func (module *Module) InitGenesis(ctx vexoapp.Context, genesis vexoapp.GenesisState) error {
	if ctx.Store == nil {
		return nil
	}
	keeper := NewKeeper(ctx.Store)
	if len(module.defaults) > 0 {
		if err := keeper.Apply(ctx.GoContext(), module.defaults); err != nil {
			return err
		}
	}
	for rawKey, rawValue := range genesis {
		if !strings.HasPrefix(rawKey, Namespace+":") {
			continue
		}
		parts := strings.SplitN(rawKey, ":", 3)
		if len(parts) != 3 {
			return ErrInvalidParam
		}
		if _, err := keeper.Set(ctx.GoContext(), Change{Module: parts[1], Key: parts[2], Value: rawValue}); err != nil {
			return err
		}
	}
	return nil
}

func (module *Module) CloneModule() vexoapp.Module {
	return &Module{defaults: append([]Change(nil), module.defaults...)}
}

func (module *Module) BeginBlock(ctx vexoapp.Context, header types.Header) error {
	module.pending = module.pending[:0]
	return nil
}

func (module *Module) DeliverTx(ctx vexoapp.Context, tx types.Tx) types.Result {
	if ctx.Store == nil {
		return types.Result{Code: 1, Log: ErrStoreMissing.Error()}
	}
	parts := strings.Split(string(tx), ":")
	if len(parts) != 6 || parts[0] != Namespace || parts[1] != "set" {
		return types.Result{Code: 2, Log: ErrInvalidTx.Error()}
	}
	value, err := base64.StdEncoding.DecodeString(parts[5])
	if err != nil {
		return types.Result{Code: 3, Log: err.Error()}
	}
	change := Change{Authority: parts[2], Module: parts[3], Key: parts[4], Value: value}
	param, err := NewKeeper(ctx.Store).Set(ctx.GoContext(), change)
	if err != nil {
		return types.Result{Code: 4, Log: err.Error()}
	}
	module.pending = append(module.pending, change)
	return types.Result{Data: []byte(strconv.FormatUint(param.Version, 10))}
}

func (module *Module) EndBlock(ctx vexoapp.Context) error { return nil }

func (module *Module) Events(ctx vexoapp.Context, tx types.Tx, result types.Result) []events.Event {
	if result.Code != 0 {
		return nil
	}
	parts := strings.Split(string(tx), ":")
	if len(parts) != 6 || parts[0] != Namespace || parts[1] != "set" {
		return nil
	}
	return []events.Event{
		{
			Type: "param_set",
			Attributes: []events.Attribute{
				{Key: "module", Value: parts[3], Index: true},
				{Key: "key", Value: parts[4], Index: true},
				{Key: "authority", Value: parts[2], Index: true},
			},
		},
	}
}

func (module *Module) Query(ctx vexoapp.Context, req vexoapp.QueryRequest) vexoapp.QueryResponse {
	if ctx.Store == nil {
		return vexoapp.QueryResponse{Code: 1, Log: ErrStoreMissing.Error()}
	}
	keeper := NewKeeper(ctx.Store)
	if len(req.Path) == 3 && req.Path[0] == "param" {
		param, found, err := keeper.Get(ctx.GoContext(), req.Path[1], req.Path[2])
		if err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
		if !found {
			return vexoapp.QueryResponse{Code: 3, Log: store.ErrKeyNotFound.Error()}
		}
		encoded, err := json.Marshal(param)
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: encoded}
	}
	if len(req.Path) == 1 && req.Path[0] == "params" {
		params, err := keeper.Export(ctx.GoContext())
		if err != nil {
			return vexoapp.QueryResponse{Code: 2, Log: err.Error()}
		}
		encoded, err := json.Marshal(params)
		if err != nil {
			return vexoapp.QueryResponse{Code: 4, Log: err.Error()}
		}
		return vexoapp.QueryResponse{Value: encoded}
	}
	return vexoapp.QueryResponse{Code: 5, Log: ErrInvalidParam.Error()}
}

func paramKey(module string, key string) []byte {
	return []byte(module + "/" + key)
}
