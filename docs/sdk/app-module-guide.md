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

The default production routing model is prefix-based:

```text
<module>:<action>:<args...>:fee=<fee>:gas=<gas>:signer=<signer>:nonce=<nonce>
```

A module named `bank` receives payloads beginning with `bank:`.

## State

Modules receive `app.Context.Store`, a namespaced KV store. Use the module name as namespace unless a module has a stronger reason not to.

## Genesis

`InitGenesis` receives module-specific genesis values in `app.GenesisState`. Existing bank genesis keys use `bank:<address>`.

## Ante Handling

Modules should not reimplement fee, nonce, gas, or signature checks. Those belong to the ante layer.

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
