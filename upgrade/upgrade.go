package upgrade

import (
	"context"
	"errors"
	"sort"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrMissingUpgradeName    = errors.New("upgrade name is required")
	ErrMissingUpgradeHeight  = errors.New("upgrade height is required")
	ErrRollbackRequired      = errors.New("upgrade rollback required")
	ErrMigrationNotFound     = errors.New("migration not found")
	ErrStoreVersionMismatch  = errors.New("store schema version mismatch")
	ErrConfigVersionMismatch = errors.New("config schema version mismatch")
)

type Plan struct {
	Name               string       `json:"name"`
	Height             types.Height `json:"height"`
	BinaryVersion      string       `json:"binary_version"`
	ConfigSchemaFrom   uint64       `json:"config_schema_from"`
	ConfigSchemaTo     uint64       `json:"config_schema_to"`
	StoreSchemaFrom    uint64       `json:"store_schema_from"`
	StoreSchemaTo      uint64       `json:"store_schema_to"`
	AppStateSchemaFrom uint64       `json:"app_state_schema_from"`
	AppStateSchemaTo   uint64       `json:"app_state_schema_to"`
	GovernanceProposal string       `json:"governance_proposal,omitempty"`
	RollbackBinary     string       `json:"rollback_binary,omitempty"`
}

type State struct {
	Height              types.Height `json:"height"`
	BinaryVersion       string       `json:"binary_version"`
	ConfigSchemaVersion uint64       `json:"config_schema_version"`
	StoreSchemaVersion  uint64       `json:"store_schema_version"`
	AppStateVersion     uint64       `json:"app_state_version"`
}

type Migration struct {
	From uint64
	To   uint64
	Run  func(context.Context) error
}

type Registry struct {
	config   map[uint64]Migration
	store    map[uint64]Migration
	appState map[uint64]Migration
}

type Result struct {
	Applied             bool         `json:"applied"`
	RollbackRequired    bool         `json:"rollback_required"`
	Height              types.Height `json:"height"`
	BinaryVersion       string       `json:"binary_version"`
	ConfigSchemaVersion uint64       `json:"config_schema_version"`
	StoreSchemaVersion  uint64       `json:"store_schema_version"`
	AppStateVersion     uint64       `json:"app_state_version"`
}

func NewRegistry() *Registry {
	return &Registry{
		config:   make(map[uint64]Migration),
		store:    make(map[uint64]Migration),
		appState: make(map[uint64]Migration),
	}
}

func (registry *Registry) RegisterConfig(migration Migration) {
	registry.config[migration.From] = migration
}

func (registry *Registry) RegisterStore(migration Migration) {
	registry.store[migration.From] = migration
}

func (registry *Registry) RegisterAppState(migration Migration) {
	registry.appState[migration.From] = migration
}

func ValidatePlan(plan Plan) error {
	if plan.Name == "" {
		return ErrMissingUpgradeName
	}
	if plan.Height == 0 {
		return ErrMissingUpgradeHeight
	}
	return nil
}

func Apply(ctx context.Context, state State, plan Plan, registry *Registry) (Result, error) {
	if err := ValidatePlan(plan); err != nil {
		return Result{}, err
	}
	result := Result{
		Height:              state.Height,
		BinaryVersion:       state.BinaryVersion,
		ConfigSchemaVersion: state.ConfigSchemaVersion,
		StoreSchemaVersion:  state.StoreSchemaVersion,
		AppStateVersion:     state.AppStateVersion,
	}
	if state.Height < plan.Height {
		return result, nil
	}
	if state.ConfigSchemaVersion != plan.ConfigSchemaFrom {
		return result, ErrConfigVersionMismatch
	}
	if state.StoreSchemaVersion != plan.StoreSchemaFrom {
		return result, ErrStoreVersionMismatch
	}
	if registry == nil {
		registry = NewRegistry()
	}
	if err := runPath(ctx, state.ConfigSchemaVersion, plan.ConfigSchemaTo, registry.config); err != nil {
		result.RollbackRequired = true
		return result, errors.Join(ErrRollbackRequired, err)
	}
	result.ConfigSchemaVersion = plan.ConfigSchemaTo
	if err := runPath(ctx, state.StoreSchemaVersion, plan.StoreSchemaTo, registry.store); err != nil {
		result.RollbackRequired = true
		return result, errors.Join(ErrRollbackRequired, err)
	}
	result.StoreSchemaVersion = plan.StoreSchemaTo
	if err := runPath(ctx, state.AppStateVersion, plan.AppStateSchemaTo, registry.appState); err != nil {
		result.RollbackRequired = true
		return result, errors.Join(ErrRollbackRequired, err)
	}
	result.AppStateVersion = plan.AppStateSchemaTo
	result.BinaryVersion = plan.BinaryVersion
	result.Height = state.Height
	result.Applied = true
	return result, nil
}

func runPath(ctx context.Context, from uint64, to uint64, migrations map[uint64]Migration) error {
	if from == to {
		return nil
	}
	keys := make([]uint64, 0, len(migrations))
	for key := range migrations {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left int, right int) bool { return keys[left] < keys[right] })
	current := from
	for current < to {
		migration, found := migrations[current]
		if !found || migration.To <= current || migration.To > to {
			return ErrMigrationNotFound
		}
		if migration.Run != nil {
			if err := migration.Run(ctx); err != nil {
				return err
			}
		}
		current = migration.To
	}
	return nil
}
