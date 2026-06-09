# Finality Proof Format

## Scope

This spec defines the proof format verified by light clients and full nodes.

## Proof Fields

`finality.Proof` contains:

- `Header`: finalized block header
- `BlockHash`: finalized block hash
- `QuorumCert`: QC for the finalized header hash
- `CommitChain`: descendant proposal links that prove the HotStuff-style 3-chain commit decision
- `ValidatorSetHeight`: height whose validator set is used for verification
- `ValidatorSetHash`: hash of the validator set used for verification

Full nodes expose the latest known finality proof at `/v1/finality/latest` and height-specific proofs at `/v1/finality/{height}`. Responses include `strict: true` when the proof carries the descendant `CommitChain` required for external 3-chain verification. These proofs are consensus-finality artifacts, while `/v1/status.latest_height` reports application state commit height.

## Header Fields

The header hash covers:

- chain ID
- height
- timestamp
- previous block hash
- app hash
- validator set hash
- consensus/data availability chunk root

## Quorum Certificate Fields

- height
- round
- block hash
- signer bitmap
- aggregate or multisignature
- voting power

## Commit Chain Fields

External commit gossip is intentionally stricter than a single block plus QC. A commit message must carry a finality proof with at least two descendant links:

- link 1: child block whose `JustifyQC` certifies the finalized block
- link 2: grandchild block whose `JustifyQC` certifies link 1

Each `CommitChain` link contains:

- descendant header
- descendant block hash
- QC that certifies the previous block in the chain

This lets a peer that missed earlier proposals verify the same 3-chain finality decision before executing the committed block. Legacy block+QC-only commit gossip is treated as non-punishable but insufficient; the receiver does not mutate local state from it.

## Verification Algorithm

1. Reject proofs without an explicit `ValidatorSetHeight`.
2. Load validator set for `ValidatorSetHeight`.
3. Verify `Proof.ValidatorSetHeight <= Header.Height`; the loaded validator set hash must still match the proof and header hashes.
4. Verify `Proof.ValidatorSetHash == loaded_set.Hash()`.
5. Verify `Header.ValidatorSetHash == loaded_set.Hash()`.
6. Verify `QuorumCert.Height == Header.Height`.
7. Verify `QuorumCert.BlockHash == HeaderHash(Header)`.
8. Parse signer bitmap and reject unknown or duplicate signers.
9. Recompute signer voting power and require quorum.
10. If QC voting power is present, require it to match recomputed voting power.
11. Verify aggregate/multisignature over finality sign bytes.
12. For external/light-client strict verification, require `CommitChain` to contain at least two links. Compatibility-only verification may accept a bare block QC, but that proof is not sufficient for 3-chain finality.
13. Verify each link extends the previous block hash by exactly one height.
14. Verify each link QC signs the previous block hash under the same validator-set hash and consensus vote domain.

## Accountable Safety Detection

Light clients and operators should retain the first valid finality proof seen for each height. If another valid proof for the same height carries a different block hash, the node can deterministically derive accountable safety evidence:

1. Verify both finality proofs with the validator set at `ValidatorSetHeight`.
2. Require both proofs to use the same height and validator-set hash.
3. Compare the finalized block hashes.
4. If hashes differ, intersect both QC signer sets.
5. Sum the overlapping validator voting power from the height-specific validator set.
6. Treat the overlapping signers as double-finality signers.

`finality.AttackDetector` implements this flow in-process. The CLI exposes the same check for incident response:

```bash
vexod proof detect-finality-conflict \
  --first first-finality-proof.json \
  --second second-finality-proof.json \
  --validator-set validator-set-at-height.json
```

If `--validator-set` is omitted, the command loads validators from the resolved genesis file. For validator-set changes after genesis, pass the exact validator set for the proof height.

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
