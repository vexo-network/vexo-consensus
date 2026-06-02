# Vexo Consensus Protocol

This document defines the production-facing consensus model implemented by `vexo-consensus`.

## Model

- Vexo uses a HotStuff-style BFT core with proposal, vote, quorum certificate, timeout certificate, and three-chain finality.
- A block is safe to vote for only when it extends the locked QC or carries a justify QC at least as new as the lock.
- A block becomes finalized through the three-chain rule: if block `B3` has a QC for parent `B2`, and `B2` has a QC for grandparent `B1`, then `B1` is finalized.
- Accountable safety depends on evidence for conflicting votes or conflicting timeout votes at the same height, round, and validator.

## Safety Invariants

- A validator must not sign two different proposals, votes, or timeout votes for the same `(chain_id, height, round, type)`.
- A QC must contain unique known signers only.
- A QC voting-power field, when present, must equal the voting power recomputed from the validator set.
- A finality proof must bind the block header, block hash, QC height, validator-set height, and validator-set hash.
- A light client verifies a proof with the validator set for the proof height; using any other set must fail by validator-set hash mismatch.

## Finality Verification

- `finality.Proof.Header` is the finalized block header being verified.
- `finality.Proof.ValidatorSetHeight` identifies the height whose validator set is used for verification.
- `finality.Proof.ValidatorSetHash` and `Header.ValidatorSetHash` must both match the verifier's validator set hash.
- `QuorumCert.Height` must equal `Header.Height`, and `QuorumCert.BlockHash` must equal the canonical header hash.
- Ed25519 finality uses ordered multisignature concatenation, not cryptographic aggregation. The signer bitmap order defines the public-key/signature order.
- BLS finality must only be enabled through an audited production adapter with domain separation, public-key validation, proof-of-possession or equivalent rogue-key defense, and subgroup checks.

## Crypto Backends

- `deterministic` is development-only and must not pass production validation.
- `ed25519` is valid for production-style testing and launch preparation, but its finality proof is a multisignature model.
- `bls` is intentionally unavailable until a production adapter is linked and audited.
- All consensus signatures use explicit domain separation for proposal, vote, timeout vote, and finality messages.

## Remote Signer Policy

- Remote signers receive the raw message plus a policy tuple: `chain_id`, `height`, `round`, `type`, and `domain`.
- A production signer must keep its own double-sign guard and reject conflicting messages for the same policy tuple.
- Local node-side checks are not enough; KMS/HSM policy must protect against compromised or restarted node processes.

## Recovery Semantics

- The last safe recovery height is the latest height where committed block metadata and app state are mutually consistent.
- If a block is executed but its state is not persisted, recovery uses the last persisted state height.
- If state exists without matching block metadata, recovery reports a mismatch and uses the lower consistent height.
- WAL replay is allowed only to rebuild consensus-local proposal/vote context; it must not create finalized state without persisted block and state records.
