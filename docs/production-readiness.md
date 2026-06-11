# Production Readiness Guide

This guide explains what must be true before a Vexo-based network should be described as production-ready.

It is written for chain teams, validators, security reviewers, and application developers who need a single map of the system instead of chasing behavior across packages. It does not replace the protocol specs or release gate; it tells you how to read them together.

## The Short Version

A Vexo network is production-ready only when four layers are ready at the same time:

1. **Protocol correctness**: consensus, finality, slashing, validator-set updates, state roots, and light-client proofs are configured and tested for the target validator set.
2. **Runtime correctness**: app modules, transaction validation, gas/fee policy, EVM/native accounting, storage commits, replay, pruning, and recovery are deterministic.
3. **Operational correctness**: keys, peer identity, RPC/admin access, observability, snapshots, upgrades, and release artifacts are managed by repeatable procedures.
4. **Evidence**: test results, long-run reports, chaos reports, audit outputs, dependency evidence, and conformance corpora are attached to the exact release candidate binary and config.

Code alone is not enough. The release process intentionally rejects plan-only, dry-run, placeholder, mock, built-in-only, or unpinned evidence for public release claims.

## How To Use This Guide

Read this guide in three passes:

1. **Architecture pass**: understand which subsystem owns each safety boundary. Do not tune config until you know whether a behavior belongs to consensus, runtime, storage, crypto, networking, or an app module.
2. **Operator pass**: walk the checklists with the exact genesis, split config files, validator keys, node keys, and binary that will be launched.
3. **Evidence pass**: attach machine-readable proof that the exact candidate was tested. Screenshots and manual notes are useful context, but the release gate should consume JSON/text artifacts with hashes.

If a feature is present in code but lacks release evidence, treat it as **not proven**. If a feature is documented but cannot be configured or tested, treat the document as a bug. If an exception is intentional, write it into the launch runbook with an owner and rollback condition.

## Readiness Levels

Vexo avoids config-level “dev/localnet/production mode” labels. A network is just a network. Readiness comes from the properties of the config, keys, evidence, and release artifact:

| Level | What It Means | Acceptable Use |
|---|---|---|
| Experimental | Feature can run, but safety evidence is missing or uses deterministic/test-only dependencies | local development and private research |
| Candidate | Config passes safety validation and CI-safe release checks, but long-run or external evidence is still being collected | private release candidate and validator rehearsal |
| Launch-ready | Exact binary/config/genesis passed network E2E, long-run, chaos, signer, economics, upgrade, state-sync, and EVM/Web3 evidence gates | public launch preparation |
| Operating | The live network has dashboards, alerts, incident process, upgrade policy, validator support, and archived release evidence | value-bearing operation |

Never promote a network by changing a mode flag. Promote it by replacing weak assumptions with verified evidence.

## System Map

