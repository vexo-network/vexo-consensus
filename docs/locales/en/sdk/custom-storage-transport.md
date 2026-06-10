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

## Storage Requirements

A production storage backend must guarantee:

- atomic block/state persistence or clear recovery semantics
- `BatchKVStore` for modules that write multiple keys per transaction, especially staking custody, EVM execution, and IBC packet send
- atomic upgrade-plan persistence for governance proposals that schedule binary/config/store/app migrations
- crash-safe latest state pointer
- durable evidence records
- deterministic state roots
- schema migration path
- rollback-safe upgrade behavior

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

## Compatibility

Different binaries can peer when they implement compatible:

- chain ID
- transport protocol
- topic names
- message encoding
- handshake/auth policy
- consensus wire schema
