# App Module Guide

## Goal

This guide explains how to add an application module to Vexo.

## Module Interface

Implement `app.Module`:

```go
type Module interface {
    Name() string
    InitGenesis(ctx app.Context, genesis app.GenesisState) error
    BeginBlock(ctx app.Context, header types.Header) error
    DeliverTx(ctx app.Context, tx types.Tx) types.Result
    EndBlock(ctx app.Context) error
}
```

Optional interfaces:

- `app.QueryHandler` for module queries
- `app.ValidatorUpdateProvider` for validator set updates
- `app.TxEventEmitter` for deterministic transaction event emission
- `app.PruneHook` for module-owned indexes, caches, and historical snapshots that must follow node retention pruning
- module CLI command provider through the app CLI command tree

## Transaction Routing

The default routing model is prefix-based:

```text
<module>:<action>:<args...>:fee=<fee>:gas=<gas>:signer=<signer>:nonce=<nonce>
```

A module named `bank` receives payloads beginning with `bank:`.

## Module Configuration

Enabled modules are configured in the node home's `module_config.json`, not in `config.json`:

```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  }
}
```

`config.json` may point to a custom module file through `module_config_path`. Keep module defaults, module enablement, execution policy, and governance policy in `module_config.json` so application developers can change module behavior without touching `network_config.json`, `consensus_config.json`, `mempool_config.json`, or `log_config.json`.

## State

Modules receive `app.Context.Store`, a namespaced KV store. Use the module name as namespace unless a module has a stronger reason not to.

Use `ctx.GoContext()` for every store, crypto signer, remote signer, query, and long-running operation. The runtime now exposes context-aware `CheckTx`, `PrepareProposal`, `ProcessProposal`, `FinalizeBlock`, and `Query` paths, so cancellation and block/RPC deadlines can propagate into module code instead of continuing in the background.

For chain-wide module parameters, prefer the `params` keeper instead of ad-hoc module keys:

```go
keeper := params.NewKeeper(ctx.Store)
_, err := keeper.Set(ctx.GoContext(), params.Change{
    Authority: "governance",
    Module: "staking",
    Key: "max_validators",
    Value: []byte("100"),
})
```

The built-in `params` module supports `params:set:<authority>:<module>:<key>:<base64-value>` transactions and `params/param/<module>/<key>` queries.

## Events and Query Proofs

Modules can emit indexable events by implementing `app.TxEventEmitter`. The runtime calls it after a successful module execution result is produced, copies the emitted events, and persists indexed attributes through `events.Indexer` when the runtime is backed by a KV store.

```go
func (module Module) Events(ctx app.Context, tx types.Tx, result types.Result) []events.Event {
    if result.Code != 0 {
        return nil
    }
    return []events.Event{{
        Type: "transfer",
        Attributes: []events.Attribute{
            {Key: "sender", Value: "alice", Index: true},
            {Key: "recipient", Value: "bob", Index: true},
        },
    }}
}
```

Keep events deterministic: the same block and transaction must emit the same event type, attributes, and index flags on every node.

For state-root-bound queries, use `queryproof.Build` and `queryproof.Verify` to wrap a namespace/key/value lookup with chain ID, height, Merkle state root, deterministic leaf hash, and either a compact membership path or compact left/right neighbor absence proof. This is Vexo's native state proof format, not Cosmos IAVL.

The CLI exposes the same Merkle query-proof envelope:

```bash
vexod proof query --home .vexo --namespace bank --key alice > proof.json
vexod proof verify --input proof.json --chain-id vexo-chain --height 10
```

Data availability commitments use canonical transaction chunks. Operators and module test harnesses can export a DA bundle, verify individual chunk proofs, plan deterministic chunk samples, and test bounded Reed-Solomon-style recovery:

```bash
vexod proof da-export --tx-hex 68656c6c6f --tx-hex 776f726c64 --data-shards 4 --parity-shards 2 > da-bundle.json
vexod proof da-proof --tx-hex 68656c6c6f --tx-hex 776f726c64 --index 0 > da-proof.json
vexod proof da-verify --input da-proof.json
vexod proof da-sample --input da-bundle.json --chain-id vexo-chain --height 10 --samples 8 --min-samples 4 > da-samples.json
vexod proof da-recover --input da-bundle.json --drop 0 --drop 1
```

