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
- production configs reject deterministic crypto
- release artifacts are reproducible enough for independent verification

## Security Assumptions

- Less than one-third of voting power is Byzantine for safety and liveness under partial synchrony.
- Validator private keys or remote signer policies are not all compromised.
- Production deployments use Ed25519 or an audited BLS adapter, never deterministic crypto.
- Remote signer/KMS enforces its own height/round/type/domain double-sign guard.
- Operators configure RPC admin tokens, P2P auth, request limits, and peer scoring.
- Storage backend preserves block/state/evidence durability or clearly reports recovery mismatch.

## Known Limitations

- BLS backend is intentionally unavailable until an audited adapter is linked.
- Ed25519 finality is ordered multisignature concatenation, not cryptographic aggregation.
- Validator registry and slashing keeper have durable store-backed implementations; full production stake custody, unbonding queues, and reward accounting remain chain-specific integration work.
- Multi-region long-running tests are planned through localnet/longrun plans but must be executed on real machines.
- Governance upgrade execution is scaffolded; production chains need durable upgrade state and operator runbooks.
- The current RPC API is unversioned but documented as v1 semantics.

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
go run ./cmd/vexod ops thresholds --json
go run ./cmd/vexod upgrade plan --json --name audit-upgrade --height 100
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
