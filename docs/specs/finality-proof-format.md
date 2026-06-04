# Finality Proof Format

## Scope

This spec defines the proof format verified by light clients and full nodes.

## Proof Fields

`finality.Proof` contains:

- `Header`: finalized block header
- `QuorumCert`: QC for the finalized header hash
- `ValidatorSetHeight`: height whose validator set is used for verification
- `ValidatorSetHash`: hash of the validator set used for verification

## Header Fields

The header hash covers:

- chain ID
- height
- timestamp
- previous block hash
- app hash
- validator set hash
- consensus/data availability hash

## Quorum Certificate Fields

- height
- round
- block hash
- signer bitmap
- aggregate or multisignature
- voting power

## Verification Algorithm

1. Reject proofs without an explicit `ValidatorSetHeight`.
2. Load validator set for `ValidatorSetHeight`.
3. Verify `Proof.ValidatorSetHeight == Header.Height`.
4. Verify `Proof.ValidatorSetHash == loaded_set.Hash()`.
5. Verify `Header.ValidatorSetHash == loaded_set.Hash()`.
6. Verify `QuorumCert.Height == Header.Height`.
7. Verify `QuorumCert.BlockHash == HeaderHash(Header)`.
8. Parse signer bitmap and reject unknown or duplicate signers.
9. Recompute signer voting power and require quorum.
10. If QC voting power is present, require it to match recomputed voting power.
11. Verify aggregate/multisignature over finality sign bytes.

## Ed25519 Model

Ed25519 uses ordered multisignature concatenation. It is not cryptographic aggregation.

## BLS Model

BLS requires an audited adapter with:

- domain separation
- public key validation
- subgroup checks
- rogue-key defense such as proof of possession
- deterministic serialization

The runtime rejects BLS until an adapter satisfying the `BLSAdapter` contract is registered. This keeps aggregate-finality verification explicit: the verifier must know the validator-set public keys at the proof height, validate those keys, verify proof-of-possession or equivalent rogue-key defense, and verify the aggregate signature under the `vexo.finality.proof.v1` domain. The production wrapper rejects aggregate verification for public keys that were not admitted through validated BLS credentials.

The framework includes a CIRCL-backed BLS12-381 adapter and still allows custom adapter registration. Operators must keep adapter audit evidence, dependency audit evidence, proof-of-possession metadata, and release artifacts together for value-bearing deployments.
