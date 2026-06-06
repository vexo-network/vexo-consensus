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

Generate launch parameter recommendations for the target network:

```bash
go run ./cmd/vexod config tune --validators 64 --tps 5000 --regions 4 --latency 120ms --json
```

Print the operator launch checklist:

```bash
go run ./cmd/vexod release launch-checklist
go run ./cmd/vexod release launch-checklist --json
```

Run the release gate before publishing a release candidate:

```bash
go run ./cmd/vexod release gate \
  --dist dist \
  --version 0.1.0-rc.1 \
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

`release gate` fails closed when required evidence is missing, empty, malformed, explicitly reports a failed `ok`/`status`/check result, or does not semantically cover the evidence category it claims to satisfy. `--allow-external-pending` is acceptable for private release candidates only; do not use it for public production launch gates.

## Artifacts

`dist/` contains:

- `vexod-<version>-<os>-<arch>` binaries
- `checksums.txt`
- `checksums.txt.asc` after signing
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- long-run, chaos, adversarial, fuzz, signer, snapshot/replay, P2P scale, state-sync/light-client, validator economics, upgrade governance, MEV/fee-market, ops runbook, formal safety, SDK conformance, external-audit, and BLS-audit evidence files with passing content when preparing a release candidate

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

The generated pack lists artifact SHA-256 values, required release files, signature status, attached long-run/adversarial/fuzz evidence, and the external audit checklist. `release gate` adds the stricter publish/no-publish decision by requiring category-specific chaos, signer, snapshot/replay, P2P scale, state-sync/light-client, validator economics, upgrade governance, MEV/fee-market, ops runbook, formal safety, SDK conformance, external audit, and BLS audit evidence.

## Release Candidate Soak Test

The `release-candidate` target runs:

- full test/vet check
- fuzz smoke tests
- ops verification
- adversarial simulation
- network load harness (`RC_DRY_RUN=1` keeps this as a plan-only dry-run)
- chaos plan
- 7-day multi-host longrun plan
- longrun harness evidence (`RC_DRY_RUN=1` keeps this as a plan-only dry-run)

Real release candidates should run `network longrun` on independent machines and attach the generated evidence JSON plus metrics, logs, pprof, snapshot, replay, KMS signing, P2P scale, light-client, economics, governance-upgrade, MEV/fee-market, and SDK conformance evidence.

## Launch Runbook

Use [Launch Runbook](./launch-runbook.md) for the prelaunch gate, release-candidate gate, genesis gate, launch-window monitoring, halt criteria, and postlaunch archive requirements.
