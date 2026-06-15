# Launch Runbook

This runbook defines the minimum operator flow for launching an independent Vexo network.

## At a Glance

Launches are easiest to think about in three phases:

1. **Prelaunch** — validate config, docs, release artifacts, and evidence before any validator starts.
2. **Launch window** — start the network, watch height/finality/peer health, and stop if safety drifts.
3. **Postlaunch archive** — preserve the exact files, logs, metrics, and evidence bundle that prove what ran.

If you only remember one rule, remember this: never “fix” a launch by changing the binary on the fly. Re-run the gate with the reviewed artifact instead.

## Prelaunch Gate

Run these checks from a clean checkout:

```bash
make check
make ops-verify
go run ./cmd/vexod release launch-checklist --json
go run ./cmd/vexod release docs-quality --docs docs --json
go run ./cmd/vexod config audit --home .vexo --strict
go run ./cmd/vexod config tune --validators 64 --tps 5000 --regions 4 --latency 120ms --json
go run ./cmd/vexod network scale-plan --validators 64 --regions 4 --hosts 8 --duration 24h --rate 100
```

Do not launch if:

- deterministic crypto is enabled for a network that is expected to carry real value or public validator traffic
- validator homes fail strict config audit
- remote signer policy or double-sign guard is not verified
- public RPC lacks TLS or an operator access boundary while admin endpoints are enabled
- peer scoring has no `MaxScore`, ban threshold, or window limits
- public validator metadata contains Docker-only service names instead of externally resolvable addresses
- parameter tuning output is missing or has failed validation checks
- release artifacts, checksums, SBOM, or audit pack are missing
- canonical or localized documentation fails `release docs-quality`
- long-run, adversarial, fuzz, snapshot, replay, or signer evidence is missing for a release candidate
- P2P scale, state-sync/light-client, validator economics, upgrade governance, MEV/fee-market, ops runbook, formal safety, or SDK conformance evidence is missing for a public release candidate
- `release gate` fails without an explicitly documented private-RC exception

## Release Candidate Gate

Required artifacts:

- signed binaries or signed checksums
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- documentation quality evidence when docs changed during the candidate
- long-run network evidence
- long-run analysis proving validator count, duration, submitted load, height growth, snapshot health, replay health, and per-node alert thresholds
- chaos test evidence
- consensus adversarial simulation evidence
- fuzz or property test evidence
- KMS or remote signer policy evidence
- snapshot/replay restore evidence
- P2P scale evidence covering discovery, reconnect, NAT, seed, addrbook, ban eviction, and backpressure
- state-sync and light-client evidence covering validator-set hash binding, snapshot restore, replay, and finality proof verification
- validator economics evidence covering custody, rewards, commission, jail, tombstone, unbonding, and slashing accounting
- upgrade governance evidence covering proposal approval, migration execution, halt, rollback, and failed-upgrade recovery
- MEV/fee-market evidence covering base fee, fair ordering, censorship-resistance, spam cost, and mempool WAL replay
- ops runbook evidence covering alert thresholds, incident drills, multi-region observability, and archive requirements
- formal safety evidence covering invariants, adversarial simulation output, and property/fuzz output
- SDK conformance evidence covering app modules, custom crypto, custom storage, custom transport, RPC versioning, upgrade hooks, built-in EVM/Web3 raw transaction fixtures, geth VM execution fixtures, and any chain-specific fixture corpus
- relayer soak-plan evidence covering acknowledgement and timeout relay jobs when IBC is enabled
- external audit disposition for public releases
- BLS adapter audit evidence when BLS is enabled

Recommended commands:

```bash
make release VERSION=<version>
make sign-release VERSION=<version>
go run ./cmd/vexod ops conformance \
  --home .vexo \
  --json > dist/sdk-conformance-evidence.json
go run ./cmd/vexod ops conformance \
  --home .vexo \
  --evm-tx-fixtures-dir ./fixtures/evm/transactions \
  --evm-tx-fixtures-sha256 <tx-fixture-corpus-sha256> \
  --evm-execution-fixtures-dir ./fixtures/evm/execution \
  --evm-execution-fixtures-sha256 <execution-fixture-corpus-sha256> \
  --strict \
  --json > dist/evm-web3-conformance-evidence.json
go run ./cmd/vexod relayer soak-plan \
  --source-rpc http://validator-1.example:26657 \
  --dest-rpc http://validator-2.example:26657 \
  --client-id client-0 \
  --sequences 16 \
  --json > dist/relayer-soak-plan.json
go run ./cmd/vexod release docs-quality --docs docs --json > dist/docs-quality.json
go run ./cmd/vexod release collect-evidence \
  --rpc http://validator-1.example:26657 \
  --rpc http://validator-2.example:26657 \
  --duration 1h \
  --dist dist
go run ./cmd/vexod network analyze-longrun \
  --input dist/longrun-evidence.json \
  --min-validators 64 \
  --min-duration 168h \
  --json > dist/longrun-analysis.json
go run ./cmd/vexod release pack \
  --dist dist \
  --version <version> \
  --require-signature \
  --longrun-evidence dist/longrun-evidence.json \
  --adversarial-evidence dist/adversarial-evidence.json \
  --fuzz-evidence dist/fuzz-evidence.txt \
  --evm-web3-conformance-evidence dist/evm-web3-conformance-evidence.json \
  --output dist/release-audit-pack.json
go run ./cmd/vexod release gate \
  --dist dist \
  --version <version> \
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
  --evm-web3-conformance-evidence dist/evm-web3-conformance-evidence.json \
  --external-audit dist/external-audit.pdf \
  --bls-audit dist/bls-audit.pdf \
  --bls-audit-sha256 <sha256> \
  --vrf-audit dist/vrf-audit.pdf \
  --vrf-audit-sha256 <sha256>
```

`release collect-evidence` only marks snapshot/replay evidence as passing when the sampled validators expose both a positive snapshot height and healthy replay diagnostics. If that check is false, run the snapshot restore/replay drill before packaging the release.

`network analyze-longrun` should pass before `longrun-evidence.json` is accepted by reviewers. It verifies that the evidence still shows validator participation, submitted load, height growth, ops-threshold health, snapshot health, and replay health instead of merely proving that a JSON file exists.

`--evm-default-fixtures` is intentionally small enough for CI but not enough for strict launch evidence. Strict EVM/Web3 evidence must use `--evm-tx-fixtures` or `--evm-tx-fixtures-dir` for chain-specific raw transaction scenarios and `--evm-execution-fixtures` or `--evm-execution-fixtures-dir` for contract, precompile, blob, opcode, and account-abstraction execution scenarios. Pin those external corpora with `--evm-tx-fixtures-sha256` and `--evm-execution-fixtures-sha256`; strict conformance exits non-zero if the corpus or digest pin is missing.

EVM/Web3 conformance evidence is separate from SDK conformance and is attached with `--evm-web3-conformance-evidence`. It must include the machine-readable `evm_fixtures`, `evm_execution`, `web3_rpc`, and `evm_corpus` reports emitted by `vexod ops conformance`. `evm_corpus` records the transaction fixture source, execution fixture source, SHA-256 digest, fixture count, and pinning status. A JSON file that only says “EVM fixtures passed” in a summary string is rejected by the release gate.

`relayer soak-plan` emits a runnable `relayer run` config that alternates acknowledgement and timeout jobs, uses checkpoint state to prevent duplicate submissions, and keeps transient proof/RPC failures visible in soak output. Archive the generated plan and any checkpoint/log evidence when IBC is part of the launch surface.

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
- peer backpressure saturation and reconnect churn
- light-client proof verification success rate
- base-fee movement, mempool WAL compaction, and fair-ordering consistency
- governance-upgrade plan, halt, apply, and rollback status

Halt the launch if:

- quorum is unstable
- conflicting finality is observed
- signer policy rejects valid consensus messages
- snapshot verification or replay recovery diverges
- peer bans spike across multiple regions
- peer scores unexpectedly pin at `MaxScore` while invalid-message, reconnect, or ban metrics also increase
- commit latency remains above the configured alert threshold
- light-client proof verification or validator-set hash binding fails
- base fee, fair ordering, or mempool replay behaves non-deterministically across nodes
- governance-upgrade migration enters rollback-required state without a verified last safe height

## Postlaunch Archive

Archive:

- final genesis and config files
- validator set and validator set hash evidence
- release pack and signed checksums
- launch metrics, logs, pprof samples, peer score snapshots, and final split config files
- docs quality report and localized documentation manifest
- long-run, chaos, adversarial, fuzz, snapshot, replay, signer, P2P scale, light-client, economics, governance-upgrade, MEV/fee-market, ops runbook, formal safety, SDK conformance, and EVM/Web3 conformance evidence with category-specific passing content plus `evidence-manifest.json` SHA-256, provenance, Ed25519 public key, and verified public-release attestation bindings

After launch, schedule:

- external security review
- multi-region long-duration soak
- slashing lifecycle review
- governance upgrade drill
- state sync restore drill
