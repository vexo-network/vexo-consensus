package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/committee"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/consensus"
	"github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/finality"
	"github.com/vexo-network/vexo-consensus/governance"
	"github.com/vexo-network/vexo-consensus/mempool"
	appslashing "github.com/vexo-network/vexo-consensus/modules/slashing"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/upgrade"
	"github.com/vexo-network/vexo-consensus/validator"
)

var ErrAtomicAppCommitUnavailable = errors.New("atomic app block commit store is required")
var ErrDurableStoreRequired = errors.New("durable store is required; use NewEphemeral only for isolated tests and examples")
var ErrEphemeralRuntimeNotAllowed = errors.New("ephemeral runtime requires a test/example/demo chain id; use NewNetworkSafeWithStore for nodes")
var ErrUpgradeExecutorMissing = errors.New("upgrade executor is required")

type Runtime struct {
	Config          config.Config
	App             app.Application
	Executor        consensus.ApplicationBlockExecutor
	Validators      validator.VersionedRegistry
	Committee       committee.Selector
	Mempool         *mempool.DAG
	Governance      governance.OperationalKeeper
	P2PScore        *p2p.ScoreKeeper
	Crypto          crypto.RuntimeSuite
	Store           store.Store
	UpgradePlan     *upgrade.Plan
	UpgradeExecutor upgrade.Executor
	UpgradeState    upgrade.State
	UpgradeHalted   bool

	// evidenceMu serializes changes of the evidence pool, which is bounded and must not be
	// overfilled by concurrent admissions.
	evidenceMu        sync.Mutex
	reconcileFailures uint64
}

type stagedValidatorUpdateRegistry interface {
	StageValidatorUpdatesAt(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) (validator.Set, []store.KVWrite, error)
	CommitStagedValidatorUpdates(height types.Height)
}

func New(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower) (*Runtime, error) {
	return nil, ErrDurableStoreRequired
}

// NewEphemeralForTesting creates an in-memory runtime for isolated tests and examples.
//
// Do not use it for a running network: it does not persist validator registry,
// governance, slashing, mempool, block, state, or finality-proof data.
func NewEphemeralForTesting(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !ephemeralChainIDAllowed(cfg.ChainID) {
		return nil, ErrEphemeralRuntimeNotAllowed
	}
	return newWithStoreAndCryptoRegistry(context.Background(), cfg, application, initialValidators, governancePower, nil, crypto.NewRuntimeSuiteRegistry(), true)
}

func ephemeralChainIDAllowed(chainID string) bool {
	normalized := strings.ToLower(strings.TrimSpace(chainID))
	return strings.Contains(normalized, "test") ||
		strings.Contains(normalized, "example") ||
		strings.Contains(normalized, "demo")
}

func NewWithStore(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store) (*Runtime, error) {
	return NewWithStoreContext(context.Background(), cfg, application, initialValidators, governancePower, storage)
}

func NewNetworkSafeWithStore(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store) (*Runtime, error) {
	return NewNetworkSafeWithStoreContext(context.Background(), cfg, application, initialValidators, governancePower, storage)
}

func NewNetworkSafeWithStoreContext(ctx context.Context, cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store) (*Runtime, error) {
	return NewNetworkSafeWithStoreAndCryptoRegistryContext(ctx, cfg, application, initialValidators, governancePower, storage, crypto.NewRuntimeSuiteRegistry())
}

func NewNetworkSafeWithStoreAndCryptoRegistry(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store, cryptoRegistry crypto.RuntimeSuiteRegistry) (*Runtime, error) {
	return NewNetworkSafeWithStoreAndCryptoRegistryContext(context.Background(), cfg, application, initialValidators, governancePower, storage, cryptoRegistry)
}

func NewNetworkSafeWithStoreAndCryptoRegistryContext(ctx context.Context, cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store, cryptoRegistry crypto.RuntimeSuiteRegistry) (*Runtime, error) {
	if err := ValidateNetworkSafeRuntimeInputs(cfg, application, storage); err != nil {
		return nil, err
	}
	return NewWithStoreAndCryptoRegistryContext(ctx, cfg, application, initialValidators, governancePower, storage, cryptoRegistry)
}

