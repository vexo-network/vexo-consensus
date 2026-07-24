package rpc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	vexocrypto "github.com/vexo-network/vexo-consensus/crypto"
	"github.com/vexo-network/vexo-consensus/modules"
	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/p2p"
	"github.com/vexo-network/vexo-consensus/transport"
	"github.com/vexo-network/vexo-consensus/types"
	"github.com/vexo-network/vexo-consensus/validator"
)

func TestWeb3DeploysCurrentSolidityBytecodeThroughRunningNode(t *testing.T) {
	const chainID = uint64(83960)
	ctx := context.Background()
	evmKey, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	sender := gethcrypto.PubkeyToAddress(evmKey.PublicKey)
	consensusSigner, err := vexocrypto.NewDeterministicSigner([]byte("rpc-evm-integration-validator"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := node.UnsafeTestConfig("vexo-test", t.TempDir())
	cfg.ValidatorID = "alice"
	cfg.Chain.Application.Modules = append(cfg.Chain.Application.Modules, "evm")
	cfg.Chain.Execution.EVMChainID = chainID
	cfg.Chain.Execution.EVMForkPreset = "latest"
	application, err := modules.NewRuntimeWithChainConfig(cfg.Chain.ChainID, cfg.Chain)
	if err != nil {
		t.Fatal(err)
	}
	genesis := node.Genesis{
		ChainID: cfg.Chain.ChainID,
		Validators: []validator.Validator{{
			ID: "alice", Address: "alice", VotingPower: 1, Stake: 1, PublicKey: consensusSigner.PublicKey(),
		}},
		AppState: map[string][]byte{
			"bank:" + sender.Hex(): []byte("100000000000000000000"),
		},
		Governance: map[types.Address]types.VotingPower{"alice": 1},
	}
	bus := transport.NewInMemoryBus()
	wire, err := bus.NewPeer(p2p.PeerID("alice"))
	if err != nil {
		t.Fatal(err)
	}
	runningNode, err := node.New(cfg, genesis, application)
	if err != nil {
		t.Fatal(err)
	}
	var eventMu sync.Mutex
	events := make([]string, 0)
	runningNode.WithSigner(consensusSigner).WithTransport(wire).WithEventLogger(func(event string, fields map[string]any) {
		encoded, _ := json.Marshal(fields)
		eventMu.Lock()
		events = append(events, event+":"+string(encoded))
		eventMu.Unlock()
	})
	if err := runningNode.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer runningNode.Stop(ctx)

	handler := NewHandler(runningNode)
	if got := web3NodeCall[string](t, handler, "eth_chainId"); got != "0x147f8" {
		t.Fatalf("unexpected chain id %q", got)
	}

	// Runtime bytecode uses Cancun MCOPY, as emitted by current Solidity.
	initCode, err := hex.DecodeString("6011600c60003960116000f3602a6000526020600060205e60206020f3")
	if err != nil {
		t.Fatal(err)
	}
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     0,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(20),
		Gas:       3_000_000,
		Value:     big.NewInt(0),
		Data:      initCode,
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), evmKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	txHash := web3NodeCall[string](t, handler, "eth_sendRawTransaction", "0x"+hex.EncodeToString(raw))
	if txHash != signed.Hash().Hex() {
		t.Fatalf("unexpected transaction hash %q", txHash)
	}

	receipt := commitWeb3Transaction(t, handler, runningNode, txHash, func() []string {
		eventMu.Lock()
		defer eventMu.Unlock()
		return append([]string(nil), events...)
	})
	if receipt["status"] != "0x1" {
		t.Fatalf("deployment failed: receipt=%+v", receipt)
	}
	contractAddress, ok := receipt["contractAddress"].(string)
	if !ok || contractAddress == "" {
		t.Fatalf("missing contract address: receipt=%+v", receipt)
	}
	code := web3NodeCall[string](t, handler, "eth_getCode", contractAddress, "latest")
	if code != "0x602a6000526020600060205e60206020f3" {
		t.Fatalf("unexpected deployed code %q", code)
	}
	output := web3NodeCall[string](t, handler, "eth_call", map[string]any{"to": contractAddress, "data": "0x"}, "latest")
	if output != "0x000000000000000000000000000000000000000000000000000000000000002a" {
		t.Fatalf("unexpected contract output %q", output)
	}
	storedTx := web3NodeCall[map[string]any](t, handler, "eth_getTransactionByHash", txHash)
	if storedTx["to"] != nil || storedTx["r"] == nil || storedTx["s"] == nil || storedTx["v"] == nil {
		t.Fatalf("invalid Ethereum creation transaction response: %+v", storedTx)
	}
	blockTx := web3NodeCall[map[string]any](t, handler, "eth_getTransactionByBlockNumberAndIndex", receipt["blockNumber"], receipt["transactionIndex"])
	if blockTx["to"] != nil || blockTx["r"] == nil || blockTx["s"] == nil || blockTx["v"] == nil {
		t.Fatalf("invalid creation transaction in block response: %+v", blockTx)
	}

	v1Creation := buildWeb3UUPSImplementationCreation(t, 1)
	v1Receipt := submitWeb3Transaction(t, handler, runningNode, evmKey, chainID, 1, nil, v1Creation, 3_000_000)
	v1Address := web3ContractAddress(t, v1Receipt)
	v2Creation := buildWeb3UUPSImplementationCreation(t, 2)
	v2Receipt := submitWeb3Transaction(t, handler, runningNode, evmKey, chainID, 2, nil, v2Creation, 3_000_000)
	v2Address := web3ContractAddress(t, v2Receipt)
	proxyCreation := buildWeb3DelegateProxyCreation(t, v1Address)
	proxyReceipt := submitWeb3Transaction(t, handler, runningNode, evmKey, chainID, 3, nil, proxyCreation, 3_000_000)
	proxyAddress := web3ContractAddress(t, proxyReceipt)

	proxyTarget := gethcrypto.CreateAddress(gethcrypto.PubkeyToAddress(evmKey.PublicKey), 3)
	if !strings.EqualFold(proxyTarget.Hex(), proxyAddress) {
		t.Fatalf("unexpected proxy address %s, want %s", proxyAddress, proxyTarget.Hex())
	}
	if got := web3NodeCall[string](t, handler, "eth_call", map[string]any{"from": sender.Hex(), "to": v1Address, "data": "0x54fd4d50"}, "latest"); got != web3Uint256(1) {
		t.Fatalf("unexpected direct implementation version %q", got)
	}
	proxyCode := web3NodeCall[string](t, handler, "eth_getCode", proxyAddress, "latest")
	proxyStorage := web3NodeCall[string](t, handler, "eth_getStorageAt", proxyAddress, "0x0", "latest")
	proxyBlock, _ := proxyReceipt["blockNumber"].(string)
	proxyHistoricalStorage := web3NodeCall[string](t, handler, "eth_getStorageAt", proxyAddress, "0x0", proxyBlock)
	if proxyCode == "0x" || !strings.EqualFold("0x"+strings.TrimLeft(strings.TrimPrefix(proxyStorage, "0x"), "0"), v1Address) {
		t.Fatalf("invalid deployed proxy state: code=%s implementation_slot=%s want=%s", proxyCode, proxyStorage, v1Address)
	}
	if !strings.EqualFold("0x"+strings.TrimLeft(strings.TrimPrefix(proxyHistoricalStorage, "0x"), "0"), v1Address) {
		t.Fatalf("invalid historical proxy state at %s: implementation_slot=%s want=%s", proxyBlock, proxyHistoricalStorage, v1Address)
	}
	if got := web3NodeCall[string](t, handler, "eth_call", map[string]any{"from": sender.Hex(), "to": proxyAddress, "data": "0x54fd4d50"}, "latest"); got != web3Uint256(1) {
		t.Fatalf("unexpected proxy version before initialization %q: code=%s storage=%s", got, proxyCode, proxyStorage)
	}
	submitWeb3Transaction(t, handler, runningNode, evmKey, chainID, 4, &proxyTarget, []byte{0x81, 0x29, 0xfc, 0x1c}, 500_000)
	if got := web3NodeCall[string](t, handler, "eth_call", map[string]any{"from": sender.Hex(), "to": proxyAddress, "data": "0x54fd4d50"}, "latest"); got != web3Uint256(1) {
		t.Fatalf("unexpected proxy version before upgrade %q", got)
	}
	upgradeInput := append([]byte{0x36, 0x59, 0xcf, 0xe6}, make([]byte, 12)...)
	upgradeInput = append(upgradeInput, gethcommon.HexToAddress(v2Address).Bytes()...)
	submitWeb3Transaction(t, handler, runningNode, evmKey, chainID, 5, &proxyTarget, upgradeInput, 500_000)
	if got := web3NodeCall[string](t, handler, "eth_call", map[string]any{"from": sender.Hex(), "to": proxyAddress, "data": "0x54fd4d50"}, "latest"); got != web3Uint256(2) {
		t.Fatalf("unexpected proxy version after upgrade %q", got)
	}

	tpsCheckCreation := web3TestCreationCode(t, "testdata/tps_check_creation.hex")
	tpsCheckReceipt := submitWeb3Transaction(t, handler, runningNode, evmKey, chainID, 6, nil, tpsCheckCreation, 3_000_000)
	tpsCheckAddress := web3ContractAddress(t, tpsCheckReceipt)
	if code := web3NodeCall[string](t, handler, "eth_getCode", tpsCheckAddress, "latest"); code == "0x" {
		t.Fatalf("TpsCheck deployment produced no runtime code: receipt=%+v", tpsCheckReceipt)
	}
	assertWeb3MinedReceiptLogs(t, tpsCheckReceipt)
}

func TestWeb3DeployAndUpgradeAcrossFourValidatorConsensus(t *testing.T) {
	const chainID = uint64(83960)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	evmKey, err := gethcrypto.HexToECDSA("4c0883a69102937d6231471b5dbb6204fe51296170827944f3a7f3f43347a8a5")
	if err != nil {
		t.Fatal(err)
	}
	sender := gethcrypto.PubkeyToAddress(evmKey.PublicKey)
	validatorIDs := []types.ValidatorID{"validator-1", "validator-2", "validator-3", "validator-4"}
	signers := make(map[types.ValidatorID]vexocrypto.Signer, len(validatorIDs))
	validators := make([]validator.Validator, 0, len(validatorIDs))
	governancePower := make(map[types.Address]types.VotingPower, len(validatorIDs))
	for _, validatorID := range validatorIDs {
		signer, err := vexocrypto.NewDeterministicSigner([]byte("rpc-four-validator-" + string(validatorID)))
		if err != nil {
			t.Fatal(err)
		}
		signers[validatorID] = signer
		validators = append(validators, validator.Validator{
			ID: validatorID, Address: types.Address(validatorID), VotingPower: 1, Stake: 1, PublicKey: signer.PublicKey(),
		})
		governancePower[types.Address(validatorID)] = 1
	}
	genesis := node.Genesis{
		ChainID:    "vexo-test",
		Validators: validators,
		AppState: map[string][]byte{
			"bank:" + sender.Hex(): []byte("100000000000000000000"),
		},
		Governance: governancePower,
	}
	bus := transport.NewInMemoryBus()
	defer bus.Close()
	runningNodes := make([]*node.Node, 0, len(validatorIDs))
	var eventMu sync.Mutex
	events := make([]string, 0)
	for _, validatorID := range validatorIDs {
		validatorID := validatorID
		cfg := node.UnsafeTestConfig("vexo-test", t.TempDir())
		cfg.ValidatorID = validatorID
		cfg.Chain.Application.Modules = append(cfg.Chain.Application.Modules, "evm")
		cfg.Chain.Execution.EVMChainID = chainID
		cfg.Chain.Execution.EVMForkPreset = "latest"
		application, err := modules.NewRuntimeWithChainConfig(cfg.Chain.ChainID, cfg.Chain)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := bus.NewPeer(p2p.PeerID(validatorID))
		if err != nil {
			t.Fatal(err)
		}
		runningNode, err := node.New(cfg, genesis, application)
		if err != nil {
			t.Fatal(err)
		}
		runningNode.WithSigner(signers[validatorID]).WithTransport(wire).WithEventLogger(func(event string, fields map[string]any) {
			if event == "vote_rejected" && fields["error"] == "stale vote" {
				return
			}
			encoded, _ := json.Marshal(fields)
			eventMu.Lock()
			events = append(events, string(validatorID)+":"+event+":"+string(encoded))
			eventMu.Unlock()
		})
		if err := runningNode.Start(ctx); err != nil {
			t.Fatal(err)
		}
		runningNodes = append(runningNodes, runningNode)
	}
	defer func() {
		for index := len(runningNodes) - 1; index >= 0; index-- {
			_ = runningNodes[index].Stop(context.Background())
		}
	}()
	loopConfig := node.ConsensusLoopConfig{
		Interval:            time.Millisecond,
		RoundTimeout:        250 * time.Millisecond,
		MaxBlockBytes:       4 * 1024 * 1024,
		CreateEmptyBlocks:   false,
		ExecutionCommitMode: node.ExecutionCommitModeFinalized,
	}
	for _, runningNode := range runningNodes {
		if err := runningNode.StartConsensusLoop(ctx, loopConfig); err != nil {
			t.Fatal(err)
		}
	}

	handler := NewHandler(runningNodes[0])
	if got := web3NodeCall[string](t, handler, "eth_chainId"); got != "0x147f8" {
		t.Fatalf("unexpected chain id %q", got)
	}
	submit := func(nonce uint64, to *gethcommon.Address, data []byte, gas uint64) map[string]any {
		t.Helper()
		tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
			ChainID: new(big.Int).SetUint64(chainID), Nonce: nonce, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(20), Gas: gas,
			To: to, Value: big.NewInt(0), Data: append([]byte(nil), data...),
		})
		signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), evmKey)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := signed.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		txHash := web3NodeCall[string](t, handler, "eth_sendRawTransaction", "0x"+hex.EncodeToString(raw))
		var receipt map[string]any
		for attempt := 0; attempt < 300000; attempt++ {
			receipt = web3NodeCall[map[string]any](t, handler, "eth_getTransactionReceipt", txHash)
			if receipt != nil {
				break
			}
			goruntime.Gosched()
			if attempt%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		if receipt == nil || receipt["status"] != "0x1" {
			eventMu.Lock()
			capturedEvents := append([]string(nil), events...)
			eventMu.Unlock()
			diagnostics := make([]map[string]any, 0, len(runningNodes))
			for index, runningNode := range runningNodes {
				machine, machineErr := runningNode.Consensus()
				entry := map[string]any{"validator": validatorIDs[index], "node": runningNode.Status(ctx), "machine_error": machineErr}
				pending, pendingErr := runningNode.PendingTxSnapshot(ctx)
				entry["pending_tx_count"] = len(pending)
				entry["pending_tx_error"] = pendingErr
				pendingSizes := make([]int, 0, len(pending))
				for _, tx := range pending {
					pendingSizes = append(pendingSizes, len(tx))
				}
				entry["pending_tx_sizes"] = pendingSizes
				if machineErr == nil {
					entry["consensus"] = machine.Status(ctx)
					entry["high_qc"] = machine.HighQC(ctx)
					entry["decisions"] = machine.CommitDecisions()
				}
				diagnostics = append(diagnostics, entry)
			}
			t.Fatalf("transaction %s did not execute successfully: receipt=%+v diagnostics=%+v events=%v", txHash, receipt, diagnostics, capturedEvents)
		}
		heightHex, ok := receipt["blockNumber"].(string)
		if !ok {
			t.Fatalf("receipt has no block number: %+v", receipt)
		}
		height, err := strconv.ParseUint(strings.TrimPrefix(heightHex, "0x"), 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 300000; attempt++ {
			allStored := true
			var blockHash types.Hash
			var appHash types.Hash
			for index, runningNode := range runningNodes {
				record, err := runningNode.BlockByHeight(ctx, types.Height(height))
				if err != nil {
					allStored = false
					break
				}
				if index == 0 {
					blockHash = record.Hash
					appHash = record.AppHash
					continue
				}
				if record.Hash != blockHash || record.AppHash != appHash {
					t.Fatalf("validator state diverged at height %d: validator=%d block=%x app=%x expected_block=%x expected_app=%x", height, index+1, record.Hash, record.AppHash, blockHash, appHash)
				}
			}
			if allStored {
				return receipt
			}
			goruntime.Gosched()
			if attempt%100 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		t.Fatalf("not every validator stored transaction block at height %d", height)
		return nil
	}

	tpsReceipt := submit(0, nil, web3TestCreationCode(t, "testdata/tps_check_creation.hex"), 3_000_000)
	if code := web3NodeCall[string](t, handler, "eth_getCode", web3ContractAddress(t, tpsReceipt), "latest"); code == "0x" {
		t.Fatal("four-validator TpsCheck deployment produced no runtime code")
	}
	v1Receipt := submit(1, nil, buildWeb3UUPSImplementationCreation(t, 1), 3_000_000)
	v1Address := web3ContractAddress(t, v1Receipt)
	v2Receipt := submit(2, nil, buildWeb3UUPSImplementationCreation(t, 2), 3_000_000)
	v2Address := web3ContractAddress(t, v2Receipt)
	proxyReceipt := submit(3, nil, buildWeb3DelegateProxyCreation(t, v1Address), 3_000_000)
	proxyAddress := gethcommon.HexToAddress(web3ContractAddress(t, proxyReceipt))
	if got := web3NodeCall[string](t, handler, "eth_call", map[string]any{"from": sender.Hex(), "to": proxyAddress.Hex(), "data": "0x54fd4d50"}, "latest"); got != web3Uint256(1) {
		t.Fatalf("unexpected four-validator proxy version before upgrade %q", got)
	}
	submit(4, &proxyAddress, []byte{0x81, 0x29, 0xfc, 0x1c}, 500_000)
	upgradeInput := append([]byte{0x36, 0x59, 0xcf, 0xe6}, make([]byte, 12)...)
	upgradeInput = append(upgradeInput, gethcommon.HexToAddress(v2Address).Bytes()...)
	submit(5, &proxyAddress, upgradeInput, 500_000)
	if got := web3NodeCall[string](t, handler, "eth_call", map[string]any{"from": sender.Hex(), "to": proxyAddress.Hex(), "data": "0x54fd4d50"}, "latest"); got != web3Uint256(2) {
		t.Fatalf("unexpected four-validator proxy version after upgrade %q", got)
	}

	eventMu.Lock()
	capturedEvents := strings.Join(events, "\n")
	eventMu.Unlock()
	for _, forbidden := range []string{"invalid transaction nonce", "double sign detected", "block height is already committed", "block commit height is not sequential"} {
		if strings.Contains(capturedEvents, forbidden) {
			t.Fatalf("four-validator consensus emitted %q:\n%s", forbidden, capturedEvents)
		}
	}
}

func web3TestCreationCode(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(string(raw)), "0x"))
	if err != nil || len(code) == 0 {
		t.Fatalf("invalid creation bytecode fixture %q", path)
	}
	return code
}

