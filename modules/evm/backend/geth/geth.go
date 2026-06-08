package geth

import (
	"bytes"
	"context"
	"encoding/json"
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
	gethlogger "github.com/ethereum/go-ethereum/eth/tracers/logger"
	gethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/vexo-network/vexo-consensus/contract"
	"github.com/vexo-network/vexo-consensus/types"
)

const GethVMName = "evm"

type GethVM struct {
	chainConfig *gethparams.ChainConfig
}

func New() GethVM {
	return NewWithChainConfig(nil)
}

func NewWithChainConfig(chainConfig *gethparams.ChainConfig) GethVM {
	return GethVM{chainConfig: normalizedChainConfig(chainConfig)}
}

func (GethVM) Name() string { return GethVMName }

func (vm GethVM) Execute(ctx context.Context, invocation contract.Invocation) (contract.Result, error) {
	caller := gethAddress(invocation.Caller)
	contractAddress := gethAddress(invocation.Contract)
	stateDB := newGethStateDB(ctx, invocation)
	stateDB.CreateAccount(caller)
	if invocation.Method != "deploy" {
		stateDB.CreateAccount(contractAddress)
	}
	if invocation.Method != "deploy" && len(invocation.Code) > 0 {
		stateDB.SetCode(contractAddress, invocation.Code, gethtracing.CodeChangeUnspecified)
	}

	traceLogger := gethlogger.NewStructLogger(&gethlogger.Config{
		EnableReturnData: true,
		DisableStack:     false,
		DisableStorage:   false,
	})
	chainConfig := vm.activeChainConfig()
	evm := gethvm.NewEVM(gethBlockContext(invocation, stateDB), stateDB, chainConfig, gethvm.Config{Tracer: traceLogger.Hooks()})
	evm.SetTxContext(gethvm.TxContext{
		Origin:     caller,
		GasPrice:   new(uint256.Int).SetUint64(invocation.GasPrice),
		BlobHashes: gethBlobHashes(invocation.BlobHashes),
	})
	rules := chainConfig.Rules(new(big.Int).SetUint64(invocation.BlockNumber), true, invocation.Timestamp)
	var destination *gethcommon.Address
	if invocation.Method != "deploy" {
		destination = &contractAddress
	}
	stateDB.Prepare(rules, caller, gethAddress(invocation.Coinbase), destination, gethvm.ActivePrecompiles(rules), gethAccessList(invocation.AccessList))
	traceLogger.OnTxStart(evm.GetVMContext(), nil, caller)

	gasLimit := invocation.GasLimit
	if gasLimit == 0 {
		gasLimit = 10_000_000
	}
	initialGas := gethvm.NewGasBudget(gasLimit)
	value, err := invocationValue(invocation)
	if err != nil {
		return contract.Result{}, err
	}
	var output []byte
	var left gethvm.GasBudget
	if invocation.Method == "deploy" {
		if len(invocation.Salt) > 0 {
			var salt uint256.Int
			salt.SetBytes(invocation.Salt)
			output, contractAddress, left, err = evm.Create2(caller, invocation.Input, initialGas, value, &salt)
		} else {
			output, contractAddress, left, err = evm.Create(caller, invocation.Input, initialGas, value)
		}
	} else if invocation.ReadOnly {
		output, left, err = evm.StaticCall(caller, contractAddress, invocation.Input, initialGas)
	} else {
		output, left, err = evm.Call(caller, contractAddress, invocation.Input, initialGas, value)
	}
	if err != nil {
		traceLogger.OnTxEnd(&gethtypes.Receipt{GasUsed: left.Used(initialGas)}, err)
		vmTrace, traceErr := gethVMTrace(traceLogger)
		if traceErr != nil {
			return contract.Result{}, traceErr
		}
		return contract.Result{
			Output:  append([]byte(nil), output...),
			GasUsed: left.Used(initialGas),
			Failed:  true,
			Error:   err.Error(),
			VMTrace: vmTrace,
		}, nil
	}
	traceLogger.OnTxEnd(&gethtypes.Receipt{GasUsed: left.Used(initialGas)}, nil)
	vmTrace, err := gethVMTrace(traceLogger)
	if err != nil {
		return contract.Result{}, err
	}
	stateDB.Finalise(true)
	balanceWrites, err := stateDB.BalanceWrites()
	if err != nil {
		return contract.Result{}, err
	}
	result := contract.Result{
		Output:           append([]byte(nil), output...),
		GasUsed:          left.Used(initialGas),
		DeployedCode:     append([]byte(nil), stateDB.GetCode(contractAddress)...),
		CodeWrites:       stateDB.CodeWrites(),
		StorageWrites:    stateDB.StorageWrites(),
		BalanceWrites:    balanceWrites,
		NonceWrites:      stateDB.NonceWrites(),
		AccountDeletions: stateDB.AccountDeletions(),
		AccessList:       stateDB.ContractAccessList(),
		Logs:             stateDB.ContractLogs(),
		VMTrace:          vmTrace,
	}
	if invocation.Method != "deploy" {
		result.DeployedCode = nil
	}
	return result, nil
}