func ValidateNetworkSafeRuntimeInputs(cfg config.Config, application app.Application, storage store.Store) error {
	if err := cfg.ValidateNetworkSafety(); err != nil {
		return err
	}
	if storage == nil {
		return ErrDurableStoreRequired
	}
	if _, ok := application.(app.AtomicBlockApplication); !ok {
		return ErrAtomicAppCommitUnavailable
	}
	if _, ok := storage.(store.AppBlockCommitStore); !ok {
		return ErrAtomicAppCommitUnavailable
	}
	return nil
}

func NewWithStoreContext(ctx context.Context, cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store) (*Runtime, error) {
	return NewWithStoreAndCryptoRegistryContext(ctx, cfg, application, initialValidators, governancePower, storage, crypto.NewRuntimeSuiteRegistry())
}

func NewWithStoreAndCryptoRegistry(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store, cryptoRegistry crypto.RuntimeSuiteRegistry) (*Runtime, error) {
	return NewWithStoreAndCryptoRegistryContext(context.Background(), cfg, application, initialValidators, governancePower, storage, cryptoRegistry)
}

func NewWithStoreAndCryptoRegistryContext(ctx context.Context, cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store, cryptoRegistry crypto.RuntimeSuiteRegistry) (*Runtime, error) {
	return newWithStoreAndCryptoRegistry(ctx, cfg, application, initialValidators, governancePower, storage, cryptoRegistry, false)
}

func newWithStoreAndCryptoRegistry(ctx context.Context, cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower, storage store.Store, cryptoRegistry crypto.RuntimeSuiteRegistry, allowEphemeral bool) (*Runtime, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if storage == nil && !allowEphemeral {
		return nil, ErrDurableStoreRequired
	}
	if storage != nil && cfg.ValidateNetworkSafety() == nil {
		if _, ok := application.(app.AtomicBlockApplication); !ok {
			return nil, ErrAtomicAppCommitUnavailable
		}
		if _, ok := storage.(store.AppBlockCommitStore); !ok {
			return nil, ErrAtomicAppCommitUnavailable
		}
	}

	admission := validator.NewConfigurableAdmissionPolicy(cfg.Validator)
	var registry validator.VersionedRegistry
	if storage != nil {
		storeRegistry, err := validator.NewStoreRegistry(ctx, storage, admission, 1, initialValidators)
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
		var loadedVRF crypto.VRF
		var err error
		if cfg.VRF.ProductionAdapter {
			loadedVRF, err = crypto.NewProductionVRF(cfg.VRF)
		} else {
			loadedVRF, err = crypto.NewVRF(cfg.VRF)
		}
		if err != nil {
			return nil, err
		}
		vrf = loadedVRF
	}
	selector, err := committee.NewSelector(cfg.Committee, vrf)
	if err != nil {
		return nil, err
	}
	cryptoSuite, err := cryptoRegistry.NewRuntimeSuite(cfg.Crypto)
	if err != nil {
		return nil, err
	}
	if err := cryptoSuite.RegisterValidators(initialValidators); err != nil {
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
		fifoConfig.ReplayCheckTx = func(ctx context.Context, tx types.Tx) error {
			if checker, ok := application.(app.ContextCheckTxApplication); ok {
				return mempoolReplayCheckResult(checker.CheckTxContext(ctx, tx))
			}
			return mempoolReplayCheckResult(application.CheckTx(tx))
		}
	}
	dag := mempool.NewDAG(mempool.NewFIFO(fifoConfig))
	if fifoConfig.WALPath != "" {
		durableDAG, err := mempool.OpenDurableDAG(ctx, fifoConfig.WALPath, mempool.NewFIFO(fifoConfig))
		if err != nil {
			return nil, err
		}
		dag = durableDAG
	}
	if storage != nil {
		removed, err := dag.RetainTxs(ctx, func(tx types.Tx) bool {
			if checker, ok := application.(app.ContextCheckTxApplication); ok {
				err := mempoolReplayCheckResult(checker.CheckTxContext(ctx, tx))
				return err == nil || errors.Is(err, mempool.ErrTxNotReady)
			}
			err := mempoolReplayCheckResult(application.CheckTx(tx))
			return err == nil || errors.Is(err, mempool.ErrTxNotReady)
		})
		if err != nil {
			return nil, err
		}
		if removed > 0 {
			if err := dag.CompactWAL(ctx); err != nil {
				return nil, err
			}
		}
	}
	governanceKeeper := governance.OperationalKeeper(governance.NewInMemoryKeeper(cfg.Governance, governancePower))
	if storage != nil {
		storeKeeper, err := governance.NewStoreKeeperContext(ctx, storage, cfg.Governance, governancePower)
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
		Mempool:    dag,
		Governance: governanceKeeper,
		P2PScore:   p2p.NewScoreKeeper(cfg.P2P),
		Crypto:     cryptoSuite,
		Store:      storage,
	}, nil
}