## IBC and Contract Extension Points

The `ibc` package provides client, connection, channel, packet commitment, acknowledgement, and receipt primitives for building an IBC-compatible module. Full relayer/light-client protocol compatibility is chain integration work.

Packet scaffolds can be generated from the CLI while chain-specific IBC modules wire packet commitments into state:

```bash
vexod ibc tx client-update 07-vexo-0 11 <validator-set-hash> <state-root> --fee 1 --gas 1000 --signer relayer --nonce 1
vexod relayer client-update --source-rpc 127.0.0.1:26657 --rpc 127.0.0.1:27657 --client-id 07-vexo-0 --fee 1 --gas 1000 --signer relayer --nonce 1 --submit
vexod ibc tx connection-open-init connection-0 07-vexo-0 connection-1 --fee 1 --gas 1000 --signer relayer --nonce 1
vexod ibc tx connection-open-ack connection-0 --fee 1 --gas 1000 --signer relayer --nonce 2
vexod ibc tx channel-open-init transfer channel-0 connection-0 channel-1 ordered --fee 1 --gas 1000 --signer relayer --nonce 3
vexod ibc tx channel-open-ack transfer channel-0 --fee 1 --gas 1000 --signer relayer --nonce 4
vexod ibc tx packet-send 1 transfer channel-0 transfer channel-1 payload --fee 1 --gas 1000 --signer relayer --nonce 1
vexod ibc tx packet-ack 1 transfer channel-0 transfer channel-1 payload ack --fee 1 --gas 1000 --signer relayer --nonce 2
vexod ibc tx packet-timeout 1 transfer channel-0 transfer channel-1 payload 100 --fee 1 --gas 1000 --signer relayer --nonce 3
vexod proof verify-ibc --home .vexo --client-id 07-vexo-0 --input ibc-proof.json
vexod relayer discover --rpc 127.0.0.1:26657 --json
vexod relayer packet-ack --rpc 127.0.0.1:26657 --proof-rpc 127.0.0.1:26657 --sequence 1 --source-port transfer --source-channel channel-0 --destination-port transfer --destination-channel channel-1 --data payload --ack ack --fee 1 --gas 1000 --signer relayer --nonce 2 --submit
vexod relayer loop --mode timeout --rpc 127.0.0.1:26657 --proof-rpc 127.0.0.1:26657 --sequence 1 --source-port transfer --source-channel channel-0 --destination-port transfer --destination-channel channel-1 --data payload --timeout-height 100 --interval 5s --continue-on-error --state relayer_state.json --submit
vexod relayer run --config relayer_config.json
vexod evm tx call evm 0xaaaa 0xbbbb transfer aabb 100000 --fee 1 --gas 100000 --signer 0xaaaa --nonce 1
vexod evm query storage 0xcontract 0x0
vexod evm query logs
vexod evm query logs 0xcontract
curl -s -X POST http://127.0.0.1:26657/ -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'
vexod ibc packet send \
  --sequence 1 \
  --source-port transfer \
  --source-channel channel-0 \
  --destination-port transfer \
  --destination-channel channel-1 \
  --data payload
```

