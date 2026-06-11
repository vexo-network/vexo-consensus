# Observability Guide

This guide explains how to tell whether a Vexo node is healthy from RPC, metrics, logs, and release evidence.

It is written for operators who need practical signals: what to watch, what each number means, and when a value should be treated as dangerous.

## Core Endpoints

| Endpoint | Use |
|---|---|
| `/v1/status` | Fast process, height, app hash, finality, and peer summary |
| `/v1/metrics` | JSON metrics for dashboards and automation |
| `/metrics/text` | Prometheus-compatible text metrics |
| `/v1/diagnostics` | Combined readiness, capabilities, status, peers, storage, and metrics checks |
| `/v1/finality/latest` | Latest finality proof for light-client and safety checks |
| `/v1/state/latest` | Latest state root and validator-set binding |
| `/v1/recovery/report` | Crash/restart consistency diagnostics |
| `/v1/snapshot` | Snapshot health and export metadata |

Admin endpoints such as prune, replay, and consensus control must be protected with scoped admin tokens and should normally be reachable only through an operator network or authenticated gateway.

## Reading `/v1/status`

Important fields:

| Field | Meaning | Operator Note |
|---|---|---|
| `running` | Node process has started and owns runtime state | `true` does not prove consensus liveness by itself |
| `latest_height` | Latest locally committed app height | Must increase over time on a live validator network |
| `latest_finalized_height` | Latest HotStuff three-chain finalized height | Should not lag indefinitely behind executed/committed height |
| `latest_app_hash` | App commit hash | Should match peers at the same height |
| `peer_count` | Backward-compatible connected/scored peer summary | Prefer the more specific peer fields below |
| `active_peer_count` | Active transport sessions, when the transport can report them | Best quick signal for live P2P connectivity |
| `configured_peer_count` | Configured or learned peer addresses | Reachability is not guaranteed |
| `scored_peer_count` | Peers known to the score table | Useful for bans/rate-limit history, not proof of live sessions |
| `banned_peers` | Peers currently banned by score policy | Spikes indicate attack, bad peer config, or too-strict limits |

Healthy example for a 4-validator single-host network: `running=true`, `latest_height` increasing, `latest_finalized_height` present, `active_peer_count` near `3`, and `banned_peers=0`.

## Prometheus Metrics

The text endpoint exposes gauges such as:

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`

`vexo_peer_count` is kept for older dashboards. New dashboards should chart `vexo_active_peer_count`, `vexo_configured_peer_count`, and `vexo_scored_peer_count` separately.

## Suggested Alert Rules

Tune numbers for the actual validator count, block interval, latency, and hardware. These are starting points, not universal constants.

| Alert | Starting Condition | Why |
|---|---|---|
| Node down | `vexo_node_running == 0` for 1 minute | Process/runtime stopped |
| Height stalled | `latest_height` unchanged for 2-3 expected block intervals | Consensus or execution stalled |
| Finality stalled | `latest_finalized_height` unchanged while blocks keep executing | Finality path or quorum issue |
| No active peers | `vexo_active_peer_count == 0` for 1 minute on a non-isolated node | P2P outage, auth mismatch, or address problem |
| Peer count too low | active peers below quorum connectivity target | Partition or bootstrap problem |
| Round timeout spike | timeout counter grows faster than normal baseline | Latency, proposer failure, or network partition |
| Commit latency high | p95/p99 approaches consensus timeout budget | Store/runtime overload |
| Mempool pressure | mempool size grows for several minutes | Fee policy, spam, or block capacity issue |
| Snapshot unhealthy | `vexo_snapshot_healthy == 0` | State sync/recovery risk |
| Replay unhealthy | `vexo_replay_healthy == 0` | Determinism or state consistency risk |
| Signer failures | `vexo_validator_signing_failures > 0` | KMS/remote signer/policy failure |
| Reconciliation failures | `vexo_post_commit_reconciliation_failures > 0` | Durable evidence or commit repair needed |
| Banned peer spike | banned peers rises suddenly | Attack, misconfigured peers, or scoring threshold issue |

## Log Events to Keep

Structured logs should be retained with node ID, validator ID, chain ID, height, round, block hash, and peer ID where relevant.

Important events:

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

For release candidates, archive logs together with metrics samples, pprof samples, config files, genesis, binary checksums, and evidence manifests.

## First Response Playbook

When an operator sees a problem:

1. Check `/v1/status` on at least two validators.
2. Compare `latest_height`, `latest_finalized_height`, `latest_app_hash`, and peer counts.
3. Check `/v1/diagnostics` for missing capabilities or unhealthy storage/replay/snapshot checks.
4. Inspect peer event logs for auth, TLS, genesis, chain ID, or backoff errors.
5. Inspect mempool and base-fee metrics if txs are not included.
6. Verify signer and remote signer logs if validator signatures fail.
7. Export recovery report before deleting or modifying data.
8. If finality conflict is suspected, stop automation, preserve logs/evidence, and run finality conflict detection.

## Dashboard Layout

A useful dashboard usually has five rows:

1. **Liveness**: node running, latest height, finalized height, height rate.
2. **Consensus latency**: round timeouts, proposal/vote/commit p95 and p99.
3. **Network**: active/configured/scored peers, banned peers, peer window messages.
4. **Execution**: mempool size, gas/base fee, tx count, commit latency.
5. **Recovery and safety**: snapshot health, replay health, signer failures, reconciliation failures.

Keep dashboards boring. The goal is not to show every internal counter; it is to make dangerous states obvious before validators diverge or users notice stalled transactions.
