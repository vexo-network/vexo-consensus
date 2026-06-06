package runtime

import (
	"context"
	"errors"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/economics"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
	"github.com/vexo-network/vexo-consensus/validator"
)

var ErrAtomicAppCommitUnavailable = errors.New("atomic app block commit store is required")

type Runtime struct {
	Config          config.Config
	App             app.Application
	Executor        consensus.ApplicationBlockExecutor
	Validators      validator.VersionedRegistry
	Committee       committee.Selector
	Mempool         *mempool.DAG
	Slashing        consensus.SlashingKeeper
	Governance      governance.OperationalKeeper
	P2PScore        *p2p.ScoreKeeper
	Crypto          crypto.RuntimeSuite
	Store           store.Store
	UpgradePlan     *upgrade.Plan
	UpgradeExecutor upgrade.Executor
	UpgradeState    upgrade.State
	UpgradeHalted   bool
	currentBaseFee  uint64
}

type stagedValidatorUpdateRegistry interface {
	StageValidatorUpdatesAt(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) (validator.Set, []store.KVWrite, error)
	CommitStagedValidatorUpdates(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) error
}

type stakingPenaltyApplier interface {
	ApplySlashingPenalty(ctx context.Context, store app.StateStore, receipt slashing.PenaltyReceipt) error
}

func New(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower) (*Runtime, error) {
	return NewWithStore(cfg, application, initialValidators, governancePower, nil)
}

func NewWithStore(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store) (*Runtime, error) {
	return NewWithStoreAndCryptoRegistry(cfg, application, initialValidators, governancePower, storage, crypto.NewRuntimeSuiteRegistry())
}

func NewWithStoreAndCryptoRegistry(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store, cryptoRegistry crypto.RuntimeSuiteRegistry) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	admission := validator.NewConfigurableAdmissionPolicy(cfg.Validator)
	var registry validator.VersionedRegistry
	if storage != nil {
		storeRegistry, err := validator.NewStoreRegistry(context.Background(), storage, admission, 1, initialValidators)
		if err != nil {
			return nil, err
		}
		registry = storeRegistry
	} else {
		memoryRegistry, err := validator.NewInMemoryRegistry(admission, initialValidators)
		if err != nil {
			return nil, err
		}
		registry = memoryRegistry
	}

	var vrf committee.VRF
	if cfg.Committee.Backend == committee.BackendVRF {
		loadedVRF, err := crypto.NewVRF(cfg.VRF)
		if err != nil {
			return nil, err
		}
		vrf = loadedVRF
	}
	selector, err := committee.NewSelector(cfg.Committee, vrf)
	if err != nil {
		return nil, err
	}
	if cfg.Crypto.Backend == config.CryptoBackendBLS && len(initialValidators) > 0 {
		credentials, err := crypto.BLSValidatorCredentialsFromValidators(initialValidators)
		if err != nil {
			return nil, err
		}
		cryptoRegistry = cryptoRegistry.RegisterBLSValidatorCredentials(credentials)
	}
	cryptoSuite, err := cryptoRegistry.NewRuntimeSuite(cfg.Crypto)
	if err != nil {
		return nil, err
	}

	fifoConfig := cfg.Mempool
	if fifoConfig.Author == "" && len(initialValidators) > 0 {
		fifoConfig.Author = initialValidators[0].ID
	}
	dag := mempool.NewDAG(mempool.NewFIFO(fifoConfig))
	if fifoConfig.WALPath != "" {
		durableDAG, err := mempool.OpenDurableDAG(context.Background(), fifoConfig.WALPath, mempool.NewFIFO(fifoConfig))
		if err != nil {
			return nil, err
		}
		dag = durableDAG
	}
	if storage != nil {
		if appRuntime, ok := application.(*app.Runtime); ok {
			appRuntime.WithStore(storage)
		}
	}
	slashingKeeper := consensus.SlashingKeeper(slashing.NewInMemoryKeeper(nil))
	if storage != nil {
		storeKeeper, err := slashing.NewStoreKeeper(storage, nil)
		if err != nil {
			return nil, err
		}
		slashingKeeper = storeKeeper
	}
	governanceKeeper := governance.OperationalKeeper(governance.NewInMemoryKeeper(cfg.Governance, governancePower))
	if storage != nil {
		storeKeeper, err := governance.NewStoreKeeper(storage, cfg.Governance, governancePower)
		if err != nil {
			return nil, err
		}
		governanceKeeper = storeKeeper
	}

	return &Runtime{
		Config:         cfg,
		App:            application,
		Executor:       consensus.ApplicationBlockExecutor{},
		Validators:     registry,
		Committee:      selector,
		Mempool:        dag,
		Slashing:       slashingKeeper,
		Governance:     governanceKeeper,
		P2PScore:       p2p.NewScoreKeeper(cfg.P2P),
		Crypto:         cryptoSuite,
		Store:          storage,
		currentBaseFee: cfg.Execution.BaseFee,
	}, nil
}

