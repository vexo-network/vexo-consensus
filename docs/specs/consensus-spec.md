# Consensus Spec

## Scope

This spec defines the consensus state machine, safety rules, liveness assumptions, and evidence surfaces for Vexo.

## Roles

- **Validator**: proposes blocks, votes, timeout-votes, and can be slashed for accountable faults.
- **Proposer**: validator selected for `(height, round)` by deterministic proposer rotation.
- **Full node**: verifies blocks, QCs, validator-set hashes, app state, and evidence.
- **Light client**: verifies finality proofs against the correct validator set for the proof height.

## State

Consensus state is keyed by:

- `chain_id`
- `height`
- `round`
- `phase`: propose, vote, commit
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`

## Message Types

- `Proposal`: block, round, proposer, justify QC, proposer signature.
- `Vote`: height, round, block hash, validator ID, validator signature.
- `TimeoutVote`: height, round, high QC, validator ID, validator signature.
- `QuorumCert`: height, round, block hash, signer bitmap, aggregate/multisig, voting power.
- `TimeoutCert`: height, round, high QC, signer bitmap, aggregate/multisig.

## Safety Rules

- A validator must not vote for two different block hashes at the same `(height, round)`.
- A validator must not timeout-vote for two different high QCs at the same `(height, round)`.
- A proposal is safe if it extends `locked_qc` or carries a justify QC at least as new as `locked_qc`.
- A vote is safe only for a known proposal that passes the same lock rule.
- A QC is valid only when signer voting power reaches `>= 2/3` of the validator set.
- QCs must contain unique known signers and the recomputed voting power must match the QC voting power when present.

## Finality Rule

Vexo uses three-chain finality:

- If block `B3` has QC for `B2`, and `B2` has QC for `B1`, then `B1` is finalized.
- Finalized block hash is recorded as `last_finalized`.
- Conflicting finalization must be impossible unless at least one validator equivocates and produces accountable evidence.

## Liveness Assumptions

- Eventually, network delay becomes bounded long enough for quorum communication.
- Less than one-third of voting power is Byzantine.
- Timeout certificates advance rounds when proposals or votes stall.

## Evidence

Consensus evidence includes:

- conflicting votes
- conflicting timeout votes
- invalid proposal evidence
- unavailable data evidence

Evidence must be validated before application, persisted, and deduplicated by stable evidence key.

Invalid proposal evidence is reason-specific:

- data-availability mismatch proofs must show a commitment mismatch.
- missing-data proofs must show missing data for a non-empty proposal.
- validator-set-hash and app-hash proofs must bind `actual_hash` to the proposed header field and include a different expected hash.
- timestamp proofs must bind `actual_time_unix_nano` to the proposed header timestamp when the header timestamp is present.
- tx-validity proofs must include deterministic expected/actual result hashes plus a verifier message.
- proposer-signature proofs still pass normal proposal envelope checks, then fail domain-separated proposer signature verification during slashing validation.
