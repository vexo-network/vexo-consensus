# vexo-consensus

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![CI](https://github.com/vexo-network/vexo-consensus/actions/workflows/ci.yml/badge.svg)](https://github.com/vexo-network/vexo-consensus/actions/workflows/ci.yml)

`vexo-consensus` is a modular consensus framework for building independent Proof-of-Stake networks.

It follows a Tendermint/Cosmos SDK-style developer experience, but it is not a Tendermint or CometBFT compatibility layer. The goal is to provide clean building blocks for consensus, validator management, finality verification, slashing, application modules, storage, networking, and release operations.

> **Maturity:** this framework includes durable storage paths, adversarial tests, release tooling, and network safety audits, but it must not be used to secure real funds or public validator infrastructure without an audited crypto backend, independent security review, and multi-machine long-run evidence.

## Highlights

- HotStuff-style BFT core with proposals, votes, quorum certificates, timeout certificates, locked-QC safety, and three-chain finality.
- Height-versioned validator registry with validator-set hash binding for consensus and light-client verification.
- Modular application runtime with pluggable modules and module-owned CLI commands.
- Split subsystem configuration through `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json`.
- Durable LevelDB storage for blocks, state, state roots, evidence, KV state, schema metadata, pruning, recovery, and snapshots.
- Signed transaction envelopes with nonce, fee units, dynamic base fee, gas metering, and ante validation paths.
- Slashing evidence lifecycle with penalty receipts, jailing, unbonding checks, and restart-aware recovery.
- gRPC/TCP/in-memory transport abstractions with peer handshake validation, scoring, bans, backoff, and rate limits.
- Operations tooling for config audit, release gates, audit packs, snapshot drills, adversarial simulations, and parameter tuning.

## Non-Goals

- This project does not provide ABCI compatibility.
- This project does not vendor an audited production BLS implementation.
- This project does not claim mainnet safety without external audit and real multi-host operational evidence.
- Chain-specific economics such as rewards, custody, commission policy, and governance authority remain integration responsibilities.

## Repository Layout

| Path | Purpose |
|---|---|
| `cmd/vexod` | CLI entrypoint, node startup flow, diagnostics, release tooling, and local harnesses |
| `app` | Application runtime interfaces, ante handling, transaction envelopes, and module contracts |
| `modules` | Built-in bank, staking, and governance application modules |
| `consensus` | Consensus state machine, certificates, evidence, safety checks, and adversarial simulations |
| `finality` | Finality proof construction and verifier API |
| `validator` | Height-versioned validator registry and validator-set hashing |
| `crypto` | Signing suites, key documents, remote signer, BLS adapter boundary, and safety validation |
| `mempool` | FIFO/DAG mempool, replay WAL, duplicate suppression, fees, and recheck support |
| `store` | LevelDB-backed durable block/state/evidence/KV store and recovery helpers |
| `transport` | In-memory, TCP, and gRPC peer transport implementations |
| `rpc` | Versioned HTTP RPC, metrics, diagnostics, admin routes, request decoding, and response mapping |
| `docs` | Protocol specs, SDK guides, release docs, and security/audit material |

## Quick Start

Prerequisites:

- Go 1.26 or newer
- `make`

Build and inspect the CLI:

```bash
make build
./bin/vexod help
./bin/vexod version
```

Run the test suite:

```bash
make check
```

Initialize and validate a demo home:

```bash
./bin/vexod init --home .vexo --chain-id vexo-chain --validator validator-1 --overwrite
./bin/vexod validate --home .vexo
```

The generated home uses split subsystem config files:

- `.vexo/config.json` for node identity, chain ID, data path, and split config pointers
- `.vexo/module_config.json` for application modules, execution policy, and governance policy
- `.vexo/network_config.json` for RPC/P2P listen addresses, peers, seeds, and peer scoring
- `.vexo/consensus_config.json` for consensus timing, crypto, validator admission, and committee policy
- `.vexo/mempool_config.json` for mempool limits, fees, priority, and WAL policy
- `.vexo/log_config.json` for log format, level, and operational event logging

Run an in-memory application demo:

```bash
./bin/vexod demo
```

Run a LevelDB-backed storage demo:

```bash
./bin/vexod store-demo
```

## Common CLI Commands

```bash
vexod help
vexod status --json
vexod config show --home .vexo
vexod config audit --home .vexo --strict
vexod config tune --validators 64 --tps 5000 --regions 4 --latency 120ms --json
vexod keys gen --home .vexo
vexod tx build --module bank --action send --args alice,bob,25 --tags fee=1gvxo,gas=1000,signer=alice,nonce=1
vexod consensus adversarial --json
vexod snapshot drill-plan --input snapshot.json --chain-id vexo-chain --json
vexod release readiness --json
```

Module commands are contributed by application modules:

```bash
vexod bank --help
vexod staking --help
vexod governance --help
```

## Documentation

Start with the documentation index:

- [Documentation Index](./docs/README.md)

Core specs:

- [Consensus Protocol Overview](./docs/consensus-protocol.md)
- [Consensus Spec](./docs/specs/consensus-spec.md)
- [Finality Proof Format](./docs/specs/finality-proof-format.md)
- [Networking Spec](./docs/specs/networking-spec.md)
- [Storage Schema](./docs/specs/storage-schema.md)
- [Transaction Format](./docs/specs/tx-format.md)
- [Validator Lifecycle](./docs/specs/validator-lifecycle.md)

SDK and extension guides:

- [App Module Guide](./docs/sdk/app-module-guide.md)
- [Custom Crypto Backend](./docs/sdk/custom-crypto-backend.md)
- [Custom Storage and Transport](./docs/sdk/custom-storage-transport.md)
- [RPC API Versioning](./docs/sdk/rpc-api-versioning.md)

Operations and release:

- [Node Initialization](./docs/operators/node-initialization.md)
- [Adding a Validator](./docs/operators/add-validator.md)
- [Security Audit Readiness](./docs/security/audit-readiness.md)
- [Launch Runbook](./docs/release/launch-runbook.md)
- [Release Pipeline](./docs/release/release-pipeline.md)
- [Version Compatibility Matrix](./docs/release/version-compatibility.md)
- [Docker Deployment](./deployments/docker/README.md)

## Development

Run the standard checks:

```bash
make check
```

Run additional security and operations smoke checks:

```bash
make fuzz-smoke
make ops-verify
```

Build release artifacts locally:

```bash
make release VERSION=0.1.0-rc.1
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a pull request.

## Security

Do not report suspected vulnerabilities through public issues. Use [SECURITY.md](./SECURITY.md) for the disclosure policy and [Security Audit Readiness](./docs/security/audit-readiness.md) for audit scope, assumptions, limitations, and required evidence.

## Support

Use [SUPPORT.md](./SUPPORT.md) for where to ask questions, how to report bugs, and what information to include.

## License

`vexo-consensus` is released under the MIT License. See [LICENSE](./LICENSE).
