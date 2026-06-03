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

Implementations may use the in-memory registry for tests or the store-backed registry for durable chains.
Both registries serve historical lookups from the latest snapshot at or below the requested height.
The in-memory registry records rotation events for joins, leaves, and voting-power changes; the
store-backed registry persists sorted validator-set snapshots by height.

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

Durable keepers persist evidence lifecycle, penalty receipts, jail-until heights, and unbonding release heights. Consensus slashing
first validates evidence, then records it, applies a stake-aware penalty, and finally writes the resulting
validator voting-power update through the registry.

Lifecycle policy includes:

- evidence max age
- appeal window
- unbonding delay

Appealed or expired evidence must not be applied.

## Slashing

Slashing records:

- evidence metadata
- penalty policy
- previous voting power
- remaining voting power
- jail duration

Incorrect slashing is a critical chain-trust failure, so slashing policy includes appeal and expiration handling.

## Jail and Unbonding

- Jailed validators are tracked by validator ID and jail-until height.
- Unbonding is blocked until the recorded release height.
- Production stake accounting should be durable and auditable.
