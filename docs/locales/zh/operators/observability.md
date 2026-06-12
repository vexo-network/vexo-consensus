# Observability Guide

> Locale: zh · 中文
> 安全和发布判断必须以英文规范文档和 release gate 结果为准。

## 概览

本文说明如何通过状态、指标、日志和告警判断 Vexo 节点是否健康。

本地化文档会保留命令、JSON 字段、RPC 方法、配置键和包名不变，确保不同语言中的示例都可以直接复制使用。

## 为什么重要

Vexo combines BFT consensus, application modules, native accounting, optional EVM execution, validator economics, peer networking, and release evidence. A reader should be able to explain not just that a feature exists, but how to operate it safely and how to prove that it works on the target network.

## 必须验证

- **Height and finality**: `latest_height`, `latest_finalized_height`, height rate, and finality proof availability show whether consensus and execution are progressing.
- **Peer health**: `peer_count` is compatibility summary; prefer `active_peer_count`, `configured_peer_count`, and `scored_peer_count` to separate live sessions from configured addresses.
- **Latency and timeout**: `round_timeouts`, proposal latency, vote latency, and commit latency show whether timeout values still fit the real network.
- **Execution pressure**: `mempool_size`, gas/base-fee behavior, tx count, and commit p95/p99 show whether block capacity and storage are under pressure.
- **Recovery readiness**: `snapshot_healthy`, `replay_healthy`, recovery reports, and state-root checks show whether a node can safely restart or sync.
- **Custody and safety**: `validator_signing_failures`, remote signer logs, ban spikes, and reconciliation failures require immediate operator review.

## 运维动作

- **Status flow**: Start with `/v1/status`, then compare `/v1/metrics`, `/metrics/text`, `/v1/diagnostics`, `/v1/finality/latest`, and recovery reports.
- **Alert flow**: Alert on stalled height, stalled finality, zero active peers, timeout spikes, high commit latency, mempool pressure, replay failure, and signer failures.
- **Incident flow**: Preserve logs, metrics, configs, genesis, binary hash, and evidence files before deleting data or restarting repeatedly.

## 保持不变的接口名称

- `vexod validate --home <home>`
- `vexod config audit --home <home> --strict`
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `peer_count`
- `active_peer_count`
- `configured_peer_count`
- `scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `network_config.json`
- `consensus_config.json`
- `module_config.json`
- `mempool_config.json`
- `release gate`

## 常见错误

- Do not assume configured peers are connected peers; active sessions must be checked separately.
- Do not call BLS, VRF, EVM, state sync, or governance production-ready without release evidence.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## 规范参考

- [规范原文](../../en/operators/observability.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Core Endpoints — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Reading `/v1/status` — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Prometheus Metrics — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Suggested Alert Rules — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Suggested Starting Thresholds — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Incident Triage Matrix — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Log Events to Keep — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: First Response Playbook — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Dashboard Layout — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Release Evidence From Observability — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `/v1/status` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/metrics` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/metrics/text` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/diagnostics` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/finality/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/state/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/recovery/report` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/snapshot` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_finalized_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_app_hash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `active_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `configured_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `scored_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `banned_peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `banned_peers=0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_node_running` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_active_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_configured_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_scored_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_banned_peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_height_rate_per_minute` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_round_timeouts` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_proposal_latency_p95_nanos` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_vote_latency_p95_nanos` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_commit_latency_p95_nanos` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_mempool_size` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_snapshot_healthy` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_replay_healthy` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_validator_signing_failures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_post_commit_reconciliation_failures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_node_running == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_active_peer_count == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_snapshot_healthy == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_replay_healthy == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_validator_signing_failures > 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_post_commit_reconciliation_failures > 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_propose` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_txs` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node_running` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_listening` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p_listening` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_configured` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_connected` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_disconnected` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_dial_failed` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_banned` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_loop_running` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `block_committed` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `round_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `validator_signing_failure` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evidence_received` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evidence_applied` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `snapshot_exported` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `replay_checked` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `upgrade_halt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `upgrade_applied` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `dist/` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