| Area | Primary Packages | Primary Docs | What Must Be Proven |
|---|---|---|---|
| Consensus/finality | `consensus`, `finality`, `node`, `signbytes` | `docs/specs/consensus-spec.md`, `docs/specs/finality-proof-format.md` | No conflicting finality, correct three-chain commit proofs, domain-separated vote signatures |
| Validator lifecycle | `validator`, `slashing`, `modules/staking`, `governance` | `docs/specs/validator-lifecycle.md` | Height-versioned validator sets, safe rotation, slash/jail/unbonding accounting, governance-controlled changes |
| Runtime/app state | `app`, `runtime`, `modules/*`, `store`, `kvbatch` | `docs/sdk/app-module-guide.md`, `docs/specs/storage-schema.md` | Atomic staged commits, deterministic replay, schema compatibility, module-owned indexes |
| Transactions/economics | `app`, `economics`, `mempool`, `modules/bank`, `modules/staking` | `docs/specs/tx-format.md`, `docs/specs/evm-native-accounting.md` | Signed tx admission, nonce policy, base fee/blob base fee, gas accounting, fee collection/distribution |
| EVM/Web3 | `contract`, `modules/evm`, `rpc` | `docs/specs/evm-native-accounting.md`, `docs/sdk/rpc-api-versioning.md` | Geth-backed VM behavior, native balance integration, Web3 RPC conformance, external fixture corpus |
| Crypto/key custody | `crypto`, `cmd/vexod keys` | `docs/sdk/custom-crypto-backend.md`, `docs/security/audit-readiness.md` | BLS/VRF metadata, PoP/rogue-key defense, remote signer policy, double-sign guard, audit evidence |
| Networking | `transport`, `p2p`, `node` | `docs/specs/networking-spec.md`, `docs/operators/node-initialization.md` | Chain/genesis-bound handshake, durable replay protection, peer scoring, ban/backoff, rate limits |
| State sync/recovery | `store`, `runtime`, `node`, `queryproof` | `docs/specs/storage-schema.md`, `docs/release/launch-runbook.md` | Snapshot verification, replay health, pruning safety, latest safe height recovery |
| Operations/release | `ops`, `cmd/vexod`, `rpc` | `docs/operators/observability.md`, `docs/release/release-pipeline.md` | Metrics thresholds, logs, signed artifacts, SBOM, evidence manifest, rollback plan |

## Configuration Review Order

Review split config files before starting validators. Do not hide network behavior in command-line flags.

1. `config.json`: chain ID, validator ID, data directory, split config paths.
2. `network_config.json`: RPC/P2P listen addresses, advertised addresses, seeds, peers, TLS, admin auth, peer score bounds, replay path.
3. `consensus_config.json`: timeout values, empty-block policy, execution commit mode, crypto backend, validator admission, committee selection.
4. `module_config.json`: enabled app modules, execution gas/fee policy, governance policy, staking policy, EVM chain ID.
5. `mempool_config.json`: max txs, WAL path, minimum fee, priority/replacement behavior, recheck policy.
6. `log_config.json`: structured logs, block commit logs, peer event logs, audit/event sinks.

The safest operator workflow is: initialize node home, review all split config files, run `vexod validate --home <home>`, run `vexod config audit --home <home> --strict`, then start.

## Consensus and Finality Checklist

- `execution_commit` should be `finalized` for value-bearing networks.
- `allow_unsafe_qc_commit` must stay disabled outside controlled testing.
- `timeout_propose`, `timeout_prevote`, `timeout_precommit`, and `timeout_commit` must match real network latency.
- `create_empty_blocks` should be chosen intentionally. If disabled, transaction-triggered liveness must be tested.
- Validator-set hash must be included in headers and finality verification paths.
- Light-client verification must know which validator set verifies which height.
- Evidence handling must reject unsupported or context-missing proof types instead of applying penalties blindly.

## Runtime and Storage Checklist

- App writes, block records, state records, roots, validator updates, and module indexes should commit through the staged/batch path.
- Replay must use isolated state and compare results instead of mutating live state.
- Recovery must identify the last safe height and reconcile block/state/app metadata.
- Pruning must preserve whatever retention window is required for RPC history, EVM state snapshots, IBC proofs, and audit queries.
- Custom stores must implement atomic batch semantics when modules require multi-key writes.
- Schema versions must be checked on startup before accepting new blocks.
- Mempool WAL compaction should write a complete replacement file, fsync it, and atomically swap it into place. A compaction crash must not corrupt the current pending transaction log.
- Snapshot restore should validate checksums, chain ID, state height, state roots, and KV-derived roots before it writes into a node store. A tampered snapshot must fail before mutating state.

### Storage Failure Model

Operators should understand what happens if the machine dies at each boundary:

| Boundary | Required Behavior |
|---|---|
| During `CheckTx` | tx may be absent after restart unless WAL append completed |
| After WAL append before block inclusion | tx should replay as pending |
| During WAL compaction | old WAL or fully compacted WAL should survive; never a half-written replacement |
| During staged app execution | staged writes must be discarded if block execution fails |
| During block/state commit | app KV writes, block record, state record, roots, and validator update writes should land in one store batch |
| After block commit before process state update | recovery should read durable state and rebuild in-memory indexes |
| During snapshot restore | invalid documents must fail before writing; valid documents should be followed by index recovery |

