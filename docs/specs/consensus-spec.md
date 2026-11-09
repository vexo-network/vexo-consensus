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
- A block header's `AppHash` commits the state after executing block `StateHeight` (the genesis state when that is `0`), the latest block the chain proves committed to every validator that verifies the header (see the header heights in the finality proof format): it is `HeaderAppHash` of the application's app hash and the root of the `validator_registry` namespace after that block, so the validator sets of past heights, which evidence is judged against, are committed with the application state. The proposer stamps it, a validator votes only for a proposal whose `AppHash` matches its own state at that height, and a node refuses to execute a certified block whose `AppHash` disagrees with its state. A certificate therefore vouches for the replicated state of that height, which is what a state-synced node checks a snapshot against.
- A validator that refuses a signed proposal because its header names another `AppHash` reports the proposer with invalid-proposal evidence of reason `app_hash`. The slashing module records, per state height, the `AppHash` each executed block header committed, for as long as evidence about that block is punishable, and judges the evidence against that certified value, so every node decides it alike and a node whose own state diverged cannot frame an honest proposer.
- A header's `NextValidatorSetHash` names the validator set the registry holds after block `StateHeight`, which certifies the header's children: a child's `ValidatorSetHeight` is its parent's `StateHeight + 1` and its `ValidatorSetHash` must equal its parent's `NextValidatorSetHash`. A validator votes only for a proposal whose next set is the one its own registry holds.
- A node holds no state below the snapshot it was restored from or below its retention, yet a header keeps naming the state of its validator set until the chain proves a commit, so a header may name such a state. For it the node uses the `AppHash` the slashing module recorded as certified for that state: it votes on, and executes, those headers as the nodes holding the state do.

## Block Time

- A block's `TimeUnixNano` is strictly greater than its parent's; the first block's is positive. A proposal that breaks this is invalid however well signed.
- The proposer stamps its own clock, raised to one nanosecond after the parent's time when its clock is behind.
- A validator does not vote for a block whose time is more than `MaxBlockTimeDrift` (2s) ahead of its own clock. The proposal stays cached; the relaying peer is not at fault.
- A certified block's time is therefore within the drift of an honest validator's clock, and a header's time bounds the time of every block below it. Light clients measure their trusting period against it, and the EVM `TIMESTAMP` reads it.

## Compact Proposals

A proposer broadcasts a proposal with transactions as a `compact_proposal`: the signed proposal without its transactions, and the 8-byte short id of each transaction in block order (the first 8 bytes of its hash). Transactions reach validators through gossip first, so a validator rebuilds the block from its mempool, and from a bounded overflow of gossiped transactions its full mempool turned away. The block format, its hash, and what validators sign are unchanged.

- A rebuilt block is judged only when its transaction root matches the header, so a short-id collision cannot make a validator judge another block.
- A validator missing some transactions sends the proposer a `proposal_request` naming their positions, and the proposer answers with `proposal_txs`. A validator missing more than half, or whose rebuilt block fails the transaction root, requests the whole proposal, which the proposer answers with the full `proposal`.
- A served transaction whose short id is not the one at its position counts against the serving peer. A compact proposal a validator cannot rebuild is not the sender's fault.
- A node serves a proposal it holds at most twice to each peer, so a peer cannot make it upload a block repeatedly. It holds the 32 most recent proposals.

The leader's upload per validator then scales with 8 bytes per transaction instead of the transaction size.

## Finality Rule

Vexo uses a two-chain commit rule over global views (`consensus.TwoChainCommitRule`):

- Block `B` is finalized when `QC(B)` formed in view `b`, its child `C` (`C.PreviousBlockHash = hash(B)`, `C.height = B.height + 1`) carries `QC(B)` as its justify certificate, and `QC(C)` formed in the next view `b + 1`. Every ancestor of a final block is final.
- Finalized block hash is recorded as `last_finalized`.
- Conflicting finalization must be impossible unless at least one validator equivocates and produces accountable evidence.

The commit rule validates the full height/hash binding before accepting the decision. A QC with a matching hash but the wrong certified height, or a child QC that is not from the view directly after the block's, is invalid for finality.

## Execution Commit Policy

Node execution is configurable and explicit:

- `execution_commit = "qc"`: execute and persist a QC-certified block as soon as its commit certificate verifies.
- `execution_commit = "finalized"`: execute and persist only the finalized ancestor selected by the two-chain commit rule.

In both modes, finality proofs remain tied to the consensus finality rule and the validator set at the proof height. `block_committed` means the application state boundary has committed according to the configured policy; it does not by itself redefine light-client finality.

## Liveness Assumptions

- Eventually, network delay becomes bounded long enough for quorum communication.
- Less than one-third of voting power is Byzantine.
- Timeout certificates advance rounds when proposals or votes stall.
- Peer dial, TLS, auth, and signed-handshake timeouts are long enough for the target network latency envelope.

