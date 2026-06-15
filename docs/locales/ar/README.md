# Documentation

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## البدء السريع

- ابنِ الملف التنفيذي باستخدام `make build`.
- أنشئ validator home عبر `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`، ثم تحقّق منه بـ `vexod validate --home .vexo-validator-1` و `vexod config audit --home .vexo-validator-1 --strict`، وبعدها شغّل `vexod start --home .vexo-validator-1`.
- شغّل شبكة Docker أولاً بـ `docker compose -f deployments/docker/compose.single-host-init.yml up` ثم `docker compose -f deployments/docker/compose.single-host.yml up`.
- يجب أن يشير Remix إلى `http://127.0.0.1:28657/web3`؛ وللتحقق من chain ID استخدم `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`.
- بعد تعديل المستندات شغّل `make docs-check` للتحقق من locale tree وحراس الترجمة.
## نظرة عامة

يساعد هذا المستند على فهم فهرس الوثائق وترتيب القراءة الموصى به وربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/ar/README.md`

## لماذا تقرأ هذا المستند

- فهرس الوثائق وترتيب القراءة الموصى به
- تحقق أولاً من عبارات MUST/SHOULD/MAY في المصدر الإنجليزي.
- هذا المستند المحلي يساعد على الفهم؛ قرارات audit وrelease وsecurity تُحسم بالمصدر الإنجليزي.

## ما الذي يجب أن تستطيع فعله بعد القراءة

- شرح قرار التنفيذ أو التشغيل الذي يدعمه هذا المستند.
- ربط المتطلبات المعيارية في المصدر الإنجليزي بإعدادات الشبكة الحالية.
- التحقق من chain ID وvalidator ID وfee/gas وعناوين peer قبل نسخ الأمثلة.

## قائمة تحقق للاستخدام الآمن

- تحقق أولاً من عبارات MUST/SHOULD/MAY في المصدر الإنجليزي.
- لا تترجم الأوامر أو config key أو أسماء RPC أو حقول JSON أو معرّفات الكود.
- قبل نسخ الأمثلة، طابق chain ID وvalidator ID وfee/gas وعناوين peer مع شبكتك.
- بعد تعديل الوثائق، شغّل `make docs-check` للتحقق من locale tree وحراس الترجمة.

## نقاط يجب الانتباه لها

- هذا المستند المحلي يساعد على الفهم؛ قرارات audit وrelease وsecurity تُحسم بالمصدر الإنجليزي.
- عند تغير التنفيذ، حدّث المصدر الإنجليزي وكل المستندات المحلية في نفس التغيير.

## واجهات يجب الحفاظ عليها كما هي

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## بنية المصدر الإنجليزي

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## المصدر المعتمد

- [الوثيقة الإنجليزية المرجعية](../en/README.md)

## قائمة المصطلحات التي تُحفظ كما هي

المصطلحات التالية لا تُترجم.

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

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