func (runtime *Runtime) WithUpgrade(plan upgrade.Plan, state upgrade.State, executor upgrade.Executor) *Runtime {
	runtime.UpgradePlan = &plan
	runtime.UpgradeState = state
	runtime.UpgradeExecutor = executor
	return runtime
}

func (runtime *Runtime) ExecuteBlock(ctx context.Context, block types.Block) (app.FinalizeBlockResponse, error) {
	if err := runtime.applyUpgradeHook(ctx, block.Header.Height); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	baseFee := runtime.CurrentBaseFee()
	runtime.setApplicationBaseFee(baseFee)
	if appRuntime, ok := runtime.App.(*app.Runtime); ok && runtime.Store != nil {
		if commitStore, ok := runtime.Store.(store.AppBlockCommitStore); ok {
			return runtime.executeBlockStaged(ctx, block, appRuntime, commitStore, baseFee)
		}
		return app.FinalizeBlockResponse{}, ErrAtomicAppCommitUnavailable
	}
	response, err := runtime.Executor.Execute(ctx, runtime.App, block)
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	nextBaseFee := runtime.NextBaseFee(response)
	validatorSetHash := block.Header.ValidatorSetHash
	if len(response.ValidatorUpdates) > 0 {
		if err := runtime.ApplyValidatorUpdatesAt(ctx, block.Header.Height+1, response.ValidatorUpdates); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
		validatorSet, err := runtime.Validators.ValidatorSet(ctx, block.Header.Height+1)
		if err != nil {
			return app.FinalizeBlockResponse{}, err
		}
		validatorSetHash = validatorSet.Hash()
	}
	if runtime.Store == nil {
		return response, nil
	}

	blockHash := consensus.HashBlock(block)
	stateRoots, err := runtime.moduleStateRoots(ctx, block.Header.Height)
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	blockRecord := store.BlockRecord{
		Block:      block,
		Hash:       blockHash,
		AppHash:    response.AppHash,
		StateRoots: stateRoots,
		TxResults:  cloneTxResults(response.Results),
	}
	stateRecord := store.StateRecord{
		Height:           block.Header.Height,
		AppHash:          response.AppHash,
		LastBlockHash:    blockHash,
		ValidatorSetHash: validatorSetHash,
		BaseFee:          baseFee,
		NextBaseFee:      nextBaseFee,
	}
	if commitStore, ok := runtime.Store.(store.BlockCommitStore); ok {
		if err := commitStore.CommitBlockState(ctx, blockRecord, stateRecord, stateRoots); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
		runtime.currentBaseFee = nextBaseFee
		return response, nil
	}
	for _, record := range stateRoots {
		if err := runtime.Store.SaveStateRoot(ctx, record); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	}
	if err := runtime.Store.SaveBlock(ctx, blockRecord); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if err := runtime.Store.SaveState(ctx, stateRecord); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	runtime.currentBaseFee = nextBaseFee
	return response, nil
}

