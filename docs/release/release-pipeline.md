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

## Artifacts

`dist/` contains:

- `vexod-<version>-<os>-<arch>` binaries
- `checksums.txt`
- `checksums.txt.asc` after signing
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`

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

## Release Candidate Soak Test

The `release-candidate` target runs:

- full test/vet check
- fuzz smoke tests
- ops verification
- adversarial simulation
- localnet load dry-run
- chaos plan
- 7-day multi-host longrun plan

Real release candidates should additionally run the longrun plan on independent machines and attach metrics, logs, pprof, snapshot, replay, and KMS signing evidence.
