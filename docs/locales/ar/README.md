> Locale: ar · العربية

# الوثائق

هذا الدليل هو المرجع العملي لـ `vexo-consensus`. وهو موجه للمطورين والمشغلين ومسؤولي الإصدار والمراجعين الذين يحتاجون إلى فهم الشبكة من دون استنتاج سلوكها من الشفرة وحدها.

يجب أن تشرح كل صفحة مسؤولية المكون والملفات والأوامر ومفاتيح الإعداد وواجهات API التي تنفذه، وشروط السلامة والأدلة اللازمة قبل تشغيل شبكة حقيقية. تبقى الإنجليزية المصدر المعياري للبروتوكول والأمان والإصدار وSDK والأوامر والإعداد وRPC؛ تساعد هذه الترجمة على القراءة لكنها لا تحل محل النص الإنجليزي في قرارات التدقيق.

للبدء استخدم الأوامر أدناه ثم اقرأ `Node Initialization` و`Docker Deployment` و`Observability Guide` و`RPC API Versioning`.

| المهمة | مسار الأوامر |
|---|---|
| بناء ثنائي محلي | __ VEXO_CODE _0__ |
| إنشاء منزل مدقق واحد | __ VEXO_CODE _1__ |
| التحقق من منزل واحد | __ VEXO_CODE _2__ و __ VEXO_CODE _3__ |
| تشغيل عقدة واحدة | __ VEXO_CODE _4__ |
| الاستعلام عن عقدة واحدة |' curl - s __ VEXO_URL _0__ |
| تشغيل شبكة Docker رباعية المصادقة | __ VEXO_CODE _5__ متبوعة بـ__ VEXO_CODE _6__ |
| Connect Remix | استخدم أداة التحقق من Docker 1 Web3 URL `__ VEXO_URL _1__ |
| تحقق من معرف سلسلة Web3 | __ VEXO_CODE _7__ |

## البداية السريعة

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## ابدأ هنا

| مستند | الغرض |
|---|---|
| [دليل جاهزية الإنتاج ](./production-readiness.md) | خريطة واحدة للبروتوكول ووقت التشغيل والعمليات والأدلة وجاهزية الإصدار |

## مواصفات البروتوكول

- تصف [Consensus Spec](./specs/consensus-spec.md) و[Finality Proof Format](./specs/finality-proof-format.md) و[Validator Lifecycle](./specs/validator-lifecycle.md) السلامة والنهائية وتغييرات validator set.
- تغطي [Networking Spec](./specs/networking-spec.md) و[Storage Schema](./specs/storage-schema.md) و[Transaction Format](./specs/tx-format.md) النقل والاسترداد الدائم وقبول المعاملات.
- تحدد [EVM and Native Accounting](./specs/evm-native-accounting.md) الحد بين الحساب الأصلي وحساب EVM.

## SDK والتوسعة

تشرح [App Module Guide](./sdk/app-module-guide.md) و[Custom Crypto Backend](./sdk/custom-crypto-backend.md) و[Custom Storage and Transport](./sdk/custom-storage-transport.md) و`RPC API Versioning` كيفية توسيع runtime من دون كسر عقود الإجماع أو RPC.

## التشغيل والإصدار والأمان

يشكل `Node Initialization` و[Adding a Validator](./operators/add-validator.md) و`Observability Guide` و[دليل الإطلاق](./release/launch-runbook.md) و`Release Pipeline` و[Version Compatibility Matrix](./release/version-compatibility.md) مسار المشغل. توثق [Security Audit Readiness](./security/audit-readiness.md) نموذج التهديد والأدلة الإلزامية.

## قاعدة النضج

وجود الشفرة وحده لا يثبت الجاهزية للإنتاج. يلزم إجراء اختبارات unit وadversarial وE2E وحفظ آثار التشغيل والافتراضات وأنماط الفشل ونتائج release gate. تبقى الأوامر وأساليب RPC ومفاتيح الإعداد متطابقة في جميع الترجمات.

## البحث والنشر

عند إعداد ورقة علمية، ابدأ بـ [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md). تفصل الوثيقة الآليات المنفذة فعليا، ومنها مهلة الجولة المتكيفة وبوابة النهائية أثناء الاسترداد والترتيب الحتمي للمعاملات، عن الأعمال السابقة. كما تجمع أسئلة البحث والفرضيات وبروتوكول التجربة والآثار القابلة لإعادة الإنتاج وضوابط أخلاقيات البحث. لا تعرض أداء غير مقاس بوصفه نتيجة، ولا تدعي أن PoS أو BFT أو HotStuff بحد ذاتها مساهمة جديدة.

تبقى أسماء الوثائق المعيارية التالية كما هي للتنقل بين اللغات: `Node Initialization` و`Docker Deployment` و`Observability Guide` و`RPC API Versioning` و`Production Readiness` و`Release Pipeline` و`Adaptive Recovery-Gated HotStuff Research Draft`.

<!-- vexo-docs:technical-parity -->
## ملحق التكافؤ التقني

يساعد هذا الملحق على ضمان أن الترجمة تحتفظ بالواجهات القابلة للتنفيذ والأقسام الأساسية من الوثيقة الإنجليزية المعتمدة. تبقى الأوامر ومفاتيح الإعداد وطرق RPC وأسماء الحزم كما هي في كل اللغات.

### تتبع الأقسام
- section: How to Read This Set — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Start Here — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Protocol Specs — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: SDK and Extension Guides — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Operations and Release — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Security — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Localized Documentation — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Writing New Docs — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Production Claim Rule — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Documentation Review Checklist — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.

### واجهات تبقى دون تغيير
- `vexo-consensus` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/*` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `make docs-check` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexod status --json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `feature_assurance` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `network_config.json:p2p.auth_replay_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `network_config.json:p2p.node_key_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `module_config.json:governance.RequireDeposit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `module_config.json:governance.MinDeposit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `consensus_config.json:consensus.execution_commit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `mempool_config.json:mempool.WALPath` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