func (runtime *Runtime) executeBlockStaged(ctx context.Context, block types.Block, application *app.Runtime, commitStore store.AppBlockCommitStore, baseFee uint64) (app.FinalizeBlockResponse, error) {
	response, writes, err := application.FinalizeBlockStaged(app.FinalizeBlockRequest{Block: block})
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	nextBaseFee := runtime.NextBaseFee(response)
	validatorSetHash := block.Header.ValidatorSetHash
	validatorUpdateHeight := block.Header.Height + 1
	var stagedValidatorRegistry stagedValidatorUpdateRegistry
	if len(response.ValidatorUpdates) > 0 {
		if registry, ok := runtime.Validators.(stagedValidatorUpdateRegistry); ok {
			validatorSet, validatorWrites, err := registry.StageValidatorUpdatesAt(ctx, validatorUpdateHeight, response.ValidatorUpdates)
			if err != nil {
				return app.FinalizeBlockResponse{}, err
			}
			writes = append(writes, validatorWrites...)
			validatorSetHash = validatorSet.Hash()
			stagedValidatorRegistry = registry
		} else {
			if err := runtime.ApplyValidatorUpdatesAt(ctx, validatorUpdateHeight, response.ValidatorUpdates); err != nil {
				return app.FinalizeBlockResponse{}, err
			}
			validatorSet, err := runtime.Validators.ValidatorSet(ctx, validatorUpdateHeight)
			if err != nil {
				return app.FinalizeBlockResponse{}, err
			}
			validatorSetHash = validatorSet.Hash()
		}
	}
	blockHash := consensus.HashBlock(block)
	stateRoots, err := runtime.moduleStateRootsWithWrites(ctx, block.Header.Height, writes)
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	blockRecord := store.BlockRecord{
		Block:      block,
		Hash:       blockHash,
		AppHash:    response.AppHash,
		StateRoots: stateRoots,
		TxResults:  cloneTxResults(response.Results),
	}
	stateRecord := store.StateRecord{
		Height:           block.Header.Height,
		AppHash:          response.AppHash,
		LastBlockHash:    blockHash,
		ValidatorSetHash: validatorSetHash,
		BaseFee:          baseFee,
		NextBaseFee:      nextBaseFee,
	}
	if err := commitStore.CommitBlockStateWithWrites(ctx, writes, blockRecord, stateRecord, stateRoots); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if stagedValidatorRegistry != nil {
		if err := stagedValidatorRegistry.CommitStagedValidatorUpdates(ctx, validatorUpdateHeight, response.ValidatorUpdates); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	}
	application.CommitStagedBlock(block.Header.Height, response.AppHash)
	runtime.currentBaseFee = nextBaseFee
	return response, nil
}

func (runtime *Runtime) CurrentBaseFee() uint64 {
	if runtime.currentBaseFee > 0 {
		return runtime.currentBaseFee
	}
	return runtime.Config.Execution.BaseFee
}

func (runtime *Runtime) NextBaseFee(response app.FinalizeBlockResponse) uint64 {
	current := runtime.CurrentBaseFee()
	if !runtime.Config.Execution.DynamicBaseFee {
		return current
	}
	return economics.NextBaseFee(economics.BaseFeeParams{
		CurrentBaseFee:    current,
		GasUsed:           totalGasUsed(response),
		TargetGas:         runtime.Config.Execution.TargetGas,
		ChangeDenominator: runtime.Config.Execution.BaseFeeChangeDenominator,
		MinBaseFee:        runtime.Config.Execution.MinBaseFee,
		MaxBaseFee:        runtime.Config.Execution.MaxBaseFee,
	})
}

func (runtime *Runtime) setApplicationBaseFee(baseFee uint64) {
	setter, ok := runtime.App.(interface{ SetBaseFee(uint64) })
	if !ok {
		return
	}
	setter.SetBaseFee(baseFee)
}

