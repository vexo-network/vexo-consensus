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
- Local encrypted key documents use AES-256-GCM with Argon2id by default and keep legacy PBKDF2-SHA512 documents readable for migration; production operators should still prefer a remote signer/KMS for validator signing.
- Remote signer/KMS enforces its own height/round/type/domain double-sign guard.
- Operators configure RPC root or scoped admin tokens, P2P auth proofs, request limits, peer scoring, `MaxScore`, and ban thresholds. Admin RPC endpoints are expected to be unusable unless an admin token is configured, and embeddings should attach the RPC admin audit sink to their structured log pipeline.
- Public release candidates attach P2P scale, state-sync/light-client, validator economics, upgrade governance, MEV/fee-market, ops runbook, formal safety, and SDK conformance evidence with IBC/relayer/proof plus EVM/Web3 fixture results to `release gate`; the gate rejects missing, empty, malformed, explicitly failed, or category-mismatched evidence content.
- Operators keep Docker-only service names out of public validator metadata unless the network is intentionally private and all peers resolve those names.
- Storage backend preserves block/state/evidence durability or clearly reports recovery mismatch.

## Known Limitations

- BLS backend defaults to the built-in supranational/blst min-pk adapter and still requires dependency audit evidence, proof-of-possession validation, subgroup/key checks, and release evidence. The CIRCL adapter remains reference/compatibility-only and does not replace an external audit.
- VRF-backed committee selection requires an adapter with proof verification and key-source evidence; the built-in ECVRF P-256 adapter provides local encrypted-key custody and the `remote-vrf-http-v1` adapter provides the KMS/HSM remote-prover boundary. Private key custody, remote service availability, replay protection, access policy, and deployment audit remain operator responsibilities.
- Ed25519 finality is ordered multisignature concatenation, not cryptographic aggregation.
- Data availability commitments use canonical transaction chunk roots with chunk-inclusion proofs, deterministic 1D/2D sample planning/reporting, and built-in GF(256) Reed-Solomon-style parity recovery. Operators still need chain-specific sampling thresholds and alert policy, but the 2D planner and verifier are code-level primitives.
- Invalid-proposal evidence supports reason-specific mismatch verification. DA mismatch and missing-data evidence are self-contained unless the proof declares an external context proof hash; validator-set-hash, app-hash, timestamp, and transaction-validity evidence require runtime-supplied height context and fail closed without it. Transaction-validity evidence is gated by an independently computed deterministic transaction-result hash, context-bound proofs must match the supplied context proof hash before slashing, and `VerifyInvalidProposalEvidenceWithBoundContext` is available for operators or modules that want to reject any context-bound invalid-proposal proof unless the context hash is embedded in the evidence itself. Runtime-required state proofs are verified against chain ID, evidence height, expected state root, namespace, key, existence, and value. Application-specific invalid-proposal reasons are registered with `RegisterInvalidProposalVerifier` or `RegisterInvalidProposalVerifierWithOptions`; context-required reasons must use bound-context verification before penalty application.
- Strict replay mode requires isolated re-execution from genesis or a retained historical snapshot and fails closed instead of falling back to stored metadata checks. If a retained historical snapshot is unavailable but block history still starts at height 1, strict replay re-executes from genesis in an isolated store and reports the requested range.
- Runtime defaults to durable slashing when a store is configured; bank mint authority is configurable, and staking includes delegation, configurable commission caps, fee reward distribution, reward claiming, entry-based matured unbonding withdrawal, jail, unbonding custody state, tombstone records for fully slashed validators, and idempotent staking-ledger slashing from penalty receipts. Evidence persistence preserves `Applied=true` monotonically so concurrent pending evidence writes cannot downgrade applied evidence. Staking custody writes and EVM execution writes fail closed unless the store supports atomic batches. Reward-policy tuning and full economic audit remain chain-specific integration work.
- Multi-region long-running and chaos tests are supported by network/longrun and chaos-plan commands. `release collect-evidence` can collect RPC-backed longrun, ops, P2P, state-sync/light-client, and snapshot evidence from actual validators, but release evidence still must come from the target host topology rather than a local dry run.
- Governance upgrade plans can be approved through governance module state, persisted by the store, loaded by the runtime at the target height, and halted on rollback-required execution records. Chain teams still need to document their upgrade authority, rollback operators, and release approval policy for each launch.
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
  --bls-audit-sha256 <sha256> \
  --vrf-audit dist/vrf-audit.pdf \
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