func submitWeb3Transaction(t *testing.T, handler http.Handler, runningNode *node.Node, key *ecdsa.PrivateKey, chainID uint64, nonce uint64, to *gethcommon.Address, data []byte, gas uint64) map[string]any {
	t.Helper()
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   new(big.Int).SetUint64(chainID),
		Nonce:     nonce,
		GasTipCap: big.NewInt(1),
		GasFeeCap: big.NewInt(20),
		Gas:       gas,
		To:        to,
		Value:     big.NewInt(0),
		Data:      append([]byte(nil), data...),
	})
	signed, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(new(big.Int).SetUint64(chainID)), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	hash := web3NodeCall[string](t, handler, "eth_sendRawTransaction", "0x"+hex.EncodeToString(raw))
	receipt := commitWeb3Transaction(t, handler, runningNode, hash, func() []string { return nil })
	if receipt["status"] != "0x1" {
		t.Fatalf("transaction %s failed: %+v", hash, receipt)
	}
	return receipt
}

func web3ContractAddress(t *testing.T, receipt map[string]any) string {
	t.Helper()
	address, ok := receipt["contractAddress"].(string)
	if !ok || address == "" {
		t.Fatalf("missing contract address: %+v", receipt)
	}
	return address
}

func web3Uint256(value byte) string {
	return "0x" + strings.Repeat("0", 62) + hex.EncodeToString([]byte{value})
}

