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
- `QuorumCert.BlockHash == HeaderHash(Header)`
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

- [English canonical document](../../en/specs/finality-proof-format.md)
