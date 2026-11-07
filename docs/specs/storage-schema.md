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
- optional quorum certificate that certified the block
- module state roots

Committed blocks written through the node commit path include the block QC in the same atomic block/state/app-write batch. Finality proof serving can use the certified child block plus grandchild header to rebuild strict two-link commit-chain proofs after restart. Blocks inserted by offline tooling may omit the QC, but those records cannot be used to satisfy strict light-client finality proof requests until enough certified descendants are available.

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
- Merkle state root for sorted namespace KV pairs

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

Common framework namespaces:

- `auth`: account sequences, advanced by the application runtime when a transaction is admitted into a block
- `bank`: unsigned 256-bit account balances used by native bank transfers, fees, staking reward/withdraw paths, and EVM value transfers
- `events`: indexed transaction event records and attribute indexes
- `feemarket`: the fee market state (`state`: base fee and blob base fee, 8 bytes each, big endian) the application runtime stores at the end of every block; transactions of block N are priced against the value block N-1 stored
- `evm`: contract VM code and storage slots
- `evm_receipts`: receipts, receipt-location index, global log index, address log index, and blob sidecars; written with each block but outside the app hash, so pruning it below the retained height is local to the node
- `ibc`: client, connection, channel, packet commitment, and receipt records
- `params`: chain-wide module parameter values and metadata
- `staking`: delegated stake, validator power, validator public keys, commission basis points, entry-based unbonding release records, unbonding custody balances, jail flags, and pending reward balances

When staged execution is available, module KV writes, block records, state records, and state roots are committed in one backend batch. If that batch fails, module KV writes are not applied.

IBC packet send writes both the packet commitment and the next sequence. Stores that do not implement `BatchKVStore` must reject packet send instead of partially writing one key. Governance upgrade execution has the same fail-closed rule: if a proposal produces an upgrade plan, the governance state transition and upgrade plan records must be written through an atomic upgrade-plan store.

For Ethereum-compatible `0x` account addresses, the built-in bank, staking, ante, and EVM paths normalize the key to a lowercase 20-byte hex address before reading or writing balance state. Writes always use the normalized key to avoid checksum/lowercase balance splits, and the bank module and the ante handler read only the normalized key, so the account a transaction signer is authorized for is the account whose balance moves.

The Web3 bridge reconstructs Ethereum account/storage tries from these committed namespaces:

- `bank/{0x_address}` for account balances
- `auth/nonce/{0x_address}` for account nonces
- `evm/code/{0x_address}` for account code hashes
- `evm/storage/{0x_address}/{slot}` for account storage roots and storage proofs

Latest `eth_getProof` and latest Web3 `stateRoot` are generated from the live reconstructed go-ethereum MPT. Historical `eth_getProof`, historical calls, historical account/code/storage reads and historical Web3 block `stateRoot` read the same keys as of the requested height from the store's state history; the EVM module keeps no per-block copy of the Ethereum state. A height whose state record is not held (never committed, pruned, or below a restored snapshot) fails explicitly. Computed historical state roots are cached in memory per height.

LevelDB keeps state history as per-key versions. Every change after the first committed state is recorded at the height whose state it belongs to: a block's writes at its height, a direct write (`Set`, `Delete`, `SetBatch`) at the next committed height. A key's first change also records its prior value at height 0, so keys written at genesis keep their genesis value at heights before their first change, and a key with no version reads as its current value. `ImportNamespace` installs a namespace's base state and drops its versions. Storage grows with the number of changes, never with the state size per block. `PruneBelow` drops the versions no retained height reads: superseded versions below the retained height, deletions, and versions of keys that did not change since. Historical reads at a height without a committed state record fail with `ErrStateNotFound`. Historical query proofs rebuild the namespace at the requested height from those versions, then verify membership with a compact Merkle path or non-membership with compact adjacent-neighbor absence proofs. Verifiers still accept legacy full namespace absence witnesses for compatibility. Evidence records are monotonic for the `Applied` flag: once an evidence record is marked applied, later pending saves in the same process cannot downgrade it back to unapplied.

When an app block produces validator updates, the store-backed validator registry stages the height `H + 1` validator-set snapshot and the matching rotation-event log as KV writes. Those writes commit in the same LevelDB batch as app writes, block metadata, state metadata, and state roots. If the block commit fails, neither the future validator-set snapshot nor its rotation-event audit record is persisted. If the process crashes after the backend batch succeeds but before in-memory post-commit hooks run, reopening the store-backed registry reloads the persisted rotation-event log.

Runtime compaction includes both backend store compaction and mempool WAL compaction. WAL compaction rewrites pending transactions after committed transactions are removed, preventing long-running nodes from retaining stale append-only mempool records indefinitely. During startup, a partially written final WAL record caused by process or host crash is truncated and ignored, while corrupt records in the middle of the WAL fail closed because silently skipping them could change transaction ordering. The in-memory mempool seen-cache also prunes expired entries on admission and commit paths when `seen_ttl` is enabled.

## Indexes

- block height index
- block hash index
- latest state pointer
- state root index
- height-versioned KV history index
- evidence index
- event attribute index
- EVM global log index by height/transaction/log index
- EVM address log index by address/height/transaction/log index

## EVM Records

The EVM module requires atomic batch writes for state-changing execution, receipt indexes, blob sidecar indexes, and VM write application. A custom storage backend that only implements single-key `Set`/`Delete` must reject EVM execution rather than partially writing contract state or native-account balances.

In the `evm` namespace:

- `code/{address}`: deployed contract bytecode.
- `storage/{address}/{slot}`: VM-returned storage slot value.

In the `evm_receipts` namespace:

- `receipts/{tx_hash}`: committed transaction receipt.
- `receipt_index/{tx_hash}`: block height and position of the transaction.
- `logs/by_height/{height}/{tx_hash}/{log_index}`: global log index.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}`: address-scoped log index.
- `blob_sidecars/by_tx/{tx_hash}` and `blob_sidecars/by_blob/{blob_hash}`: blob sidecars and their blob-hash index.

EVM account balances are persisted as unsigned 256-bit big-endian values. Execution failures that occur inside the VM are persisted as Ethereum-style receipts with `status = 0`, `error`, consumed gas, return data, and trace data when available; malformed invocations still fail before receipt persistence.

Legacy array-style `logs` and `logs/{address}` values remain query-compatible, but new writes use prefix indexes for bounded incremental scans.

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
