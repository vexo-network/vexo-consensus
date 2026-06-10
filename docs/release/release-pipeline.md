# Release Pipeline

## Goals

A release should provide:

- signed release binaries
- checksums
- SBOM
- reproducible Docker image inputs
- version compatibility matrix
- release candidate soak-test evidence

## Release Commands

Build cross-platform binaries, checksums, SBOM, and manifest:

```bash
make release VERSION=0.1.0
```

Sign checksums with GPG:

```bash
make sign-release VERSION=0.1.0
```

Build Docker image with pinned metadata:

```bash
make docker-image VERSION=0.1.0 IMAGE=vexo-consensus IMAGE_TAG=0.1.0
```

Run release candidate verification:

```bash
make release-candidate VERSION=0.1.0-rc.1
```

Generate or refresh the SHA-256-bound evidence manifest after collecting evidence files:

```bash
go run ./cmd/vexod network longrun \
  --validators 64 \
  --duration 168h \
  --rate 100 \
  --output dist/longrun-evidence.json
go run ./cmd/vexod network analyze-longrun \
  --input dist/longrun-evidence.json \
  --min-validators 64 \
  --min-duration 168h \
  --json > dist/longrun-analysis.json
go run ./cmd/vexod release collect-evidence \
  --rpc http://validator-1.example:26657 \
  --rpc http://validator-2.example:26657 \
  --duration 1h \
  --dist dist
go run ./cmd/vexod release evidence-manifest --dist dist --output dist/evidence-manifest.json
make release-evidence-manifest
```

`network analyze-longrun` re-checks machine-readable long-run evidence for schema, validator count, duration, submitted/failed load, height growth, per-node ops thresholds, snapshot health, and replay health. `release collect-evidence` samples validator RPC endpoints before and after the observation window and writes RPC-backed `longrun`, `ops-runbook`, `p2p-scale`, `state-sync-light-client`, and `snapshot-replay` evidence files plus a manifest. The snapshot evidence only passes when both a positive snapshot height and healthy replay diagnostics are observed. It does not fabricate chaos, KMS, economics, governance, MEV, external-audit, or BLS-audit evidence; those artifacts still need their dedicated drills or reviews.

Generate launch parameter recommendations for the target network:

```bash
go run ./cmd/vexod config tune --validators 64 --tps 5000 --regions 4 --latency 120ms --json
```

Print the operator launch checklist:

```bash
go run ./cmd/vexod release launch-checklist
go run ./cmd/vexod release launch-checklist --json
go run ./cmd/vexod release docs-quality --docs docs --json
```

Run the release gate before publishing a release candidate:

```bash
go run ./cmd/vexod release gate \
  --dist dist \
  --version 0.1.0-rc.1 \
  --evidence-manifest dist/evidence-manifest.json \
  --longrun-evidence dist/longrun-evidence.json \
  --chaos-evidence dist/chaos-evidence.json \
  --adversarial-evidence dist/adversarial-evidence.json \
  --fuzz-evidence dist/fuzz-evidence.txt \
  --kms-evidence dist/kms-evidence.json \
  --snapshot-evidence dist/snapshot-replay-evidence.json \
  --p2p-scale-evidence dist/p2p-scale-evidence.json \
  --state-sync-light-client-evidence dist/state-sync-light-client-evidence.json \
  --validator-economics-evidence dist/validator-economics-evidence.json \
  --upgrade-governance-evidence dist/upgrade-governance-evidence.json \
  --mev-fee-market-evidence dist/mev-fee-market-evidence.json \
  --ops-runbook-evidence dist/ops-runbook-evidence.json \
  --formal-safety-evidence dist/formal-safety-evidence.json \
  --sdk-conformance-evidence dist/sdk-conformance-evidence.json \
  --external-audit dist/external-audit.pdf \
  --bls-audit dist/bls-audit.pdf \
  --json
```

`release gate` fails closed when required evidence is missing, empty, malformed, explicitly reports a failed `ok`/`status`/check result, does not semantically cover the evidence category it claims to satisfy, or is not bound to `evidence-manifest.json` by SHA-256. `--allow-external-pending` requires both `--private-rc` and a private/RC-style version label containing `rc`, `alpha`, `beta`, or `private`; do not use it for public production launch gates.

## Artifacts

`dist/` contains:

