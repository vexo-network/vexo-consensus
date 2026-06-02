# RPC API Versioning

## Stability Goal

Vexo RPC should be stable enough for operators, wallets, dashboards, and automation harnesses.

## Current Base API

Current endpoints are unversioned but should be treated as `v1` semantics:

- `/healthz`
- `/readyz`
- `/status`
- `/diagnostics`
- `/metrics`
- `/metrics/text`
- `/peers`
- `/tx`
- `/evidence`
- `/recovery`
- `/snapshot/latest`
- `/snapshot/export`
- `/blocks`
- `/blocks/latest`
- `/blocks/{height}`
- `/state/latest`
- `/state/{height}/{namespace}`
- `/validators/{height}`
- `/committee/{height}/{round}`

Admin endpoints:

- `/prune`
- `/replay`
- `/consensus/start`
- `/consensus/stop`

## Versioning Rules

- Additive response fields are minor-compatible.
- Removing or renaming fields requires a new version.
- Changing error semantics requires a new version or explicit compatibility flag.
- Mutating endpoints must remain admin-token protected.
- JSON decoders for public endpoints should reject unknown fields where request safety matters.

## Recommended Future Layout

Future stable APIs should expose:

```text
/v1/status
/v1/metrics
/v1/tx
/v1/evidence
/v1/blocks/{height}
```

The unversioned API can remain as a compatibility alias during the deprecation window.

## Error Format

Errors should use:

```json
{"error":"message"}
```

Do not leak private key material, auth token values, or internal file contents in RPC errors.

## Operational Compatibility

Dashboards and alerting should prefer machine-readable JSON endpoints and use `/metrics/text` only for Prometheus-style scraping.