func buildWeb3UUPSImplementationCreation(t *testing.T, version byte) []byte {
	t.Helper()
	builder := newWeb3BytecodeBuilder()
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x54, 0xfd, 0x4d, 0x50, 0x14)
	builder.pushLabel("version")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x81, 0x29, 0xfc, 0x1c, 0x14)
	builder.pushLabel("initialize")
	builder.op(0x57)
	builder.op(0x60, 0x00, 0x35, 0x60, 0xe0, 0x1c)
	builder.op(0x63, 0x36, 0x59, 0xcf, 0xe6, 0x14)
	builder.pushLabel("upgrade")
	builder.op(0x57, 0x60, 0x00, 0x60, 0x00, 0xfd)
	builder.label("version")
	builder.op(0x60, version, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3)
	builder.label("initialize")
	builder.op(0x33, 0x60, 0x01, 0x55, 0x00)
	builder.label("upgrade")
	builder.op(0x33, 0x60, 0x01, 0x54, 0x14)
	builder.pushLabel("upgrade_ok")
	builder.op(0x57, 0x60, 0x00, 0x60, 0x00, 0xfd)
	builder.label("upgrade_ok")
	builder.op(0x60, 0x04, 0x35, 0x73)
	builder.op(make([]byte, 20)...)
	for index := len(builder.code) - 20; index < len(builder.code); index++ {
		builder.code[index] = 0xff
	}
	builder.op(0x16, 0x60, 0x00, 0x55, 0x00)
	runtime := builder.bytes(t)
	return web3CreationCode(t, nil, runtime)
}

