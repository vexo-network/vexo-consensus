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

### Storage and Runtime

- ABCI-like application runtime
- Block executor
- LevelDB block/state/state-root storage
- Recovery and replay helpers
- Runtime validator update application

## Packages

| Package | Description |
|---|---|
| `app` | ABCI-like application and module interfaces |
| `cmd/vexod` | CLI entrypoint |
| `committee` | Committee selection and epoch rotation |
| `config` | Default chain configuration and validation |
| `consensus` | Consensus state machine, votes, proposals, QC, conflict evidence |
| `dataavailability` | Transaction data commitments and availability checks |
| `crypto` | Deterministic test signer and aggregate signer |
| `fairordering` | Deterministic transaction ordering |
| `finality` | Finality proofs and light-client verifier |
| `governance` | Proposal, voting, quorum, veto, and timelock module |
| `mempool` | FIFO mempool and DAG batch graph |
| `p2p` | Peer scoring and rate-limit defense |
| `runtime` | Module wiring, block execution, proof building, recovery, replay |
| `slashing` | Evidence validation and penalty keeper |
| `store` | LevelDB-backed block, state, state-root, and KV storage |
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
p2p.max_messages_per_window: 1000
```

Run a minimal block execution demo:

```bash
go run ./cmd/vexod demo
```

Run a LevelDB-backed storage demo:

```bash
go run ./cmd/vexod store-demo
```

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