func mempoolReplayCheckResult(response app.CheckTxResponse) error {
	if response.Result.Code == 0 {
		return nil
	}
	if nonce, expected, ok := app.ParseInvalidNonceLog(response.Result.Log); ok && nonce > expected {
		return mempool.ErrTxNotReady
	}
	return errors.New(response.Result.Log)
}

// genesisNamespace and genesisKey locate the chain's InitChain request in the store. The namespace
// belongs to no module, so the request is part of no state root.
const genesisNamespace = "runtime"

var genesisKey = []byte("init_chain")

// genesisHeaderAppHashKey locates the header app hash (finality.HeaderAppHash) of the genesis
// state, the state block headers name until the validator set first changes registry height.
var genesisHeaderAppHashKey = []byte("init_chain_header_app_hash")

// ErrAppHashUnavailable reports a height whose header app hash the node does not hold: not
// executed yet, or pruned or below the snapshot the node was restored from and named by no block
// whose certified app hash the replicated state records.
var ErrAppHashUnavailable = errors.New("app hash of the height is not held")

// InitChain initializes the application from the chain's genesis. It persists the request first, so
// that a replay can initialize its own application from the same genesis; a restart after a crash
// in between simply runs InitChain again with the same request.
func (runtime *Runtime) InitChain(ctx context.Context, request app.InitChainRequest) (app.InitChainResponse, error) {
	if runtime.Store != nil {
		encoded, err := json.Marshal(request)
		if err != nil {
			return app.InitChainResponse{}, err
		}
		if err := runtime.Store.Set(ctx, genesisNamespace, genesisKey, encoded); err != nil {
			return app.InitChainResponse{}, err
		}
	}
	response, err := runtime.App.InitChain(request)
	if err != nil {
		return app.InitChainResponse{}, err
	}
	if runtime.Store != nil {
		registryRoot, err := runtime.Store.Root(ctx, validator.RegistryNamespace)
		if err != nil {
			return app.InitChainResponse{}, err
		}
		headerAppHash := finality.HeaderAppHash(response.AppHash, registryRoot)
		if err := runtime.Store.Set(ctx, genesisNamespace, genesisHeaderAppHashKey, headerAppHash[:]); err != nil {
			return app.InitChainResponse{}, err
		}
	}
	return response, nil
}

// HeaderAppHashAt returns the hash a block header commits for the state after executing block
// height, the genesis state for height 0: finality.HeaderAppHash of the state's app hash and the
// root of its validator registry.
//
// A node restored from a snapshot holds no state below it, and a pruning node none below its
// retention, yet headers may still name such a state: a header keeps naming the state of its
// validator set until the chain proves a commit. For those it returns the header app hash
// certified blocks committed for the state, which the replicated state records (slashing
// CertifiedAppHash): every header naming the state must commit that hash, so the node judges such
// headers as the nodes that hold the state do.
func (runtime *Runtime) HeaderAppHashAt(ctx context.Context, height types.Height) (types.Hash, error) {
	if runtime.Store == nil {
		return types.Hash{}, ErrAppHashUnavailable
	}
	hash, err := runtime.localHeaderAppHashAt(ctx, height)
	if !errors.Is(err, ErrAppHashUnavailable) {
		return hash, err
	}
	certified, found, certifiedErr := appslashing.CertifiedAppHash(ctx, runtime.Store, height)
	if certifiedErr != nil {
		return types.Hash{}, certifiedErr
	}
	if !found {
		return types.Hash{}, err
	}
	return certified, nil
}

