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

## Operational Compatibility

Dashboards and alerting should prefer machine-readable JSON endpoints and use `/metrics/text` only for Prometheus-style scraping.