Contract VM adapters return `contract.Result`. The default `evm` adapter is backed by go-ethereum's EVM interpreter and executes EVM bytecode for deploy/call paths. Geth-specific interpreter code is isolated under `modules/evm/backend/geth`, and signed Ethereum transaction decoding plus Ethereum transaction/receipt/state trie logic is isolated under `modules/evm/ethcompat`, so geth API changes should be contained to those compatibility packages and their conformance tests. `eth_sendRawTransaction` accepts signed Ethereum legacy/access-list/dynamic-fee typed transactions, verifies sender recovery and chain ID, preserves the Ethereum transaction hash, and maps calls or contract creations into Vexo canonical `evm` transactions. Ethereum contract creation uses the standard sender/nonce CREATE address unless an explicit salt is supplied, in which case the geth adapter uses CREATE2. EVM values and account balances preserve unsigned 256-bit Ethereum semantics across execution, receipts, account queries, snapshots, and `eth_getProof`; values larger than 256 bits fail closed. The EVM module persists runtime code, VM `CodeWrites`, `StorageWrites` to `evm/storage/{address}/{slot}`, `NonceWrites` into the canonical account sequence namespace, `AccountDeletions` from `SELFDESTRUCT`/empty-account finalisation, VM balance writes into the shared `bank` namespace, receipts by transaction hash, receipt-location indexes by transaction hash, logs by height/transaction/log index plus address, and height-indexed Ethereum account snapshots under the auxiliary `evm_ethstate/{height}` namespace. Receipts include receipt-backed `state_diff` generated from actual VM writes, and the built-in geth adapter stores struct-logger opcode traces under `vm_trace`; Web3 replay methods expose those values as `stateDiff` and `vmTrace`. Account deletions are applied after code/storage/balance/nonce writes so deleted accounts do not leave stale sequence state behind. Ethereum hex account keys are normalized before bank reads/writes, so checksum and lowercase forms of the same 20-byte address resolve to one account. The module reconstructs go-ethereum-compatible account/storage MPTs from committed bank/auth/EVM state for latest `eth_getProof` and from retained snapshots for historical `eth_getProof`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getCode`, `eth_getStorageAt`, and Web3 `stateRoot`. Read-only call queries pass block height, base fee, and effective gas price context into the VM invocation. Txpool RPC splits contiguous executable sender nonces into `pending` and future nonce gaps into `queued`. This keeps `eth_getBalance`, `eth_getProof`, `eth_getCode`, `eth_getStorageAt`, `eth_call`, `eth_estimateGas`, `eth_createAccessList`, `eth_getTransactionReceipt`, `eth_getBlockReceipts`, `eth_getTransactionByHash`, address-scoped `eth_getLogs`, and global `eth_getLogs` backed by committed module state instead of process memory.

Minimal `relayer_config.json`:

```json
{
  "schema_version": "v1",
  "jobs": [
    {
      "name": "timeout-transfer",
      "mode": "timeout",
      "rpc": "127.0.0.1:26657",
      "proof_rpc": "127.0.0.1:26657",
      "submit": true,
      "state_path": "relayer_state.json",
      "interval": "5s",
      "continue_on_error": true,
      "packet": {
        "sequence": 1,
        "source_port": "transfer",
        "source_channel": "channel-0",
        "destination_port": "transfer",
        "destination_channel": "channel-1",
        "data": "payload",
        "timeout_height": 100
      }
    }
  ]
}
```

Relayer-facing reads are available through RPC:

```bash
curl 'http://127.0.0.1:26657/v1/ibc/client/07-vexo-0'
curl 'http://127.0.0.1:26657/v1/ibc/packet/1/transfer/channel-0/transfer/channel-1'
curl 'http://127.0.0.1:26657/v1/ibc/proof/packet/1/transfer/channel-0/transfer/channel-1'
```

IBC clients can be updated with the counterparty latest height, validator-set hash, and state root. Relayers can fetch those fields from the counterparty `/v1/state/latest` endpoint with `relayer client-update --source-rpc`, then submit the generated update to the destination chain. Connections and channels support init/try/ack/confirm handshake states before packet flow. Packet receipts carry acknowledgement and timeout lifecycle fields, so relayers can submit packet-send, observe receipt state, discover packet-send events from the RPC event index, submit packet-ack or packet-timeout, fetch an IBC namespace proof for the packet commitment key at a specific height, verify that proof against the trusted local IBC client with namespace/key/value checks, optionally submit the built relayer transaction through RPC, run a bounded or continuous polling loop, persist relay checkpoints to avoid duplicate submissions after restart, and manage multiple relay jobs from a JSON config file. Ack-with-proof requires a counterparty receipt proof whose acknowledgement bytes match the submitted ack; timeout-with-proof accepts an absence proof or an unacknowledged receipt proof and rejects acknowledged receipt proofs.