// localHeaderAppHashAt computes the header app hash of height from the state the node holds.
func (runtime *Runtime) localHeaderAppHashAt(ctx context.Context, height types.Height) (types.Hash, error) {
	if height == 0 {
		encoded, err := runtime.Store.Get(ctx, genesisNamespace, genesisHeaderAppHashKey)
		if errors.Is(err, store.ErrKeyNotFound) {
			return types.Hash{}, fmt.Errorf("%w: genesis", ErrAppHashUnavailable)
		}
		if err != nil {
			return types.Hash{}, err
		}
		var hash types.Hash
		if len(encoded) != len(hash) {
			return types.Hash{}, fmt.Errorf("%w: genesis app hash has %d bytes", store.ErrInvalidStateRecord, len(encoded))
		}
		copy(hash[:], encoded)
		return hash, nil
	}
	state, err := runtime.Store.StateByHeight(ctx, height)
	if errors.Is(err, store.ErrStateNotFound) {
		return types.Hash{}, fmt.Errorf("%w: height %d", ErrAppHashUnavailable, height)
	}
	if err != nil {
		return types.Hash{}, err
	}
	registry, err := runtime.Store.StateRoot(ctx, height, validator.RegistryNamespace)
	if errors.Is(err, store.ErrStateRootNotFound) {
		return types.Hash{}, fmt.Errorf("%w: validator registry root of height %d", ErrAppHashUnavailable, height)
	}
	if err != nil {
		return types.Hash{}, err
	}
	return finality.HeaderAppHash(state.AppHash, registry.Root), nil
}

// storedInitChain returns the InitChain request the chain started from, if one was persisted.
func (runtime *Runtime) storedInitChain(ctx context.Context) (app.InitChainRequest, bool, error) {
	encoded, err := runtime.Store.Get(ctx, genesisNamespace, genesisKey)
	if errors.Is(err, store.ErrKeyNotFound) {
		return app.InitChainRequest{}, false, nil
	}
	if err != nil {
		return app.InitChainRequest{}, false, err
	}
	var request app.InitChainRequest
	if err := json.Unmarshal(encoded, &request); err != nil {
		return app.InitChainRequest{}, false, err
	}
	return request, true, nil
}

func (runtime *Runtime) WithUpgrade(plan upgrade.Plan, state upgrade.State, executor upgrade.Executor) *Runtime {
	runtime.UpgradePlan = &plan
	runtime.UpgradeState = state
	runtime.UpgradeExecutor = executor
	return runtime
}

func (runtime *Runtime) ExecuteBlock(ctx context.Context, block types.Block) (app.FinalizeBlockResponse, error) {
	return runtime.executeBlock(ctx, block, nil)
}

func (runtime *Runtime) ExecuteCertifiedBlock(ctx context.Context, block types.Block, quorumCert finality.QuorumCert) (app.FinalizeBlockResponse, error) {
	blockHash := consensus.HashBlock(block)
	if quorumCert.Height != block.Header.Height {
		return app.FinalizeBlockResponse{}, finality.ErrHeightMismatch
	}
	if quorumCert.BlockHash != blockHash {
		return app.FinalizeBlockResponse{}, finality.ErrBlockHashMismatch
	}
	return runtime.executeBlock(ctx, block, &quorumCert)
}

func (runtime *Runtime) executeBlock(ctx context.Context, block types.Block, quorumCert *finality.QuorumCert) (app.FinalizeBlockResponse, error) {
	if err := runtime.applyUpgradeHook(ctx, block.Header.Height); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if runtime.Store != nil {
		atomicApp, ok := runtime.App.(app.AtomicBlockApplication)
		if !ok {
			return app.FinalizeBlockResponse{}, ErrAtomicAppCommitUnavailable
		}
		commitStore, ok := runtime.Store.(store.AppBlockCommitStore)
		if !ok {
			return app.FinalizeBlockResponse{}, ErrAtomicAppCommitUnavailable
		}
		return runtime.executeBlockStaged(ctx, block, quorumCert, atomicApp, commitStore)
	}
	response, err := runtime.Executor.Execute(ctx, runtime.App, block)
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	if len(response.ValidatorUpdates) > 0 {
		if err := runtime.ApplyValidatorUpdatesAt(ctx, block.Header.Height+1, response.ValidatorUpdates); err != nil {
			return app.FinalizeBlockResponse{}, err
		}
	}
	return response, nil
}

