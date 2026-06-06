# Consensus Protocol Overview

This page is the high-level entry point for Vexo consensus documentation. For a broader documentation map, see [Documentation](./README.md).

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

The implementation binds the three-chain decision to explicit block, parent, and grandparent heights. The block QC must certify the parent height/hash, and the parent QC must certify the grandparent height/hash; synthetic or height-skipped QC chains are rejected before a finality decision is recorded.

## Execution Terms

Vexo uses these terms consistently:

- **QC certified**: a block has enough votes to form a quorum certificate.
- **Finalized**: the HotStuff three-chain rule finalizes an ancestor block.
- **Executed**: the application has run `FinalizeBlock` for a block.
- **State committed**: application KV writes, block record, state record, and module state roots have been durably committed.

The node execution path uses two separate boundaries:

- **Execution commit boundary**: a QC-certified block can be executed and atomically persisted as app writes + block record + state record + state roots.
- **Consensus finality boundary**: the three-chain rule finalizes an ancestor and is the only source for light-client finality proofs.

`consensus_config.json` exposes this choice through `execution_commit`. The default `qc` mode favors low-latency execution of QC-certified blocks. The stricter `finalized` mode only executes the ancestor selected by the three-chain finality rule. Operators and SDK users should treat `block_committed` logs as state-commit events for the configured execution boundary, not as proof that the same height is three-chain finalized unless `execution_commit` is `finalized`. Finality proofs describe consensus finality at their validator-set height.

## Safety Boundary

Safety depends on:

- less than one-third Byzantine voting power
- domain-separated proposal, vote, timeout-vote, and finality signatures
- validator-set hash binding at the relevant proof height
- unique known signers in QCs and finality proofs
- accountable evidence for validator equivocation
- rejection of conflicting commit decisions at the same finalized height

## Crypto Boundary

- `deterministic` is test-only and fails network safety validation.
- `ed25519` is supported for public-network testing and launch preparation.
- `bls` requires proof-of-possession or equivalent rogue-key defense, subgroup checks, public-key validation, dependency audit evidence, and release-gate evidence. The built-in CIRCL adapter can satisfy the runtime interface; operators still need deployment audit evidence.
- Network safety validation requires VRF adapter metadata for VRF committee selection. The built-in ECVRF adapter can satisfy the runtime interface; deterministic VRF remains test-only and should not be used for value-bearing networks.

## Operational Boundary

The code includes production-oriented checks, but public deployments still require:

- strict config audit for every validator home
- release-gate evidence
- external security review
- multi-host long-run and chaos evidence
- signer/KMS policy evidence
- chain-specific economic and governance policy review

See [Security Audit Readiness](./security/audit-readiness.md) and [Release Pipeline](./release/release-pipeline.md) before treating a release as production-ready.