func buildWeb3DelegateProxyCreation(t *testing.T, implementation string) []byte {
	t.Helper()
	implementationBytes, err := hex.DecodeString(strings.TrimPrefix(implementation, "0x"))
	if err != nil || len(implementationBytes) != 20 {
		t.Fatalf("invalid implementation address %q", implementation)
	}
	builder := newWeb3BytecodeBuilder()
	builder.op(0x36, 0x60, 0x00, 0x60, 0x00, 0x37)
	builder.op(0x60, 0x00, 0x60, 0x00, 0x36, 0x60, 0x00, 0x60, 0x00, 0x54, 0x5a, 0xf4)
	builder.op(0x3d, 0x60, 0x00, 0x60, 0x00, 0x3e)
	builder.pushLabel("success")
	builder.op(0x57, 0x3d, 0x60, 0x00, 0xfd)
	builder.label("success")
	builder.op(0x3d, 0x60, 0x00, 0xf3)
	prefix := append([]byte{0x73}, implementationBytes...)
	prefix = append(prefix, 0x60, 0x00, 0x55)
	return web3CreationCode(t, prefix, builder.bytes(t))
}

func web3CreationCode(t *testing.T, prefix []byte, runtime []byte) []byte {
	t.Helper()
	offset := len(prefix) + 12
	if len(runtime) > 0xff || offset > 0xff {
		t.Fatal("test bytecode exceeds PUSH1 creation wrapper")
	}
	code := append([]byte(nil), prefix...)
	code = append(code, 0x60, byte(len(runtime)), 0x60, byte(offset), 0x60, 0x00, 0x39, 0x60, byte(len(runtime)), 0x60, 0x00, 0xf3)
	return append(code, runtime...)
}

