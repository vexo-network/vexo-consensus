# RPC API Versioning

## Stability Goal

Vexo RPC should be stable enough for operators, wallets, dashboards, and automation harnesses.

## Current Stable API

Stable endpoints are exposed under `/v1`. The unversioned paths remain compatibility aliases.

- `/v1/healthz`
- `/v1/readyz`
- `/v1/status`
- `/v1/diagnostics`
- `/v1/metrics`
- `/v1/metrics/text`
- `/v1/peers`
- `/v1/tx`
- `/v1/evidence`
- `/v1/recovery`
- `/v1/snapshot/latest`
- `/v1/snapshot/export`
- `/v1/snapshot/chunk?index=0&size=10000`
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/ibc/client/{client_id}`
- `/v1/ibc/connection/{connection_id}`
- `/v1/ibc/channel/{port_id}/{channel_id}`
- `/v1/ibc/packet/{sequence}/{source_port}/{source_channel}/{destination_port}/{destination_channel}`
- `/v1/ibc/proof/packet/{sequence}/{source_port}/{source_channel}/{destination_port}/{destination_channel}`
- `/v1/validators/{height}`
- `/v1/committee/{height}/{round}`

Admin endpoints:

- `/v1/prune`
- `/v1/replay`
- `/v1/consensus/start`
- `/v1/consensus/stop`

Admin endpoints require configured authorization. If no token is configured, the endpoint returns `401` instead of treating the empty token as an operator bypass.

Operators may use one root token or scoped tokens in `network_config.json`:

```json
{
  "rpc": {
    "admin_token": "root-token",
    "admin_tokens": {
      "prune-token": ["prune"],
      "replay-token": ["replay"],
      "ops-token": ["recovery", "consensus"]
    }
  }
}
```

Supported scopes are `recovery`, `prune`, `replay`, and `consensus`. A scoped token with `["*"]` is equivalent to a root admin token. RPC middleware emits structured admin audit events when an audit sink is configured by the embedding node.

`/v1/replay` accepts `strict: true` to require isolated re-execution from genesis or a retained historical snapshot. Non-strict replay may fall back to stored block/state consistency checks when isolated replay prerequisites are unavailable; strict replay fails closed instead.

## Versioning Rules

- Additive response fields are minor-compatible.
- Removing or renaming fields requires a new version.
- Changing error semantics requires a new version or explicit compatibility flag.
- Mutating endpoints must remain admin-token protected.
- Admin-token checks must fail closed when token configuration is absent.
- JSON decoders for public endpoints should reject unknown fields where request safety matters.

## Compatibility Aliases

Unversioned paths such as `/status`, `/tx`, and `/blocks/latest` are compatibility aliases for `/v1/status`, `/v1/tx`, and `/v1/blocks/latest`.

New clients should use `/v1/*`. Future breaking API changes should use a new prefix such as `/v2/*`.

## Error Format

Errors should use:

```json
{"error":"message"}
```

Do not leak private key material, auth token values, or internal file contents in RPC errors.

## Query Proofs

`/v1/proof` returns a state-root-bound Merkle query-proof envelope for KV state. If `height` is omitted, the latest committed height is used:

```bash
curl 'http://127.0.0.1:26657/v1/proof?namespace=bank&key=alice'
curl 'http://127.0.0.1:26657/v1/proof?height=10&namespace=bank&key=alice'
```

LevelDB stores height-versioned KV writes during atomic block commits, so historical proofs rebuild the namespace snapshot at the requested height and bind membership or non-membership to the state root saved for that height. Existing-key proofs include a compact Merkle path. Missing-key proofs include compact left/right neighbor absence proofs; legacy full namespace absence witnesses remain verifier-compatible. Stores that do not implement historical namespace reads must reject historical proof requests instead of returning latest values for older heights.

`/v1/finality/latest` and `/v1/finality/{height}` return the latest or height-specific three-chain finality proof known by the live consensus state. These endpoints expose consensus finality, while `/v1/status.latest_height` reports application state commit height.

## Event Queries

`/v1/events` queries indexed transaction events by attribute key/value:

```bash
curl 'http://127.0.0.1:26657/v1/events?key=sender&value=alice'
```

Only attributes emitted with `Index: true` are queryable. Modules must keep event emission deterministic.

## IBC Queries

The IBC module exposes relayer-facing read endpoints:

```bash
curl 'http://127.0.0.1:26657/v1/ibc/client/07-vexo-0'
curl 'http://127.0.0.1:26657/v1/ibc/connection/connection-0'
curl 'http://127.0.0.1:26657/v1/ibc/channel/transfer/channel-0'
curl 'http://127.0.0.1:26657/v1/ibc/packet/1/transfer/channel-0/transfer/channel-1'
curl 'http://127.0.0.1:26657/v1/ibc/proof/packet/1/transfer/channel-0/transfer/channel-1'
```

Responses wrap the module JSON state in `{ "path": [...], "value": ... }`. Missing IBC state returns `404`.

Packet proof responses reuse the standard Merkle query-proof envelope with namespace `ibc` and packet commitment key `packets/{source_port}/{source_channel}/{sequence}`. Relayers can use this endpoint to prove sent, acknowledged, or timed-out packet receipt state at a specific height. The keeper validates client chain ID, trusted height, trusted state root, namespace, key, existence, Merkle proof, and decoded packet receipt before accepting a packet commitment proof.

## Web3 JSON-RPC Bridge

The Web3 bridge provides Ethereum execution and wallet/tooling compatibility inside a Vexo network. It does not expose Ethereum devp2p, Ethereum fork choice, or Ethereum sync semantics. Vexo nodes keep Vexo consensus, validator lifecycle, P2P, state sync, and block formats.

### Supported Method Groups

The bridge supports single requests, batch requests, notifications, string block tags, EIP-1898 selectors, and these method families:

- **Client/network**: `rpc_modules`, `web3_clientVersion`, `web3_sha3`, `net_version`, `net_listening`, `net_peerCount`, `eth_chainId`, `eth_protocolVersion`, `eth_syncing`, `eth_mining`, `eth_hashrate`.
- **Blocks/fees**: `eth_blockNumber`, block lookups by number/hash, transaction count by block, uncle compatibility methods, `eth_gasPrice`, `eth_blobBaseFee`, `eth_maxPriorityFeePerGas`, `eth_feeHistory`.
- **Accounts/state**: `eth_accounts`, `eth_coinbase`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getProof`, `eth_getCode`, `eth_getStorageAt`.
- **Transactions/receipts**: `eth_sendRawTransaction`, `eth_sendTransaction`, `eth_signTransaction`, `eth_sign`, `personal_sign`, transaction lookup methods, raw transaction lookup methods, `eth_getTransactionReceipt`, `eth_getBlockReceipts`.
- **Logs/filters**: `eth_getLogs`, `eth_newFilter`, `eth_getFilterChanges`, `eth_getFilterLogs`, `eth_uninstallFilter`.
- **Simulation/tracing**: `eth_call`, `eth_estimateGas`, `eth_createAccessList`, `debug_traceTransaction`, `debug_traceCall`, `debug_traceBlockByNumber`, `debug_traceBlockByHash`, `trace_call`, `trace_transaction`, `trace_get`, `trace_block`, `trace_filter`, `trace_replayTransaction`, `trace_replayBlockTransactions`.
- **Txpool**: `eth_pendingTransactions`, `txpool_status`, `txpool_content`, `txpool_contentFrom`, `txpool_inspect`.
- **Vexo blob sidecars**: `vexo_sendRawBlobTransaction`, `vexo_getBlobSidecarByTxHash`, `vexo_getBlobSidecarByBlobHash`.

### Chain ID and Raw Transactions

- `eth_chainId`, `net_version`, and signed raw transaction validation use the configured execution `evm_chain_id`; nodes reject `0`.
- `eth_sendRawTransaction` accepts signed Ethereum RLP/typed transactions, verifies sender and chain ID with go-ethereum signers, preserves the Ethereum transaction hash/access list/blob metadata, and translates the payload into the canonical internal `evm` transaction format.
- Raw transaction admission and block execution both enforce fee-cap relationships, blob fee caps, protected legacy settings, and current base/blob-base fees. Mempool wrapper tags are not trusted at execution time.

### Native Coin and State

- Vexo native coin and EVM account balances are one asset. EVM balance writes persist to the canonical `bank` namespace, and `eth_getBalance`/`eth_getProof` reconstruct Ethereum account proofs from committed Vexo state.
- The EVM module stores nonce writes in the canonical account sequence namespace and snapshots Ethereum account/code/storage state during `EndBlock` for retained historical Web3 reads.
- Mixed Vexo module blocks use deterministic Vexo roots where Ethereum transaction/receipt trie roots would be misleading. Blocks containing only Ethereum raw transactions with EVM receipts compute Ethereum-style transaction and receipt roots with go-ethereum `DeriveSha`.
- When `execution.strict_evm_state_root` is enabled, Web3 block responses and `newHeads` fail closed if a retained EVM state root is unavailable instead of substituting the Vexo app hash.

### Calls, Gas, and Traces

- `eth_call`, `eth_estimateGas`, and `eth_createAccessList` run through a non-persisting geth `ApplyMessage` simulation path with requested block height, base fee, blob base fee, access list, optional EIP-7702 authorization list, state overrides, and block overrides.
- Calls without explicit gas price or fee caps use zero-gas-price `NoBaseFee` read semantics; explicit fee fields still use geth state-transition checks.
- Gas estimation applies go-ethereum intrinsic and floor-data gas rules, then probes the VM adapter and binary-searches the lowest non-failing gas limit.
- Receipt-backed traces expose persisted state diff, VM trace frames, call trees, log data, and optional replay sections when the VM adapter provides them. Unknown optional tracer sections return `null` rather than failing the whole request.

### Mempool and Subscriptions

- Pending transaction and txpool methods read the live Vexo mempool when the provider exposes raw pending transactions. Contiguous nonces are reported as `pending`; nonce gaps are reported as `queued`.
- Same-signer/same-nonce replacement requires the configured replacement bump.
- WebSocket `newHeads`, `logs`, and `newPendingTransactions` delivery is bounded per polling tick, and socket writes use deadlines so slow clients cannot force unbounded catch-up work.

## Web3 EVM Configuration

EVM behavior is configured from execution config, not by pretending the node is an Ethereum devp2p client:

- `execution.evm_fork_preset` defaults to `latest`; use `custom` only when `execution.evm_chain_config_json` pins a raw go-ethereum `params.ChainConfig` JSON.
- `execution.evm_chain_config_json` is passed to the geth VM adapter and validated with go-ethereum fork-order checks before the node starts.
- `execution.allow_unprotected_legacy_tx` defaults to `false`; unprotected Homestead-style legacy raw transactions are rejected on every Web3 raw-transaction admission path unless a chain deliberately opts into them.
- `execution.max_blob_sidecar_blobs` and `execution.max_blob_sidecar_bytes` bound `vexo_sendRawBlobTransaction` sidecars at transaction admission and execution.
- `execution.max_blob_gas` is enforced at block execution before application state is mutated.
- Historical `eth_call`, `eth_estimateGas`, `eth_createAccessList`, and `eth_feeHistory` use height-matched persisted base-fee and blob-base-fee state instead of latest-only fee context. Historical EVM calls read retained EVM account/code/storage snapshots and fail closed once the requested snapshot has been pruned.
- `eth_call` executes with Ethereum call semantics and discards writes after simulation; it is not mapped to `STATICCALL`. Omitting `to` simulates contract creation and returns the VM creation output without persisting deployed code.
- `eth_estimateGas` uses go-ethereum intrinsic and floor-data gas helpers for calldata, access-list, contract-creation, and fork-rule costs before probing the VM adapter, so low caller gas limits cannot under-report below the protocol floor.
- `trace_filter` supports bounded block ranges plus `fromAddress`, `toAddress`, `after`, and `count` filtering over committed receipt-backed traces.
- Ethereum raw transaction admission rejects invalid fee-cap relationships, including `maxFeePerGas < baseFee`, `maxPriorityFeePerGas > maxFeePerGas`, blob fee cap below blob base fee, and unprotected legacy transactions when the compatibility opt-in is disabled. During block execution, the EVM module revalidates raw transactions against the current block base fee/blob base fee so repriced mempool transactions cannot bypass inclusion-time economics.
- Ethereum raw transaction nonces use Ethereum semantics: the first accepted nonce is `0`, and the account sequence persisted after execution is the next expected nonce.
- Optional managed Web3 accounts require `network_config.json` RPC `evm_managed_accounts: true` plus `evm_account_private_keys` entries, or repeated `vexod start --evm-account-key` flags for private operator workflows. Managed accounts power `eth_accounts`, `eth_coinbase`, `eth_sign`, `personal_sign`, `eth_signTransaction`, and `eth_sendTransaction`; `eth_sendTransaction` signs an Ethereum-format transaction locally, verifies it with the configured Vexo EVM chain ID, then submits the translated Vexo canonical transaction. Startup safety rejects configured EVM account private keys on public RPC listeners. Production operators should prefer external wallets or remote signing flows and avoid storing hot private keys in plain JSON.
- Vexo does not expose Ethereum stateless execution witnesses. Use Vexo native query proofs, finality proofs, and retained EVM state snapshots for light-client and audit verification instead of relying on geth `debug_executionWitness` semantics.
- `modules/evm/ethcompat.RunTransactionFixturesJSON` and `vexod ops conformance --evm-tx-fixtures <file>` are the CI entry points for Ethereum raw-transaction fixture/conformance suites; keep fixture corpora outside normal source unless they are small, licensed, and intentionally versioned.

Bytecode semantics come from the registered `contract.VM` adapter; the built-in path targets Ethereum execution semantics and Web3 JSON-RPC compatibility inside Vexo consensus, not Ethereum devp2p/consensus compatibility.

## Operational Compatibility

Dashboards and alerting should prefer machine-readable JSON endpoints and use `/metrics/text` only for Prometheus-style scraping.
