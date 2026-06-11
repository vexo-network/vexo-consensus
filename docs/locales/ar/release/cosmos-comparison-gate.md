# Cosmos/Tendermint Comparison Gate

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## نظرة عامة

يساعد هذا المستند على فهم بوابة الإصدار مقارنة بتوقعات نمط Cosmos/Tendermintوربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/ar/release/cosmos-comparison-gate.md`

## لماذا تقرأ هذا المستند

- بوابة الإصدار مقارنة بتوقعات نمط Cosmos/Tendermint
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

- `release gate`
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## بنية المصدر الإنجليزي

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## المصدر المعتمد

- [الوثيقة الإنجليزية المرجعية](../../en/release/cosmos-comparison-gate.md)
