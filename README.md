# vexo-consensus

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Tests](https://img.shields.io/badge/tests-go%20test%20./...-brightgreen)](#testing)

`vexo-consensus` is an experimental, modular consensus engine skeleton for building high-throughput Proof-of-Stake chains with a Tendermint/Cosmos SDK-style development model.

It focuses on clean module boundaries for consensus, validator management, committee selection, mempool design, finality verification, slashing, governance, data availability, fair ordering, storage, operations, and P2P defense.

> This repository is an experimental consensus framework skeleton. It is not production consensus software.

## Table of Contents

- [Why vexo-consensus?](#why-vexo-consensus)
- [Architecture](#architecture)
- [Features](#features)
- [Packages](#packages)
- [Documentation](#documentation)
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
                │ modular app API
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
- Signature domain separation for consensus, timeout, and finality messages
- Domain-verified consensus proposal, vote, and timeout-vote signatures
- Signed vote aggregation for quorum and timeout certificates
- Validator keyring abstraction for active key rotation
- Height-aware validator key activation windows
- Slashing evidence generation
- Accountable timeout-vote equivocation evidence
- Consensus WAL for local proposal/vote/timeout-vote persistence
- Restart-safe local double-sign guard
- Locked-QC proposal and vote safety rules
- Timeout certificates carrying high-QC
- Three-chain finality decisions
- Deterministic scenario and adversarial simulation helpers for offline minorities, offline majorities, weighted quorum, conflicting votes, timeout equivocation, unsafe forks, and split partitions
- Transport message codec and reactor for proposal/vote/timeout routing
- Deterministic proposer rotation by height and round
- gRPC peer transport with optional mTLS, binary framing, persistent streams, seed bootstrap, peer exchange, protocol/network/chain/genesis/node-id/auth-token handshake validation, message-size limits, reconnect backoff, peer-limit eviction, subscriber backpressure drop accounting, and send retry after stale-session failures
- File-backed P2P address book for persistent peer discovery across restarts

### Validator and Committee

- In-memory and LevelDB-backed height-versioned validator registry
- Permissionless or whitelist-style admission policy
- Minimum stake policy
- Maximum validator count policy
- Deterministic committee selection by seed, epoch, and round
- Epoch calculation and rotation policy
- Validator set update and rotation support
- Persistent slashing lifecycle with evidence status, penalty receipts, jailing, and slashing-driven voting-power updates

### Mempool

- FIFO transaction pool
- Transaction validation boundaries
- Recently-seen transaction TTL for duplicate-gossip suppression
- Configurable minimum-fee admission
- Optional priority and fee-aware batch construction
- Batch construction
- DAG batch parent/tip tracking
- Duplicate batch and unknown parent rejection

### Ordering and Data Availability

- Height-salted deterministic transaction ordering
- Proposal-level transaction reordering rejection
- Lightweight MEV-resistant fair ordering hook
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
- Per-peer and global P2P flood limits
- Flood, overflow, duplicate, and invalid-transaction regression tests
- Strict single-document JSON decoding for RPC mutation endpoints
- Fuzz targets for signed transactions, consensus wire messages, data availability, fair ordering, mempool batching, and RPC request decoders

### Storage and Runtime

- Modular application runtime
- Signed transaction envelopes with ante checks for fee, gas, account nonce validation, fee collection, and gas-used result metadata
- Config-driven application module registry and builder
- Bank module with mint, send, balance query, and persisted state
- Block executor
- LevelDB block/state/state-root/evidence storage
- Versioned state lookup by height
- Retention-based pruning
- Index recovery after partial metadata loss
- LevelDB compaction hook
- Recovery, replay, snapshot, and restore helpers
- State sync snapshots with module KV payloads, chain metadata, and checksum verification
- Runtime validator update application
- Node config, genesis, data directory, and lifecycle skeleton
- Node-level transport reactor wiring for in-memory multi-node simulations

### Operations

- HTTP health, readiness, status, diagnostics, peer, block, state, validator, committee, and metrics endpoints
- Prometheus-style text metrics with uptime and process start timestamps
- Optional `/debug/pprof` endpoints
- Admin-token protection for mutation endpoints
- JSON or text startup logs with level, version, pid, and Go runtime metadata
- Deployment audit checks for production readiness
- Cross-platform release builds and checksum generation through `make release`
- Encrypted validator key files with passphrase-based loading
- Remote signer key documents for KMS/HSM-backed validator signing
- Snapshot export and restore commands
- Offline doctor command for config, key, store, snapshot, and index-recovery checks
- Localnet lifecycle commands and built-binary E2E coverage
- Persistent peer address book with dial-failure tracking, temporary bans, and non-permanent peer eviction
- Transport-level peer gate and consensus-gossip scoring for banned or malformed peer traffic
- Immediate peer disconnect and dial-set removal when score or address-book policy bans a peer
- Persistent peer-score snapshots so bans and rate-limit state survive node restarts

## Packages

| Package | Description |
|---|---|
| `app` | Application and module interfaces |
| `app/bank` | Bank balances, mint/send transactions, and balance queries |
| `app/modules` | Config-driven default application module registry and execution ante wiring |
| `cmd/vexod` | CLI entrypoint |
| `committee` | Committee selection and epoch rotation |
| `config` | Default chain configuration and validation |
| `consensus` | Consensus state machine, votes, proposals, QC, conflict evidence |
| `dataavailability` | Transaction data commitments and availability checks |
| `crypto` | Deterministic and Ed25519 signers, signature domain separation, aggregate verification, keyring rotation, and key files |
| `fairordering` | Height-salted deterministic transaction ordering |
| `finality` | Finality proofs and light-client verifier |
| `governance` | Proposal, voting, quorum, veto, and timelock module |
| `mempool` | FIFO mempool, fee/priority policy, duplicate suppression, and DAG batch graph |
| `node` | Node config, genesis, lifecycle, runtime/store wiring, operations helpers |
| `p2p` | Peer scoring, rate-limit, flood defense, and persistent address book |
| `rpc` | HTTP health, readiness, status, metrics, admin, pprof, and query endpoints |
| `runtime` | Module wiring, block execution, proof building, recovery, replay |
| `slashing` | Evidence validation, lifecycle tracking, penalty receipts, and keeper implementations |
| `store` | LevelDB-backed block, versioned state, state-root, evidence, KV, recovery, pruning, and compaction storage |
| `transport` | In-memory, TCP, and gRPC message transport with pub/sub interfaces |
| `types` | Shared primitive types |
| `validator` | In-memory and store-backed validator registries with admission policy |

## Documentation

Protocol specifications:

- [Consensus Spec](./docs/specs/consensus-spec.md)
- [Networking Spec](./docs/specs/networking-spec.md)
- [Storage Schema](./docs/specs/storage-schema.md)
- [Transaction Format](./docs/specs/tx-format.md)
- [Validator Lifecycle](./docs/specs/validator-lifecycle.md)
- [Finality Proof Format](./docs/specs/finality-proof-format.md)
- [Consensus Protocol Notes](./docs/consensus-protocol.md)

SDK and extension guides:

- [App Module Guide](./docs/sdk/app-module-guide.md)
- [Custom Crypto Backend Guide](./docs/sdk/custom-crypto-backend.md)
- [Custom Storage and Transport Guide](./docs/sdk/custom-storage-transport.md)
- [RPC API Versioning](./docs/sdk/rpc-api-versioning.md)

Security audit and release documents:

- [Security Audit Readiness](./docs/security/audit-readiness.md)
- [Release Pipeline](./docs/release/release-pipeline.md)
- [Version Compatibility Matrix](./docs/release/version-compatibility.md)

## Quick Start

### Requirements

- Go 1.26+

### Run the CLI

```bash
go run ./cmd/vexod
```

Run a local node with RPC and the consensus loop:

```bash
go run ./cmd/vexod start --home .vexo --run
```

Run with gRPC P2P enabled and persistent peers:

```bash
go run ./cmd/vexod start --home .vexo --run \
  --p2p-listen 0.0.0.0:26656 \
  --peer validator-2=127.0.0.1:26666 \
  --p2p-auth-token shared-secret
```

Bootstrap from a seed peer and learn additional peers through the gRPC handshake:

```bash
go run ./cmd/vexod start --home .vexo --run \
  --p2p-listen 0.0.0.0:26656 \
  --seed seed-1=127.0.0.1:36656 \
  --p2p-auth-token shared-secret
```

Learned P2P peers are saved to `.vexo/addrbook.json` by default. Repeatedly failing peers are temporarily banned and non-permanent banned peers can be evicted from the dial set:

```bash
go run ./cmd/vexod config paths --home .vexo
go run ./cmd/vexod start --home .vexo --run \
  --addr-book .vexo/addrbook.json \
  --addr-book-max-failures 3
```

Run with structured JSON operational logs and pprof:

```bash
go run ./cmd/vexod start --home .vexo --run --log-format json --log-level info --rpc-pprof
```

Generate a 4-validator localnet:

```bash
go run ./cmd/vexod init --home .vexo-localnet --chain-id vexo-local --validators 4
go run ./cmd/vexod start --home .vexo-localnet/validator-1 --run
```

Run and manage a local multi-node network:

```bash
go run ./cmd/vexod localnet up --home .vexo-localnet --validators 4 --overwrite --keep-running
go run ./cmd/vexod localnet up --home .vexo-testnet --validators 4 --p2p-base-port 27656 --rpc-base-port 27657 --overwrite

go run ./cmd/vexod localnet init --home .vexo-localnet --validators 4
go run ./cmd/vexod localnet start --home .vexo-localnet --validators 4
go run ./cmd/vexod localnet smoke --home .vexo-localnet --validators 4
go run ./cmd/vexod localnet status --home .vexo-localnet --validators 4
go run ./cmd/vexod localnet stop --home .vexo-localnet --validators 4
```

Plan sustained load, chaos, and multi-machine long-run validation:

```bash
go run ./cmd/vexod localnet load --validators 4 --duration 1h --rate 50 --dry-run
go run ./cmd/vexod localnet chaos-plan --validators 4 --duration 24h --regions 3
go run ./cmd/vexod localnet longrun-plan --validators 4 --duration 168h --regions 3 --hosts 4
```

Example output:

```text
vexo-consensus status
chain_id: vexo-local
application.modules: [bank]
execution.min_fee: 0
execution.min_gas: 0
execution.max_gas: 10000000
execution.require_nonce: false
execution.require_signed: false
execution.fee_collector: fee_collector
validator.permissionless: true
validator.min_stake: 1
committee.epoch_length: 100
committee.size: 128
mempool.max_txs: 100000
mempool.min_fee: 0
mempool.priority_enabled: false
fair_ordering.deterministic: true
fair_ordering.height_salted: true
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

Run consensus adversarial simulations:

```bash
go run ./cmd/vexod consensus adversarial
go run ./cmd/vexod consensus adversarial --json
```

Application modules can expose their own CLI commands, and enabled module commands appear in `vexod help`:

```bash
go run ./cmd/vexod bank tx mint alice 100
go run ./cmd/vexod bank tx send alice bob 25
go run ./cmd/vexod bank query balance alice
```

Initialize node files:

```bash
go run ./cmd/vexod init --home .vexo --chain-id vexo-local --validator validator-1
```

This writes `.vexo/config.json`, `.vexo/genesis.json`, and `.vexo/data`.

Application modules are selected in `.vexo/config.json`:

```json
{
  "chain": {
    "Application": {
      "Modules": ["bank"]
    }
  }
}
```

Validate node files:

```bash
go run ./cmd/vexod validate --home .vexo
```

Generate a validator key:

```bash
go run ./cmd/vexod keys gen --home .vexo
```

Generate an encrypted validator key:

```bash
VEXO_KEY_PASSPHRASE='change-me' go run ./cmd/vexod keys gen --home .vexo --encrypt --id validator-key-1 --active-from 1
```

Show the public validator key without printing the private key:

```bash
go run ./cmd/vexod keys show --home .vexo --json
```

Encrypted keys can be shown or loaded with `VEXO_KEY_PASSPHRASE` or `--passphrase`.

Sign a transaction envelope:

```bash
go run ./cmd/vexod keys sign-tx --home .vexo --chain-id vexo-local \
  --tx 'bank:send:alice:bob:25:fee=1:gas=1000:signer=alice:nonce=1'
```

Register a remote KMS/HSM signer instead of storing local private key material:

```bash
go run ./cmd/vexod keys remote --home .vexo \
  --public-key <base64-public-key> \
  --url http://127.0.0.1:9000/sign
```

Verify a remote KMS/HSM signer with a challenge signature:

```bash
go run ./cmd/vexod keys verify-remote --home .vexo --challenge mainnet-kms --json
```

Inspect paths and startup readiness:

```bash
go run ./cmd/vexod config paths --home .vexo --json
go run ./cmd/vexod config audit --home .vexo --json
go run ./cmd/vexod start --home .vexo --dry-run
```

Generate an external audit evidence checklist and mainnet parameter template:

```bash
go run ./cmd/vexod config audit-pack --json
go run ./cmd/vexod config mainnet-template --json
```

Strict production checks can be enforced before startup:

```bash
go run ./cmd/vexod config audit --home .vexo --strict
go run ./cmd/vexod start --home .vexo --run --strict-production \
  --rpc-admin-token <token> \
  --p2p-auth-token <token> \
  --rpc-max-request-bytes 1048576 \
  --rpc-rate-limit-max 100 \
  --p2p-max-message-bytes 1048576
```

Run a minimal block execution demo:

```bash
go run ./cmd/vexod demo
```

Run a LevelDB-backed storage demo:

```bash
go run ./cmd/vexod store-demo
```

Export and restore the latest persisted state snapshot:

```bash
go run ./cmd/vexod snapshot export --home .vexo --output snapshot.json
go run ./cmd/vexod snapshot verify --home .vexo-restore --input snapshot.json
go run ./cmd/vexod snapshot restore --home .vexo-restore --input snapshot.json
```

Fetch or directly sync a new node from a peer's RPC snapshot export. Snapshot sync restores state metadata, module state roots, and exported module KV pairs:

```bash
go run ./cmd/vexod snapshot fetch --url http://127.0.0.1:26657/snapshot/export --output snapshot.json
go run ./cmd/vexod snapshot sync --home .vexo-new --url http://127.0.0.1:26657/snapshot/export
```

Run an offline operational readiness check, optionally rebuilding LevelDB block/evidence indexes:

```bash
go run ./cmd/vexod doctor --home .vexo
go run ./cmd/vexod doctor --home .vexo --repair-indexes --json
```

Build the node binary:

```bash
make build
```

Build cross-platform release artifacts and checksums:

```bash
make release VERSION=0.1.0
ls dist/
```

Generate signed release metadata, SBOM, Docker image, and release-candidate evidence:

```bash
make sign-release VERSION=0.1.0
make docker-image VERSION=0.1.0 IMAGE=vexo-consensus IMAGE_TAG=0.1.0
make release-candidate VERSION=0.1.0-rc.1
```

Run all checks:

```bash
make check
```

Run operational verification commands:

```bash
make ops-verify
```

Print operational alert thresholds or evaluate a metrics sample:

```bash
go run ./cmd/vexod ops thresholds --json
go run ./cmd/vexod ops alerts \
  --height-rate 0 \
  --round-timeouts 10 \
  --proposal-latency 1s \
  --vote-latency 1s \
  --peer-bans 1 \
  --mempool-size 20000 \
  --commit-latency 2s \
  --signing-failures 1
```

Build a governance-driven binary/config/store/app-state migration plan:

```bash
go run ./cmd/vexod upgrade plan \
  --name v0.2.0 \
  --height 100000 \
  --binary-version v0.2.0 \
  --config-from 1 --config-to 2 \
  --store-from 1 --store-to 2 \
  --app-from 1 --app-to 2 \
  --proposal 42 \
  --rollback-binary v0.1.0
```

Run short fuzz smoke checks:

```bash
make fuzz-smoke
```

Build a container image:

```bash
docker build -t vexo-consensus .
```

Expose operational status over HTTP from an embedded node:

```go
server := rpc.NewServer(node, rpc.Config{Address: "127.0.0.1:26657"})
```

Available endpoints: `/healthz`, `/readyz`, `/status`, `/diagnostics`, `/recovery`, `/metrics`, `/metrics/text`, `/peers`, `/tx`, `/evidence`, `/prune`, `/replay`, `/consensus/start`, `/consensus/stop`, `/snapshot/latest`, `/blocks`, `/blocks/latest`, `/blocks/{height}`, `/state/latest`, `/state/{height}/{namespace}`, `/validators/{height}`, `/committee/{height}/{round}`.

When pprof is enabled, `/debug/pprof/*` is also available.

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

Run the built-binary 4-validator localnet E2E test:

```bash
VEXO_LOCALNET_E2E=1 go test ./cmd/vexod -run TestLocalnetUpBuiltBinaryE2E -count=1 -v
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

Consensus protocol details, safety invariants, finality verification rules, crypto backend boundaries, remote signer policy, recovery semantics, and extension guides are documented in the [Documentation](#documentation) section.

## Security Notice

`vexo-consensus` is experimental software.

Do not use it to secure real funds, production validator infrastructure, or critical systems without independent review, production cryptography, persistent state, real networking, and protocol-level audits.

## Contributing

Contributions are welcome.

Before opening a change, keep the current design rule in mind:

> Prefer small, testable modules over clever monoliths.

## License

`vexo-consensus` is released under the MIT License. See [LICENSE](./LICENSE).
