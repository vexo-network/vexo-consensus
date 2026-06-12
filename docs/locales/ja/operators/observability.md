# Observability Guide

> Locale: ja · 日本語
> セキュリティとリリース判断は英語の正本と release gate の結果で確定します。

## 概要

この文書は、状態、メトリクス、ログ、アラートで Vexo ノードの健全性を判断する方法を説明します。

このローカライズ文書では、コマンド、JSON フィールド、RPC メソッド、設定キー、パッケージ名をそのまま残し、どの言語でも例をコピーして使えるようにします。

## なぜ重要か

Vexo combines BFT consensus, application modules, native accounting, optional EVM execution, validator economics, peer networking, and release evidence. A reader should be able to explain not just that a feature exists, but how to operate it safely and how to prove that it works on the target network.

## 必ず確認すること

- **Height and finality**: `latest_height`, `latest_finalized_height`, height rate, and finality proof availability show whether consensus and execution are progressing.
- **Peer health**: `peer_count` is compatibility summary; prefer `active_peer_count`, `configured_peer_count`, and `scored_peer_count` to separate live sessions from configured addresses.
- **Latency and timeout**: `round_timeouts`, proposal latency, vote latency, and commit latency show whether timeout values still fit the real network.
- **Execution pressure**: `mempool_size`, gas/base-fee behavior, tx count, and commit p95/p99 show whether block capacity and storage are under pressure.
- **Recovery readiness**: `snapshot_healthy`, `replay_healthy`, recovery reports, and state-root checks show whether a node can safely restart or sync.
- **Custody and safety**: `validator_signing_failures`, remote signer logs, ban spikes, and reconciliation failures require immediate operator review.

## 運用者の作業

- **Status flow**: Start with `/v1/status`, then compare `/v1/metrics`, `/metrics/text`, `/v1/diagnostics`, `/v1/finality/latest`, and recovery reports.
- **Alert flow**: Alert on stalled height, stalled finality, zero active peers, timeout spikes, high commit latency, mempool pressure, replay failure, and signer failures.
- **Incident flow**: Preserve logs, metrics, configs, genesis, binary hash, and evidence files before deleting data or restarting repeatedly.

## 変更しないインターフェイス名

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

## よくある間違い

- Do not assume configured peers are connected peers; active sessions must be checked separately.
- Do not call BLS, VRF, EVM, state sync, or governance production-ready without release evidence.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## 規範参照

- [規範となる原文](../../en/operators/observability.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Core Endpoints — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Reading `/v1/status` — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Prometheus Metrics — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Suggested Alert Rules — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Suggested Starting Thresholds — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Incident Triage Matrix — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Log Events to Keep — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: First Response Playbook — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Dashboard Layout — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Release Evidence From Observability — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `/v1/status` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/metrics` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/metrics/text` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/diagnostics` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/finality/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/state/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/recovery/report` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/snapshot` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `latest_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `latest_finalized_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `latest_app_hash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `active_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `configured_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `scored_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `banned_peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `banned_peers=0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_node_running` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_latest_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_active_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_configured_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_scored_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_banned_peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_height_rate_per_minute` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_round_timeouts` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_proposal_latency_p95_nanos` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_vote_latency_p95_nanos` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_commit_latency_p95_nanos` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_mempool_size` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_snapshot_healthy` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_replay_healthy` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_validator_signing_failures` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_post_commit_reconciliation_failures` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_node_running == 0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_active_peer_count == 0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_snapshot_healthy == 0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_replay_healthy == 0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_validator_signing_failures > 0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_post_commit_reconciliation_failures > 0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_propose` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `max_txs` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node_running` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_listening` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_listening` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `peer_configured` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `peer_connected` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `peer_disconnected` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `peer_dial_failed` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `peer_banned` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_loop_running` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `block_committed` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `round_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator_signing_failure` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evidence_received` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evidence_applied` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `snapshot_exported` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `replay_checked` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `upgrade_halt` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `upgrade_applied` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `dist/` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
