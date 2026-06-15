# Custom Storage and Transport Guide

## Custom Storage

Implement `store.Store`:

- block save/query by height and hash
- block index
- state save/query by height/latest
- state root save/query
- evidence save/query/index
- pruning
- index recovery
- compaction
- historical namespace export by height through `store.HistoricalSnapshotKVStore`
- close

Snapshot support implements `store.SnapshotKVStore`:

- export namespace
- import namespace

Historical snapshot support is required for runtime construction. Custom stores that only implement latest-state reads are rejected because historical query proofs, replay, light-client proof serving, Web3 historical account state, and state-sync verification must fail at startup rather than later under load.

If you are deciding whether a storage backend is ready, ask one simple question: can it still recover the same chain after a crash, a pruning pass, a snapshot restore, and a historical proof request? If the answer is no, it is not ready for a Vexo network that keeps real history.

## Storage Requirements

A production storage backend must guarantee:

- atomic block/state persistence or clear recovery semantics
- `BatchKVStore` for modules that write multiple keys per transaction, especially staking custody, EVM execution, and IBC packet send
- `store.AppBlockCommitStore` when network safety is required, so app writes, block metadata, state metadata, and state roots commit as one unit
- atomic upgrade-plan persistence for governance proposals that schedule binary/config/store/app migrations
- crash-safe latest state pointer
- durable evidence records
- deterministic state roots
- schema migration path
- rollback-safe upgrade behavior

SDK embedders that want the same fail-closed startup behavior as `vexod start` should construct runtimes with `runtime.NewNetworkSafeWithStore`, `runtime.NewNetworkSafeWithStoreContext`, or `runtime.NewNetworkSafeWithStoreAndCryptoRegistryContext`. These constructors run `config.ValidateNetworkSafety`, reject missing durable stores, require `app.AtomicBlockApplication`, and require `store.AppBlockCommitStore` before the node can start.

## Custom Transport

Implement `transport.Transport`:

- start/stop lifecycle
- publish messages by topic
- subscribe to topics
- peer identity
- peer addressing

## Transport Requirements

A production transport should provide:

- handshake authentication
- protocol version negotiation
- chain ID binding
- max message size
- peer scoring hooks
- reconnect/backoff
- ban/disconnect support
- metrics for latency, failures, and invalid messages
- TLS or an equivalent authenticated encryption layer for public peer links
- config-file wiring for cert/key/CA/server-name material instead of long-lived command-line overrides
- `transport.GRPCConfig.RequireTLS` when the caller wants construction to fail instead of silently falling back to insecure gRPC credentials

In practice, a transport is not production-ready until a restart, a peer ban, and a reconnect all leave the peer graph in a state the node can explain from logs and metrics alone.

## Compatibility

Different binaries can peer when they implement compatible:

- chain ID
- transport protocol
- topic names
- message encoding
- handshake/auth policy
- consensus wire schema
