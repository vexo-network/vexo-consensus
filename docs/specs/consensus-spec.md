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

The commit rule validates the full height/hash binding before accepting the three-chain decision: `B3.height = B2.height + 1`, `B2.height = B1.height + 1`, `B3`'s block QC certifies `B2`, and `B2`'s parent QC certifies `B1`. A QC with a matching hash but the wrong certified height is invalid for finality.

## Execution Commit Policy

Node execution is configurable and explicit:

- `execution_commit = "qc"`: execute and persist a QC-certified block as soon as its commit certificate verifies.
- `execution_commit = "finalized"`: execute and persist only the finalized ancestor selected by the three-chain rule.

In both modes, finality proofs remain tied to the consensus finality rule and the validator set at the proof height. `block_committed` means the application state boundary has committed according to the configured policy; it does not by itself redefine light-client finality.

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

Current invalid-proposal evidence is a runtime-verifiable mismatch proof format. DA mismatch and missing-data evidence are self-contained. Validator-set-hash, app-hash, timestamp, and transaction-validity evidence deliberately fail closed unless the runtime injects the height-specific verification context. Validator-set-hash evidence is checked against the height-specific validator set. App-hash evidence can be applied when the node has the committed state record for the evidence height and can inject that expected app hash into the slashing verifier. Timestamp evidence uses the committed block header at the evidence height when retained; otherwise it fails closed instead of guessing. Transaction-validity evidence is fail-closed: it must bind the proposed transaction-set hash and an independently computed deterministic transaction-result hash before slashing can apply. Context-bound invalid-proposal evidence may also carry a native Merkle query proof; the verifier checks chain ID, evidence height, expected state root, namespace, key, existence, and value before the penalty path can continue.

The consensus/data-availability hash is the Merkle root of canonical length-prefixed transaction chunks.
Nodes can verify individual chunk samples against that root and recover bounded missing chunks with the
built-in GF(256) Reed-Solomon-style parity backend. The proof records the configured data-shard and
parity-shard counts, and recovery can tolerate up to `parity_shards` missing data chunks per shard group.
The data-availability package also exposes deterministic one-dimensional and two-dimensional sample
planning plus sample-report verification against chunk inclusion proofs. The 2D planner samples
deterministic row and column crosses over the configured chunk grid, so operators can tune large-network
sampling policy without replacing the commitment or proof format.

The native query-proof binding is intentionally narrow: it proves namespace/key membership or non-membership against a retained Vexo state root. Chains can still add richer reason-specific proof decoders for application-specific claims, but the default invalid-proposal path no longer accepts context-bound hash claims without optional state-root proof verification when the runtime requires it.