func gethBlobHashes(hashes []types.Hash) []gethcommon.Hash {
	if len(hashes) == 0 {
		return nil
	}
	out := make([]gethcommon.Hash, len(hashes))
	for index, hash := range hashes {
		out[index] = gethcommon.Hash(hash)
	}
	return out
}

func invocationValue(invocation contract.Invocation) (*uint256.Int, error) {
	if invocation.ValueBig != nil {
		value, overflow := uint256.FromBig(invocation.ValueBig)
		if overflow {
			return nil, contract.ErrInvalidInvocation
		}
		return value, nil
	}
	return new(uint256.Int).SetUint64(invocation.Value), nil
}

func gethAccessList(entries []contract.AccessListEntry) gethtypes.AccessList {
	if len(entries) == 0 {
		return nil
	}
	out := make(gethtypes.AccessList, 0, len(entries))
	for _, entry := range entries {
		access := gethtypes.AccessTuple{
			Address:     gethAddress(entry.Address),
			StorageKeys: make([]gethcommon.Hash, 0, len(entry.StorageKeys)),
		}
		for _, slot := range entry.StorageKeys {
			access.StorageKeys = append(access.StorageKeys, gethcommon.HexToHash(slot))
		}
		out = append(out, access)
	}
	return out
}

func gethVMTrace(traceLogger *gethlogger.StructLogger) (any, error) {
	raw, err := traceLogger.GetResult()
	if err != nil {
		return nil, err
	}
	var trace map[string]any
	if err := json.Unmarshal(raw, &trace); err != nil {
		return nil, err
	}
	return trace, nil
}

