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
- close

Optional snapshot support implements `store.SnapshotKVStore`:

- export namespace
- import namespace

## Storage Requirements

A production storage backend must guarantee:

- atomic block/state persistence or clear recovery semantics
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

## Compatibility

Different binaries can peer when they implement compatible:

- chain ID
- transport protocol
- topic names
- message encoding
- handshake/auth policy
- consensus wire schema
