package modules

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	vexoapp "github.com/vexo-network/vexo-consensus/app"
	"github.com/vexo-network/vexo-consensus/config"
	"github.com/vexo-network/vexo-consensus/modules/bank"
	appevm "github.com/vexo-network/vexo-consensus/modules/evm"
	"github.com/vexo-network/vexo-consensus/modules/staking"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

func TestBuildDefaultModules(t *testing.T) {
	modules, err := Build(config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 6 || modules[0].Name() != "bank" || modules[1].Name() != "staking" || modules[2].Name() != "governance" || modules[3].Name() != "params" || modules[4].Name() != "ibc" || modules[5].Name() != "slashing" {
		t.Fatalf("expected default bank module, got %+v", modules)
	}
}

func TestBuildDefaultCLICommands(t *testing.T) {
	commands, err := BuildCLICommands(config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 5 ||
		commands[0].Name != "bank" ||
		commands[1].Name != "staking" ||
		commands[2].Name != "governance" ||
		commands[3].Name != "params" ||
		commands[4].Name != "ibc" ||
		len(commands[0].Children) == 0 ||
		len(commands[1].Children) == 0 ||
		len(commands[2].Children) == 0 ||
		len(commands[3].Children) == 0 ||
		len(commands[4].Children) == 0 {
		t.Fatalf("expected default bank cli command, got %+v", commands)
	}
}

func TestBuildAllowsNoApplicationModules(t *testing.T) {
	modules, err := Build(config.ApplicationConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 0 {
		t.Fatalf("expected no modules, got %+v", modules)
	}
}

func TestBuildRejectsUnknownModule(t *testing.T) {
	_, err := Build(config.ApplicationConfig{Modules: []string{"unknown"}})
	if !errors.Is(err, vexoapp.ErrModuleNotFound) {
		t.Fatalf("expected unknown module error, got %v", err)
	}
}

func TestBuildWithExecutionInjectsStakingFeeCollector(t *testing.T) {
	modules, err := BuildWithExecution(config.ApplicationConfig{Modules: []string{"staking"}}, config.ExecutionConfig{FeeCollector: "treasury"})
	if err != nil {
		t.Fatal(err)
	}
	stakingModule, ok := modules[0].(*staking.Module)
	if !ok {
		t.Fatalf("expected staking module, got %T", modules[0])
	}
	if stakingModule.FeeCollector() != "treasury" {
		t.Fatalf("expected treasury fee collector, got %s", stakingModule.FeeCollector())
	}
}

func TestBuildWithChainConfigInjectsStakingPolicy(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"staking"}
	cfg.Execution.FeeCollector = "treasury"
	cfg.Staking.UnbondingDelay = 42
	cfg.Staking.MaxCommissionBPS = 750
	modules, err := BuildWithChainConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	stakingModule, ok := modules[0].(*staking.Module)
	if !ok {
		t.Fatalf("expected staking module, got %T", modules[0])
	}
	policy := stakingModule.Policy()
	if policy.FeeCollector != "treasury" || policy.UnbondingDelay != 42 || policy.MaxCommissionBPS != 750 {
		t.Fatalf("unexpected staking policy: %+v", policy)
	}
}

func TestBuildWithChainConfigInjectsBankMintAuthority(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"bank"}
	cfg.Bank.MintAuthority = "governance"
	modules, err := BuildWithChainConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	bankModule, ok := modules[0].(bank.Module)
	if !ok {
		t.Fatalf("expected bank module, got %T", modules[0])
	}
	if bankModule.MintAuthority() != "governance" {
		t.Fatalf("expected governance mint authority, got %s", bankModule.MintAuthority())
	}
}

func TestBuildWithChainConfigWiresEVMPolicy(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"evm"}
	cfg.Execution.EVMChainID = 77
	cfg.Execution.EVMForkPreset = "latest"
	cfg.Execution.MaxBlobSidecarBlobs = 2
	cfg.Execution.MaxBlobSidecarBytes = 1000
	modules, err := BuildWithChainConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 {
		t.Fatalf("expected one module, got %d", len(modules))
	}
	if _, ok := modules[0].(appevm.Module); !ok {
		t.Fatalf("expected EVM module, got %T", modules[0])
	}
}

