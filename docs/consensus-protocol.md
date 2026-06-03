# Consensus Protocol Overview

This page is the high-level entry point for Vexo consensus documentation.

For normative details, use the spec files:

- [Consensus Spec](./specs/consensus-spec.md)
- [Finality Proof Format](./specs/finality-proof-format.md)
- [Validator Lifecycle](./specs/validator-lifecycle.md)
- [Storage Schema](./specs/storage-schema.md)
- [Networking Spec](./specs/networking-spec.md)
- [Transaction Format](./specs/tx-format.md)

## Model

Vexo uses a HotStuff-style BFT core with proposals, votes, quorum certificates, timeout certificates, locked-QC safety, and three-chain finality.

A block is safe to vote for only when it extends the locked QC or carries a justify QC at least as new as the lock. A block becomes finalized when the three-chain rule proves a safe parent/grandparent chain extension.

## Safety Boundary

Safety depends on:

- less than one-third Byzantine voting power
- domain-separated proposal, vote, timeout-vote, and finality signatures
- validator-set hash binding at the relevant proof height
- unique known signers in QCs and finality proofs
- accountable evidence for validator equivocation
- rejection of conflicting commit decisions at the same finalized height

## Crypto Boundary

- `deterministic` is development-only and fails production validation.
- `ed25519` is supported for production-style testing and launch preparation.
- `bls` requires an audited adapter, proof-of-possession or equivalent rogue-key defense, subgroup checks, public-key validation, dependency audit evidence, and release-gate evidence.

## Operational Boundary

The code includes production-oriented checks, but public deployments still require:

- strict config audit for every validator home
- release-gate evidence
- external security review
- multi-host long-run and chaos evidence
- signer/KMS policy evidence
- chain-specific economic and governance policy review

See [Security Audit Readiness](./security/audit-readiness.md) and [Release Pipeline](./release/release-pipeline.md) before treating a release as production-ready.