type web3BytecodeBuilder struct {
	code    []byte
	labels  map[string]int
	patches []web3BytecodePatch
}

type web3BytecodePatch struct {
	position int
	label    string
}

func newWeb3BytecodeBuilder() *web3BytecodeBuilder {
	return &web3BytecodeBuilder{labels: make(map[string]int)}
}

func (builder *web3BytecodeBuilder) op(values ...byte) {
	builder.code = append(builder.code, values...)
}

func (builder *web3BytecodeBuilder) label(name string) {
	builder.labels[name] = len(builder.code)
	builder.op(0x5b)
}

func (builder *web3BytecodeBuilder) pushLabel(name string) {
	builder.op(0x61, 0x00, 0x00)
	builder.patches = append(builder.patches, web3BytecodePatch{position: len(builder.code) - 2, label: name})
}

func (builder *web3BytecodeBuilder) bytes(t *testing.T) []byte {
	t.Helper()
	result := append([]byte(nil), builder.code...)
	for _, patch := range builder.patches {
		offset, ok := builder.labels[patch.label]
		if !ok || offset > 0xffff {
			t.Fatalf("invalid bytecode label %q", patch.label)
		}
		result[patch.position] = byte(offset >> 8)
		result[patch.position+1] = byte(offset)
	}
	return result
}

