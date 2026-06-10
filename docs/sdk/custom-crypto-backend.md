# Custom Crypto Backend Guide

## Goal

This guide explains how to add a custom crypto backend, including audited BLS and VRF adapters.

`vexo-consensus` ships adapter contracts, registry hooks, metadata validation, runtime wiring, a `supranational/blst` BLS12-381 min-pk adapter, a CIRCL-backed BLS12-381 reference adapter, and an ECVRF P-256 adapter. Operators can register audited adapters for value-bearing deployments, and audit evidence, key custody, and release-gate validation remain deployment responsibilities.

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

Production BLS is registered through `BLSAdapter` and must pass `ValidateBLSAdapter` before it can be used as a signer or runtime finality backend. Adapter metadata must declare audit status, audit report identity, dependency audit identity, public-key validation, subgroup checks, rogue-key defense, deterministic encoding, malformed-input fuzz coverage, and proof-of-possession support. The network-safety gate now requires BLS for public/value-bearing configs; Ed25519 is intentionally outside that gate because it cannot provide aggregate-finality verification.

Registering metadata is not a substitute for a real audited implementation. The adapter package must perform the actual subgroup checks, key validation, proof-of-possession verification, signature verification, aggregate verification, and malformed-input rejection.

Adapter packages should register implementations from `init()`:

```go
func init() {
    crypto.RegisterBLSAdapter("audited-bls-v1", func() (crypto.BLSAdapter, error) {
        return NewAuditedBLSAdapter()
    })
}
```

`crypto.adapter_name` must match `BLSAdapter.Metadata().Name`; otherwise runtime startup fails. This prevents config-only “BLS enabled” states where no audited implementation is actually linked into the binary. `crypto.audit_evidence_sha256` must also be a 32-byte hex digest so release tooling can bind the config to the external BLS audit artifact.

Validator public keys should be admitted through `BLSValidatorCredential` records or validator metadata key `bls_pop`. `ValidateBLSValidatorCredentials` rejects missing IDs, missing keys, duplicate public keys, invalid keys, and invalid proof-of-possession values. `NewBLSAggregateVerifier` wraps the audited adapter so finality verification only accepts registered validator keys.

The default production-oriented adapter is `blst-bls12381-minpk-v1`. It uses `github.com/supranational/blst`, compressed min-pk encoding, public keys in G1, signatures in G2, proof-of-possession checks, subgroup validation, domain-separated signing, and fast aggregate verification for same-message finality votes. The built-in CIRCL adapter is still registered as `circl-bls12381-g1sigg2-basic-v1` for reference and compatibility tests, but it is intentionally not accepted by the network safety gate as a production BLS adapter. Config metadata cannot promote an unsafe adapter into an audited adapter. `NewBLSTBLSKeyDocument` and `NewCIRCLBLSKeyDocument` both write `bls_proof_of_possession` metadata so validator genesis metadata can carry the rogue-key defense proof.

CLI helpers:

```bash
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
vexod init validator --home .vexo-validator --validator validator-1 --key-type bls
```

The init flow copies `bls_proof_of_possession` from the key document into genesis metadata key `bls_pop`.

## Production VRF Requirements

VRF-backed committee selection uses the same registration pattern:

```go
func init() {
    crypto.RegisterVRFAdapter("audited-vrf-v1", func(cfg config.VRFConfig) (crypto.VRFAdapter, error) {
        return NewAuditedVRFAdapter(cfg)
    })
}
```

`vrf.adapter_name`, `vrf.audit_report`, `vrf.dependency_audit`, `vrf.audit_evidence_sha256`, and `vrf.key_source` must match the adapter metadata and release evidence. When `committee.backend` is `vrf`, runtime startup fails if no matching adapter is linked instead of silently falling back to deterministic VRF. SDK integrations that must never fall back to deterministic VRF should call `crypto.NewProductionVRF`, which rejects configs without `production_adapter: true`. When committee selection is deterministic, runtime does not load a VRF adapter.

The built-in ECVRF adapter is registered as `ecvrf-p256-sha256-tai-v1`. It uses P-256/SHA-256 try-and-increment ECVRF proofs. Validators may put a base64 VRF public key in metadata key `vrf_public_key`; otherwise committee selection falls back to the validator consensus public key.

