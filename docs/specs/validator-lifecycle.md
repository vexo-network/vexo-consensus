# Validator Lifecycle

## Scope

This spec defines validator admission, set updates, evidence handling, slashing, jailing, and unbonding expectations.

## Admission

Validator admission can be:

- permissionless with minimum stake
- restricted by configuration
- capped by maximum validator count
- optionally requiring a non-empty validator public key

Candidates must satisfy the configured admission policy before joining.

## Validator Set

Each height has a validator set hash. Consensus proposals and finality proofs bind to this hash.

Validator updates are applied through app/runtime output and become effective for the next height.

Validator records use three address namespaces derived from the validator key:

- `vexovaloper...` operator address in the validator record `address` field.
- `vexovalcons...` consensus address in validator metadata.
- `vexo...` account address in validator metadata for account-level transactions.

Implementations may use the in-memory registry for tests or the store-backed registry for durable chains.
Both registries serve historical lookups from the latest snapshot at or below the requested height.
The in-memory registry records rotation events for joins, leaves, and voting-power changes. The
store-backed registry persists sorted validator-set snapshots by height and persists the matching
rotation-event audit log. Staged app-driven validator updates commit the future set and event log in
the same backend batch as the block/state metadata, so restart recovery can reconstruct both the
height-versioned validator set and the operator-facing rotation history.

## Rotation

Committee/proposer rotation is height and round dependent. Deterministic rotation is the default; VRF-backed rotation can be configured.

Runtime validator updates produced while executing height `H` are written for height `H + 1`. This avoids
mid-block validator-set ambiguity and keeps finality proofs bound to a single validator set per height.

## Evidence Lifecycle

Evidence states:

- submitted
- applied
- appealed
- expired

Evidence must be validated, deduplicated, persisted, and only then applied.

Durable keepers persist evidence lifecycle, penalty receipts, jail-until heights, unbonding release heights, and unbonding custody balances. Consensus slashing
first validates evidence, then records it, applies a stake-aware penalty, and finally writes the resulting
validator voting-power update through the registry.

When the staking module is enabled, the node runtime also applies the same penalty receipt to staking state.
Delegations to the slashed validator are reduced proportionally, the staking validator-power key is updated to
match the penalty receipt, and matured undelegations are returned only through `staking tx withdraw-unbonded`
after the configured unbonding delay. Delegation, undelegation, reward claiming, unbonding withdrawal, fee
distribution, and slashing updates require an atomic batch-capable store so custody and validator power cannot
diverge on partial write failure.
the remaining power, and an evidence-derived slash marker makes restart reconciliation idempotent.

Store-backed keepers distinguish missing records from corrupt or failed reads. Missing evidence, receipts, jail state, or unbonding state can be treated as absent; corrupt JSON and storage read errors must abort startup, reconciliation, or penalty execution instead of silently resetting state.

Lifecycle policy includes:

- evidence max age
- appeal window
- unbonding delay

Appealed or expired evidence must not be applied.

## Slashing

Slashing records:

- evidence metadata
- evidence proof type, including vote conflicts, timeout conflicts, invalid proposals, unavailable data, and finality conflicts
- penalty policy
- previous voting power
- remaining voting power
- jail duration

Incorrect slashing is a critical chain-trust failure, so slashing policy includes appeal and expiration handling.
Penalty application must keep slashing receipts, validator registry power, and staking delegation power aligned.

## Jail and Unbonding

- Jailed validators are tracked by validator ID and jail-until height.
- Unbonding is blocked until the recorded release height.
- Production stake accounting should be durable and auditable.
