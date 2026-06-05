package evm

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"sort"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethstate "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	gethtracing "github.com/ethereum/go-ethereum/core/tracing"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/bal"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	gethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

const GethVMName = "evm"

type GethVM struct{}

func NewGethVM() GethVM {
	return GethVM{}
}

func (GethVM) Name() string { return GethVMName }

func (GethVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	caller := gethAddress(invocation.Caller)
	contractAddress := gethAddress(invocation.Contract)
	stateDB := newGethStateDB(ctx, invocation)
	stateDB.CreateAccount(caller)
	stateDB.AddBalance(caller, new(uint256.Int).SetUint64(math.MaxUint64), gethtracing.BalanceIncreaseGenesisBalance)
	if invocation.Method != "deploy" {
		stateDB.CreateAccount(contractAddress)
	}
	if invocation.Method != "deploy" && len(invocation.Code) > 0 {
		stateDB.SetCode(contractAddress, invocation.Code, gethtracing.CodeChangeUnspecified)
	}

	evm := gethvm.NewEVM(gethBlockContext(invocation, stateDB), stateDB, gethparams.AllEthashProtocolChanges, gethvm.Config{})
	evm.SetTxContext(gethvm.TxContext{
		Origin:   caller,
		GasPrice: new(uint256.Int).SetUint64(invocation.GasPrice),
	})

	gasLimit := invocation.GasLimit
	if gasLimit == 0 {
		gasLimit = 10_000_000
	}
	initialGas := gethvm.NewGasBudget(gasLimit)
	value := new(uint256.Int).SetUint64(invocation.Value)
	var output []byte
	var left gethvm.GasBudget
	var err error
	if invocation.Method == "deploy" {
		var salt uint256.Int
		salt.SetBytes(invocation.Salt)
		output, contractAddress, left, err = evm.Create2(caller, invocation.Input, initialGas, value, &salt)
	} else if invocation.ReadOnly {
		output, left, err = evm.StaticCall(caller, contractAddress, invocation.Input, initialGas)
	} else {
		output, left, err = evm.Call(caller, contractAddress, invocation.Input, initialGas, value)
	}
	if err != nil {
		return contract.Result{}, err
	}
	result := contract.Result{
		Output:        append([]byte(nil), output...),
		GasUsed:       left.Used(initialGas),
		DeployedCode:  append([]byte(nil), stateDB.GetCode(contractAddress)...),
		StorageWrites: stateDB.StorageWrites(),
		Logs:          stateDB.ContractLogs(),
	}
	if invocation.Method != "deploy" {
		result.DeployedCode = nil
	}
	return result, nil
}

func gethBlockContext(invocation contract.Invocation, stateDB *gethStateDB) gethvm.BlockContext {
	baseFee := new(big.Int).SetUint64(invocation.BaseFee)
	blobBaseFee := new(big.Int)
	random := gethcommon.Hash{}
	return gethvm.BlockContext{
		CanTransfer: func(db gethvm.StateDB, address gethcommon.Address, amount *uint256.Int) bool {
			return db.GetBalance(address).Cmp(amount) >= 0
		},
		Transfer: func(db gethvm.StateDB, from gethcommon.Address, to gethcommon.Address, amount *uint256.Int, rules *gethparams.Rules) {
			db.SubBalance(from, amount, gethtracing.BalanceChangeTransfer)
			db.AddBalance(to, amount, gethtracing.BalanceChangeTransfer)
		},
		GetHash:     func(uint64) gethcommon.Hash { return gethcommon.Hash{} },
		GasLimit:    invocation.GasLimit,
		BlockNumber: new(big.Int).SetUint64(invocation.BlockNumber),
		Time:        invocation.Timestamp,
		Difficulty:  new(big.Int),
		BaseFee:     baseFee,
		BlobBaseFee: blobBaseFee,
		Random:      &random,
	}
}

type gethStateDB struct {
	ctx          context.Context
	reader       contract.StateReader
	accounts     map[gethcommon.Address]*gethAccount
	refund       uint64
	logs         []*gethtypes.Log
	preimages    map[gethcommon.Hash][]byte
	snapshots    []gethSnapshot
	accessList   map[gethcommon.Address]map[gethcommon.Hash]struct{}
	accessEvents *gethstate.AccessEvents
}

type gethAccount struct {
	balance        uint256.Int
	nonce          uint64
	code           []byte
	storage        map[gethcommon.Hash]gethcommon.Hash
	committed      map[gethcommon.Hash]gethcommon.Hash
	transient      map[gethcommon.Hash]gethcommon.Hash
	selfDestructed bool
	newContract    bool
	touched        bool
}

