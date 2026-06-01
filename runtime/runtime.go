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
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

type Runtime struct {
	Config     config.Config
	App        app.Application
	Executor   consensus.ApplicationBlockExecutor
	Validators *validator.InMemoryRegistry
	Committee  committee.DeterministicSelector
	Mempool    *mempool.DAG
	Slashing   *slashing.InMemoryKeeper
	Governance *governance.InMemoryKeeper
	P2PScore   *p2p.ScoreKeeper
	Crypto     crypto.DeterministicAggregateSigner
}

func New(cfg config.Config, application app.Application, initialValidators []validator.Validator, governancePower map[types.Address]types.VotingPower) (*Runtime, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	admission := validator.NewConfigurableAdmissionPolicy(cfg.Validator)
	registry, err := validator.NewInMemoryRegistry(admission, initialValidators)
	if err != nil {
		return nil, err
	}

	selector, err := committee.NewDeterministicSelector(cfg.Committee)
	if err != nil {
		return nil, err
	}

	fifoConfig := cfg.Mempool
	if fifoConfig.Author == "" && len(initialValidators) > 0 {
		fifoConfig.Author = initialValidators[0].ID
	}

	return &Runtime{
		Config:     cfg,
		App:        application,
		Executor:   consensus.ApplicationBlockExecutor{},
		Validators: registry,
		Committee:  selector,
		Mempool:    mempool.NewDAG(mempool.NewFIFO(fifoConfig)),
		Slashing:   slashing.NewInMemoryKeeper(nil),
		Governance: governance.NewInMemoryKeeper(cfg.Governance, governancePower),
		P2PScore:   p2p.NewScoreKeeper(cfg.P2P),
		Crypto:     crypto.DeterministicAggregateSigner{},
	}, nil
}

func (runtime *Runtime) ExecuteBlock(ctx context.Context, block types.Block) (app.FinalizeBlockResponse, error) {
	return runtime.Executor.Execute(ctx, runtime.App, block)
}

func (runtime *Runtime) NewConsensusStateMachine(ctx context.Context, height types.Height) (*consensus.StateMachine, error) {
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return nil, err
	}
	return consensus.NewStateMachine(consensus.StateMachineConfig{
		ChainID:      runtime.Config.ChainID,
		ValidatorSet: validatorSet,
	})
}

func (runtime *Runtime) NewFinalityVerifier(ctx context.Context, height types.Height) (finality.Verifier, error) {
	validatorSet, err := runtime.Validators.ValidatorSet(ctx, height)
	if err != nil {
		return finality.Verifier{}, err
	}
	return finality.NewVerifier(validatorSet, runtime.Crypto), nil
}
