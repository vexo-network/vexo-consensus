# vexo-consensus

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)
[![CI](https://github.com/vexo-network/vexo-consensus/actions/workflows/ci.yml/badge.svg)](https://github.com/vexo-network/vexo-consensus/actions/workflows/ci.yml)

`vexo-consensus` is a modular consensus framework for building independent Proof-of-Stake networks.

It follows a Tendermint/Cosmos SDK-style developer experience, but it is not a Tendermint or CometBFT compatibility layer. The goal is to provide clean building blocks for consensus, validator management, finality verification, slashing, application modules, storage, networking, and release operations.

> **Maturity:** this framework includes durable storage paths, adversarial tests, release tooling, and network safety audits, but it must not be used to secure real funds or public validator infrastructure without an audited crypto backend, independent security review, and multi-machine long-run evidence.

## Highlights

- HotStuff-style BFT core with proposals, votes, quorum certificates, timeout certificates, locked-QC safety, and three-chain finality.
- Height-versioned validator registry with validator-set hash binding for consensus and light-client verification.
- Modular application runtime with pluggable modules and module-owned CLI commands.
- Split subsystem configuration through `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json`.
- Durable LevelDB storage for blocks, state, Merkle state roots, evidence, KV state, schema metadata, pruning, recovery, and snapshots.
- Signed transaction envelopes with nonce, fee units, dynamic base fee, dynamic blob base fee, gas metering, and ante validation paths.
- Data availability commitments with chunk proofs, deterministic 1D/2D sample verification, and bounded parity recovery.
- Slashing evidence lifecycle with monotonic applied-evidence persistence, penalty receipts, jailing, tombstone records, entry-based unbonding checks, and restart-aware recovery.
- gRPC/TCP/in-memory transport abstractions with peer handshake validation, scoring, bans, backoff, and rate limits.
- Operations tooling for config audit, release gates, audit packs, snapshot drills, adversarial simulations, and parameter tuning.

## Non-Goals

- This project does not provide ABCI compatibility.
- This project includes a `supranational/blst` BLS12-381 min-pk adapter, ECVRF adapter wiring, and custom crypto hooks, but operators are still responsible for audit evidence, key custody, and release-gate validation before value-bearing deployment.
- This project does not claim public value-bearing network safety without external audit and real multi-host operational evidence.
- Chain-specific economics such as token custody, reward policy tuning, and governance authority remain integration responsibilities.

## Repository Layout

| Path | Purpose |
|---|---|
| `cmd/vexod` | CLI entrypoint, node startup flow, diagnostics, release tooling, and local harnesses |
| `app` | Application runtime interfaces, ante handling, transaction envelopes, and module contracts |
| `modules` | Built-in bank, staking, governance, params, and IBC application modules |
| `params` | Cosmos-style module parameter keeper and parameter transaction/query module |
| `events` | Event indexing primitives for block/tx attributes |
| `ibc` | IBC client, freeze/expiry, connection, channel, proof, and packet lifecycle primitives |
| `queryproof` | Latest and historical Merkle query proofs for namespace/key state-root binding |
| `stateproof` | Deterministic namespace Merkle tree and proof verification helpers |
| `contract` | VM registry boundary used by the built-in geth-backed EVM adapter and custom WASM-compatible modules |
| `modules/evm/backend/geth` | Isolated go-ethereum adapter layer for EVM bytecode execution |
| `modules/evm/ethcompat` | Isolated go-ethereum transaction compatibility layer for signed Ethereum raw transactions |
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

This section is the shortest path from a fresh checkout to a running network. For the full operator explanation, read [Node Initialization](./docs/operators/node-initialization.md) and [Docker Deployment](./deployments/docker/README.md).

### 1. Build the binary

Prerequisites:

- Go 1.26 or newer
- `make`
- Docker only if you want the container examples

```bash
make build
./bin/vexod --help
./bin/vexod version
```

Run the normal local checks:

```bash
make check
```

### 2. Start one validator node

Use this when you only want to inspect the generated files and run a single process:

```bash
export VEXO_KEY_PASSPHRASE='change-me'
./bin/vexod init validator \
  --home .vexo \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo
./bin/vexod config audit --home .vexo --strict
./bin/vexod start --home .vexo
```

In another terminal:

```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```

If `consensus_config.json` has `"create_empty_blocks": false`, height can stay unchanged until a transaction enters the mempool. That is expected.

### 3. Start a 4-validator Docker network

Use this when you want to see peers connect and blocks commit on one machine:

```bash
export VEXO_KEY_PASSPHRASE='change-me'

docker compose \
  -f deployments/docker/compose.single-host.init.yml \
  -f deployments/docker/compose.single-host.init.build.cgo.yml \
  run --rm init

docker compose \
  -f deployments/docker/compose.single-host.yml \
  -f deployments/docker/compose.single-host.build.cgo.yml \
  up --build
```

Status endpoints from the host:

```bash
curl -s http://127.0.0.1:28657/v1/status
curl -s http://127.0.0.1:28667/v1/status
curl -s http://127.0.0.1:28677/v1/status
curl -s http://127.0.0.1:28687/v1/status
```

Stop it with:

```bash
docker compose -f deployments/docker/compose.single-host.yml down
```

### 4. Connect Remix or another Web3 tool

Vexo exposes Ethereum-style JSON-RPC methods under `/web3`. Browser tools such as Remix need CORS preflight support, which the RPC server enables by default.

Use this custom provider URL:

```text
http://127.0.0.1:28657/web3
```

Quick checks:

```bash
curl -s http://127.0.0.1:28657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'

curl -s http://127.0.0.1:28657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"vexo_web3Capabilities","params":[]}'
```

If Remix shows `Failed to fetch eth_chainId`, check that the URL ends with `/web3`, the node RPC port is reachable from the browser, and the Docker host port is the exposed host port such as `28657`, not the container-internal `26657`.

### 5. Know the generated files

Node behavior is configured through files, not long `start` flags:

- `.vexo/config.json`: node identity, chain ID, data path, and split config pointers
- `.vexo/module_config.json`: app modules, execution policy, EVM chain ID, gas, fees, and governance policy
- `.vexo/network_config.json`: RPC/P2P listen addresses, peers, seeds, Web3 RPC, state sync, and peer scoring
- `.vexo/consensus_config.json`: consensus timing, empty-block policy, finality/execution boundary, crypto, VRF, and committee policy
- `.vexo/mempool_config.json`: mempool size, fees, replacement policy, TTL, and WAL persistence
- `.vexo/log_config.json`: log format, level, block commit logs, and peer event logs
- `.vexo/genesis.json`: immutable genesis validators, validator metadata, and module genesis state

### 6. Useful commands after startup

```bash
./bin/vexod status --json
./bin/vexod config show --home .vexo
./bin/vexod config paths --home .vexo
./bin/vexod doctor --home .vexo
./bin/vexod bank --help
./bin/vexod staking --help
./bin/vexod governance --help
./bin/vexod evm --help
./bin/vexod proof --help
./bin/vexod snapshot --help
./bin/vexod release readiness --json
```

Governance proposal deposits are native bank balances. Store-backed execution escrows a proposal deposit from the submitter, refunds it when execution succeeds, and moves it to the rejected-deposit module account when a proposal is rejected.

## Documentation

Start with the documentation index:

- [Documentation Index](./docs/README.md)
- [Production Readiness Guide](./docs/production-readiness.md)

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
- [Observability Guide](./docs/operators/observability.md)
- [Security Audit Readiness](./docs/security/audit-readiness.md)
- [Launch Runbook](./docs/release/launch-runbook.md)
- [Release Pipeline](./docs/release/release-pipeline.md)
- [Cosmos/Tendermint Comparison Gate](./docs/release/cosmos-comparison-gate.md)
- [Version Compatibility Matrix](./docs/release/version-compatibility.md)
- [Docker Deployment](./deployments/docker/README.md)

## Contributing

Run the standard checks:

```bash
make check
```

CI uses `make release-candidate-smoke VERSION=ci` for pull-request release-path coverage. Use `make release-candidate VERSION=<rc>` only for a real candidate with externally pinned EVM/Web3 corpora, BLS-capable release settings, network E2E, live load, long-run, chaos, and release evidence artifacts.

Run additional security and operations smoke checks:

```bash
make fuzz-smoke
make ops-verify
```

Build release artifacts locally:

```bash
RELEASE_CGO_ENABLED=1 make release VERSION=0.1.0-rc.1
```

Release artifacts fail closed unless BLS-capable builds set `RELEASE_CGO_ENABLED=1`, because the built-in `supranational/blst` adapter is cgo-backed. Use `make release-portable RELEASE_REQUIRE_BLS=0` only for non-BLS portability smoke artifacts; do not publish those as BLS-capable release evidence.

When `RELEASE_CGO_ENABLED=1` and `RELEASE_TARGETS` is not set, `make release` builds the current host target only. Set `RELEASE_TARGETS` explicitly only when the runner has matching cgo cross-compilers for every requested target.

See [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a pull request.

## Security

Do not report suspected vulnerabilities through public issues. Use [SECURITY.md](./SECURITY.md) for the disclosure policy and [Security Audit Readiness](./docs/security/audit-readiness.md) for audit scope, assumptions, limitations, and required evidence.

## Support

Use [SUPPORT.md](./SUPPORT.md) for where to ask questions, how to report bugs, and what information to include.

## License

`vexo-consensus` is released under the Apache License, Version 2.0. See [LICENSE](./LICENSE).