type gethSnapshot struct {
	accounts   map[gethcommon.Address]*gethAccount
	refund     uint64
	logs       []*gethtypes.Log
	preimages  map[gethcommon.Hash][]byte
	accessList map[gethcommon.Address]map[gethcommon.Hash]struct{}
}

func newGethStateDB(ctx context.Context, invocation contract.Invocation) *gethStateDB {
	return &gethStateDB{
		ctx:          ctx,
		reader:       invocation.State,
		accounts:     make(map[gethcommon.Address]*gethAccount),
		preimages:    make(map[gethcommon.Hash][]byte),
		accessList:   make(map[gethcommon.Address]map[gethcommon.Hash]struct{}),
		accessEvents: gethstate.NewAccessEvents(),
	}
}

func (db *gethStateDB) CreateAccount(address gethcommon.Address) {
	db.account(address)
}

func (db *gethStateDB) CreateContract(address gethcommon.Address) {
	account := db.account(address)
	account.newContract = true
}

func (db *gethStateDB) SubBalance(address gethcommon.Address, amount *uint256.Int, reason gethtracing.BalanceChangeReason) uint256.Int {
	account := db.account(address)
	previous := account.balance
	if account.balance.Cmp(amount) < 0 {
		account.balance.Clear()
		return previous
	}
	account.balance.Sub(&account.balance, amount)
	return previous
}

func (db *gethStateDB) AddBalance(address gethcommon.Address, amount *uint256.Int, reason gethtracing.BalanceChangeReason) uint256.Int {
	account := db.account(address)
	previous := account.balance
	account.balance.Add(&account.balance, amount)
	return previous
}

func (db *gethStateDB) GetBalance(address gethcommon.Address) *uint256.Int {
	account := db.account(address)
	return new(uint256.Int).Set(&account.balance)
}

func (db *gethStateDB) GetNonce(address gethcommon.Address) uint64 {
	return db.account(address).nonce
}

func (db *gethStateDB) SetNonce(address gethcommon.Address, nonce uint64, reason gethtracing.NonceChangeReason) {
	db.account(address).nonce = nonce
}

func (db *gethStateDB) GetCodeHash(address gethcommon.Address) gethcommon.Hash {
	code := db.GetCode(address)
	if len(code) == 0 {
		return gethcommon.Hash{}
	}
	return gethcrypto.Keccak256Hash(code)
}

func (db *gethStateDB) GetCode(address gethcommon.Address) []byte {
	return append([]byte(nil), db.account(address).code...)
}

func (db *gethStateDB) SetCode(address gethcommon.Address, code []byte, reason gethtracing.CodeChangeReason) []byte {
	account := db.account(address)
	previous := append([]byte(nil), account.code...)
	account.code = append([]byte(nil), code...)
	return previous
}

func (db *gethStateDB) GetCodeSize(address gethcommon.Address) int {
	return len(db.account(address).code)
}

func (db *gethStateDB) AddRefund(gas uint64) {
	db.refund += gas
}

func (db *gethStateDB) SubRefund(gas uint64) {
	if gas >= db.refund {
		db.refund = 0
		return
	}
	db.refund -= gas
}

func (db *gethStateDB) GetRefund() uint64 {
	return db.refund
}

func (db *gethStateDB) GetStateAndCommittedState(address gethcommon.Address, slot gethcommon.Hash) (gethcommon.Hash, gethcommon.Hash) {
	value := db.GetState(address, slot)
	committed := db.account(address).committed[slot]
	return value, committed
}

func (db *gethStateDB) GetState(address gethcommon.Address, slot gethcommon.Hash) gethcommon.Hash {
	account := db.account(address)
	if value, found := account.storage[slot]; found {
		return value
	}
	value := db.loadStorage(address, slot)
	account.storage[slot] = value
	account.committed[slot] = value
	return value
}

func (db *gethStateDB) SetState(address gethcommon.Address, slot gethcommon.Hash, value gethcommon.Hash) gethcommon.Hash {
	account := db.account(address)
	previous := db.GetState(address, slot)
	account.storage[slot] = value
	return previous
}

func (db *gethStateDB) GetTransientState(address gethcommon.Address, slot gethcommon.Hash) gethcommon.Hash {
	return db.account(address).transient[slot]
}

func (db *gethStateDB) SetTransientState(address gethcommon.Address, slot gethcommon.Hash, value gethcommon.Hash) {
	db.account(address).transient[slot] = value
}

