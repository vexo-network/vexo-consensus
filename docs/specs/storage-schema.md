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

- `bank`: account balances used by native bank transfers, fees, staking, and EVM value transfers
- `events`: indexed transaction event records and attribute indexes
- `evm`: contract VM code, storage slots, receipts, global log index, and address log index
- `ibc`: client, connection, channel, packet commitment, and receipt records
- `params`: chain-wide module parameter values and metadata
- `staking`: delegated stake, validator power, validator public keys, commission basis points, entry-based unbonding release records, unbonding custody balances, jail flags, and pending reward balances

When staged execution is available, module KV writes, block records, state records, and state roots are committed in one backend batch. If that batch fails, module KV writes are not applied.

For Ethereum-compatible `0x` account addresses, the built-in bank, staking, ante, and EVM paths normalize the key to a lowercase 20-byte hex address before reading or writing balance state. Legacy raw keys are still read as fallback, but new writes use the normalized key to avoid checksum/lowercase balance splits.

The Web3 bridge reconstructs Ethereum account/storage tries from these committed namespaces:

- `bank/{0x_address}` for account balances
- `auth/nonce/{0x_address}` for account nonces
- `evm/code/{0x_address}` for account code hashes
- `evm/storage/{0x_address}/{slot}` for account storage roots and storage proofs
- `evm_ethstate/{height}/meta` for the retained Ethereum state root
- `evm_ethstate/{height}/accounts/{0x_address}` for retained account/storage proof inputs

Latest `eth_getProof` and latest Web3 `stateRoot` are generated from the live reconstructed go-ethereum MPT. Historical `eth_getProof` and historical Web3 block `stateRoot` use the retained auxiliary `evm_ethstate/{height}` snapshots written during `EndBlock`. Runtime pruning calls module pruning hooks, and the EVM module deletes Ethereum snapshots below the retained height so archival nodes keep proofs while pruned nodes fail old proof requests explicitly. The auxiliary namespace is not an application module root, so pruning retained Web3 proof snapshots does not mutate consensus application state.

LevelDB also writes height-versioned KV history records for each atomic block write. Historical query proofs rebuild the namespace at the requested height from those records, then verify membership with a compact Merkle path or non-membership with compact adjacent-neighbor absence proofs. Verifiers still accept legacy full namespace absence witnesses for compatibility. Evidence records are monotonic for the `Applied` flag: once an evidence record is marked applied, later pending saves in the same process cannot downgrade it back to unapplied.

When an app block produces validator updates, the store-backed validator registry stages the height `H + 1` validator-set snapshot as KV writes and commits those writes in the same LevelDB batch as app writes, block metadata, state metadata, and state roots. If the block commit fails, the future validator-set snapshot is not persisted.

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

The EVM module requires atomic batch writes for state-changing execution, receipt indexes, blob sidecar indexes, account snapshots, and VM write application. A custom storage backend that only implements single-key `Set`/`Delete` must reject EVM execution rather than partially writing contract state or native-account balances.

- `code/{address}`: deployed contract bytecode.
- `storage/{address}/{slot}`: VM-returned storage slot value.
- `receipts/{tx_hash}`: committed transaction receipt.
- `logs/by_height/{height}/{tx_hash}/{log_index}`: global log index.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}`: address-scoped log index.

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
