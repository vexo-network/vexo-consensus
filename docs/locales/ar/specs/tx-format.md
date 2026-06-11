# Transaction Format

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## نظرة عامة

يساعد هذا المستند على فهم قواعد transaction format وsigning وfee وgasوربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/ar/specs/tx-format.md`

## لماذا تقرأ هذا المستند

- قواعد transaction format وsigning وfee وgas
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

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## بنية المصدر الإنجليزي

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## المصدر المعتمد

- [الوثيقة الإنجليزية المرجعية](../../en/specs/tx-format.md)
