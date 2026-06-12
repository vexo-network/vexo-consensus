# Finality Proof Format

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## نظرة عامة

يساعد هذا المستند على فهم حقول finality proof وترتيب التحقق وvalidator set bindingوربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/ar/specs/finality-proof-format.md`

## لماذا تقرأ هذا المستند

- حقول finality proof وترتيب التحقق وvalidator set binding
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

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## بنية المصدر الإنجليزي

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## المصدر المعتمد

- [الوثيقة الإنجليزية المرجعية](../../en/specs/finality-proof-format.md)

<!-- vexo-docs:technical-parity -->
## ملحق التكافؤ التقني

يساعد هذا الملحق على ضمان أن الترجمة تحتفظ بالواجهات القابلة للتنفيذ والأقسام الأساسية من الوثيقة الإنجليزية المعتمدة. تبقى الأوامر ومفاتيح الإعداد وطرق RPC وأسماء الحزم كما هي في كل اللغات.

### تتبع الأقسام
- section: Scope — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Proof Fields — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Header Fields — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Quorum Certificate Fields — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Commit Chain Fields — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Verification Algorithm — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Accountable Safety Detection — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Ed25519 Model — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: BLS Model — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.

### واجهات تبقى دون تغيير
- `finality.Proof` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/finality/latest` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/finality/{height}` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `strict: true` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/status.latest_height` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `/v1/finality/*` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `Proof.ValidatorSetHeight <= Header.Height` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `Proof.ValidatorSetHash == loaded_set.Hash()` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `Header.ValidatorSetHash == loaded_set.Hash()` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `QuorumCert.Height == Header.Height` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `Header.TxRoot` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `HeaderHash(link.Header)` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `finality.AttackDetector` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--validator-set` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `blst-bls12381-minpk-v1` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `supranational/blst` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
