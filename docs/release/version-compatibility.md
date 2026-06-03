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
| dev / alpha | `/v1/*` stable paths, unversioned aliases | 1 | 1 | 1 | Experimental; not for production funds |

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

## Rollback Drill

Generate a rollback drill plan before activating an upgrade:

```bash
go run ./cmd/vexod upgrade rollback-plan \
  --plan-file upgrade-plan.json \
  --record-file .vexo/upgrade-records.json \
  --last-safe-height <upgrade-height-minus-one> \
  --snapshot .vexo/snapshots/<safe-height>.json
```

The drill must verify:

- rollback binary or artifact is declared
- last safe height is lower than the upgrade height
- restore snapshot evidence is attached
- failed `rollback_required` records block retry until rollback is completed
- validators restart from identical genesis/config inputs
- replay, signer policy, height growth, and light-client finality proofs remain valid after rollback
