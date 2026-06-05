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
- finality conflict evidence

Evidence must be validated before application, persisted, and deduplicated by stable evidence key.

Finality conflict evidence contains two independently valid finality proofs for the same height and validator set but different finalized block hashes. The verifier checks both QCs with the height-specific validator set, intersects both signer sets, and only slashes validators that are present in the overlapping accountable signer set.

Invalid proposal evidence is reason-specific:

- data-availability mismatch proofs must show a chunk-root commitment mismatch.
- missing-data proofs must show missing data for a non-empty proposal.
- validator-set-hash and app-hash proofs must bind `actual_hash` to the proposed header field and include a different expected hash.
- timestamp proofs must bind `actual_time_unix_nano` to the proposed header timestamp when the header timestamp is present.
- tx-validity proofs must include the deterministic expected transaction-result hash, the proposed transaction-set hash, and a verifier message.
- proposer-signature proofs still pass normal proposal envelope checks, then fail domain-separated proposer signature verification during slashing validation.

Current invalid-proposal evidence is a runtime-verifiable mismatch proof format. Validator-set-hash evidence is checked against the height-specific validator set. App-hash evidence can be applied when the node has the committed state record for the evidence height and can inject that expected app hash into the slashing verifier. Timestamp evidence likewise requires an explicit expected timestamp context. Transaction-validity evidence is fail-closed: it must bind the proposed transaction-set hash and an independently computed deterministic transaction-result hash before slashing can apply.

The consensus/data-availability hash is the Merkle root of canonical length-prefixed transaction chunks.
Nodes can verify individual chunk samples against that root and can recover one missing chunk per parity group
from XOR parity chunks. This is deterministic recovery for bounded missing data; chains that need stronger
availability sampling against many missing chunks should add a Reed-Solomon or 2D erasure-coding backend.

It is not a full light-client Merkle/state-proof system. Chains that require slashing from independently verifiable state membership or non-membership proofs must add reason-specific Merkle/state proof verification before applying penalties for those reasons.
