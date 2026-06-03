package runtime

import (
	"context"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/mempool"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

type Runtime struct {
	Config     config.Config
	App        app.Application
	Executor   consensus.ApplicationBlockExecutor
	Validators validator.VersionedRegistry
	Committee  committee.Selector
	Mempool    *mempool.DAG
	Slashing   consensus.SlashingKeeper
	Governance governance.OperationalKeeper
	P2PScore   *p2p.ScoreKeeper
	Crypto     crypto.RuntimeSuite
	Store      store.Store
}

func New(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower) (*Runtime, error) {
	return NewWithStore(cfg, application, initialValidators, governancePower, nil)
}

func NewWithStore(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store) (*Runtime, error) {
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

	selector, err := committee.NewSelector(cfg.Committee, crypto.NewDeterministicVRF(cfg.VRF.Keys))
	if err != nil {
		return nil, err
	}
	cryptoSuite, err := crypto.NewRuntimeSuite(cfg.Crypto)
	if err != nil {
		return nil, err
	}

	fifoConfig := cfg.Mempool
	if fifoConfig.Author == "" && len(initialValidators) > 0 {
		fifoConfig.Author = initialValidators[0].ID
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
		Config:     cfg,
		App:        application,
		Executor:   consensus.ApplicationBlockExecutor{},
		Validators: registry,
		Committee:  selector,
		Mempool:    mempool.NewDAG(mempool.NewFIFO(fifoConfig)),
		Slashing:   slashingKeeper,
		Governance: governanceKeeper,
		P2PScore:   p2p.NewScoreKeeper(cfg.P2P),
		Crypto:     cryptoSuite,
		Store:      storage,
	}, nil
}

func (runtime *Runtime) ExecuteBlock(ctx context.Context, block types.Block) (app.FinalizeBlockResponse, error) {
	response, err := runtime.Executor.Execute(ctx, runtime.App, block)
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
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
	if err := runtime.Store.SaveBlock(ctx, store.BlockRecord{
		Block:      block,
		Hash:       blockHash,
		AppHash:    response.AppHash,
		StateRoots: stateRoots,
	}); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if err := runtime.Store.SaveState(ctx, store.StateRecord{
		Height:           block.Header.Height,
		AppHash:          response.AppHash,
		LastBlockHash:    blockHash,
		ValidatorSetHash: validatorSetHash,
	}); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	return response, nil
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
		if err := runtime.Store.SaveStateRoot(ctx, record); err != nil {
			return nil, err
		}
		roots = append(roots, record)
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
	return runtime.NewConsensusStateMachineWithSignatures(ctx, height, nil)
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
	}
	return state, nil
}
