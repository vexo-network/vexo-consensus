# دليل جاهزية الإنتاج

> Locale: ar · العربية
> قرارات الأمان والإصدار تعتمد على المصدر الإنجليزي ونتيجة release gate.

## نظرة عامة

تشرح هذه الوثيقة ما يجب التحقق منه قبل وصف شبكة مبنية على Vexo بأنها جاهزة للإنتاج.

يحافظ هذا الدليل المحلي على الأوامر وحقول JSON وطرق RPC ومفاتيح الإعداد وأسماء الحزم كما هي، حتى تبقى الأمثلة قابلة للنسخ عبر اللغات.

## لماذا هذا مهم

يجمع Vexo بين إجماع BFT، ووحدات التطبيق، والمحاسبة الأصلية، وتنفيذ EVM الاختياري، واقتصاديات المدققين، وشبكة الأقران، وأدلة الإطلاق. يجب أن يتمكن القارئ من شرح ليس فقط وجود الميزة، بل كيفية تشغيلها بأمان وكيفية إثبات عملها على الشبكة المستهدفة.

## ما يجب التحقق منه

- **Protocol correctness**: `consensus`, `finality`, validator-set hash, vote sign bytes, timeout certificate, and three-chain finality must agree for the target validator set.
- **Runtime correctness**: `app`, `runtime`, `store`, and module writes must commit atomically, replay deterministically, and recover from crash boundaries.
- **Crypto custody**: BLS, VRF, remote signer, KMS/HSM, proof-of-possession, replay nonce, and double-sign guard evidence must match the release binary.
- **Networking safety**: `network_config.json` must bind chain ID, genesis hash, node ID, TLS/auth policy, durable replay path, peer scoring, ban, and backoff settings.
- **EVM/native accounting**: The EVM module uses the native Vexo coin as the balance asset; gas, base fee, blob base fee, receipts, proofs, and traces must pass external corpora.
- **Release evidence**: Release claims need signed artifacts, SBOM, evidence manifest, longrun, chaos, E2E, state sync, economics, governance, MEV, SDK, and EVM/Web3 evidence.

## إجراءات المشغّل

- **System view**: A Vexo network is safe only when protocol, runtime, operations, and evidence are ready together. Do not treat enabled code as a production claim.
- **Configuration review**: Review `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before `vexod start`.
- **Release decision**: Attach evidence from the exact binary, genesis, config schema, module set, and validator topology that will be released.

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

- لا تفترض أن الأقران المُهيّئين متصلون فعلاً؛ يجب التحقق من الجلسات النشطة بشكل منفصل.
- لا تُعلن BLS أو VRF أو EVM أو state sync أو الحوكمة جاهزة للإنتاج دون أدلة إطلاق.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## المرجع المعياري

- [المصدر المعياري](../en/production-readiness.md)

<!-- vexo-docs:technical-parity -->
## ملحق التكافؤ التقني

يساعد هذا الملحق على ضمان أن الترجمة تحتفظ بالواجهات القابلة للتنفيذ والأقسام الأساسية من الوثيقة الإنجليزية المعتمدة. تبقى الأوامر ومفاتيح الإعداد وطرق RPC وأسماء الحزم كما هي في كل اللغات.

### تتبع الأقسام
- section: The Short Version — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: How To Use This Guide — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Readiness Levels — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: System Map — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Configuration Review Order — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Consensus and Finality Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Runtime and Storage Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: EVM and Native Coin Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Crypto and Key Custody Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Networking Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Observability Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Release Evidence Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Common Failure Modes — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: What This Guide Does Not Claim — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.

### واجهات تبقى دون تغيير
- `docs/specs/consensus-spec.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/specs/finality-proof-format.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `modules/staking` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/specs/validator-lifecycle.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `modules/*` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/sdk/app-module-guide.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/specs/storage-schema.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `modules/bank` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/specs/tx-format.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/specs/evm-native-accounting.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `modules/evm` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/sdk/rpc-api-versioning.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `cmd/vexod keys` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/sdk/custom-crypto-backend.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/security/audit-readiness.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/specs/networking-spec.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/operators/node-initialization.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/release/launch-runbook.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `cmd/vexod` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/operators/observability.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/release/release-pipeline.md` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `network_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc.tls_cert_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc.tls_key_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc.tls_ca_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `consensus_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `module_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `mempool_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `log_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexod validate --home <home>` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexod config audit --home <home> --strict` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `execution_commit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `allow_unsafe_qc_commit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_propose` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_prevote` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_precommit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_commit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `create_empty_blocks` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `eth_getProof` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `go.mod` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `max_score` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `latest_height` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `make check` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `v1/status` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `active_peer_count` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexo_web3Capabilities` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