This is the difference between “the database contains bytes” and “the node can safely decide its last committed height.”

## EVM and Native Coin Checklist

The built-in EVM module targets Ethereum execution and Web3 JSON-RPC semantics inside the Vexo network. It is not an Ethereum devp2p or Engine API node.

- Native Vexo balances and EVM balances are the same asset.
- EVM balance writes persist through the canonical bank namespace.
- Ethereum raw transactions are signed and verified with go-ethereum transaction rules.
- Gas, base fee, blob base fee, refunds, access lists, logs, receipts, and traces must be validated by fixture tests.
- `eth_getProof`, historical state reads, and pruning behavior must be tested for the chosen retention policy.
- Public release claims require external raw-transaction and geth VM execution corpora pinned by SHA-256.

### EVM Readiness Explained

Using go-ethereum as a library is the right maintenance boundary: when geth changes, Vexo should mostly update `go.mod` and keep the adapter surface stable. That does not automatically prove user-facing equivalence. The following paths still need evidence for every release candidate that claims EVM/Web3 readiness:

| Area | What To Prove |
|---|---|
| Raw tx decode | legacy, access-list, dynamic-fee, blob, set-code, chain ID, signature recovery, fee cap errors |
| Execution | call, create, create2, precompiles, refunds, access list warming, storage writes, logs, revert data, SELFDESTRUCT-era behavior for the configured hard fork |
| Native accounting | EVM value, gas fee, blob fee, refunds, and fee collector must move the same native Vexo asset used by bank/staking |
| RPC shape | block, receipt, transaction, fee history, filter, trace, proof, and estimate responses must match common Ethereum client expectations |
| Tooling | ethers, web3.js, MetaMask, Hardhat, Foundry, and indexers should pass the supported method set |
| History/pruning | `eth_getProof`, logs, receipts, traces, and historical state reads must either work inside the configured retention window or fail predictably |

Unsupported Ethereum namespaces are intentional when they refer to Ethereum node roles Vexo does not implement, such as devp2p mining or Engine API. Unsupported execution or JSON-RPC semantics inside the supported namespace are not intentional; they should be treated as conformance bugs.

## Crypto and Key Custody Checklist

- BLS validators need proof-of-possession metadata and audit evidence for the selected adapter.
- VRF committee selection needs verifiable proofs, audited metadata, and key-source evidence.
- Deterministic crypto must remain test-only.
- Remote signers must enforce height/round/type sign policy and keep a durable double-sign guard.
- Remote signer and remote VRF services need replay-nonce storage, auth, TLS/mTLS or pinned CA, audit logs, and key access controls.
- Operators should document key generation, backup, rotation, revocation, and incident response before joining a public network.

### Remote Signer Minimum Bar

A validator remote signer is allowed to be simple, but it must be strict:

1. The node must send chain ID, height, round, sign type, and domain with every consensus/finality signing request.
2. The signer must reject missing policy, wrong chain ID, disallowed sign type, out-of-window height, replayed nonce, and conflicting double-sign attempts.
3. The signer process must keep nonce and double-sign guard state on durable storage.
4. The transport must use at least bearer auth and should use TLS/mTLS or a pinned CA when it leaves localhost.
5. Signer logs should be retained as security evidence, but private key material, passphrases, and auth tokens must never be logged.

If any item is missing, run the validator as a private rehearsal only.

## Networking Checklist

- Peer identity is separate from validator identity.
- Handshakes must bind protocol version, network ID, chain ID, genesis hash, node ID, advertised address, and replay nonce.
- Public authenticated P2P should use a durable replay store.
- Peer scores should have a finite `max_score`, ban threshold, recovery rate, and ban duration.
- Status and metrics should be read using both active peer count and configured/scored peer count; configured peers are not proof of live connectivity.
- Seed/bootstrap policy should be documented before launch.