func TestBuildWithChainConfigRejectsInvalidEVMChainConfig(t *testing.T) {
	cfg := config.Default("vexo-test")
	cfg.Application.Modules = []string{"evm"}
	cfg.Execution.EVMChainConfigJSON = `{invalid`
	if _, err := BuildWithChainConfig(cfg); err == nil {
		t.Fatal("expected invalid geth chain config JSON to fail")
	}
}

func TestNewRuntimeUsesConfiguredModules(t *testing.T) {
	runtime, err := NewRuntime("vexo-test", config.Default("vexo-test").Application)
	if err != nil {
		t.Fatal(err)
	}
	modules := runtime.Modules()
	if len(modules) != 6 || modules[0].Name() != "bank" || modules[1].Name() != "staking" || modules[2].Name() != "governance" || modules[3].Name() != "params" || modules[4].Name() != "ibc" || modules[5].Name() != "slashing" {
		t.Fatalf("unexpected runtime modules: %+v", modules)
	}
}

// authenticatingModule is a module that vouches for every Ethereum-tagged
// transaction routed to it.
type authenticatingModule struct{ name string }

func (module authenticatingModule) Name() string { return module.name }

func (authenticatingModule) InitGenesis(vexoapp.Context, vexoapp.GenesisState) error { return nil }

func (authenticatingModule) BeginBlock(vexoapp.Context, types.Header) error { return nil }

func (authenticatingModule) DeliverTx(vexoapp.Context, types.Tx) types.Result { return types.Result{} }

func (authenticatingModule) EndBlock(vexoapp.Context) error { return nil }

func (authenticatingModule) AuthenticateTx(vexoapp.Context, types.Tx) error { return nil }

// Only the module named evm may authenticate an unsigned Ethereum
// transaction. A bank transaction with Ethereum tags was a signature bypass.
func TestAnteDelegatesEthereumAuthenticationToTheEVMModuleOnly(t *testing.T) {
	execution := config.Default("vexo-test").Execution
	runtime, err := vexoapp.NewRuntime("vexo-test", []vexoapp.Module{
		authenticatingModule{name: "bank"},
		authenticatingModule{name: "evm"},
	}, vexoapp.PrefixRouter{})
	if err != nil {
		t.Fatal(err)
	}
	runtime.WithAnte(newAnteKeeper(execution, runtime.Modules()))

	evmTx := types.Tx("evm:call:signer=0xabc:nonce=0:fee=0:gas=21000:eth_raw=00:eth_hash=00")
	if response := runtime.CheckTx(evmTx); response.Result.Code != 0 {
		t.Fatalf("expected the evm module to authenticate its own transaction: %+v", response.Result)
	}
	bankTx := types.Tx("bank:mint:attacker:1000000000:gas=10:signer=governance:nonce=0:eth_raw=00:eth_hash=00")
	if response := runtime.CheckTx(bankTx); response.Result.Code == 0 {
		t.Fatal("expected an Ethereum-tagged bank transaction to be rejected")
	}
}

// The base fee is application state moved by the execution configuration: an
// empty block uses no gas, so a dynamic base fee falls towards its floor.
func TestRuntimeMovesTheBaseFeeByTheExecutionConfig(t *testing.T) {
	execution := config.Default("vexo-test").Execution
	execution.BaseFee = 1000
	execution.DynamicBaseFee = true
	execution.TargetGas = 100
	execution.BaseFeeChangeDenominator = 8
	execution.MinBaseFee = 1
	execution.MaxBaseFee = 1_000_000
	runtime, err := NewRuntimeWithExecution("vexo-test", config.ApplicationConfig{}, execution)
	if err != nil {
		t.Fatal(err)
	}
	storage, err := store.OpenLevelDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	runtime.WithStore(storage)

	if _, err := runtime.FinalizeBlock(vexoapp.FinalizeBlockRequest{Block: types.Block{Header: types.Header{ChainID: "vexo-test", Height: 1}}}); err != nil {
		t.Fatal(err)
	}
	state, err := storage.Get(context.Background(), "feemarket", []byte("state"))
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 16 || binary.BigEndian.Uint64(state[:8]) != 875 {
		t.Fatalf("expected the base fee to fall from 1000 to 875, got %x", state)
	}
}
