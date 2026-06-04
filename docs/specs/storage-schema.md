# Storage Schema

## Scope

This spec defines durable storage records and recovery expectations.

## Backend

The default backend is LevelDB. Storage is accessed through the `store.Store` interface so custom backends can be added.

## Records

### Block Record

Keyed by height and hash.

Fields:

- block header
- transactions
- block hash
- app hash
- module state roots

### State Record

Keyed by height and latest pointer.

Fields:

- height
- app hash
- last block hash
- validator set hash
- base fee used for the block
- next base fee derived from the block gas usage

### State Root Record

Keyed by `(height, namespace)`.

Fields:

- height
- module namespace
- state root

### Evidence Record

Keyed by stable evidence key.

Fields:

- evidence type
- validator
- height
- round
- proof
- applied flag
- created timestamp

### KV Namespace

Module data is stored by namespace and key.

When staged execution is available, module KV writes, block records, state records, and state roots are committed in one backend batch. If that batch fails, module KV writes are not applied.

Runtime compaction includes both backend store compaction and mempool WAL compaction. WAL compaction rewrites pending transactions after committed transactions are removed, preventing long-running nodes from retaining stale append-only mempool records indefinitely.

## Indexes

- block height index
- block hash index
- latest state pointer
- state root index
- evidence index

## Recovery Rules

- Last safe height is the latest height where block metadata and state record agree.
- A block without state is not considered safely committed after crash.
- State without block metadata is reported as inconsistent and recovery uses the lower safe height.
- Indexes can be rebuilt from canonical records.

## Snapshot Validation

- Snapshot documents include only active namespaces that have state roots or exported KV.
- Every declared namespace must have exactly one state root at the snapshot height.
- KV entries must belong to declared namespaces and must have non-empty keys.
- Snapshot checksum covers chain ID, state metadata, state roots, and sorted KV pairs.

## Schema Migration

Store migrations must be height-gated through an upgrade plan and support rollback on failure.