func gethBlockContext(invocation contract.Invocation, stateDB *gethStateDB) gethvm.BlockContext {
	baseFee := new(big.Int).SetUint64(invocation.BaseFee)
	blobBaseFee := new(big.Int).SetUint64(invocation.BlobBaseFee)
	random := gethcommon.Hash(invocation.PrevRandao)
	gasLimit := invocation.BlockGasLimit
	if gasLimit == 0 {
		gasLimit = invocation.GasLimit
	}
	return gethvm.BlockContext{
		CanTransfer: func(db gethvm.StateDB, address gethcommon.Address, amount *uint256.Int) bool {
			return db.GetBalance(address).Cmp(amount) >= 0
		},
		Transfer: func(db gethvm.StateDB, from gethcommon.Address, to gethcommon.Address, amount *uint256.Int, rules *gethparams.Rules) {
			db.SubBalance(from, amount, gethtracing.BalanceChangeTransfer)
			db.AddBalance(to, amount, gethtracing.BalanceChangeTransfer)
		},
		Coinbase:    gethAddress(invocation.Coinbase),
		GetHash:     stateDB.blockHash,
		GasLimit:    gasLimit,
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
	committedBal   uint256.Int
	nonce          uint64
	committedNonce uint64
	code           []byte
	committedCode  []byte
	storage        map[gethcommon.Hash]gethcommon.Hash
	committed      map[gethcommon.Hash]gethcommon.Hash
	transient      map[gethcommon.Hash]gethcommon.Hash
	selfDestructed bool
	deleted        bool
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
	return db.account(address).empty()
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
	for _, account := range db.accounts {
		if !account.selfDestructed && (!deleteEmptyObjects || !account.touched || !account.empty()) {
			continue
		}
		account.deleted = true
		account.code = nil
		account.balance.Clear()
		account.nonce = 0
		for slot := range account.storage {
			account.storage[slot] = gethcommon.Hash{}
		}
	}
	return nil
}

func (db *gethStateDB) CodeWrites() []contract.CodeWrite {
	writes := make([]contract.CodeWrite, 0)
	addresses := db.sortedAddresses()
	for _, address := range addresses {
		account := db.accounts[address]
		if account.deleted {
			writes = append(writes, contract.CodeWrite{Address: types.Address(address.Hex()), Delete: true})
			continue
		}
		if bytes.Equal(account.code, account.committedCode) {
			continue
		}
		writes = append(writes, contract.CodeWrite{
			Address: types.Address(address.Hex()),
			Code:    append([]byte(nil), account.code...),
		})
	}
	return writes
}

func (db *gethStateDB) AccountDeletions() []contract.AccountDeletion {
	deletions := make([]contract.AccountDeletion, 0)
	for _, address := range db.sortedAddresses() {
		if db.accounts[address].deleted {
			deletions = append(deletions, contract.AccountDeletion{Address: types.Address(address.Hex())})
		}
	}
	return deletions
}

func (db *gethStateDB) StorageWrites() []contract.StorageWrite {
	writes := make([]contract.StorageWrite, 0)
	for _, address := range db.sortedAddresses() {
		account := db.accounts[address]
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

func (db *gethStateDB) BalanceWrites() ([]contract.BalanceWrite, error) {
	writes := make([]contract.BalanceWrite, 0)
	for _, address := range db.sortedAddresses() {
		account := db.accounts[address]
		if account.balance.Eq(&account.committedBal) {
			continue
		}
		write := contract.BalanceWrite{
			Address:    types.Address(address.Hex()),
			BalanceBig: account.balance.ToBig(),
		}
		if account.balance.IsUint64() {
			write.Balance = account.balance.Uint64()
		}
		writes = append(writes, write)
	}
	return writes, nil
}

func (db *gethStateDB) NonceWrites() []contract.NonceWrite {
	writes := make([]contract.NonceWrite, 0)
	for _, address := range db.sortedAddresses() {
		account := db.accounts[address]
		if account.nonce == account.committedNonce {
			continue
		}
		writes = append(writes, contract.NonceWrite{
			Address: types.Address(address.Hex()),
			Nonce:   account.nonce,
		})
	}
	return writes
}

func (db *gethStateDB) ContractAccessList() []contract.AccessListEntry {
	entries := make([]contract.AccessListEntry, 0, len(db.accessList))
	addresses := make([]gethcommon.Address, 0, len(db.accessList))
	for address := range db.accessList {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(first int, second int) bool {
		return addresses[first].Hex() < addresses[second].Hex()
	})
	for _, address := range addresses {
		slots := make([]gethcommon.Hash, 0, len(db.accessList[address]))
		for slot := range db.accessList[address] {
			slots = append(slots, slot)
		}
		sort.Slice(slots, func(first int, second int) bool {
			return slots[first].Hex() < slots[second].Hex()
		})
		entry := contract.AccessListEntry{Address: types.Address(address.Hex()), StorageKeys: make([]string, 0, len(slots))}
		for _, slot := range slots {
			entry.StorageKeys = append(entry.StorageKeys, slot.Hex())
		}
		entries = append(entries, entry)
	}
	return entries
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
			account.committedCode = append([]byte(nil), code...)
		}
		if balanceReader, ok := db.reader.(contract.BalanceBigReader); ok {
			if balance, err := balanceReader.BalanceBig(db.ctx, types.Address(address.Hex())); err == nil && balance != nil && balance.Sign() >= 0 {
				if value, overflow := uint256.FromBig(balance); !overflow {
					account.balance = *value
					account.committedBal = *value
				}
			}
		} else if balanceReader, ok := db.reader.(contract.BalanceReader); ok {
			if balance, err := balanceReader.Balance(db.ctx, types.Address(address.Hex())); err == nil {
				account.balance.SetUint64(balance)
				account.committedBal.SetUint64(balance)
			}
		}
		if nonceReader, ok := db.reader.(contract.NonceReader); ok {
			if nonce, err := nonceReader.Nonce(db.ctx, types.Address(address.Hex())); err == nil {
				account.nonce = nonce
				account.committedNonce = nonce
			}
		}
	}
	db.accounts[address] = account
	return account
}

func (account *gethAccount) empty() bool {
	return account.nonce == 0 && account.balance.IsZero() && len(account.code) == 0
}

func (db *gethStateDB) sortedAddresses() []gethcommon.Address {
	addresses := make([]gethcommon.Address, 0, len(db.accounts))
	for address := range db.accounts {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(first int, second int) bool {
		return addresses[first].Hex() < addresses[second].Hex()
	})
	return addresses
}

func (db *gethStateDB) blockHash(height uint64) gethcommon.Hash {
	if db.reader == nil {
		return gethcommon.Hash{}
	}
	reader, ok := db.reader.(contract.BlockHashReader)
	if !ok {
		return gethcommon.Hash{}
	}
	hash, err := reader.BlockHash(db.ctx, height)
	if err != nil {
		return gethcommon.Hash{}
	}
	return gethcommon.Hash(hash)
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

func gethAddress(address types.Address) gethcommon.Address {
	return gethcommon.HexToAddress(string(address))
}

func cloneAccounts(accounts map[gethcommon.Address]*gethAccount) map[gethcommon.Address]*gethAccount {
	cloned := make(map[gethcommon.Address]*gethAccount, len(accounts))
	for address, account := range accounts {
		copyAccount := *account
		copyAccount.code = append([]byte(nil), account.code...)
		copyAccount.committedCode = append([]byte(nil), account.committedCode...)
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
