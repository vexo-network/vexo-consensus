# Security Audit Readiness

## Scope

This package is intended for independent reviewers evaluating Vexo consensus, networking, storage, crypto boundaries, slashing, and operational safety.

## Threat Model

### Assets

- validator private keys and remote signer policy state
- finalized block history
- validator set history and validator set hashes
- application state and module state roots
- evidence records and slashing receipts
- RPC admin token and node configuration
- peer score and ban state

### Adversaries

- Byzantine validators with less than one-third voting power
- network attackers causing delay, partition, replay, malformed gossip, or peer flooding
- Sybil peers attempting to exhaust mempool, RPC, or P2P resources
- compromised node process attempting double-sign through a remote signer
- malicious operator submitting false evidence
- crash/restart adversary exploiting partial persistence
- supply-chain adversary tampering with release binaries or container images

### Security Goals

- no conflicting finality without accountable evidence
- invalid finality proofs rejected by light clients
- deterministic validator-set hash binding at proof height
- no replayed transaction accepted in signed/nonce mode
- evidence validation before penalty application
- network-safety-gated configs reject deterministic crypto and unsafe transaction policy
- release artifacts are reproducible enough for independent verification

## Security Assumptions

- Less than one-third of voting power is Byzantine for safety and liveness under partial synchrony.
- Validator private keys or remote signer policies are not all compromised.
- Value-bearing or public-validator networks use Ed25519 or an audited BLS adapter, never deterministic crypto.
- Local encrypted key documents use AES-256-GCM with PBKDF2-SHA512, 600,000 iterations, and a 32-byte salt; production operators should still prefer a remote signer/KMS for validator signing.
- Remote signer/KMS enforces its own height/round/type/domain double-sign guard.
- Operators configure RPC admin tokens, P2P auth proofs, request limits, peer scoring, `MaxScore`, and ban thresholds. Admin RPC endpoints are expected to be unusable unless an admin token is configured.
- Public release candidates attach P2P scale, state-sync/light-client, validator economics, upgrade governance, MEV/fee-market, ops runbook, formal safety, and SDK conformance evidence to `release gate`; the gate rejects missing, empty, malformed, explicitly failed, or category-mismatched evidence content.
- Operators keep Docker-only service names out of public validator metadata unless the network is intentionally private and all peers resolve those names.
- Storage backend preserves block/state/evidence durability or clearly reports recovery mismatch.

## Known Limitations

- BLS backend requires an adapter with dependency audit evidence, proof-of-possession validation, subgroup/key checks, and release evidence; the built-in CIRCL adapter provides the implementation boundary but does not replace an external audit.
- VRF-backed committee selection requires an adapter with proof verification and key-source evidence; the built-in ECVRF P-256 adapter provides the implementation boundary but private key custody and deployment audit remain operator responsibilities.
- Ed25519 finality is ordered multisignature concatenation, not cryptographic aggregation.
- Data availability commitments use canonical transaction chunk roots with chunk-inclusion proofs and built-in GF(256) Reed-Solomon-style parity recovery. Large-network probabilistic 2D sampling remains a chain-specific policy layer.
- Invalid-proposal evidence supports reason-specific mismatch verification. DA mismatch and missing-data evidence are self-contained; validator-set-hash, app-hash, timestamp, and transaction-validity evidence require runtime-supplied height context and fail closed without it. Transaction-validity evidence is gated by an independently computed deterministic transaction-result hash. This is still not a complete light-client Merkle/state-proof framework for every possible invalid proposal claim.
- Runtime defaults to durable slashing when a store is configured; bank mint authority is configurable, and staking includes delegation, configurable commission caps, fee reward distribution, reward claiming, jail, unbonding state, and idempotent staking-ledger slashing from penalty receipts. Reward-policy tuning, tombstone authority, and full economic audit remain chain-specific integration work.
- Multi-region long-running and chaos tests are planned through network/longrun plans but must be executed on independent real machines.
- Governance upgrade execution records applied, pending, and rollback-required outcomes; durable chain-specific governance state, execution authority, rollback runbooks, and release governance policy remain production integration work.
- The current RPC API exposes `/v1/*` stable routes while retaining unversioned compatibility aliases.

## Formal-ish Safety Argument

1. Each valid proposal binds `chain_id`, height, round, block hash, and validator-set hash.
2. Each valid vote binds height, round, block hash, and validator identity under a consensus vote domain.
3. A validator cannot produce two valid votes for different block hashes at the same `(height, round)` without generating conflicting-vote evidence.
4. A QC requires at least two-thirds voting power from unique known validators in the validator set for that height.
5. Any two QCs for conflicting blocks at the same height require overlapping voting power. With less than one-third Byzantine power, at least one honest validator would have to double-vote, which honest logic rejects.
6. Locking rejects votes/proposals that do not extend the locked QC or carry a sufficiently new justify QC.
7. Three-chain finality only finalizes a grandparent when descendant and parent QCs prove a safe chain extension.
8. Therefore conflicting finality requires either a quorum intersection violation, invalid validator-set binding, or validator equivocation. The verifier rejects the former two and evidence accounts for equivocation.

## Required Evidence for Audit

Run and attach output from:

```bash
make check
make fuzz-smoke
go run ./cmd/vexod consensus adversarial --json
go run ./cmd/vexod config audit-pack --json
go run ./cmd/vexod config tune --validators <n> --tps <target> --regions <r> --latency <duration> --json
go run ./cmd/vexod ops thresholds --json
go run ./cmd/vexod upgrade plan --json --name audit-upgrade --height 100
go run ./cmd/vexod network longrun --validators 4 --duration 1h --rate 50 --output dist/longrun-evidence.json
go run ./cmd/vexod release gate \
  --dist dist \
  --version <version> \
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

Attach release artifacts from:

```bash
make release VERSION=<version>
make sign-release VERSION=<version>
```

## Auditor Focus Areas

- finality verifier and validator-set hash binding
- consensus lock safety and three-chain finality
- signature domain separation
- remote signer policy and double-sign guard assumptions
- evidence lifecycle and false-slashing resistance
- storage recovery safe-height semantics
- upgrade rollback and migration failure handling
- RPC admin access controls and rate limits
- release artifact reproducibility and signing
