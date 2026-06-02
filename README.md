# vexo-consensus

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Tests](https://img.shields.io/badge/tests-go%20test%20./...-brightgreen)](#testing)

`vexo-consensus` is an experimental, modular consensus engine skeleton for building high-throughput Proof-of-Stake chains with a Tendermint/Cosmos SDK-style development model.

It focuses on clean module boundaries for consensus, validator management, committee selection, mempool design, finality verification, slashing, governance, data availability, fair ordering, storage, and P2P defense.

> This repository is an experimental consensus framework skeleton. It is not production consensus software.

## Table of Contents

- [Why vexo-consensus?](#why-vexo-consensus)
- [Architecture](#architecture)
- [Features](#features)
- [Packages](#packages)
- [Quick Start](#quick-start)
- [Testing](#testing)
- [Design Principles](#design-principles)
- [Security Notice](#security-notice)
- [Contributing](#contributing)
- [License](#license)

## Why vexo-consensus?

Consensus systems involve hard tradeoffs between decentralization, speed, finality, validator scalability, and implementation complexity.

`vexo-consensus` keeps those tradeoffs isolated behind small, testable interfaces:

```text
validator participation
        ↓
committee-based consensus
        ↓
DAG-ready mempool + fair ordering
        ↓
BFT finality + data availability
        ↓
light-client verification
```

The design is intentionally modular so individual components can be replaced without rewriting the entire node.

## Architecture

```text
┌──────────────────────────────┐
│      Application Modules      │
└───────────────▲──────────────┘
                │ ABCI-like API
┌───────────────▼──────────────┐
│          App Runtime          │
└───────────────▲──────────────┘
                │ block execution
┌───────────────▼──────────────┐
│       Consensus Runtime       │
└───────┬────────┬────────┬────┘
        │        │        │
        ▼        ▼        ▼
   Validator  Mempool  Finality
        │        │        │
        ▼        ▼        ▼
   Committee    DAG    Light Client
        │
        ▼
   Slashing / Governance / Storage / P2P Defense
```

## Features

### Consensus

- HotStuff-style proposal/vote skeleton
- Weighted voting-power quorum checks
- Quorum certificate generation
- Conflicting vote detection
- Slashing evidence generation
- Locked-QC proposal and vote safety rules
- Timeout certificates carrying high-QC
- Three-chain finality decisions
- Deterministic scenario and adversarial simulation helpers
- Transport message codec and reactor for proposal/vote/timeout routing

### Validator and Committee

- In-memory validator registry
- Permissionless or whitelist-style admission policy
- Minimum stake policy
- Maximum validator count policy
- Deterministic committee selection by seed, epoch, and round
- Epoch calculation and rotation policy
- Validator set update and rotation support
- Slashing-driven voting-power updates

### Mempool

- FIFO transaction pool
- Transaction validation boundaries
- Batch construction
- DAG batch parent/tip tracking
- Duplicate batch and unknown parent rejection

### Ordering and Data Availability

- Deterministic transaction ordering
- Proposal-level transaction reordering rejection
- Transaction data commitments
- Data availability proof metadata
- Missing or mismatched data commitment rejection

### Finality and Light Client

- Finality proof type
- Header hash and sign bytes
- Signer bitmap encoding/parsing
- Validator set hash checks
- Quorum checks
- Aggregate signature verification hook
- Stored-block finality proof construction

### Safety Modules

- Slashing evidence validation
- Penalty receipt recording
- Validator voting-power reduction after verified evidence
- Penalty policy lookup
- Governance quorum, veto, voting period, and timelock
- P2P peer scoring
- P2P rate-limit and ban threshold logic
- Flood, overflow, duplicate, and invalid-transaction regression tests

### Storage and Runtime

- ABCI-like application runtime
- Bank module with mint, send, balance query, and persisted state
- Block executor
- LevelDB block/state/state-root storage
- Recovery and replay helpers
- Runtime validator update application
- Node config, genesis, data directory, and lifecycle skeleton
- Node-level transport reactor wiring for in-memory multi-node simulations

## Packages

| Package | Description |
|---|---|
| `app` | ABCI-like application and module interfaces |
| `app/bank` | Bank balances, mint/send transactions, and balance queries |
| `cmd/vexod` | CLI entrypoint |
| `committee` | Committee selection and epoch rotation |
| `config` | Default chain configuration and validation |
| `consensus` | Consensus state machine, votes, proposals, QC, conflict evidence |
| `dataavailability` | Transaction data commitments and availability checks |
| `crypto` | Deterministic and Ed25519 signers, aggregate verification, and key files |
| `fairordering` | Deterministic transaction ordering |
| `finality` | Finality proofs and light-client verifier |
| `governance` | Proposal, voting, quorum, veto, and timelock module |
| `mempool` | FIFO mempool and DAG batch graph |
| `node` | Node config, genesis, lifecycle, runtime/store wiring |
| `p2p` | Peer scoring and rate-limit defense |
| `rpc` | HTTP health, readiness, status, and peer metrics endpoints |
| `runtime` | Module wiring, block execution, proof building, recovery, replay |
| `slashing` | Evidence validation and penalty keeper |
| `store` | LevelDB-backed block, state, state-root, and KV storage |
| `transport` | In-memory and TCP message transport with pub/sub interfaces |
| `types` | Shared primitive types |
| `validator` | Validator registry and admission policy |

## Quick Start

### Requirements

- Go 1.26+

### Run the CLI

```bash
go run ./cmd/vexod
```

Example output:

```text
vexo-consensus status
chain_id: vexo-local
validator.permissionless: true
validator.min_stake: 1
committee.epoch_length: 100
committee.size: 128
mempool.max_txs: 100000
fair_ordering.deterministic: true
data_availability.commitments: true
storage.backend: leveldb
p2p.initial_score: 100
p2p.valid_message_reward: 1
p2p.invalid_message_cost: 10
p2p.rate_limit_cost: 5
p2p.ban_threshold: 0
p2p.max_messages_per_window: 1000
p2p.window_reset_interval: 1s
p2p.score_recovery: 1
p2p.ban_duration: 10m0s
```

Machine-readable status:

```bash
go run ./cmd/vexod status --json
```

Show CLI help and version:

```bash
go run ./cmd/vexod help
go run ./cmd/vexod version
```

Initialize node files:

```bash
go run ./cmd/vexod init --home .vexo --chain-id vexo-local --validator validator-1
```

This writes `.vexo/config.json`, `.vexo/genesis.json`, and `.vexo/data`.

Validate node files:

```bash
go run ./cmd/vexod validate --home .vexo
```

Generate a validator key:

```bash
go run ./cmd/vexod keys gen --home .vexo
```

Show the public validator key without printing the private key:

```bash
go run ./cmd/vexod keys show --home .vexo --json
```

Inspect paths and startup readiness:

```bash
go run ./cmd/vexod config paths --home .vexo --json
go run ./cmd/vexod start --home .vexo --dry-run
```

Run a minimal block execution demo:

```bash
go run ./cmd/vexod demo
```

Run a LevelDB-backed storage demo:

```bash
go run ./cmd/vexod store-demo
```

Build the node binary:

```bash
make build
```

Run all checks:

```bash
make check
```

Build a container image:

```bash
docker build -t vexo-consensus .
```

Expose operational status over HTTP from an embedded node:

```go
server := rpc.NewServer(node, rpc.Config{Address: "127.0.0.1:26657"})
```

Available endpoints: `/healthz`, `/readyz`, `/status`, `/diagnostics`, `/metrics`, `/metrics/text`, `/peers`, `/tx`, `/evidence`, `/prune`, `/replay`, `/consensus/start`, `/consensus/stop`, `/snapshot/latest`, `/blocks`, `/blocks/latest`, `/blocks/{height}`, `/state/latest`, `/state/{height}/{namespace}`, `/validators/{height}`, `/committee/{height}/{round}`.

Set `rpc.Config.AdminToken` to require `Authorization: Bearer <token>` for admin endpoints such as `/prune`, `/replay`, `/consensus/start`, and `/consensus/stop`.

## Testing

Run all tests:

```bash
go test ./...
```

If your local environment blocks the default Go build cache, use a workspace-local cache:

```bash
mkdir -p .gocache
GOCACHE=$(pwd)/.gocache go test ./...
rm -rf .gocache
```

## Design Principles

- **Modularity**: every major subsystem should be replaceable.
- **Simple verification**: finality proofs should be light-client friendly.
- **Configurable participation**: validator admission should support permissionless and restricted modes.
- **Safe defaults**: invalid or dangerous runtime parameters should fail validation early.
- **Accountable safety**: malicious behavior should produce evidence.
- **DoS resistance**: networking should include scoring and rate limits.
- **Governance safety**: parameter changes should use voting windows and timelocks.
- **Crypto agility**: cryptographic implementations should be pluggable.

## Security Notice

`vexo-consensus` is experimental software.

Do not use it to secure real funds, production validator infrastructure, or critical systems without independent review, production cryptography, persistent state, real networking, and protocol-level audits.

## Contributing

Contributions are welcome.

Before opening a change, keep the current design rule in mind:

> Prefer small, testable modules over clever monoliths.

## License

`vexo-consensus` is released under the MIT License. See [LICENSE](./LICENSE).
