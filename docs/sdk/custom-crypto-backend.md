# Custom Crypto Backend Guide

## Goal

This guide explains how to add a custom crypto backend, including audited BLS and VRF adapters.

`vexo-consensus` ships adapter contracts, registry hooks, metadata validation, runtime wiring, a CIRCL-backed BLS12-381 adapter, and an ECVRF P-256 adapter. Operators can use the built-ins or register their own adapters, but audit evidence, key custody, and release-gate validation remain deployment responsibilities.

## Interfaces

Implement:

```go
type Signer interface {
    PublicKey() types.PublicKey
    Sign(message []byte) (types.Signature, error)
    Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

type AggregateSigner interface {
    Aggregate(signatures []types.Signature) (types.AggregateSignature, error)
    VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}
```

## Runtime Suite

A backend must provide:

- consensus signer
- finality verifier
- consensus aggregator
- key validation
- deterministic serialization

## Domain Separation

All signatures must use explicit domains:

- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`

Never sign raw messages directly in production paths.

## Production BLS Requirements

A BLS adapter must include:

- audited library dependency
- public key validation
- signature validation
- subgroup checks
- proof-of-possession or equivalent rogue-key defense
- domain separation
- deterministic aggregate encoding
- dependency audit for the adapter and transitive crypto dependencies
- test vectors
- fuzz tests for malformed keys/signatures

Production BLS is registered through `BLSAdapter` and must pass `ValidateBLSAdapter` before it can be used as a signer or runtime finality backend. Adapter metadata must declare audit status, audit report identity, dependency audit identity, public-key validation, subgroup checks, rogue-key defense, deterministic encoding, malformed-input fuzz coverage, and proof-of-possession support.

Registering metadata is not a substitute for a real audited implementation. The adapter package must perform the actual subgroup checks, key validation, proof-of-possession verification, signature verification, aggregate verification, and malformed-input rejection.

Adapter packages should register implementations from `init()`:

```go
func init() {
    crypto.RegisterBLSAdapter("audited-bls-v1", func() (crypto.BLSAdapter, error) {
        return NewAuditedBLSAdapter()
    })
}
```

`crypto.adapter_name` must match `BLSAdapter.Metadata().Name`; otherwise runtime startup fails. This prevents config-only “BLS enabled” states where no audited implementation is actually linked into the binary.

Validator public keys should be admitted through `BLSValidatorCredential` records or validator metadata key `bls_pop`. `ValidateBLSValidatorCredentials` rejects missing IDs, missing keys, duplicate public keys, invalid keys, and invalid proof-of-possession values. `NewBLSAggregateVerifier` wraps the audited adapter so finality verification only accepts registered validator keys.

The built-in CIRCL adapter is registered as `circl-bls12381-g1sigg2-basic-v1`. It provides BLS12-381 basic signatures, aggregate verification, compressed deterministic encoding, public-key validation, and proof-of-possession helpers. `NewCIRCLBLSKeyDocument` writes `bls_proof_of_possession` metadata so validator genesis metadata can carry the rogue-key defense proof.

## Production VRF Requirements

VRF-backed committee selection uses the same registration pattern:

```go
func init() {
    crypto.RegisterVRFAdapter("audited-vrf-v1", func(cfg config.VRFConfig) (crypto.VRFAdapter, error) {
        return NewAuditedVRFAdapter(cfg)
    })
}
```

`vrf.adapter_name`, `vrf.audit_report`, and `vrf.key_source` must match the adapter metadata. When `committee.backend` is `vrf`, runtime startup fails if no matching adapter is linked instead of silently falling back to deterministic VRF. When committee selection is deterministic, runtime does not load a VRF adapter.

The built-in ECVRF adapter is registered as `ecvrf-p256-sha256-tai-v1`. It uses P-256/SHA-256 try-and-increment ECVRF proofs. `vrf.keys` maps a base64-encoded VRF public key to its private scalar bytes, and validators may put a base64 VRF public key in metadata key `vrf_public_key`; otherwise committee selection falls back to the validator consensus public key. For public networks, keep VRF private material in a signer/KMS boundary rather than plain config files.

## Remote Signer Requirements

Remote signers must enforce their own policy tuple:

```text
(chain_id, height, round, type, domain)
```

They must reject conflicting messages for the same tuple even if the node process restarts or is compromised.

`vexo-consensus` also provides a node-side and HTTP KMS/HSM `DoubleSignGuard` helper. For built-in serving, run `vexod keys serve-remote` with a durable `--guard-path`; external production KMS/HSM implementations must keep an equivalent durable policy database. The guard key includes domain separation:

```text
chain_id/height/round/type/domain
```

Valid sign type and domain pairs are:

- `consensus_proposal` → `vexo.consensus.proposal.v1`
- `consensus_vote` → `vexo.consensus.vote.v1`
- `consensus_timeout_vote` → `vexo.consensus.timeout_vote.v1`
- `finality_proof` → `vexo.finality.proof.v1`

## Development Backends

`deterministic` is test-only. It must not pass network safety validation and must not be used for value-bearing deployments.
