# 운영 가시성 가이드

> Locale: ko · 한국어
> 보안·릴리즈 판단은 영어 정본과 release gate 결과를 기준으로 확정합니다.

## 개요

이 문서는 노드가 건강한지 상태, metrics, logs, alert 기준으로 판단하는 방법을 설명합니다.

이 현지화 문서는 명령어, JSON 필드, RPC 메서드, 설정 키, 패키지 이름을 그대로 유지해 모든 언어에서 예제를 그대로 복사해 실행할 수 있게 합니다.

## 왜 중요한가

이 가이드는 RPC, metrics, logs, release evidence를 통해 Vexo 노드가 건강한지 판단하는 방법을 설명합니다.

## 반드시 확인할 것

- **Height and finality**: `latest_height`, `latest_finalized_height`, height rate, and finality proof availability show whether consensus and execution are progressing.
- **Peer health**: `peer_count` is compatibility summary; prefer `active_peer_count`, `configured_peer_count`, and `scored_peer_count` to separate live sessions from configured addresses.
- **Latency and timeout**: `round_timeouts`, proposal latency, vote latency, and commit latency show whether timeout values still fit the real network.
- **Execution pressure**: `mempool_size`, gas/base-fee behavior, tx count, and commit p95/p99 show whether block capacity and storage are under pressure.
- **Recovery readiness**: `snapshot_healthy`, `replay_healthy`, recovery reports, and state-root checks show whether a node can safely restart or sync.
- **Custody and safety**: `validator_signing_failures`, remote signer logs, ban spikes, and reconciliation failures require immediate operator review.

## 운영자가 할 일

- **Status flow**: Start with `/v1/status`, then compare `/v1/metrics`, `/metrics/text`, `/v1/diagnostics`, `/v1/finality/latest`, and recovery reports.
- **Alert flow**: Alert on stalled height, stalled finality, zero active peers, timeout spikes, high commit latency, mempool pressure, replay failure, and signer failures.
- **Incident flow**: Preserve logs, metrics, configs, genesis, binary hash, and evidence files before deleting data or restarting repeatedly.

## 그대로 유지할 인터페이스 이름

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

## 자주 하는 실수

- 설정된 peer가 실제로 연결됐다고 가정하지 마세요. 활성 세션을 별도로 확인해야 합니다.
- release evidence 없이 BLS, VRF, EVM, state sync, governance를 production-ready라고 부르지 마세요.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## 규범 원문

- [규범 원문](../../en/operators/observability.md)

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Core Endpoints — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Reading `/v1/status` — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Prometheus Metrics — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Suggested Alert Rules — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Suggested Starting Thresholds — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Incident Triage Matrix — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Log Events to Keep — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: First Response Playbook — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Dashboard Layout — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Release Evidence From Observability — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `/v1/status` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/metrics` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/metrics/text` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/diagnostics` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/finality/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/state/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/recovery/report` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/snapshot` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `latest_height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `latest_finalized_height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `latest_app_hash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `active_peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `configured_peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `scored_peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `banned_peers` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `banned_peers=0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_node_running` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_latest_height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_active_peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_configured_peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_scored_peer_count` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_banned_peers` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_height_rate_per_minute` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_round_timeouts` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_proposal_latency_p95_nanos` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_vote_latency_p95_nanos` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_commit_latency_p95_nanos` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_mempool_size` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_snapshot_healthy` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_replay_healthy` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_validator_signing_failures` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_post_commit_reconciliation_failures` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_node_running == 0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_active_peer_count == 0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_snapshot_healthy == 0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_replay_healthy == 0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_validator_signing_failures > 0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_post_commit_reconciliation_failures > 0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `timeout_propose` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `max_txs` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `node_running` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc_listening` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p_listening` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `peer_configured` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `peer_connected` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `peer_disconnected` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `peer_dial_failed` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `peer_banned` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_loop_running` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `block_committed` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `round_timeout` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `validator_signing_failure` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evidence_received` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evidence_applied` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `snapshot_exported` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `replay_checked` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `upgrade_halt` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `upgrade_applied` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `dist/` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
