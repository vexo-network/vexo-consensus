# Storage Schema

## Scope

This spec defines durable storage records and recovery expectations.

## Backend

The default backend is Pebble (`github.com/cockroachdb/pebble/v2`), opened with a 256 MiB block cache and 64 MiB memtables. Storage is accessed through the `store.Store` interface so custom backends can be added.

Records are stored in the canonical binary encoding of package `types`: minimal uvarints, length-prefixed byte strings and lists, hashes as their 32 bytes, an optional value behind a presence byte of `0` or `1`, and fields in the order of the struct that declares them. A decoder refuses anything else, trailing bytes included, so a damaged record is reported rather than read as another value. Each record has exactly one encoding.

A data directory written by an earlier build (LevelDB, JSON records) is not readable and is not converted: Pebble refuses to open it with `pebble: database "<dir>" was written in format major version 1, which is no longer supported`. Restore such a node from a snapshot, or sync it into an empty data directory.

## Records

### Block Record

Keyed by height. The hash index maps a block hash to its height, so the record is stored once.

Fields:

- block header
- transactions
- block hash
- app hash
- optional quorum certificate that certified the block
- module state roots

Committed blocks written through the node commit path include the block QC in the same atomic block/state/app-write batch, together with the finality proofs that QC completes (`CommitBlockStateWithWrites` takes them as its last argument), so a commit costs one durable write instead of one more per proof; a proof the store would refuse is left out of the batch and stored by the post-commit step, which reports the failure. Finality proof serving can use the certified child block plus grandchild header to rebuild strict two-link commit-chain proofs after restart. Blocks inserted by offline tooling may omit the QC, but those records cannot be used to satisfy strict light-client finality proof requests until enough certified descendants are available.

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
- state root of the namespace: the root of its binary Merkle radix tree keyed by `sha256(key)` (see State Trees)

### State Trees

The store keeps the state tree of every namespace (`stateproof`) next to its KV pairs, under `tree:`. A key's path is `sha256(key)`; an empty subtree hashes to zero, a subtree holding one key is that key's leaf node, placed as high as it is alone, and any other subtree is the internal node of its two halves. The shape depends only on the set of keys, so every replica derives the same root. Every write that changes a pair rewrites only the nodes on its path in the same atomic batch, so a block's state roots cost `O(writes × depth)` instead of rehashing each namespace, and a latest-state proof reads the nodes on the key's path. `ImportNamespace` rebuilds the namespace's tree.

Two in-memory structures keep a block's tree work small; neither is persisted and neither changes a result. The tree node cache holds up to 2^18 encoded nodes of the current trees. Every batch the store commits updates it with the tree nodes it sets and deletes, and a generation counter, odd while such a batch commits, lets a reader use the cache only at the state of its snapshot. The runtime asks for the root of every namespace over a block's writes for the app hash and again for the state roots, and the commit applies the same writes. So the store keeps, per namespace, the root and node changes of its last computation, keyed by that generation and a SHA-256 digest of the namespace's writes in order, and a later computation or the commit of the same writes over the same state reuses them.

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

The store keeps state history as an undo log of per-key versions. When a key changes at a height (a block's writes at its height, a direct write (`Set`, `Delete`, `SetBatch`) after the first committed state at the next committed height), the store records the value the key held before, absent included, as the key's version at that height, once per key and height; direct writes made before the block that commits their height keep the version the first of them recorded (`hdirect` names that height). The value of a key at a committed height is the value its lowest version above that height records, or its current value when it has none. Recording a change reads only the key's current value, never its history, and a historical read is one seek into the key's versions. `ImportNamespace` installs a namespace's base state and drops its versions. Storage grows with the number of changes, never with the state size per block. `PruneBelow` drops every version at or below the height it retains from: those versions only record values of heights below it. Historical reads at a height without a committed state record fail with `ErrStateNotFound`. Historical query proofs read the namespace's radix tree at the requested height: every tree node change is versioned like a key, so a proof at a past height costs what a proof of the current state costs and never rebuilds the namespace. Membership proves with the path to the key's leaf; non-membership with the path to the leaf or empty branch the key's path ends at. Every version is also listed under its height in a version index, so `PruneBelow` visits only the versions it drops, and the block, state and state root records it drops are keyed by height first; pruning costs what it drops, not the size of the history. Evidence records are monotonic for the `Applied` flag: once an evidence record is marked applied, later pending saves in the same process cannot downgrade it back to unapplied.

When an app block produces validator updates, the store-backed validator registry stages the height `H + 1` validator-set snapshot as KV writes. Those writes commit in the same Pebble batch as app writes, block metadata, state metadata, and state roots. If the block commit fails, the future validator-set snapshot is not persisted. If the process crashes after the backend batch succeeds but before in-memory post-commit hooks run, reopening the store-backed registry reloads the persisted height-versioned sets.

Runtime compaction includes both backend store compaction and mempool WAL compaction. WAL compaction rewrites the live pool (its pending transactions, then its batches) after committed transactions are removed, preventing long-running nodes from retaining stale append-only mempool records indefinitely. A commit compacts the log only once it holds at least 4 MiB and twice what it held after it was last compacted or opened, so each appended byte is rewritten about once and the log a restart replays stays within twice the live content or 4 MiB; an explicit compaction (`Node.Compact`, or a start that drops replayed transactions the application refuses) always rewrites it. A commit removes its transactions from the pool and queues the removal record without waiting for its sync; the next commit waits for that record to be durable before it executes its block, because a restart repeats the removal for the last committed block alone. The removal usually shares the sync of a later admission. `InitChain` writes the genesis state of every module as one batch, so a genesis of many accounts costs one durable write, and a genesis that fails leaves nothing in the store. During startup, a partially written final WAL record caused by process or host crash is truncated and ignored, while corrupt records in the middle of the WAL fail closed because silently skipping them could change transaction ordering. The in-memory mempool seen-cache also prunes expired entries on admission and commit paths when `seen_ttl` is enabled.

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
- `receipt_height/{height}/{tx_hash}`: the transactions of a block, which receipt pruning visits by height.
- `receipt_pruned_below`: the height below which receipts are pruned.
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

- A snapshot is a manifest followed by pages of KV pairs. The manifest carries the chain ID, the state record, the state root of every replicated namespace, the block of the state with the headers that link it to a certified anchor header, and the anchor header.
- The replicated namespaces must be exactly the ones the node replicates, each with one state root at the snapshot height, and those roots must give the state's app hash and the anchor header's app hash.
- Pages carry the pairs of one namespace in strictly ascending key order; the radix-tree root of a namespace's pairs must equal its state root, so a snapshot cannot add, drop, or alter a pair.
- Integrity comes from the trusted anchor header and the state roots it commits to, not from a checksum.

## Schema Migration

Store migrations must be height-gated through an upgrade plan and support rollback on failure.
