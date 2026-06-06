package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrMissingUpgradeName    = errors.New("upgrade name is required")
	ErrMissingUpgradeHeight  = errors.New("upgrade height is required")
	ErrRollbackRequired      = errors.New("upgrade rollback required")
	ErrMigrationNotFound     = errors.New("migration not found")
	ErrStoreVersionMismatch  = errors.New("store schema version mismatch")
	ErrConfigVersionMismatch = errors.New("config schema version mismatch")
	ErrAppVersionMismatch    = errors.New("app state schema version mismatch")
)

type Plan struct {
	Name                string       `json:"name"`
	Height              types.Height `json:"height"`
	BinaryVersion       string       `json:"binary_version"`
	ConfigSchemaFrom    uint64       `json:"config_schema_from"`
	ConfigSchemaTo      uint64       `json:"config_schema_to"`
	StoreSchemaFrom     uint64       `json:"store_schema_from"`
	StoreSchemaTo       uint64       `json:"store_schema_to"`
	AppStateSchemaFrom  uint64       `json:"app_state_schema_from"`
	AppStateSchemaTo    uint64       `json:"app_state_schema_to"`
	GovernanceProposal  string       `json:"governance_proposal,omitempty"`
	RollbackBinary      string       `json:"rollback_binary,omitempty"`
	AllowNoopMigrations bool         `json:"allow_noop_migrations,omitempty"`
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

type ExecutionStatus string

const (
	ExecutionPending          ExecutionStatus = "pending"
	ExecutionApplied          ExecutionStatus = "applied"
	ExecutionRollbackRequired ExecutionStatus = "rollback_required"
)

type ExecutionRecord struct {
	Plan      Plan            `json:"plan"`
	Before    State           `json:"before"`
	Result    Result          `json:"result"`
	Status    ExecutionStatus `json:"status"`
	Error     string          `json:"error,omitempty"`
	UpdatedAt int64           `json:"updated_at"`
}

type Recorder interface {
	Load(ctx context.Context, name string) (ExecutionRecord, bool, error)
	Save(ctx context.Context, record ExecutionRecord) error
}

type PlanStore interface {
	SaveUpgradePlan(ctx context.Context, plan Plan) error
	UpgradePlanByHeight(ctx context.Context, height types.Height) (Plan, bool, error)
	UpgradePlanByName(ctx context.Context, name string) (Plan, bool, error)
}

type Executor struct {
	Registry *Registry
	Recorder Recorder
	Now      func() time.Time
}

func NewRegistry() *Registry {
	return &Registry{
		config:   make(map[uint64]Migration),
		store:    make(map[uint64]Migration),
		appState: make(map[uint64]Migration),
	}
}

func NewExecutor(registry *Registry, recorder Recorder) Executor {
	if registry == nil {
		registry = NewRegistry()
	}
	return Executor{Registry: registry, Recorder: recorder, Now: time.Now}
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
	if state.AppStateVersion != plan.AppStateSchemaFrom {
		return result, ErrAppVersionMismatch
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

func (executor Executor) Execute(ctx context.Context, state State, plan Plan) (ExecutionRecord, error) {
	if err := ValidatePlan(plan); err != nil {
		return ExecutionRecord{}, err
	}
	if executor.Registry == nil {
		executor.Registry = NewRegistry()
	}
	if executor.Now == nil {
		executor.Now = time.Now
	}
	if executor.Recorder != nil {
		record, found, err := executor.Recorder.Load(ctx, plan.Name)
		if err != nil {
			return ExecutionRecord{}, err
		}
		if found && record.Status == ExecutionApplied {
			return record, nil
		}
		if found && record.Status == ExecutionRollbackRequired {
			return record, ErrRollbackRequired
		}
	}
	result, err := Apply(ctx, state, plan, executor.Registry)
	record := ExecutionRecord{
		Plan:      plan,
		Before:    state,
		Result:    result,
		Status:    ExecutionPending,
		UpdatedAt: executor.Now().Unix(),
	}
	if result.Applied {
		record.Status = ExecutionApplied
	}
	if result.RollbackRequired {
		record.Status = ExecutionRollbackRequired
	}
	if err != nil {
		record.Error = err.Error()
	}
	if executor.Recorder != nil {
		if saveErr := executor.Recorder.Save(ctx, record); saveErr != nil {
			return record, saveErr
		}
	}
	return record, err
}

type MemoryRecorder struct {
	Records map[string]ExecutionRecord
}

func NewMemoryRecorder() *MemoryRecorder {
	return &MemoryRecorder{Records: make(map[string]ExecutionRecord)}
}

func (recorder *MemoryRecorder) Load(ctx context.Context, name string) (ExecutionRecord, bool, error) {
	select {
	case <-ctx.Done():
		return ExecutionRecord{}, false, ctx.Err()
	default:
	}
	if recorder.Records == nil {
		return ExecutionRecord{}, false, nil
	}
	record, found := recorder.Records[name]
	return record, found, nil
}

func (recorder *MemoryRecorder) Save(ctx context.Context, record ExecutionRecord) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if recorder.Records == nil {
		recorder.Records = make(map[string]ExecutionRecord)
	}
	recorder.Records[record.Plan.Name] = record
	return nil
}

type JSONFileRecorder struct {
	Path string
}

func (recorder JSONFileRecorder) Load(ctx context.Context, name string) (ExecutionRecord, bool, error) {
	records, err := recorder.loadAll(ctx)
	if err != nil {
		return ExecutionRecord{}, false, err
	}
	record, found := records[name]
	return record, found, nil
}

func (recorder JSONFileRecorder) Save(ctx context.Context, record ExecutionRecord) error {
	records, err := recorder.loadAll(ctx)
	if err != nil {
		return err
	}
	records[record.Plan.Name] = record
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(recorder.Path), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(recorder.Path, append(data, '\n'), 0o644)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tempPath := path + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return nil
	}
	defer dir.Close()
	return dir.Sync()
}

func (recorder JSONFileRecorder) loadAll(ctx context.Context) (map[string]ExecutionRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if recorder.Path == "" {
		return nil, errors.New("upgrade record path is required")
	}
	data, err := os.ReadFile(recorder.Path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]ExecutionRecord), nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]ExecutionRecord), nil
	}
	var records map[string]ExecutionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	if records == nil {
		records = make(map[string]ExecutionRecord)
	}
	return records, nil
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