func totalGasUsed(response app.FinalizeBlockResponse) uint64 {
	var total uint64
	for _, result := range response.Results {
		if total > ^uint64(0)-result.GasUsed {
			return ^uint64(0)
		}
		total += result.GasUsed
	}
	return total
}

func cloneTxResults(results []types.Result) []types.Result {
	if len(results) == 0 {
		return nil
	}
	cloned := make([]types.Result, len(results))
	for index, result := range results {
		cloned[index] = result
		cloned[index].Data = append([]byte(nil), result.Data...)
	}
	return cloned
}

func (runtime *Runtime) applyUpgradeHook(ctx context.Context, height types.Height) error {
	if runtime.UpgradeHalted {
		return upgrade.ErrRollbackRequired
	}
	if runtime.UpgradePlan == nil {
		if planStore, ok := runtime.Store.(upgrade.PlanStore); ok {
			plan, found, err := planStore.UpgradePlanByHeight(ctx, height)
			if err != nil {
				return err
			}
			if found {
				runtime.UpgradePlan = &plan
			}
		}
	}
	if runtime.UpgradePlan == nil || height < runtime.UpgradePlan.Height {
		return nil
	}
	if runtime.UpgradeState.Height < height {
		runtime.UpgradeState.Height = height
	}
	record, err := runtime.UpgradeExecutor.Execute(ctx, runtime.UpgradeState, *runtime.UpgradePlan)
	if err != nil {
		if record.Status == upgrade.ExecutionRollbackRequired {
			runtime.UpgradeHalted = true
		}
		return err
	}
	if record.Status == upgrade.ExecutionRollbackRequired {
		runtime.UpgradeHalted = true
		return upgrade.ErrRollbackRequired
	}
	if record.Status == upgrade.ExecutionApplied {
		runtime.UpgradeState = upgrade.State{
			Height:              record.Result.Height,
			BinaryVersion:       record.Result.BinaryVersion,
			ConfigSchemaVersion: record.Result.ConfigSchemaVersion,
			StoreSchemaVersion:  record.Result.StoreSchemaVersion,
			AppStateVersion:     record.Result.AppStateVersion,
		}
		if schemaStore, ok := runtime.Store.(store.SchemaStateStore); ok {
			if err := schemaStore.SaveSchemaState(ctx, runtime.UpgradeState); err != nil {
				return err
			}
		}
	}
	return nil
}

func (runtime *Runtime) ApplyValidatorUpdates(ctx context.Context, updates []types.ValidatorUpdate) error {
	return runtime.ApplyValidatorUpdatesAt(ctx, 0, updates)
}

