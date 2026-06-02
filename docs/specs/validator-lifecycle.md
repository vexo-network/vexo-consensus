# Validator Lifecycle

## Scope

This spec defines validator admission, set updates, evidence handling, slashing, jailing, and unbonding expectations.

## Admission

Validator admission can be:

- permissionless with minimum stake
- restricted by configuration
- capped by maximum validator count

Candidates must satisfy the configured admission policy before joining.

## Validator Set

Each height has a validator set hash. Consensus proposals and finality proofs bind to this hash.

Validator updates are applied through app/runtime output and become effective for the next height.

Implementations may use the in-memory registry for tests or the store-backed registry for durable chains.
The store-backed registry persists sorted validator-set snapshots by height and serves historical lookups
from the latest snapshot at or below the requested height.

## Rotation

Committee/proposer rotation is height and round dependent. Deterministic rotation is the default; VRF-backed rotation can be configured.

## Evidence Lifecycle

Evidence states:

- submitted
- applied
- appealed
- expired

Evidence must be validated, deduplicated, persisted, and only then applied.

Durable keepers persist evidence lifecycle, penalty receipts, and jail-until heights. Consensus slashing
first validates evidence, then records it, applies a stake-aware penalty, and finally writes the resulting
validator voting-power update through the registry.

## Slashing

Slashing records:

- evidence metadata
- penalty policy
- previous voting power
- remaining voting power
- jail duration

Incorrect slashing is a critical chain-trust failure, so production deployments need appeal and expiration policy.

## Jail and Unbonding

- Jailed validators should be excluded or power-reduced according to policy.
- Unbonding should prevent immediate stake exit after accountable evidence windows.
- Production stake accounting should be durable and auditable.
