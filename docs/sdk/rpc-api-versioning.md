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
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`
- `/v1/ibc/client/{client_id}`
- `/v1/ibc/connection/{connection_id}`
- `/v1/ibc/channel/{port_id}/{channel_id}`
- `/v1/ibc/packet/{sequence}/{source_port}/{source_channel}/{destination_port}/{destination_channel}`
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

`/v1/proof` returns a state-root-bound query-proof envelope for the latest KV state:

```bash
curl 'http://127.0.0.1:26657/v1/proof?namespace=bank&key=alice'
```

Historical query proofs are rejected unless a chain integrates historical KV snapshots. This prevents nodes from returning a latest-value proof while pretending it belongs to an older height.

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
```

Responses wrap the module JSON state in `{ "path": [...], "value": ... }`. Missing IBC state returns `404`.

## Operational Compatibility

Dashboards and alerting should prefer machine-readable JSON endpoints and use `/metrics/text` only for Prometheus-style scraping.