The `contract` package provides a VM registry and invocation boundary for EVM/WASM-compatible modules. The `evm` module stores contract code, execution receipts, storage slots, logs, VM code writes, VM nonce writes, VM account deletions, and VM balance writes, and the RPC server exposes Web3 JSON-RPC bridge methods such as `rpc_modules`, `web3_clientVersion`, `web3_sha3`, `net_version`, `net_listening`, `net_peerCount`, `eth_chainId`, `eth_protocolVersion`, `eth_syncing`, `eth_mining`, `eth_hashrate`, `eth_accounts`, `eth_coinbase`, `eth_blockNumber`, `eth_getBlockByNumber`, `eth_getBlockByHash`, `eth_getBlockTransactionCountByNumber`, `eth_getBlockTransactionCountByHash`, `eth_getTransactionByBlockNumberAndIndex`, `eth_getTransactionByBlockHashAndIndex`, `eth_getUncleCountByBlockNumber`, `eth_getUncleCountByBlockHash`, `eth_getUncleByBlockNumberAndIndex`, `eth_getUncleByBlockHashAndIndex`, `eth_gasPrice`, `eth_blobBaseFee`, `eth_maxPriorityFeePerGas`, `eth_feeHistory`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getProof`, `eth_getCode`, `eth_getStorageAt`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getBlockReceipts`, `eth_getTransactionByHash`, `eth_getRawTransactionByHash`, `eth_getRawTransactionByBlockNumberAndIndex`, `eth_getRawTransactionByBlockHashAndIndex`, `eth_pendingTransactions`, `eth_getLogs`, `eth_newFilter`, `eth_getFilterChanges`, `eth_getFilterLogs`, `eth_uninstallFilter`, `eth_call`, `eth_estimateGas`, `eth_createAccessList`, `txpool_status`, `txpool_content`, `txpool_contentFrom`, `txpool_inspect`, `debug_traceTransaction`, `debug_traceCall`, `debug_traceBlockByNumber`, `debug_traceBlockByHash`, `trace_call`, `trace_transaction`, `trace_get`, `trace_block`, `trace_filter`, `trace_replayTransaction`, and `trace_replayBlockTransactions`. Chains can use the built-in geth-backed adapter named `evm`, update geth by validating `modules/evm/backend/geth` plus `modules/evm/ethcompat`, or replace the execution side with a custom `contract.VM` adapter. Web3 lookup paths keep pending/queued transactions, retained block transactions, retained historical account/code/storage snapshots, receipts, deterministic receipt-backed traces, polling filters, and WebSocket subscriptions aligned so Ethereum tooling can discover transactions before and after commit even when a secondary receipt index must be rebuilt from committed block records.

The built-in staking module includes delegation, undelegation, unjail, validator commission, fee reward distribution, reward queries, reward claiming, and staking-ledger slashing. Fees collected by the ante layer into the configured fee collector are distributed at end block by validator power, then by delegator stake after validator commission. When consensus slashing applies a penalty receipt, the runtime asks staking to reduce delegations to that validator proportionally and writes an idempotency marker so restart reconciliation cannot slash the same evidence twice.

```bash
vexod staking tx set-commission validator-1 500 --signer validator-1
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
vexod staking tx claim-rewards alice validator-1 --fee 1 --gas 1000 --signer alice --nonce 2
```

## Genesis

`InitGenesis` receives module-specific genesis values in `app.GenesisState`. Existing bank genesis keys use `bank:<address>`.

## Ante Handling

Modules should not reimplement fee units, base fee, nonce, gas limit parsing, or signature checks. Those belong to the ante layer.
Modules that execute transactions should implement `EstimateGas(ctx, tx)` and call `ctx.ConsumeGas(amount)` inside `DeliverTx` so under-gas transactions fail before proposal acceptance and during execution.

## CLI Commands

A module should expose structured CLI commands with:

- command name
- usage
- description
- arguments
- flags
- examples
- child commands

CLI should generate transaction payloads, not execute local state changes directly.

## Tests

Every module should test:

- genesis initialization
- valid and invalid transactions
- query responses
- ante compatibility
- deterministic state roots
- validator updates, if any