func commitWeb3Transaction(t *testing.T, handler http.Handler, runningNode *node.Node, txHash string, eventSnapshot func() []string) map[string]any {
	t.Helper()
	var lastResult node.ConsensusStepResult
	loopConfig := node.ConsensusLoopConfig{
		Interval:            time.Millisecond,
		RoundTimeout:        time.Hour,
		MaxBlockBytes:       4 * 1024 * 1024,
		CreateEmptyBlocks:   false,
		ExecutionCommitMode: node.ExecutionCommitModeFinalized,
	}
	for step := 0; step < 100; step++ {
		if receipt := web3NodeCall[map[string]any](t, handler, "eth_getTransactionReceipt", txHash); receipt != nil {
			return receipt
		}
		result, err := runningNode.StepConsensusWithConfig(context.Background(), loopConfig)
		if err != nil {
			t.Fatalf("consensus step %d: %v", step, err)
		}
		lastResult = result
		time.Sleep(time.Millisecond)
	}
	machine, err := runningNode.Consensus()
	if err != nil {
		t.Fatal(err)
	}
	pending, pendingErr := runningNode.PendingTxs(context.Background())
	t.Fatalf("transaction %s was not committed: app=%+v machine=%+v high_qc=%+v decisions=%+v pending=%d pending_err=%v last_step=%+v events=%v", txHash, runningNode.Status(context.Background()), machine.Status(context.Background()), machine.HighQC(context.Background()), machine.CommitDecisions(), len(pending), pendingErr, lastResult, eventSnapshot())
	return nil
}

func web3NodeCall[T any](t *testing.T, handler http.Handler, method string, params ...any) T {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/web3", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s returned HTTP %d: %s", method, response.Code, response.Body.String())
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *JSONRPCError   `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error != nil {
		t.Fatalf("%s failed: code=%d message=%s", method, envelope.Error.Code, envelope.Error.Message)
	}
	var result T
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode %s result %s: %v", method, envelope.Result, err)
	}
	return result
}

func assertWeb3MinedReceiptLogs(t *testing.T, receipt map[string]any) {
	t.Helper()
	logs, ok := receipt["logs"].([]any)
	if !ok || len(logs) == 0 {
		t.Fatalf("expected mined receipt logs: %+v", receipt)
	}
	for _, value := range logs {
		log, ok := value.(map[string]any)
		if !ok ||
			log["blockHash"] == nil ||
			log["blockNumber"] == nil ||
			log["transactionHash"] == nil ||
			log["transactionIndex"] == nil ||
			log["logIndex"] == nil ||
			log["removed"] != false {
			t.Fatalf("receipt log is not Ethereum-compatible: %+v", value)
		}
	}
}
