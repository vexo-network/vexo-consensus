> Locale: ar · العربية

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- أقل من ثلث قوة التصويت البيزنطية
- توقيعات الاقتراح والتصويت ومهلة التصويت والنهائية المفصولة عن النطاق
- ربط تجزئة مجموعة المدقق بارتفاع الإثبات ذي الصلة
- الموقّعين المعروفين الفريدين في مراقبة الجودة والبراهين النهائية
- دليل خاضع للمساءلة على التباس المدقق
- رفض قرارات الالتزام المتضاربة بنفس الارتفاع النهائي

## حدود العملات المشفرة

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- تدقيق صارم للتكوين لكل منزل مدقق
- أدلة بوابة الإصدار
- مراجعة الأمن الخارجي
- أدلة متعددة المضيفين على المدى الطويل والفوضى
- أدلة سياسة الموقع/نظام إدارة المعرفة
- استعراض السياسات الاقتصادية والحوكمة الخاصة بسلسلة محددة

راجع [جاهزية تدقيق الأمن](./security/audit-readiness.md) و [خط أنابيب الإصدار](./release/release-pipeline.md) قبل التعامل مع الإصدار على أنه جاهز للإنتاج.
<!-- vexo-docs:technical-parity -->
## ملحق التكافؤ التقني

يحافظ هذا الملحق على المصطلحات التقنية والواجهات التي لا يجب أن تتغير بين النسخة المرجعية والترجمة.

### تتبع الأقسام
- section: Model - يجب قراءة HotStuff وfinality ثلاثية السلسلة وQC وtimeout certificate وlocked-QC safety معًا.
- section: Execution Terms - يجب أن يبقى الفرق بين qc certified وfinalized وexecuted وstate committed واضحًا.
- section: Safety Boundary - تحقق من حد byzantine الأقل من الثلث، وdomain separation، وربط hash الخاص بـ validator-set، وaccountable evidence.
- section: Crypto Boundary - احتفظ بالمحددات `deterministic` و`ed25519` و`bls` و`blst-bls12381-minpk-v1` و`ecvrf-p256-sha256-tai-v1`.
- section: Operational Boundary - راقب `vexo_quorum_health_ratio` و`adaptive_round_timeout_enabled` و`recovery_finality_gate_enabled` وإشارات snapshot/replay معًا.
- يجب أن يبقى `require_network_safety` و`block_committed` ظاهرين كما هما في الترجمة.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### الواجهات التي يجب الحفاظ عليها
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## ملاحظات تشغيلية

عند إنشاء منزل validator جديد، افحص `config.json` مع `module_config.json` و`network_config.json` و`consensus_config.json` و`mempool_config.json` و`log_config.json`.
في الإنتاج، يجب مراقبة `vexo_quorum_health_ratio` و`adaptive_round_timeout_enabled` معًا.

- `execution_commit=finalized` له الأولوية.
- يجب تفعيل `qc` فقط في شبكات اختبار مضبوطة.
- يجب التحقق من `recovery_finality_gate_enabled` مع أدلة snapshot و replay.
