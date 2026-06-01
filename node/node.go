package node

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/vexo-network/vexo-consensus/app"
	vexoruntime "github.com/vexo-network/vexo-consensus/runtime"
	"github.com/vexo-network/vexo-consensus/store"
	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrMissingApplication = errors.New("application is required")
	ErrNodeAlreadyRunning = errors.New("node already running")
	ErrNodeNotRunning     = errors.New("node is not running")
)

type Status struct {
	ChainID       string
	Running       bool
	LatestHeight  types.Height
	LatestAppHash types.Hash
	DataDir       string
}

type Node struct {
	cfg     Config
	genesis Genesis
	app     app.Application

	mu      sync.Mutex
	runtime *vexoruntime.Runtime
	store   store.Store
	running bool
}

func New(cfg Config, genesis Genesis, application app.Application) (*Node, error) {
	if application == nil {
		return nil, ErrMissingApplication
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := genesis.Validate(cfg.Chain.ChainID); err != nil {
		return nil, err
	}
	return &Node{
		cfg:     cfg,
		genesis: genesis,
		app:     application,
	}, nil
}

func (node *Node) Start(ctx context.Context) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	if node.running {
		return ErrNodeAlreadyRunning
	}
	if err := os.MkdirAll(node.cfg.StoreDir(), 0o755); err != nil {
		return err
	}
	storage, err := store.OpenLevelDB(node.cfg.StoreDir())
	if err != nil {
		return err
	}

	runtime, err := vexoruntime.NewWithStore(
		node.cfg.Chain,
		node.app,
		node.genesis.Validators,
		node.genesis.Governance,
		storage,
	)
	if err != nil {
		storage.Close()
		return err
	}
	if _, err := node.app.InitChain(app.InitChainRequest{
		ChainID: node.genesis.ChainID,
		Genesis: node.genesis.AppState,
	}); err != nil {
		storage.Close()
		return err
	}

	node.runtime = runtime
	node.store = storage
	node.running = true
	return nil
}

func (node *Node) Stop(ctx context.Context) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if !node.running {
		return ErrNodeNotRunning
	}
	err := node.store.Close()
	node.running = false
	node.runtime = nil
	node.store = nil
	return err
}

func (node *Node) Runtime() (*vexoruntime.Runtime, error) {
	node.mu.Lock()
	defer node.mu.Unlock()

	if !node.running {
		return nil, ErrNodeNotRunning
	}
	return node.runtime, nil
}

func (node *Node) Status(ctx context.Context) Status {
	node.mu.Lock()
	defer node.mu.Unlock()

	status := Status{
		ChainID: node.cfg.Chain.ChainID,
		Running: node.running,
		DataDir: node.cfg.DataDir,
	}
	if !node.running || node.runtime == nil {
		return status
	}
	commit, err := node.runtime.App.Commit()
	if err == nil {
		status.LatestHeight = commit.Height
		status.LatestAppHash = commit.AppHash
	}
	return status
}