func (db *gethStateDB) SelfDestruct(address gethcommon.Address) {
	db.account(address).selfDestructed = true
}

func (db *gethStateDB) HasSelfDestructed(address gethcommon.Address) bool {
	return db.account(address).selfDestructed
}

func (db *gethStateDB) Exist(address gethcommon.Address) bool {
	account := db.account(address)
	return account.touched || len(account.code) > 0 || !account.balance.IsZero() || account.nonce > 0 || len(account.storage) > 0
}

func (db *gethStateDB) Touch(address gethcommon.Address) {
	db.account(address).touched = true
}

func (db *gethStateDB) IsNewContract(address gethcommon.Address) bool {
	return db.account(address).newContract
}

func (db *gethStateDB) Empty(address gethcommon.Address) bool {
	account := db.account(address)
	return account.nonce == 0 && account.balance.IsZero() && len(account.code) == 0
}

func (db *gethStateDB) AddressInAccessList(address gethcommon.Address) bool {
	_, found := db.accessList[address]
	return found
}

func (db *gethStateDB) SlotInAccessList(address gethcommon.Address, slot gethcommon.Hash) (bool, bool) {
	slots, addressOK := db.accessList[address]
	if !addressOK {
		return false, false
	}
	_, slotOK := slots[slot]
	return true, slotOK
}

func (db *gethStateDB) AddAddressToAccessList(address gethcommon.Address) {
	if db.accessList[address] == nil {
		db.accessList[address] = make(map[gethcommon.Hash]struct{})
	}
}

func (db *gethStateDB) AddSlotToAccessList(address gethcommon.Address, slot gethcommon.Hash) {
	db.AddAddressToAccessList(address)
	db.accessList[address][slot] = struct{}{}
}

func (db *gethStateDB) Prepare(rules gethparams.Rules, sender gethcommon.Address, coinbase gethcommon.Address, dest *gethcommon.Address, precompiles []gethcommon.Address, txAccesses gethtypes.AccessList) {
	db.AddAddressToAccessList(sender)
	db.AddAddressToAccessList(coinbase)
	if dest != nil {
		db.AddAddressToAccessList(*dest)
	}
	for _, address := range precompiles {
		db.AddAddressToAccessList(address)
	}
	for _, access := range txAccesses {
		db.AddAddressToAccessList(access.Address)
		for _, slot := range access.StorageKeys {
			db.AddSlotToAccessList(access.Address, slot)
		}
	}
}

func (db *gethStateDB) Snapshot() int {
	snapshot := gethSnapshot{
		accounts:   cloneAccounts(db.accounts),
		refund:     db.refund,
		logs:       append([]*gethtypes.Log(nil), db.logs...),
		preimages:  cloneBytesMap(db.preimages),
		accessList: cloneAccessList(db.accessList),
	}
	db.snapshots = append(db.snapshots, snapshot)
	return len(db.snapshots) - 1
}

func (db *gethStateDB) RevertToSnapshot(id int) {
	if id < 0 || id >= len(db.snapshots) {
		return
	}
	snapshot := db.snapshots[id]
	db.accounts = cloneAccounts(snapshot.accounts)
	db.refund = snapshot.refund
	db.logs = append([]*gethtypes.Log(nil), snapshot.logs...)
	db.preimages = cloneBytesMap(snapshot.preimages)
	db.accessList = cloneAccessList(snapshot.accessList)
	db.snapshots = db.snapshots[:id]
}

func (db *gethStateDB) AddLog(log *gethtypes.Log) {
	if log == nil {
		return
	}
	copied := *log
	copied.Topics = append([]gethcommon.Hash(nil), log.Topics...)
	copied.Data = append([]byte(nil), log.Data...)
	db.logs = append(db.logs, &copied)
}

func (db *gethStateDB) LogsForBurnAccounts() []*gethtypes.Log {
	return nil
}

func (db *gethStateDB) AddPreimage(hash gethcommon.Hash, preimage []byte) {
	db.preimages[hash] = append([]byte(nil), preimage...)
}

func (db *gethStateDB) Witness() *stateless.Witness {
	return nil
}

func (db *gethStateDB) AccessEvents() *gethstate.AccessEvents {
	return db.accessEvents
}

func (db *gethStateDB) Finalise(deleteEmptyObjects bool) *bal.StateAccessList {
	return nil
}

