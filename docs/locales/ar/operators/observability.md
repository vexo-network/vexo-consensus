# Observability Guide

> Locale: ar · العربية
> قرارات الأمان والإصدار تعتمد على المصدر الإنجليزي ونتيجة release gate.

## نظرة عامة

تشرح هذه الوثيقة كيف يقيّم المشغّل صحة عقدة Vexo عبر الحالة والمقاييس والسجلات والتنبيهات.

يحافظ هذا الدليل المحلي على الأوامر وحقول JSON وطرق RPC ومفاتيح الإعداد وأسماء الحزم كما هي، حتى تبقى الأمثلة قابلة للنسخ عبر اللغات.

## لماذا هذا مهم

Vexo combines BFT consensus, application modules, native accounting, optional EVM execution, validator economics, peer networking, and release evidence. A reader should be able to explain not just that a feature exists, but how to operate it safely and how to prove that it works on the target network.

## ما يجب التحقق منه

- **Height and finality**: `latest_height`, `latest_finalized_height`, height rate, and finality proof availability show whether consensus and execution are progressing.
- **Peer health**: `peer_count` is compatibility summary; prefer `active_peer_count`, `configured_peer_count`, and `scored_peer_count` to separate live sessions from configured addresses.
- **Latency and timeout**: `round_timeouts`, proposal latency, vote latency, and commit latency show whether timeout values still fit the real network.
- **Execution pressure**: `mempool_size`, gas/base-fee behavior, tx count, and commit p95/p99 show whether block capacity and storage are under pressure.
- **Recovery readiness**: `snapshot_healthy`, `replay_healthy`, recovery reports, and state-root checks show whether a node can safely restart or sync.
- **Custody and safety**: `validator_signing_failures`, remote signer logs, ban spikes, and reconciliation failures require immediate operator review.

## إجراءات المشغّل

- **Status flow**: Start with `/v1/status`, then compare `/v1/metrics`, `/metrics/text`, `/v1/diagnostics`, `/v1/finality/latest`, and recovery reports.
- **Alert flow**: Alert on stalled height, stalled finality, zero active peers, timeout spikes, high commit latency, mempool pressure, replay failure, and signer failures.
- **Incident flow**: Preserve logs, metrics, configs, genesis, binary hash, and evidence files before deleting data or restarting repeatedly.

## أسماء الواجهات التي لا تُترجم

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

## أخطاء شائعة

- Do not assume configured peers are connected peers; active sessions must be checked separately.
- Do not call BLS, VRF, EVM, state sync, or governance production-ready without release evidence.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## المرجع المعياري

- [المصدر المعياري](../../en/operators/observability.md)

<!-- vexo-docs:technical-parity -->
## ملحق التكافؤ التقني

يساعد هذا الملحق على ضمان أن الترجمة تحتفظ بالواجهات القابلة للتنفيذ والأقسام الأساسية من الوثيقة الإنجليزية المعتمدة. تبقى الأوامر ومفاتيح الإعداد وطرق RPC وأسماء الحزم كما هي في كل اللغات.

### تتبع الأقسام
- section: Core Endpoints — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Reading `/v1/status` — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Prometheus Metrics — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Suggested Alert Rules — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Suggested Starting Thresholds — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Incident Triage Matrix — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Log Events to Keep — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: First Response Playbook — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Dashboard Layout — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Release Evidence From Observability — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.

### واجهات تبقى دون تغيير
- `/v1/status` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/metrics` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/metrics/text` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/diagnostics` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/finality/latest` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/state/latest` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/recovery/report` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/snapshot` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `latest_height` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `latest_finalized_height` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `latest_app_hash` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `active_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `configured_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `scored_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `banned_peers` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `banned_peers=0` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_node_running` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_latest_height` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_active_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_configured_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_scored_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_banned_peers` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_height_rate_per_minute` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_round_timeouts` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_proposal_latency_p95_nanos` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_vote_latency_p95_nanos` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_commit_latency_p95_nanos` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_mempool_size` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_snapshot_healthy` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_replay_healthy` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_validator_signing_failures` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_post_commit_reconciliation_failures` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_node_running == 0` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_active_peer_count == 0` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_snapshot_healthy == 0` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_replay_healthy == 0` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_validator_signing_failures > 0` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_post_commit_reconciliation_failures > 0` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_propose` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `max_txs` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `node_running` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc_listening` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p_listening` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `peer_configured` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `peer_connected` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `peer_disconnected` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `peer_dial_failed` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `peer_banned` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `consensus_loop_running` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `block_committed` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `round_timeout` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `validator_signing_failure` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `evidence_received` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `evidence_applied` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `snapshot_exported` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `replay_checked` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `upgrade_halt` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `upgrade_applied` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `dist/` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
