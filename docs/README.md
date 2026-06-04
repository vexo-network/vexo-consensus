# Documentation

This directory contains protocol specs, SDK extension guides, security material, and release operations documents for `vexo-consensus`.

If you are new to the project, read the documents in this order.

## Start Here

1. [Consensus Protocol Overview](./consensus-protocol.md)
2. [Consensus Spec](./specs/consensus-spec.md)
3. [Transaction Format](./specs/tx-format.md)
4. [Validator Lifecycle](./specs/validator-lifecycle.md)
5. [Security Audit Readiness](./security/audit-readiness.md)

## Protocol Specs

| Document | Purpose |
|---|---|
| [Consensus Spec](./specs/consensus-spec.md) | Consensus state machine, safety rules, liveness assumptions, and evidence surfaces |
| [Finality Proof Format](./specs/finality-proof-format.md) | Proof fields and verifier rules for full nodes and light clients |
| [Networking Spec](./specs/networking-spec.md) | Transport expectations, handshake policy, peer scoring, backoff, and DoS defenses |
| [Storage Schema](./specs/storage-schema.md) | Durable records, indexes, recovery rules, snapshots, and schema migration expectations |
| [Transaction Format](./specs/tx-format.md) | Canonical transaction payload, signed envelope, nonce, fee, gas, and CheckTx requirements |
| [Validator Lifecycle](./specs/validator-lifecycle.md) | Admission, rotation, evidence lifecycle, slashing, jailing, and unbonding |

## SDK and Extension Guides

| Document | Purpose |
|---|---|
| [App Module Guide](./sdk/app-module-guide.md) | Add a custom application module and module CLI commands |
| [Custom Crypto Backend](./sdk/custom-crypto-backend.md) | Add signing/finality backends and production BLS adapter metadata |
| [Custom Storage and Transport](./sdk/custom-storage-transport.md) | Implement custom stores or peer transports |
| [RPC API Versioning](./sdk/rpc-api-versioning.md) | Understand `/v1/*` compatibility rules and endpoint stability |

## Operations and Release

| Document | Purpose |
|---|---|
| [Node Initialization](./operators/node-initialization.md) | Initialize validator/archive nodes and manage split subsystem config files |
| [Adding a Validator](./operators/add-validator.md) | Operator flow for adding a validator and verifying height-specific validator-set updates |
| [Launch Runbook](./release/launch-runbook.md) | Operator launch flow, halt criteria, monitoring, and postlaunch archive requirements |
| [Release Pipeline](./release/release-pipeline.md) | Build, sign, package, and gate release artifacts |
| [Cosmos/Tendermint Comparison Gate](./release/cosmos-comparison-gate.md) | Maps Tendermint/Cosmos maturity advantages to required Vexo release evidence |
| [Version Compatibility Matrix](./release/version-compatibility.md) | Compatibility expectations across binary, config, store, app, RPC, and proof formats |

## Security

| Document | Purpose |
|---|---|
| [Security Audit Readiness](./security/audit-readiness.md) | Threat model, assumptions, limitations, safety argument, and required audit evidence |

## Writing New Docs

Documentation should:

- state whether it is normative specification, implementation guide, or operator guidance
- include the relevant commands or package paths
- describe safety boundaries and failure modes
- avoid production-readiness claims without evidence
- keep examples copy-pasteable when possible