func (db *gethStateDB) StorageWrites() []contract.StorageWrite {
	writes := make([]contract.StorageWrite, 0)
	for address, account := range db.accounts {
		slots := make([]gethcommon.Hash, 0, len(account.storage))
		for slot := range account.storage {
			if account.storage[slot] != account.committed[slot] {
				slots = append(slots, slot)
			}
		}
		sort.Slice(slots, func(first int, second int) bool {
			return slots[first].Hex() < slots[second].Hex()
		})
		for _, slot := range slots {
			value := account.storage[slot]
			write := contract.StorageWrite{
				Address: types.Address(address.Hex()),
				Slot:    slot.Hex(),
				Value:   append([]byte(nil), value.Bytes()...),
			}
			if value == (gethcommon.Hash{}) {
				write.Delete = true
				write.Value = nil
			}
			writes = append(writes, write)
		}
	}
	sort.Slice(writes, func(first int, second int) bool {
		if writes[first].Address == writes[second].Address {
			return writes[first].Slot < writes[second].Slot
		}
		return writes[first].Address < writes[second].Address
	})
	return writes
}

func (db *gethStateDB) ContractLogs() []contract.Log {
	logs := make([]contract.Log, 0, len(db.logs))
	for _, log := range db.logs {
		topics := make([]string, 0, len(log.Topics))
		for _, topic := range log.Topics {
			topics = append(topics, topic.Hex())
		}
		logs = append(logs, contract.Log{
			Address: types.Address(log.Address.Hex()),
			Topics:  topics,
			Data:    append([]byte(nil), log.Data...),
		})
	}
	return logs
}

func (db *gethStateDB) account(address gethcommon.Address) *gethAccount {
	account := db.accounts[address]
	if account != nil {
		return account
	}
	account = &gethAccount{
		storage:   make(map[gethcommon.Hash]gethcommon.Hash),
		committed: make(map[gethcommon.Hash]gethcommon.Hash),
		transient: make(map[gethcommon.Hash]gethcommon.Hash),
	}
	if db.reader != nil {
		if code, err := db.reader.Code(db.ctx, types.Address(address.Hex())); err == nil && len(code) > 0 {
			account.code = append([]byte(nil), code...)
		}
	}
	db.accounts[address] = account
	return account
}

func (db *gethStateDB) loadStorage(address gethcommon.Address, slot gethcommon.Hash) gethcommon.Hash {
	if db.reader == nil {
		return gethcommon.Hash{}
	}
	value, err := db.reader.Storage(db.ctx, types.Address(address.Hex()), slot.Hex())
	if err != nil || len(value) == 0 {
		return gethcommon.Hash{}
	}
	return gethcommon.BytesToHash(value)
}

func cloneAccounts(accounts map[gethcommon.Address]*gethAccount) map[gethcommon.Address]*gethAccount {
	cloned := make(map[gethcommon.Address]*gethAccount, len(accounts))
	for address, account := range accounts {
		copyAccount := *account
		copyAccount.code = append([]byte(nil), account.code...)
		copyAccount.storage = make(map[gethcommon.Hash]gethcommon.Hash, len(account.storage))
		for slot, value := range account.storage {
			copyAccount.storage[slot] = value
		}
		copyAccount.committed = make(map[gethcommon.Hash]gethcommon.Hash, len(account.committed))
		for slot, value := range account.committed {
			copyAccount.committed[slot] = value
		}
		copyAccount.transient = make(map[gethcommon.Hash]gethcommon.Hash, len(account.transient))
		for slot, value := range account.transient {
			copyAccount.transient[slot] = value
		}
		cloned[address] = &copyAccount
	}
	return cloned
}

func cloneBytesMap(values map[gethcommon.Hash][]byte) map[gethcommon.Hash][]byte {
	cloned := make(map[gethcommon.Hash][]byte, len(values))
	for key, value := range values {
		cloned[key] = append([]byte(nil), value...)
	}
	return cloned
}

func cloneAccessList(values map[gethcommon.Address]map[gethcommon.Hash]struct{}) map[gethcommon.Address]map[gethcommon.Hash]struct{} {
	cloned := make(map[gethcommon.Address]map[gethcommon.Hash]struct{}, len(values))
	for address, slots := range values {
		cloned[address] = make(map[gethcommon.Hash]struct{}, len(slots))
		for slot := range slots {
			cloned[address][slot] = struct{}{}
		}
	}
	return cloned
}

func bytesToHex(value []byte) string {
	if len(value) == 0 {
		return "0x"
	}
	return "0x" + hex.EncodeToString(value)
}
