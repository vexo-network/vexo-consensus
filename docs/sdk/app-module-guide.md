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

Contract VM adapters return `contract.Result`. The EVM module persists `StorageWrites` to `evm/storage/{address}/{slot}`, persists receipts by transaction hash, and indexes logs by height/transaction/log index plus address. This keeps `eth_getStorageAt`, `eth_getTransactionReceipt`, address-scoped `eth_getLogs`, and global `eth_getLogs` backed by committed module state instead of process memory.

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

IBC clients can be updated with the counterparty latest height, validator-set hash, and state root. Relayers can fetch those fields from the counterparty `/v1/state/latest` endpoint with `relayer client-update --source-rpc`, then submit the generated update to the destination chain. Connections and channels support init/try/ack/confirm handshake states before packet flow. Packet receipts carry acknowledgement and timeout lifecycle fields, so relayers can submit packet-send, observe receipt state, discover packet-send events from the RPC event index, submit packet-ack or packet-timeout, fetch an IBC namespace proof for the packet commitment key at a specific height, verify that proof against the trusted local IBC client with namespace/key/value checks, optionally submit the built relayer transaction through RPC, run a bounded or continuous polling loop, persist relay checkpoints to avoid duplicate submissions after restart, and manage multiple relay jobs from a JSON config file.

The `contract` package provides a VM registry and invocation boundary for EVM/WASM-compatible modules. The `evm` module stores contract code, execution receipts, storage slots, and logs, and the RPC server exposes Web3 JSON-RPC bridge methods such as `web3_clientVersion`, `net_version`, `eth_chainId`, `eth_blockNumber`, `eth_getBlockByNumber`, `eth_getBlockByHash`, `eth_gasPrice`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getCode`, `eth_getStorageAt`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getTransactionByHash`, `eth_getLogs`, `eth_newFilter`, `eth_getFilterChanges`, `eth_getFilterLogs`, `eth_uninstallFilter`, `eth_call`, and `eth_estimateGas`. A chain should register a vetted `contract.VM` adapter named `evm` to execute Ethereum-equivalent bytecode.

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
