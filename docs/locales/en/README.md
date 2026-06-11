# Documentation

This directory is the practical manual for `vexo-consensus`.

It is written for people who need to understand, build, operate, review, or release a network without guessing from source code alone. A good page in this tree should answer four questions quickly:

1. **What is this part of the system responsible for?**
2. **Which files, commands, config keys, or APIs implement it?**
3. **What must be true for it to be safe?**
4. **What evidence proves it is ready for a real network?**

English is the canonical source for protocol, security, release, SDK, command, config, and RPC behavior. Localized documents mirror this tree and help non-English readers, but release and audit decisions must always be checked against the English source.

## How to Read This Set

Use the path that matches what you are trying to do. If you are not sure, start with the first row.

| Goal | Read First | Then Verify |
|---|---|---|
| Understand the protocol | Consensus overview, consensus spec, finality proof format | Safety assumptions, validator lifecycle, evidence rules |
| Build an app chain | App module guide, tx format, storage schema | Module store writes, gas/fee policy, RPC compatibility |
| Enable EVM features | EVM/native accounting, tx format, RPC versioning | Native balance accounting, gas/base fee behavior, Web3 compatibility evidence |
| Run nodes | Node initialization, adding a validator, networking spec | Split config files, peer identity, key custody, status/metrics |
| Prepare a release | Audit readiness, release pipeline, launch runbook | Required evidence files, release gate output, rollback plan |

If you are new to the project, start in this order:

1. [Consensus Protocol Overview](./consensus-protocol.md)
2. [Consensus Spec](./specs/consensus-spec.md)
3. [Transaction Format](./specs/tx-format.md)
4. [Validator Lifecycle](./specs/validator-lifecycle.md)
5. [Node Initialization](./operators/node-initialization.md)
6. [Security Audit Readiness](./security/audit-readiness.md)

## Protocol Specs

| Document | Purpose |
|---|---|
| [Consensus Spec](./specs/consensus-spec.md) | Consensus state machine, safety rules, liveness assumptions, and evidence surfaces |
| [Finality Proof Format](./specs/finality-proof-format.md) | Proof fields and verifier rules for full nodes and light clients |
| [Networking Spec](./specs/networking-spec.md) | Transport expectations, handshake policy, peer scoring, backoff, and DoS defenses |
| [Storage Schema](./specs/storage-schema.md) | Durable records, indexes, recovery rules, snapshots, and schema migration expectations |
| [Transaction Format](./specs/tx-format.md) | Canonical transaction payload, signed envelope, nonce, fee, gas, and CheckTx requirements |
| [EVM and Native Accounting](./specs/evm-native-accounting.md) | Shared native/EVM balance model, 256-bit amounts, fee handling, and compatibility boundary |
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

## Localized Documentation

Locale files are not allowed to drift from the canonical tree. They keep commands, JSON fields, RPC names, config keys, and code identifiers unchanged so examples stay copy-pasteable across languages.

| Document | Purpose |
|---|---|
| [Documentation Locales](./locales/README.md) | Locale directory map and translation policy |
| [English Canonical Docs](./locales/en/README.md) | Normative English documentation tree |
| [Korean Docs](./locales/ko/README.md) | Korean locale tree |
| [Chinese Docs](./locales/zh/README.md) | Chinese locale tree |
| [Japanese Docs](./locales/ja/README.md) | Japanese locale tree |
| [French Docs](./locales/fr/README.md) | French locale tree |
| [German Docs](./locales/de/README.md) | German locale tree |

## Writing New Docs

Documentation should:

- start with the reader goal and the decision the page supports
- state whether it is a normative spec, implementation guide, operator guide, or release/audit checklist
- include relevant commands, package paths, config keys, RPC methods, and JSON fields
- explain safety boundaries, failure modes, and unsafe shortcuts
- avoid production-readiness claims without evidence
- keep examples copy-pasteable when possible, but clearly mark values that must be changed
- keep every Markdown file mirrored under `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- pass `make docs-check` so localized directory trees cannot drift from the canonical docs

## Production Claim Rule

Do not call a feature production-ready just because code exists. A production claim needs:

- implementation code
- unit/property/adversarial tests
- operational or E2E evidence when the feature crosses process or machine boundaries
- documentation of assumptions and failure modes
- release-gate evidence for security-sensitive categories such as BLS, VRF, Web3/EVM compatibility, slashing, state sync, upgrades, and validator economics

`vexod status --json` follows the same rule. The `features` map says whether a code path is enabled by config. The `feature_assurance` map says whether that enabled feature is merely implemented, requires operator artifacts, requires release evidence, or requires external audit evidence.

Operator-facing safety defaults live in split config files rather than command flags. When reviewing a node home, check these first:

- `network_config.json:p2p.auth_replay_path` for restart-safe P2P handshake replay protection
- `network_config.json:p2p.node_key_path` for the peer-authentication key, separate from validator consensus custody
- `module_config.json:governance.RequireDeposit` and `module_config.json:governance.MinDeposit` for proposal spam/economic-friction policy
- `consensus_config.json:consensus.execution_commit` for the execution/finality boundary
- `mempool_config.json:mempool.WALPath` for restart-safe pending transaction recovery

## Documentation Review Checklist

Before merging documentation changes:

- confirm the English document is still precise enough to be used as a release/audit source
- confirm every locale file points back to the right English canonical document
- preserve all commands, RPC names, config keys, JSON fields, and package names exactly
- run `make docs-check`
- run the broader project checks when command examples, config schemas, or generated artifacts changed
