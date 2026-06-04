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
    "Modules": ["bank", "staking", "governance", "params"]
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

Modules can emit indexable events using the `events.Event` shape. Node integrations may persist those through `events.Indexer` so operators and explorers can query by indexed attributes.

For state-root-bound queries, use `queryproof.Build` and `queryproof.Verify` to wrap a namespace/key/value lookup with chain ID, height, state root, and deterministic leaf hash. This is a query-proof envelope, not a full Cosmos IAVL proof.

## IBC and Contract Extension Points

The `ibc` package provides client, connection, channel, packet commitment, acknowledgement, and receipt primitives for building an IBC-compatible module. Full relayer/light-client protocol compatibility is chain integration work.

The `contract` package provides a VM registry and invocation boundary for future EVM/WASM-compatible modules. VM implementations plug in behind `contract.VM` and must enforce their own gas/account/state semantics.

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