The built-in remote VRF adapter is registered as `remote-vrf-http-v1`. Set `vrf.adapter_name` to that value and `vrf.key_source` to `remote-http:<base-url>` or the plain HTTPS base URL. The adapter calls `POST /prove` with base64 `public_key`, `seed`, a random `nonce`, `issued_at_unix_nano`, `deadline_unix_nano`, and domain `vexo.remote_vrf.prove.v1`; the response must echo the same `nonce` and return base64 `output` plus `proof`. It calls `POST /verify` with the same challenge fields and domain `vexo.remote_vrf.verify.v1`; verification only succeeds when the service returns `{ "valid": true, "nonce": "<same nonce>" }`. If the `VEXO_REMOTE_VRF_TOKEN` environment variable is set, requests include `Authorization: Bearer <token>`. `vrf.tls_cert_path`, `vrf.tls_key_path`, `vrf.tls_ca_path`, and `vrf.tls_server_name` enable mTLS or pinned CA validation for the remote prover. Cert/key must be configured together; invalid TLS material fails adapter construction instead of silently falling back to unauthenticated HTTP. This is the preferred integration point for KMS/HSM-backed VRF custody, but the remote service still needs independent audit evidence matching `vrf.audit_report` and operational evidence for availability, replay protection, nonce/audit logging, TLS/mTLS, authorization, and key access policy.

Remote VRF implementations should use the context-aware `ProveWithContext` and `VerifyWithContext` methods whenever selection is driven by consensus or RPC deadlines. The legacy `Prove` and `Verify` methods are convenience wrappers; production paths should propagate cancellation so a slow remote prover cannot outlive the block or request timeout.

Prefer encrypted VRF key documents referenced from `consensus_config.json`:

```json
{
  "vrf_key_paths": ["validator.vrf.key.json"],
  "vrf": {
    "adapter_name": "ecvrf-p256-sha256-tai-v1",
    "audit_report": "operator-audit-reference",
    "dependency_audit": "github.com/vechain/go-ecvrf@v0.0.0-20251211112124-5d5a3ef70fc9",
    "audit_evidence_sha256": "<sha256-of-vrf-audit-evidence>",
    "key_source": "config.vrf.keys",
    "production_adapter": true
  }
}
```

For a remote VRF prover, prefer HTTPS plus mTLS/pinned CA:

```json
{
  "vrf": {
    "adapter_name": "remote-vrf-http-v1",
    "audit_report": "operator-remote-vrf-audit",
    "dependency_audit": "external:remote-vrf-service-audit-2026",
    "audit_evidence_sha256": "<sha256-of-remote-vrf-audit-evidence>",
    "key_source": "remote-http:https://vrf.example.internal",
    "production_adapter": true,
    "tls_cert_path": "vrf-client.crt",
    "tls_key_path": "vrf-client.key",
    "tls_ca_path": "vrf-ca.pem",
    "tls_server_name": "vrf.example.internal"
  }
}
```

Generate an encrypted VRF key document with:

```bash
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```

At startup, `vexod` resolves relative `vrf_key_paths` from the directory containing `consensus_config.json`, decrypts encrypted key documents through `VEXO_KEY_PASSPHRASE`, and injects the private key into the runtime VRF adapter. Direct `vrf.keys` remains available for tests or custom loaders, but operators should avoid storing raw private scalars in config files. For public value-bearing networks, a remote signer/KMS-backed VRF prover is still preferred over local key custody.

## Remote Signer Requirements

Remote signers must enforce their own policy tuple:

```text
(chain_id, height, round, type, domain)
```

They must reject conflicting messages for the same tuple even if the node process restarts or is compromised.

`vexo-consensus` also provides a node-side and HTTP KMS/HSM `DoubleSignGuard` helper plus a durable remote-signer nonce replay guard. For built-in serving, run `vexod keys serve-remote` with durable `--guard-path` and `--nonce-path`; external production KMS/HSM implementations must keep equivalent durable policy and replay-nonce databases. The guard key includes domain separation:

```text
chain_id/height/round/type/domain
```

Valid sign type and domain pairs are:

- `consensus_proposal` → `vexo.consensus.proposal.v1`
- `consensus_vote` → `vexo.consensus.vote.v1`
- `consensus_timeout_vote` → `vexo.consensus.timeout_vote.v1`
- `finality_proof` → `vexo.finality.proof.v1`

## Test Backends

`deterministic` is test-only. It must not pass network safety validation and must not be used for value-bearing deployments.
