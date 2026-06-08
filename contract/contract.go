package contract

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"sync"

	"github.com/vexo-network/vexo-consensus/types"
)

var (
	ErrVMNotFound        = errors.New("contract VM not found")
	ErrVMAlreadyExists   = errors.New("contract VM already exists")
	ErrInvalidInvocation = errors.New("invalid contract invocation")
)

type Invocation struct {
	VM         string            `json:"vm"`
	Caller     types.Address     `json:"caller"`
	Contract   types.Address     `json:"contract"`
	Method     string            `json:"method"`
	Input      []byte            `json:"input,omitempty"`
	GasLimit   uint64            `json:"gas_limit,omitempty"`
	Value      uint64            `json:"value,omitempty"`
	ValueBig   *big.Int          `json:"-"`
	Code       []byte            `json:"-"`
	Salt       []byte            `json:"-"`
	State      StateReader       `json:"-"`
	ReadOnly   bool              `json:"-"`
	AccessList []AccessListEntry `json:"-"`

	BlockNumber               uint64        `json:"-"`
	Nonce                     uint64        `json:"-"`
	Timestamp                 uint64        `json:"-"`
	BaseFee                   uint64        `json:"-"`
	BlobBaseFee               uint64        `json:"-"`
	BlobHashes                []types.Hash  `json:"-"`
	GasPrice                  uint64        `json:"-"`
	GasFeeCap                 uint64        `json:"-"`
	GasTipCap                 uint64        `json:"-"`
	BlobGasFeeCap             uint64        `json:"-"`
	BlockGasLimit             uint64        `json:"-"`
	Coinbase                  types.Address `json:"-"`
	PrevRandao                types.Hash    `json:"-"`
	EthereumTx                bool          `json:"-"`
	EthereumSimulation        bool          `json:"-"`
	RawEthereumTx             string        `json:"-"`
	SetCodeAuthorizationsJSON string        `json:"-"`
}

type Result struct {
	Output           []byte            `json:"output,omitempty"`
	GasUsed          uint64            `json:"gas_used,omitempty"`
	Failed           bool              `json:"failed,omitempty"`
	Error            string            `json:"error,omitempty"`
	DeployedCode     []byte            `json:"deployed_code,omitempty"`
	Logs             []Log             `json:"logs,omitempty"`
	VMTrace          any               `json:"vm_trace,omitempty"`
	CodeWrites       []CodeWrite       `json:"code_writes,omitempty"`
	StorageWrites    []StorageWrite    `json:"storage_writes,omitempty"`
	BalanceWrites    []BalanceWrite    `json:"balance_writes,omitempty"`
	NonceWrites      []NonceWrite      `json:"nonce_writes,omitempty"`
	AccountDeletions []AccountDeletion `json:"account_deletions,omitempty"`
	AccessList       []AccessListEntry `json:"access_list,omitempty"`
}

type Log struct {
	Address types.Address     `json:"address"`
	Topics  []string          `json:"topics,omitempty"`
	Data    []byte            `json:"data,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

type StorageWrite struct {
	Address types.Address `json:"address,omitempty"`
	Slot    string        `json:"slot"`
	Value   []byte        `json:"value,omitempty"`
	Delete  bool          `json:"delete,omitempty"`
}

type CodeWrite struct {
	Address types.Address `json:"address"`
	Code    []byte        `json:"code,omitempty"`
	Delete  bool          `json:"delete,omitempty"`
}

type BalanceWrite struct {
	Address    types.Address `json:"address"`
	Balance    uint64        `json:"balance"`
	BalanceBig *big.Int      `json:"-"`
}

type NonceWrite struct {
	Address types.Address `json:"address"`
	Nonce   uint64        `json:"nonce"`
}

type AccountDeletion struct {
	Address types.Address `json:"address"`
}

type AccessListEntry struct {
	Address     types.Address `json:"address"`
	StorageKeys []string      `json:"storage_keys,omitempty"`
}

type VM interface {
	Name() string
	Execute(ctx context.Context, invocation Invocation) (Result, error)
}

type StateReader interface {
	Code(ctx context.Context, address types.Address) ([]byte, error)
	Storage(ctx context.Context, address types.Address, slot string) ([]byte, error)
}

type BalanceReader interface {
	Balance(ctx context.Context, address types.Address) (uint64, error)
}

type BalanceBigReader interface {
	BalanceBig(ctx context.Context, address types.Address) (*big.Int, error)
}

type NonceReader interface {
	Nonce(ctx context.Context, address types.Address) (uint64, error)
}

type BlockHashReader interface {
	BlockHash(ctx context.Context, height uint64) (types.Hash, error)
}

type EthereumHeader struct {
	Hash         types.Hash
	ParentHash   types.Hash
	StateRoot    types.Hash
	ReceiptRoot  types.Hash
	Number       uint64
	TimeUnixNano int64
}

type EthereumHeaderReader interface {
	EthereumHeader(ctx context.Context, height uint64) (EthereumHeader, error)
}

type Registry struct {
	mu  sync.RWMutex
	vms map[string]VM
}

func NewRegistry() *Registry {
	return &Registry{vms: make(map[string]VM)}
}

func (registry *Registry) Register(vm VM) error {
	if vm == nil || vm.Name() == "" {
		return ErrVMNotFound
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, found := registry.vms[vm.Name()]; found {
		return ErrVMAlreadyExists
	}
	registry.vms[vm.Name()] = vm
	return nil
}

func (registry *Registry) Execute(ctx context.Context, invocation Invocation) (Result, error) {
	if invocation.VM == "" || invocation.Caller == "" || invocation.Contract == "" || invocation.Method == "" {
		return Result{}, ErrInvalidInvocation
	}
	registry.mu.RLock()
	vm := registry.vms[invocation.VM]
	registry.mu.RUnlock()
	if vm == nil {
		return Result{}, ErrVMNotFound
	}
	return vm.Execute(ctx, cloneInvocation(invocation))
}

func (registry *Registry) Names() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	names := make([]string, 0, len(registry.vms))
	for name := range registry.vms {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cloneInvocation(invocation Invocation) Invocation {
	invocation.Input = append([]byte(nil), invocation.Input...)
	if invocation.ValueBig != nil {
		invocation.ValueBig = new(big.Int).Set(invocation.ValueBig)
	}
	invocation.Code = append([]byte(nil), invocation.Code...)
	invocation.Salt = append([]byte(nil), invocation.Salt...)
	invocation.AccessList = cloneAccessList(invocation.AccessList)
	invocation.BlobHashes = append([]types.Hash(nil), invocation.BlobHashes...)
	return invocation
}

func cloneAccessList(entries []AccessListEntry) []AccessListEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]AccessListEntry, len(entries))
	for index, entry := range entries {
		cloned[index] = entry
		cloned[index].StorageKeys = append([]string(nil), entry.StorageKeys...)
	}
	return cloned
}
