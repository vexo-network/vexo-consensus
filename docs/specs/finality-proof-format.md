# Finality Proof Format

## Scope

This spec defines the proof format verified by light clients and full nodes. A proof shows that a block is final under the 2-chain commit rule (`consensus.TwoChainCommitRule`): block B is final when B has a quorum certificate formed in view `b`, and its child C has a quorum certificate formed in view `b+1`. Views are global and never reset when the height advances.

## Proof Fields

`finality.Proof` contains:

- `Header`: header of the finalized block B
- `BlockHash`: finalized block hash; it must equal `HeaderHash(Header)`
- `QuorumCert`: QC(B), the quorum certificate for the finalized block hash
- `CommitChain`: exactly one link, the child C of B, carrying QC(C)
- `ValidatorSetHeight`: registry height of the validator set that certifies B; it must equal `Header.ValidatorSetHeight`
- `ValidatorSetHash`: hash of that validator set; it must equal `Header.ValidatorSetHash`

Full nodes expose the latest known finality proof at `/v1/finality/latest` and height-specific proofs at `/v1/finality/{height}`. Responses include `strict: true` when the proof carries the child link required for external 2-chain verification. These proofs are consensus-finality artifacts, while `/v1/status.latest_height` reports application state commit height.

## Header Fields

The header hash covers:

- chain ID
- height
- timestamp
- previous block hash
- app hash: `HeaderAppHash` of the replicated state after executing block `state_height`
- validator set hash: the set that certifies the block
- next validator set hash: the set that certifies the block's children, the set the registry holds after block `state_height`
- consensus/data availability chunk root
- transaction root
- round, the view the block is proposed in, and justify round, the view of the parent certificate the proposal carries
- validator set height: the registry height of the set that certifies the block
- state height: the height of the executed block whose state the app hash commits

The heights follow from the parent header alone, so every node and light client derives the same values. The first block names the genesis set (`validator_set_height = 1`) and the genesis state (`state_height = 0`). A child names `state_height = parent.height - 1` when the parent was proposed in the view right after its justify certificate (the child's certificate of the parent then commits the grandparent under the 2-chain rule), and the parent's `state_height` otherwise. Its `validator_set_height` is the parent's `state_height + 1`, and its validator set hash must be the parent's next validator set hash. A certified header therefore fixes the set of its children before that set signs anything, and a validator set update applies one commit after the state that produced it.

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
2. Verify `Proof.ValidatorSetHeight == Header.ValidatorSetHeight` and `Proof.ValidatorSetHash == Header.ValidatorSetHash`. The header heights must be ones a block of that height can name: the first block names set height `1` and state height `0`; a later block names a state height of at most `Header.Height - 2` and a set height of at most its state height plus one.
3. Load the validator set. The registry verifier loads the set of `Header.ValidatorSetHeight`, which the certified header hash covers: a prover cannot pick another height without changing the block hash the certificate signs.
4. Verify `Header.ValidatorSetHash == loaded_set.Hash()`.
5. Verify `Proof.BlockHash == HeaderHash(Header)`. `Header.TxRoot` must already commit to the block transaction list, so this hash is the consensus block hash, not a header-only shortcut.
6. Verify `QuorumCert.Height == Header.Height` and `QuorumCert.BlockHash == Proof.BlockHash`.
7. Require `CommitChain` to hold exactly one link; none is too short and more than one is too long.
8. Verify the link: `link.BlockHash == HeaderHash(link.Header)`, the same chain ID as `Header`, `link.Header.Height == Header.Height + 1`, `link.Header.PreviousBlockHash == Proof.BlockHash`, the set and state heights the parent rule above fixes for the child, `link.Header.ValidatorSetHash == Header.NextValidatorSetHash`, and `link.QuorumCert` certifies the link's own height and hash. QC(C) is verified with the set of `link.Header.ValidatorSetHeight`.
9. Verify the views are consecutive: `link.QuorumCert.Round == QuorumCert.Round + 1`, without overflow.
10. For QC(B) and QC(C): parse the signer bitmap and reject unknown or duplicate signers, recompute the signer voting power and require quorum, and if the certificate carries a voting power require it to match the recomputed power.
11. Verify the aggregate/multisignature of each certificate over the vote sign bytes (`height`, `round`, `block hash`) in the consensus vote domain.

## Light Client Verification

`finality.LightClient` moves trust from a final block the client already trusts to a later one, with data an untrusted full node serves at `/v1/light/{height}`: the block's finality proof and the validator sets its header and its commit child name. `vexod light verify` runs it against a node's RPC.

The client trusts block T, learned outside the node, and T's next validator set: the set T's header names by `next_validator_set_hash`, which certifies T's children. `finality.VerifyLightBlock` accepts block B above T when all of these hold:

1. T's time is within the trusting period of the local clock. B's time is after T's and at most the clock drift ahead of the local clock.
2. B is final: its proof passes the verification algorithm above, with the two served sets. Each set must hash to what its header names.
3. Validators that T's next set holds, with the keys they signed with, hold more than one third of that set's voting power among the signers of QC(B).

The trust assumption is that less than one third of a trusted set's power is faulty until its trusting period ends. Under it, check 3 means an honest validator voted for B. An honest validator votes only for a header that names the sets its own replicated state holds, so B's set and the next set B names are genuine, and QC(C), which must be signed by that next set, is the chain's.

The trusting period must be shorter than the time a validator stays slashable after it leaves the set. Validators who sign a block outside the canonical chain are then still accountable.

When B's set shares too little with T's set, the client bisects. It first trusts the block halfway, which needs less overlap with T's set, and retries B from there.

Every header names the set of its children, so the block right after T is certified by T's next set itself and check 3 holds with a full quorum. Bisection therefore always reaches the target, however much the set changes between blocks; against an honest source the client stops only on a block that does not verify.

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
