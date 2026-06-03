# Launch Runbook

This runbook defines the minimum operator flow for launching an independent Vexo network.

## Prelaunch Gate

Run these checks from a clean checkout:

```bash
make check
make ops-verify
go run ./cmd/vexod release launch-checklist --json
go run ./cmd/vexod config audit --home .vexo --strict
go run ./cmd/vexod network scale-plan --validators 64 --regions 4 --hosts 8 --duration 24h --rate 100
```

Do not launch if:

- deterministic crypto is enabled outside development
- validator homes fail strict config audit
- remote signer policy or double-sign guard is not verified
- release artifacts, checksums, SBOM, or audit pack are missing
- long-run, adversarial, fuzz, snapshot, replay, or signer evidence is missing for a release candidate

## Release Candidate Gate

Required artifacts:

- signed binaries or signed checksums
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- long-run network evidence
- consensus adversarial simulation evidence
- fuzz or property test evidence

Recommended commands:

```bash
make release VERSION=<version>
make sign-release VERSION=<version>
go run ./cmd/vexod release pack \
  --dist dist \
  --version <version> \
  --require-signature \
  --longrun-evidence dist/longrun-evidence.json \
  --adversarial-evidence dist/adversarial-evidence.json \
  --fuzz-evidence dist/fuzz-evidence.txt \
  --output dist/release-audit-pack.json
```

## Genesis Gate

Before public start:

- freeze `chain_id`, initial validator set, validator set hash, and app state
- copy identical genesis/config files to every validator
- verify Ed25519 keys or BLS proof-of-possession metadata
- verify remote signer reachability and signer-side height/round/type policy
- define the last safe recovery height and rollback procedure

## Launch Window

Start sequence:

1. start seed validators
2. start remaining validators by region
3. wait for quorum and height growth
4. submit low-rate signed smoke transactions
5. increase load only after metrics remain healthy

Monitor these values continuously:

- height increase rate
- round timeout frequency
- proposal and vote processing latency
- peer ban count
- mempool size
- commit latency
- snapshot and replay health
- validator signing failures

Halt the launch if:

- quorum is unstable
- conflicting finality is observed
- signer policy rejects valid consensus messages
- snapshot verification or replay recovery diverges
- peer bans spike across multiple regions
- commit latency remains above the configured alert threshold

## Postlaunch Archive

Archive:

- final genesis and config files
- validator set and validator set hash evidence
- release pack and signed checksums
- launch metrics, logs, pprof samples, and peer score snapshots
- long-run, chaos, adversarial, fuzz, snapshot, replay, and signer evidence

After launch, schedule:

- external security review
- multi-region long-duration soak
- slashing lifecycle review
- governance upgrade drill
- state sync restore drill
