# Observability Guide

This guide explains how to tell whether a Vexo node is healthy from RPC, metrics, logs, and release evidence.

It is written for operators who need practical signals: what to watch, what each number means, and when a value should be treated as dangerous.

## At a Glance

If a node looks wrong, check these in order:

1. `running` and `latest_height` in `/v1/status`
2. `latest_finalized_height` and peer counts
3. `round_timeout`, proposal/vote latency, mempool size, and commit latency metrics
4. signer failures, snapshot health, and replay health
5. peer bans and peer-dial failures

That order matters because it separates “the process is alive” from “the chain is actually making safe progress.”

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

Admin endpoints such as prune, replay, and consensus control should normally be reachable only through loopback, an operator network, mTLS, or an authenticated gateway. Scoped admin tokens remain optional and are enforced when configured.

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
- `vexo_adaptive_round_timeout_enabled`
- `vexo_round_timeouts`
- `vexo_adaptive_round_timeout_nanos`
- `vexo_recovery_finality_gate_enabled`
- `vexo_recovery_finality_deferrals`
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
| Adaptive policy off | `vexo_adaptive_round_timeout_enabled == 0` on a node that should be running adaptive pacing | Config or experiment disabled the pacemaker |
| Adaptive timeout high | `vexo_adaptive_round_timeout_nanos` grows well above the launch baseline | Network latency spike or slower quorum formation |
| Missing peers widen timeout | `vexo_active_peer_count` falls below `vexo_configured_peer_count` and the adaptive timeout rises | Quorum health is degrading and the pacemaker is compensating |
| Commit latency high | p95/p99 approaches consensus timeout budget | Store/runtime overload |
| Mempool pressure | mempool size grows for several minutes | Fee policy, spam, or block capacity issue |
| Snapshot unhealthy | `vexo_snapshot_healthy == 0` | State sync/recovery risk |
| Replay unhealthy | `vexo_replay_healthy == 0` | Determinism or state consistency risk |
| Signer failures | `vexo_validator_signing_failures > 0` | KMS/remote signer/policy failure |
| Reconciliation failures | `vexo_post_commit_reconciliation_failures > 0` | Durable evidence or commit repair needed |
| Recovery gate off | `vexo_recovery_finality_gate_enabled == 0` on a node expected to enforce recovery gating | Finalized commits may bypass the recovery safety gate |
| Recovery deferrals | `vexo_recovery_finality_deferrals` increases | Finality commits are being deferred by a recovery mismatch |
| Banned peer spike | banned peers rises suddenly | Attack, misconfigured peers, or scoring threshold issue |

## Suggested Starting Thresholds

Use these as initial alert values, then tune after a real long-run baseline:

| Signal | Warning | Critical | First Action |
|---|---:|---:|---|
| Height rate | below 50% of expected for 2 windows | zero growth for 2-3 block intervals | compare all validators, check proposer/signing/peer logs |
| Finalized height lag | grows for 5 minutes | grows while executed height keeps increasing for 10 minutes | inspect QC/finality proof logs and validator-set hash |
| Active peers | below quorum connectivity target | zero active peers | check advertised address, TLS/auth, genesis/chain ID mismatch |
| Round timeouts | 3x normal baseline | continuous timeout loop | raise timeout budget or investigate latency/partition |
| Proposal latency p95 | above 50% of `timeout_propose` | above 80% of `timeout_propose` | profile proposer, mempool, DA commitment, disk |
| Vote latency p95 | above 50% of prevote/precommit budget | above 80% of budget | inspect CPU, signer, transport, gossip backpressure |
| Commit latency p95 | above 50% of block interval | above 80% of block interval | inspect LevelDB, state roots, EVM execution, snapshots |
| Mempool size | increasing for 5 minutes | near `max_txs` or sustained replacement churn | inspect base fee, min fee, tx validity, spam |
| Signer failures | any non-zero value | repeated failures in one height window | stop validator if double-sign guard or key mismatch appears |
| Snapshot health | one failed check | repeated failed export/verify/restore | pause state-sync serving and run recovery report |
| Replay health | one strict replay failure | replay mismatch at latest safe height | preserve data dir and halt unsafe upgrade/release |
| Banned peers | sudden spike | many peers banned after config rollout | check score caps, TLS CA, peer identity, optional auth proof, and clock skew |

The most important rule: alert on **change over time**. A single number can be misleading; height rate, finality lag, peer churn, mempool growth, and signer failures together tell the real story.

## Incident Triage Matrix

| Situation | Likely Layer | What To Preserve | Safe Next Step |
|---|---|---|---|
| Height stopped, peers healthy | consensus/signer/runtime | consensus logs, signer logs, mempool sample | verify proposer key and round timeout logs |
| Peers dropped after deploy | networking/config | network config, TLS certs, addrbook, peer logs | roll back advertised address/TLS/auth change |
| App hashes differ at same height | execution/storage | data dirs, block records, app logs, replay output | halt affected nodes and run strict replay |
| Finality proof rejected | finality/validator set | proof JSON, validator set at proof height | verify validator-set hash and sign bytes domain |
| Snapshot restore fails | state sync/storage | snapshot file, checksum, state roots, restore logs | do not retry against live data; restore into clean dir |
| Remote signer rejects requests | key custody | signer audit log, guard file, nonce file, node logs | distinguish policy rejection from transport outage |
| Banned peers spike | P2P/security | peer score snapshots and ban reasons | inspect malformed gossip or shared wrong config |

During incidents, prefer preserving data over “cleaning up.” Deleting WALs, addrbooks, signer guards, or LevelDB directories can destroy the evidence needed to distinguish a bug from operator error.

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

## Release Evidence From Observability

For a release candidate, observability is not just live monitoring. It becomes evidence:

1. Collect baseline `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest`, and `/v1/recovery/report` from every validator.
2. Run load for the chosen duration and rate.
3. Inject at least one restart, one peer disruption, and one snapshot export/verify/restore drill.
4. Collect final metrics from every validator.
5. Store the before/after samples, logs, pprof samples, signer audit logs, and evidence manifest in `dist/`.

A good evidence bundle lets a reviewer answer: did height grow, did finality progress, did peers recover, did txs commit, did snapshots verify, did replay stay healthy, did signers avoid double-signing, and did the exact release binary produce the results?