// executeBlockStaged executes block and commits it together with everything it changed in one
// atomic write. The application is the only authority on what a block does: the fee market it
// reports is what the state record keeps, and a validator update it reports can never fail the
// block, because the registry skips an update it cannot apply.
func (runtime *Runtime) executeBlockStaged(ctx context.Context, block types.Block, quorumCert *finality.QuorumCert, application app.AtomicBlockApplication, commitStore store.AppBlockCommitStore) (app.FinalizeBlockResponse, error) {
	response, writes, err := application.FinalizeBlockStagedContext(ctx, app.FinalizeBlockRequest{Block: block})
	if err != nil {
		return app.FinalizeBlockResponse{}, err
	}
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
	stateRoots, err := runtime.stateRootsWithWrites(ctx, block.Header.Height, writes)
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
	if quorumCert != nil {
		certRecord := storeQuorumCertRecord(*quorumCert)
		blockRecord.QuorumCert = &certRecord
	}
	feeMarket := response.FeeMarket
	stateRecord := store.StateRecord{
		Height:           block.Header.Height,
		AppHash:          response.AppHash,
		LastBlockHash:    blockHash,
		ValidatorSetHash: validatorSetHash,
		BaseFee:          feeMarket.BaseFee,
		NextBaseFee:      feeMarket.NextBaseFee,
		BlobBaseFee:      feeMarket.BlobBaseFee,
		NextBlobBaseFee:  feeMarket.NextBlobBaseFee,
		BlobGasUsed:      feeMarket.BlobGasUsed,
		ExcessBlobGas:    excessBlobGas(feeMarket.BlobGasUsed, runtime.Config.Execution.TargetBlobGas),
	}
	if err := commitStore.CommitBlockStateWithWrites(ctx, writes, blockRecord, stateRecord, stateRoots); err != nil {
		return app.FinalizeBlockResponse{}, err
	}
	application.CommitStagedBlock(block.Header.Height, response.AppHash)
	runtime.reconcileAfterCommit(ctx, stagedValidatorRegistry, validatorUpdateHeight, response.ValidatorUpdates)
	return response, nil
}

// reconcileAfterCommit refreshes what is derived from the block that just committed: the registry's
// effective height, the crypto keys of a new validator set and the evidence pool. The keys and the
// pool are rebuilt from committed state on start (RegisterValidatorsAt, ReconcileEvidence), so a
// failure is counted and never fails a block that is already durable.
func (runtime *Runtime) reconcileAfterCommit(ctx context.Context, registry stagedValidatorUpdateRegistry, validatorUpdateHeight types.Height, updates []types.ValidatorUpdate) {
	if registry != nil {
		registry.CommitStagedValidatorUpdates(validatorUpdateHeight)
	}
	if len(updates) > 0 {
		// The new validator set verifies the certificates of the next heights; admit its keys now.
		if err := runtime.RegisterValidatorsAt(ctx, validatorUpdateHeight); err != nil {
			atomic.AddUint64(&runtime.reconcileFailures, 1)
		}
	}
	if err := runtime.pruneEvidence(ctx, validatorUpdateHeight); err != nil {
		atomic.AddUint64(&runtime.reconcileFailures, 1)
	}
}

func storeQuorumCertRecord(quorumCert finality.QuorumCert) store.QuorumCertRecord {
	return store.QuorumCertRecord{
		Height:      quorumCert.Height,
		Round:       quorumCert.Round,
		BlockHash:   quorumCert.BlockHash,
		Signers:     quorumCert.Signers,
		Signature:   quorumCert.Signature,
		VotingPower: quorumCert.VotingPower,
	}
}

