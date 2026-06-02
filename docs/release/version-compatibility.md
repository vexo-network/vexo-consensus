# Version Compatibility Matrix

| Component | Compatibility Rule |
|---|---|
| Consensus wire schema | Breaking changes require protocol version bump |
| RPC API | Additive fields are compatible; removals require new API version |
| Config schema | Migrations must be captured in an upgrade plan |
| Store schema | Migrations must be height-gated and rollback-aware |
| App module state | Module migrations must be deterministic and replay-safe |
| Finality proof format | Breaking changes require light-client compatibility plan |
| Crypto backend | BLS adapters require audited implementation and explicit activation |
| Docker image | Image metadata must match binary version/commit/build date |

## Current Matrix

| Version | RPC | Config Schema | Store Schema | App State Schema | Notes |
|---|---|---:|---:|---:|---|
| dev / alpha | v1 semantics, unversioned paths | 1 | 1 | 1 | Experimental; not for production funds |

## Upgrade Compatibility Checklist

- Define governance-approved upgrade height.
- Define target binary version.
- Define config/store/app-state schema from/to versions.
- Generate upgrade plan with `vexod upgrade plan --json`.
- Execute the plan with `vexod upgrade apply` at the approved height and persist the execution record.
- Treat any `rollback_required` record as an operator stop condition until rollback is completed.
- Run `make release-candidate`.
- Sign checksums and publish SBOM.
- Execute rollback drill before deployment activation.
