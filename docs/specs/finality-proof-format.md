# Finality Proof Format

## Scope

This spec defines the proof format verified by light clients and full nodes. A proof shows that a block is final under the 2-chain commit rule (`consensus.TwoChainCommitRule`): block B is final when B has a quorum certificate formed in view `b`, and its child C has a quorum certificate formed in view `b+1`. Views are global and never reset when the height advances.

## Proof Fields

`finality.Proof` contains:

- `Header`: header of the finalized block B
- `BlockHash`: finalized block hash; it must equal `HeaderHash(Header)`
- `QuorumCert`: QC(B), the quorum certificate for the finalized block hash
- `CommitChain`: exactly one link, the child C of B, carrying QC(C)
- `ValidatorSetHeight`: height whose validator set is used for verification; it must lie in `1..Header.Height`
- `ValidatorSetHash`: hash of the validator set used for verification

Full nodes expose the latest known finality proof at `/v1/finality/latest` and height-specific proofs at `/v1/finality/{height}`. Responses include `strict: true` when the proof carries the child link required for external 2-chain verification. These proofs are consensus-finality artifacts, while `/v1/status.latest_height` reports application state commit height.

## Header Fields

The header hash covers:

- chain ID
- height
- timestamp
- previous block hash
- app hash
- validator set hash
- consensus/data availability chunk root
- transaction root

## Quorum Certificate Fields

- height
- round (the view in which the block was proposed)
- block hash
- signer bitmap
- aggregate or multisignature
- voting power

## Commit Chain Fields

External commit gossip is intentionally stricter than a single block plus QC. A commit message must carry a finality proof whose `CommitChain` holds exactly one link: the child block C whose `JustifyQC` certifies the finalized block B. The link contains:

- the header of C
- the hash of C, which must equal `HeaderHash(link.Header)`
- `QuorumCert`: QC(C), the certificate C itself gathered, in the view directly after the view of QC(B)

The certificate C carries for B is the proof's own `QuorumCert`. This lets a peer that missed earlier proposals verify the same 2-chain finality decision before executing the committed block. A proof without the child link only shows that B is certified, not that it is final; the receiver treats block+QC-only commit gossip as insufficient and does not mutate local state from it.

When a node commits a certified block, the block record also stores the QC that certified that block. This lets a restarted node reconstruct the child link from durable block records instead of relying only on in-memory pending proposals. Node-level `/v1/finality/*` queries fail closed when a strict proof cannot be built.

## Verification Algorithm

The structure of the proof is checked before any signature.

1. Require the verifier to hold a signature verifier; without one verification fails closed.
2. Load the validator set. The registry verifier loads the set of `Header.Height`; the set is never selected by the prover-controlled `ValidatorSetHeight`, so a proof cannot name a stale validator set whose keys it holds.
3. Verify `Proof.ValidatorSetHash == loaded_set.Hash()` and `Header.ValidatorSetHash == loaded_set.Hash()`.
4. Verify `0 < Proof.ValidatorSetHeight <= Header.Height`.
5. Verify `Proof.BlockHash == HeaderHash(Header)`. `Header.TxRoot` must already commit to the block transaction list, so this hash is the consensus block hash, not a header-only shortcut.
6. Verify `QuorumCert.Height == Header.Height` and `QuorumCert.BlockHash == Proof.BlockHash`.
7. Require `CommitChain` to hold exactly one link; none is too short and more than one is too long.
8. Verify the link: `link.BlockHash == HeaderHash(link.Header)`, the same chain ID and validator set hash as `Header`, `link.Header.Height == Header.Height + 1`, `link.Header.PreviousBlockHash == Proof.BlockHash`, and `link.QuorumCert` certifies the link's own height and hash.
9. Verify the views are consecutive: `link.QuorumCert.Round == QuorumCert.Round + 1`, without overflow.
10. For QC(B) and QC(C): parse the signer bitmap and reject unknown or duplicate signers, recompute the signer voting power and require quorum, and if the certificate carries a voting power require it to match the recomputed power.
11. Verify the aggregate/multisignature of each certificate over the vote sign bytes (`height`, `round`, `block hash`) in the consensus vote domain.

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

The runtime uses the registered `BLSAdapter` contract and defaults to `blst-bls12381-minpk-v1` for BLS networks. This keeps aggregate-finality verification explicit: the verifier must know the validator-set public keys at the proof height, validate those keys, verify proof-of-possession or equivalent rogue-key defense, and verify the aggregate signature under the finality vote domain. The production wrapper rejects aggregate verification for public keys that were not registered with a verified proof of possession for the validator-set snapshot (`RuntimeSuite.RegisterValidators`). The proof of possession is verified under its own domain-separation tag, distinct from the one used for vote signatures.

The framework includes a `supranational/blst` min-pk adapter for performance-oriented BLS12-381 aggregate signatures. A CIRCL-backed adapter remains available as a reference integration and custom adapter registration remains supported. Operators must keep adapter audit evidence, dependency audit evidence, proof-of-possession metadata, and release artifacts together for value-bearing deployments.