- `vexod-<version>-<os>-<arch>` binaries
- `checksums.txt`
- `checksums.txt.asc` after signing
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`, binding each release evidence file name/path to its SHA-256 hash
- `longrun-analysis.json` and optional `docs-quality.json` when produced by the release pipeline
- long-run, chaos, adversarial, fuzz, signer, snapshot/replay, P2P scale, state-sync/light-client, validator economics, upgrade governance, MEV/fee-market, ops runbook, formal safety, SDK conformance including EVM/Web3 conformance, external-audit, and BLS-audit evidence files with passing content when preparing a release candidate

## Reproducibility Notes

Builds use:

- `CGO_ENABLED=0`
- `go build -trimpath`
- explicit version, commit, and build date ldflags
- Docker build args for version metadata

For stricter reproducibility, set a deterministic `BUILD_DATE`, use a clean checkout, and compare `checksums.txt` across builders.

## Signed Binaries

Binaries are verified through signed checksums:

```bash
gpg --verify dist/checksums.txt.asc dist/checksums.txt
shasum -a 256 -c dist/checksums.txt
```

## SBOM

The current SBOM is Go-module based:

```bash
cat dist/sbom-go-modules.json
```

External pipelines may replace or augment it with SPDX/CycloneDX tools.

## Audit Pack

Package release artifacts and reviewer evidence metadata:

```bash
go run ./cmd/vexod release pack --dist dist --version 0.1.0 --output dist/release-audit-pack.json
go run ./cmd/vexod release pack --dist dist --version 0.1.0 --require-signature
go run ./cmd/vexod release pack --dist dist --version 0.1.0 \
  --longrun-evidence dist/longrun-evidence.json \
  --adversarial-evidence dist/adversarial-evidence.json \
  --fuzz-evidence dist/fuzz-evidence.txt
```

The generated pack lists artifact SHA-256 values, required release files, signature status, attached long-run/adversarial/fuzz evidence, and the external audit checklist. `release gate` adds the stricter publish/no-publish decision by requiring category-specific chaos, signer, snapshot/replay, P2P scale, state-sync/light-client, validator economics, upgrade governance, MEV/fee-market, ops runbook, formal safety, SDK conformance including EVM/Web3 fixtures, evidence-manifest SHA-256 bindings, external audit, and BLS audit evidence.

## Release Candidate Soak Test

The `release-candidate` target runs:

- full test/vet check
- fuzz smoke tests
- ops verification
- built-binary network E2E (`make network-e2e`)
- adversarial simulation
- SDK/EVM conformance evidence. If the `evm` module is enabled, `vexod ops conformance` treats missing `--evm-default-fixtures`, `--evm-tx-fixtures`, or `--evm-tx-fixtures-dir` as an error, not a warning. The built-in fixture set is a baseline for dynamic-fee, access-list, protected legacy, unprotected legacy rejection, chain-ID, malformed raw, fee-cap behavior, geth VM call return data, contract creation execution, revert behavior, and persistent storage writes. Attach any chain-specific raw transaction corpus with `--evm-tx-fixtures <file>` or `--evm-tx-fixtures-dir <dir>` and any chain-specific VM execution corpus with `--evm-execution-fixtures <file>` or `--evm-execution-fixtures-dir <dir>` before making broader Web3/EVM compatibility claims.
- network load harness (`RC_DRY_RUN=1` keeps this as a plan-only dry-run; `make release-candidate-real` forces `RC_DRY_RUN=0`)
- chaos plan
- IBC relayer soak plan with `vexod relayer soak-plan --json`
- 7-day multi-host longrun plan
- longrun harness evidence (`RC_DRY_RUN=1` keeps this as a plan-only dry-run; `make release-candidate-real` forces real load/longrun execution)
- longrun analysis with `vexod network analyze-longrun`
- locale and canonical documentation quality with `vexod release docs-quality`
- evidence manifest generation for whatever RC evidence files are present in `dist/`

Real release candidates should run `network longrun` on independent machines and attach the generated evidence JSON plus metrics, logs, pprof, snapshot, replay, KMS signing, P2P scale, light-client, economics, governance-upgrade, MEV/fee-market, SDK conformance, EVM/Web3 raw transaction conformance, geth VM execution conformance, BLS adapter audit, and VRF adapter/KMS/TLS audit evidence.
The longrun harness distributes load across validator RPC endpoints and records per-validator submission counts in the evidence payload. The analyzer should pass before the evidence is attached to the release gate. Relayer soak plans should include both acknowledgement and timeout jobs and should be archived with checkpoint state. Upgrade plans that rely on no-op schema migrations must explicitly set `allow_noop_migrations=true`; `vexod upgrade apply --allow-empty-migrations` rejects plans that do not opt in. Public release gates require `--vrf-audit` evidence covering the VRF adapter implementation, dependency audit, TLS/mTLS or pinned CA, authorization, nonce replay defense, and KMS/HSM custody policy.

## Launch Runbook

Use [Launch Runbook](./launch-runbook.md) for the prelaunch gate, release-candidate gate, genesis gate, launch-window monitoring, halt criteria, and postlaunch archive requirements.