func (runtime *Runtime) ApplyValidatorUpdatesAt(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) error {
	if height > 0 {
		runtime.Validators.SetEffectiveHeight(height)
	}
	for _, update := range updates {
		if update.ID == "" {
			update.ID = types.ValidatorID(update.Address)
		}
		if update.Address == "" {
			update.Address = types.Address(update.ID)
		}
		if update.VotingPower == 0 {
			if err := runtime.Validators.ApplyLeaveAt(ctx, height, update.ID); err != nil {
				return err
			}
			continue
		}
		if _, found := runtime.validatorByID(ctx, update.ID); found {
			if err := runtime.Validators.UpdateVotingPowerAt(ctx, height, update.ID, update.VotingPower); err != nil {
				return err
			}
			continue
		}
		stake := update.Stake
		if stake == 0 {
			stake = uint64(update.VotingPower)
		}
		if _, err := runtime.Validators.ApplyJoinAt(ctx, height, validator.Candidate{
			Address:   update.Address,
			PublicKey: update.PublicKey,
			Stake:     stake,
			Metadata:  update.Metadata,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (runtime *Runtime) ApplyStakingSlashingPenalty(ctx context.Context, receipt slashing.PenaltyReceipt) error {
	if runtime.Store == nil || receipt.Evidence.Validator == "" {
		return nil
	}
	for _, module := range runtime.AppModules() {
		applier, ok := module.(stakingPenaltyApplier)
		if !ok {
			continue
		}
		if err := applier.ApplySlashingPenalty(ctx, runtime.Store, receipt); err != nil {
			return err
		}
	}
	return nil
}

func (runtime *Runtime) validatorByID(ctx context.Context, id types.ValidatorID) (validator.Validator, bool) {
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, 0)
	if err != nil {
		return validator.Validator{}, false
	}
	return validatorSet.Get(id)
}

func (runtime *Runtime) moduleStateRoots(ctx context.Context, height types.Height) ([]store.StateRootRecord, error) {
	if runtime.Store == nil {
		return nil, nil
	}
	roots := make([]store.StateRootRecord, 0)
	for _, module := range runtime.AppModules() {
		root, err := runtime.Store.Root(ctx, module.Name())
		if err != nil {
			return nil, err
		}
		record := store.StateRootRecord{
			Height:    height,
			Namespace: module.Name(),
			Root:      root,
		}
		roots = append(roots, record)
	}
	return roots, nil
}

func (runtime *Runtime) moduleStateRootsWithWrites(ctx context.Context, height types.Height, writes []store.KVWrite) ([]store.StateRootRecord, error) {
	if runtime.Store == nil {
		return nil, nil
	}
	rootStore, ok := runtime.Store.(store.RootWithWritesStore)
	if !ok {
		return runtime.moduleStateRoots(ctx, height)
	}
	roots := make([]store.StateRootRecord, 0)
	for _, module := range runtime.AppModules() {
		root, err := rootStore.RootWithWrites(ctx, module.Name(), writes)
		if err != nil {
			return nil, err
		}
		roots = append(roots, store.StateRootRecord{
			Height:    height,
			Namespace: module.Name(),
			Root:      root,
		})
	}
	return roots, nil
}

func (runtime *Runtime) AppModules() []app.Module {
	appRuntime, ok := runtime.App.(*app.Runtime)
	if !ok {
		return nil
	}
	return appRuntime.Modules()
}

func (runtime *Runtime) NewConsensusStateMachine(ctx context.Context, height types.Height) (*consensus.StateMachine, error) {
	return runtime.NewConsensusStateMachineWithSignatures(ctx, height, runtime.Crypto.ConsensusVerifier)
}

func (runtime *Runtime) NewConsensusStateMachineWithSignatures(ctx context.Context, height types.Height, signatures interface {
	Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}) (*consensus.StateMachine, error) {
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return nil, err
	}
	return consensus.NewStateMachine(consensus.StateMachineConfig{
		ChainID:      runtime.Config.ChainID,
		ValidatorSet: validatorSet,
		Signatures:   signatures,
		Aggregator:   runtime.Crypto.ConsensusAggregator,
	})
}

func (runtime *Runtime) NewFinalityVerifier(ctx context.Context, height types.Height) (finality.Verifier, error) {
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return finality.Verifier{}, err
	}
	return finality.NewVerifier(validatorSet, runtime.Crypto.FinalityVerifier), nil
}

func (runtime *Runtime) Recover(ctx context.Context) (store.StateRecord, error) {
	if runtime.Store == nil {
		return store.StateRecord{}, store.ErrStateNotFound
	}
	state, err := runtime.Store.LatestState(ctx)
	if err != nil {
		return store.StateRecord{}, err
	}
	if appRuntime, ok := runtime.App.(*app.Runtime); ok {
		appRuntime.Restore(state.Height, state.AppHash)
		if err := appRuntime.BindStore(); err != nil {
			return store.StateRecord{}, err
		}
	}
	if state.NextBaseFee > 0 {
		runtime.currentBaseFee = state.NextBaseFee
	} else if state.BaseFee > 0 {
		runtime.currentBaseFee = state.BaseFee
	}
	return state, nil
}
