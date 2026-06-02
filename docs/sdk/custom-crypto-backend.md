# Custom Crypto Backend Guide

## Goal

This guide explains how to add a custom crypto backend, including production BLS.

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
- test vectors
- fuzz tests for malformed keys/signatures

## Remote Signer Requirements

Remote signers must enforce their own policy tuple:

```text
(chain_id, height, round, type, domain)
```

They must reject conflicting messages for the same tuple even if the node process restarts or is compromised.

## Development Backends

`deterministic` is development-only. It must not pass `ValidateProduction()` and must not be used for value-bearing deployments.
