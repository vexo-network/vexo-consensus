> Locale: ar · العربية

# نظرة عامة على بروتوكول الإجماع

هذه الصفحة هي المدخل العام لوثائق إجماع Vexo. توجد التفاصيل المعيارية في [Consensus Spec](./specs/consensus-spec.md) و[Finality Proof Format](./specs/finality-proof-format.md) و[Validator Lifecycle](./specs/validator-lifecycle.md) و[Storage Schema](./specs/storage-schema.md) و[Networking Spec](./specs/networking-spec.md) و[Transaction Format](./specs/tx-format.md).

## النموذج

يستخدم Vexo نواة BFT على نمط HotStuff تتضمن proposal وvote وquorum certificate(QC) وtimeout certificate وقاعدة locked-QC ونهائية السلاسل الثلاث. يكون التصويت للكتلة آمنا فقط عندما تمتد من locked QC أو تحمل justify QC لا يقل حداثة عنه. ترفض سلاسل QC الاصطناعية أو التي تتجاوز ارتفاعا من دون ربط صريح لارتفاعات وتجزئات الكتلة والأصل والجد قبل تقرير النهائية.

## هوية البروتوكول وحد البحث

Vexo ليس اسما جديدا لـ HotStuff غير المعدل، ولا يطابق بروتوكول أو تنفيذ AptosBFT أو DiemBFT أو Jolteon أو Ditto أو Tendermint أو CometBFT. فهو يجمع في Go runtime مستقل مفاهيم أمان HotStuff مع توقيت جولة متكيف واسترداد دائم وترتيب حتمي للمعاملات وتنفيذ معياري وvalidator sets ذات إصدارات حسب الارتفاع.

يستخدم مسار التصويت الفعلي validator set الكامل للارتفاع وproposer حتميا. يتوفر VRF committee selector كمكون وواجهة استعلام، لكنه لا يتحكم بعد في proposal eligibility أو quorum formation. لذلك يوصف كعمل مستقبلي لا كخاصية مفعلة. راجع [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) لحدود المساهمة وبروتوكول التجربة.

## حد التنفيذ والاسترداد

شهادة QC ونهائية HotStuff وتنفيذ التطبيق وcommit الحالة أحداث منفصلة. ينفذ الإعداد الافتراضي `execution_commit=finalized` السلف الذي تختاره قاعدة السلاسل الثلاث فقط. يتحكم pacemaker المتكيف و`recovery_finality_gate_enabled` في التأخير والاسترداد من دون تغيير proposer أو quorum power أو safe-vote أو قاعدة النهائية.

## حد السلامة

- أقل من ثلث قوة التصويت البيزنطية
- توقيعات الاقتراح والتصويت ومهلة التصويت والنهائية المفصولة عن النطاق
- ربط تجزئة مجموعة المدقق بارتفاع الإثبات ذي الصلة
- الموقّعين المعروفين الفريدين في مراقبة الجودة والبراهين النهائية
- دليل خاضع للمساءلة على التباس المدقق
- رفض قرارات الالتزام المتضاربة بنفس الارتفاع النهائي

## حدود العملات المشفرة

- backend المسمى `deterministic` مخصص للاختبار ولا يجتاز تحقق network safety.
- يدعم `ed25519` اختبارات الشبكات العامة والاستعداد للإطلاق.
- يستخدم `bls` افتراضيا `blst-bls12381-minpk-v1` ويتطلب proof-of-possession وفحص المجموعة الفرعية والتحقق من المفتاح وتدقيق الاعتماديات ودليل release-gate.
- يتطلب التحقق بيانات VRF adapter الوصفية، لكن ذلك لا يعني أن VRF committee جزء من مسار الإجماع الفعلي.

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