## Adaptive Round Timeout Policy

When `adaptive_round_timeout_enabled = true`, the node computes a bounded operational timeout from the configured base budget, rolling p95 proposal/vote/commit processing latency, progress or timeout outcome, and active-peer deficit. A timeout grows the current budget by 1.5x, successful progress decays it by 0.8x, observed processing latency contributes a 3x safety budget, and the result is clamped between the base and 8x the base.

This policy changes only when the node attempts timeout recovery. It does not change proposer selection, proposal validity, the safe-vote predicate, quorum power, QC verification, or the three-chain finality rule. Idle time and local execution/storage errors do not consume rounds.

## Recovery Finality Gate

When `recovery_finality_gate_enabled = true`, the finalized execution path compares durable application-state and block-index heights. If both exist and differ, the node computes the safe recovery height as their minimum and defers finalized application commits above that boundary. The gate is a local persistence restriction; it is not an extra vote phase or a network-wide certificate.

Operators can observe the enabled policy, current adaptive timeout, processing latency, and recovery deferrals through `/v1/metrics` and `/metrics/text`.

## Deterministic Transaction Ordering

Proposal construction derives a height-specific salt from `chain_id` and height. It groups transactions with signer/nonce metadata into per-signer chains, sorts each chain by ascending nonce, and deterministically merges chain heads by salted transaction hash. Proposal validation recomputes the same order.

The rule removes local mempool arrival order from an identical candidate set and preserves nonce dependencies. It does not guarantee first-seen fairness, censorship resistance, confidentiality, or a formal order-fairness property.

## Empty Blocks and Round Recovery

`create_empty_blocks = false` changes only when proposals are created; it does not weaken safety. A validator should not propose an empty block just to advance height. If every mempool is empty, the latest height can remain stable and that is normal idle behavior.

When a valid transaction enters the mempool, the local consensus loop proposes only when this validator is the deterministic proposer for the current `(height, round)`. A non-proposer never jumps to another round locally. Rounds advance only through a valid timeout certificate or a certified finality transition, and a failed block execution or storage step is not treated as a timeout. This prevents multiple validators from proposing different blocks for the same `(height, round)` and prevents a failed transaction from consuming rounds.

- idle consensus with no pending work does not consume rounds;
- a proposer change requires the normal timeout-certificate or finality path;
- a pending proposal is never replaced by an uncertified local recovery proposal;
- state commitment still requires the normal QC/finality path.

Operators should therefore read `latest_height = 0` with an empty mempool as idle, not failed. Read `latest_height` stuck with non-empty mempool, rising round timeouts, or failed peer handshakes as a liveness incident.

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

- `da_mismatch` proofs must show a chunk-root commitment mismatch.
- `missing_data` proofs must show missing data for a non-empty proposal.
- `validator_set_hash`, `next_validator_set_hash`, and `app_hash` proofs must bind `actual_hash` to the proposed header field and include a different expected hash.
- `parent_linkage` proofs carry the parent header. The parent must hash to the proposal's `PreviousBlockHash`, and the proposal must break a rule the parent alone fixes: a height other than the parent's plus one, a time not after the parent's, or a validator set or state height other than the ones the parent fixes for its children. For the first block no parent is carried, and the proof shows a non-zero previous hash, a non-positive time, or a set or state height other than the genesis ones.
- Every proof is signed by the proposer under the proposal domain, so only a proposal the proposer signed is evidence.

Invalid-proposal evidence proves a proposal's header carries another hash than the one its height requires, and every reason is decidable from data every node holds alike. `da_mismatch` and `missing_data` evidence is decided from the proposal's own transactions. `validator_set_hash` evidence is judged against the validator set of the registry height the header names, and `next_validator_set_hash` evidence against the set after the state the header names; a header naming a height of zero or above its own is refused. An honest proposer names the sets its replicated state holds, so the evidence cannot frame it. `parent_linkage` evidence is decided from the two headers the proof carries, which the hash link binds. `app_hash` evidence is judged against the app hash certified blocks committed for the state the proposal names, which replicated state records (see above); without that record the evidence is refused. Faults that would need other chain data, a local clock, or a re-execution, such as a block time ahead of the clock or an invalid transaction, are not evidence: nodes could judge them differently, in particular one that pruned the data, and the chain refuses a proof that names them.

The consensus/data-availability hash is the Merkle root of canonical length-prefixed transaction chunks.
Nodes can verify individual chunk samples against that root and recover bounded missing chunks with the
built-in GF(256) Reed-Solomon-style parity backend. The proof records the configured data-shard and
parity-shard counts, and recovery can tolerate up to `parity_shards` missing data chunks per shard group.
The data-availability package also exposes deterministic one-dimensional and two-dimensional sample
planning plus sample-report verification against chunk inclusion proofs. The 2D planner samples
deterministic row and column crosses over the configured chunk grid, so operators can tune large-network
sampling policy without replacing the commitment or proof format.