func excessBlobGas(blobGasUsed uint64, target uint64) uint64 {
	if target == 0 || blobGasUsed <= target {
		return 0
	}
	return blobGasUsed - target
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

// applyUpgradeHook runs the scheduled upgrade plan once the chain reaches its height. The plan is
// the next one at or above the height of the block, so a plan scheduled after an earlier one was
// applied is picked up; an applied plan is cleared so it never runs, or blocks its successor, twice.
func (runtime *Runtime) applyUpgradeHook(ctx context.Context, height types.Height) error {
	if runtime.UpgradeHalted {
		return upgrade.ErrRollbackRequired
	}
	if runtime.UpgradePlan == nil {
		if planStore, ok := runtime.Store.(upgrade.PlanStore); ok {
			plan, found, err := planStore.NextUpgradePlan(ctx, height)
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
	if runtime.UpgradeExecutor.Recorder == nil {
		runtime.UpgradeHalted = true
		return ErrUpgradeExecutorMissing
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
		runtime.UpgradePlan = nil
	}
	return nil
}

func (runtime *Runtime) ApplyValidatorUpdates(ctx context.Context, updates []types.ValidatorUpdate) error {
	return runtime.ApplyValidatorUpdatesAt(ctx, 0, updates)
}

// ApplyValidatorUpdatesAt applies the validator updates a block reported to the registry at height.
// The block is already certified, so an update the registry rejects on its content (an unknown
// validator, a join without a key or refused by the admission policy, a removal of the last
// validator) is skipped; only a storage failure or a cancelled context is returned.
func (runtime *Runtime) ApplyValidatorUpdatesAt(ctx context.Context, height types.Height, updates []types.ValidatorUpdate) error {
	if height > 0 {
		runtime.Validators.SetEffectiveHeight(height)
	}
	for _, update := range updates {
		if err := runtime.applyValidatorUpdate(ctx, height, update); err != nil && !errors.Is(err, validator.ErrInvalidUpdate) {
			return err
		}
	}
	return nil
}

func (runtime *Runtime) applyValidatorUpdate(ctx context.Context, height types.Height, update types.ValidatorUpdate) error {
	if update.ID == "" {
		update.ID = types.ValidatorID(update.Address)
	}
	if update.Address == "" {
		update.Address = types.Address(update.ID)
	}
	if update.VotingPower == 0 {
		set, err := runtime.Validators.ValidatorSet(ctx, height)
		if err != nil {
			return err
		}
		if _, found := set.Get(update.ID); found && len(set.List()) == 1 {
			return nil
		}
		return runtime.Validators.ApplyLeaveAt(ctx, height, update.ID)
	}
	if _, found := runtime.validatorByID(ctx, update.ID); found {
		return runtime.Validators.UpdateVotingPowerAt(ctx, height, update.ID, update.VotingPower)
	}
	stake := update.Stake
	if stake == 0 {
		stake = uint64(update.VotingPower)
	}
	_, err := runtime.Validators.ApplyJoinAt(ctx, height, validator.Candidate{
		Address:   update.Address,
		PublicKey: update.PublicKey,
		Stake:     stake,
		Metadata:  update.Metadata,
	})
	return err
}

func (runtime *Runtime) validatorByID(ctx context.Context, id types.ValidatorID) (validator.Validator, bool) {
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, 0)
	if err != nil {
		return validator.Validator{}, false
	}
	return validatorSet.Get(id)
}

// StateNamespaces is every namespace of the replicated state, in a fixed order: the modules', the
// runtime's own (account sequences and the fee market) and the validator registry. A block records a
// state root for each of them and a snapshot carries each of them, so a node restored from a
// snapshot holds the same state as the nodes that executed every block: without the sequences it
// would accept replayed transactions, and without the registry's validator-set history it would judge
// evidence of a past height differently from its peers.
func (runtime *Runtime) StateNamespaces() []string {
	modules := runtime.AppModules()
	namespaces := make([]string, 0, len(modules)+3)
	for _, module := range modules {
		namespaces = append(namespaces, module.Name())
	}
	namespaces = append(namespaces, app.CoreNamespaces()...)
	return append(namespaces, validator.RegistryNamespace)
}

func (runtime *Runtime) stateRootsWithWrites(ctx context.Context, height types.Height, writes []store.KVWrite) ([]store.StateRootRecord, error) {
	if runtime.Store == nil {
		return nil, nil
	}
	root := runtime.Store.Root
	if rootStore, ok := runtime.Store.(store.RootWithWritesStore); ok {
		root = func(ctx context.Context, namespace string) (types.Hash, error) {
			return rootStore.RootWithWrites(ctx, namespace, writes)
		}
	}
	namespaces := runtime.StateNamespaces()
	roots := make([]store.StateRootRecord, 0, len(namespaces))
	for _, namespace := range namespaces {
		hash, err := root(ctx, namespace)
		if err != nil {
			return nil, err
		}
		roots = append(roots, store.StateRootRecord{Height: height, Namespace: namespace, Root: hash})
	}
	return roots, nil
}

func (runtime *Runtime) AppModules() []app.Module {
	appRuntime, ok := runtime.App.(interface{ Modules() []app.Module })
	if !ok {
		return nil
	}
	return appRuntime.Modules()
}

func (runtime *Runtime) NewConsensusStateMachine(ctx context.Context, height types.Height) (*consensus.StateMachine, error) {
	validatorSet, err := runtime.registeredValidatorSet(ctx, height)
	if err != nil {
		return nil, err
	}
	return consensus.NewStateMachine(consensus.StateMachineConfig{
		ChainID:                  runtime.Config.ChainID,
		ValidatorSet:             validatorSet,
		Signatures:               runtime.Crypto.ConsensusVerifier,
		Aggregator:               runtime.Crypto.ConsensusAggregator,
		AggregateVerifier:        runtime.Crypto.FinalityVerifier,
		EnforceProposerSelection: true,
		// The machine outlives the context it is built in; an app hash lookup reads the store.
		AppHashes: func(height types.Height) (types.Hash, error) {
			return runtime.HeaderAppHashAt(context.Background(), height)
		},
	})
}

// NewFinalityVerifier returns a verifier that checks every certified header against the validator
// set the header names, loaded (and admitted with the crypto suite) through registeredValidatorSet.
func (runtime *Runtime) NewFinalityVerifier() finality.RegistryVerifier {
	return finality.NewRegistryVerifier(registeredSets{runtime: runtime}, runtime.Crypto.FinalityVerifier)
}

// registeredSets serves the validator sets of the registry with their keys admitted. The set of
// registry height h is fixed once block h-1 is executed; a later height is not known yet and fails
// with consensus.ErrValidatorSetPending rather than returning the latest set in its place.
type registeredSets struct {
	runtime *Runtime
}

func (sets registeredSets) ValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	if height == 0 {
		return nil, consensus.ErrUnknownValidatorSet
	}
	commit, err := sets.runtime.App.Commit()
	if err != nil {
		return nil, err
	}
	if height > commit.Height+1 {
		return nil, fmt.Errorf("%w: registry height %d after executed height %d", consensus.ErrValidatorSetPending, height, commit.Height)
	}
	return sets.runtime.registeredValidatorSet(ctx, height)
}

// RegisterValidatorsAt admits the consensus keys of the validator set in effect at height with the
// crypto suite. The BLS backend verifies a certificate only over keys whose proof of possession it
// checked, so every path that loads a validator set to verify certificates or evidence must call
// it, or load the set through registeredValidatorSet, first.
func (runtime *Runtime) RegisterValidatorsAt(ctx context.Context, height types.Height) error {
	_, err := runtime.registeredValidatorSet(ctx, height)
	return err
}

// registeredValidatorSet returns the validator set in effect at height after registering its keys.
func (runtime *Runtime) registeredValidatorSet(ctx context.Context, height types.Height) (validator.Set, error) {
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return nil, err
	}
	if err := runtime.Crypto.RegisterValidators(validatorSet.List()); err != nil {
		return nil, err
	}
	return validatorSet, nil
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
	if state.Height > 0 && runtime.Validators != nil {
		runtime.Validators.SetEffectiveHeight(state.Height + 1)
		if err := runtime.RegisterValidatorsAt(ctx, state.Height+1); err != nil {
			return store.StateRecord{}, err
		}
	}
	return state, nil
}

func (runtime *Runtime) PostCommitReconciliationFailures() uint64 {
	return atomic.LoadUint64(&runtime.reconcileFailures)
}
