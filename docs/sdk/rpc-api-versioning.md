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

Admin endpoints require a configured admin token. If no token is configured, the endpoint returns `401` instead of treating the empty token as an operator bypass.

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

The Web3 JSON-RPC bridge includes block, fee, account, code, storage, transaction, receipt, log, call, estimate, and HTTP polling-filter methods: `web3_clientVersion`, `net_version`, `eth_chainId`, `eth_blockNumber`, `eth_getBlockByNumber`, `eth_getBlockByHash`, `eth_gasPrice`, `eth_maxPriorityFeePerGas`, `eth_feeHistory`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getCode`, `eth_getStorageAt`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getBlockReceipts`, `eth_getTransactionByHash`, `eth_getLogs`, `eth_newFilter`, `eth_getFilterChanges`, `eth_getFilterLogs`, `eth_uninstallFilter`, `eth_call`, and `eth_estimateGas`. `eth_sendRawTransaction` accepts signed Ethereum RLP/typed transactions, verifies the sender and chain ID through go-ethereum transaction signers, preserves the Ethereum transaction hash, and translates the payload into the internal `evm` canonical transaction format before mempool submission.

Committed block records persist transaction execution results. Web3 block responses and `newHeads` websocket notifications derive `receiptsRoot`, `logsBloom`, and `gasUsed` from those committed results. When every transaction/result in a block is an Ethereum raw transaction with an EVM receipt, `transactionsRoot` and `receiptsRoot` are computed with go-ethereum `DeriveSha` over Ethereum transaction and receipt RLP. Mixed Vexo module blocks fall back to deterministic Vexo roots rather than pretending to be Ethereum trie roots. Full transaction responses derive `from`, `to`, `nonce`, `value`, `input`, `type`, `chainId`, `gas`, effective `gasPrice`, and dynamic-fee caps from Ethereum raw transaction tags when present. `eth_getTransactionReceipt` and `eth_getBlockReceipts` return Ethereum-shaped receipt fields, including `logsBloom`, `effectiveGasPrice`, `type`, contract creation `to: null`, and the preserved Ethereum transaction hash. `eth_feeHistory` reports the persisted base-fee path with bounded block count, and `eth_maxPriorityFeePerGas` returns the node's conservative priority-fee hint. `eth_getLogs` supports both address-scoped filters and global log filters when no address is supplied. EVM logs are indexed with prefix keys by height/transaction/log index and address, so log queries do not rely on one ever-growing global JSON array. HTTP polling filters are bounded and evict oldest filter IDs when the in-process filter store reaches its configured limit.

Bytecode semantics come from the registered `contract.VM` adapter; chains that need Ethereum-equivalent execution must register and audit an Ethereum-compatible VM adapter.

## Operational Compatibility

Dashboards and alerting should prefer machine-readable JSON endpoints and use `/metrics/text` only for Prometheus-style scraping.