## Observability Checklist

Operators should alert on behavior, not only process liveness.

| Signal | Why It Matters | Typical Risk Pattern |
|---|---|---|
| `latest_height` and height rate | Liveness | Height stalls or drops below expected rate |
| finalized height | Safety/finality progress | Executed height grows but finalized height does not |
| active peer count | Network health | Configured peers exist but active sessions are zero |
| round timeouts | Latency/partition | Timeouts spike after deploy or region issue |
| proposal/vote latency | Consensus processing | Latency approaches timeout values |
| mempool size | Load/backpressure | Sustained growth with low commit throughput |
| commit latency | Runtime/storage pressure | Commit p95/p99 increases with block size |
| snapshot/replay health | Recovery readiness | Snapshot verification or replay diagnostics fail |
| signer failures | Key custody/KMS | Remote signer timeout, policy reject, or double-sign guard errors |
| banned peers | P2P attack/misconfig | Ban count spikes after peer rollout |

See `docs/operators/observability.md` for concrete RPC endpoints and metrics.

## Release Evidence Checklist

Before a production release candidate, collect evidence from the exact binary, config schema, genesis, and module set that will be launched.

- `make check`, fuzz smoke, adversarial simulation, network E2E.
- Long-run multi-host evidence with real load.
- Chaos evidence for restart, partition, packet loss, and region isolation.
- Snapshot/export/restore/replay evidence.
- KMS/HSM or remote signer evidence.
- P2P scale and state-sync/light-client evidence.
- Validator economics and governance-upgrade rehearsal evidence.
- MEV/fee-market policy evidence.
- SDK and EVM/Web3 conformance evidence, including external fixture corpora when EVM is enabled.
- External security audit and crypto adapter audit evidence.
- Signed checksums, SBOM, release manifest, and evidence manifest with SHA-256 bindings.

### Release Blockers

Stop the release if any of these are true:

- `release gate` fails or accepts only plan/dry-run/mock evidence.
- EVM/Web3 is enabled but external raw-transaction and execution fixture corpora are missing.
- BLS/VRF are configured but audit/dependency/key-source evidence is missing or digest-pinning does not match.
- Network E2E cannot start validators, submit a tx, increase height, and stop cleanly with the built binary.
- Long-run or chaos evidence cannot show height growth, no conflicting finality, snapshot/replay health, and signer health.
- Any validator key, node key, auth token, or private config was committed to the repository.
- Recovery report shows block/state/root mismatch on a candidate data directory.

Release blockers are not documentation tasks. Fix the code, config, or evidence first; then update documentation to describe the verified behavior.

## Common Failure Modes

| Symptom | Likely Cause | First Check |
|---|---|---|
| Height stays at zero | No proposals, no tx trigger, signer missing, validator not in set | `v1/status`, logs, validator key, consensus config |
| Peers configured but no gossip | Address not reachable, TLS/auth mismatch, genesis mismatch, replay store issue | `active_peer_count`, peer event logs, networking spec |
| Tx accepted locally but never committed | Mempool fee/nonce/gas mismatch or proposal filtering | `mempool` metrics, CheckTx response, base fee |
| EVM tooling sees missing namespace | Tool expects Ethereum full-node namespace | `vexo_web3Capabilities`, unsupported namespace list |
| Finality proof rejected | Wrong validator set, missing three-chain proof, wrong domain/sign bytes | finality proof format and validator-set hash |
| Recovery reports mismatch | App state, block record, or state root diverged | recovery report, replay diagnostics, storage schema |
| Release gate fails | Evidence missing, dry-run, placeholder, unpinned, or wrong category | release gate JSON and evidence manifest |

## What This Guide Does Not Claim

This guide does not claim a network is safe because these features exist. It gives the checklist used to decide whether the code, configuration, operators, evidence, and release candidate are aligned. If any item is not true for the target network, document the exception and do not market it as production-ready.
