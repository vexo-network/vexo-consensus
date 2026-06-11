# دليل الإطلاق

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## نظرة عامة

يساعد هذا المستند على فهم قائمة تحقق المشغل وإجراءات ما قبل إطلاق الشبكة وربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/ar/release/launch-runbook.md`

## لماذا تقرأ هذا المستند

- قائمة تحقق المشغل وإجراءات ما قبل إطلاق الشبكة
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

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--evm-default-fixtures`
- `chain_id`

## بنية المصدر الإنجليزي

- دليل الإطلاق
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## المصدر المعتمد

- [الوثيقة الإنجليزية المرجعية](../../en/release/launch-runbook.md)
- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`

## VRF audit evidence SHA-256

عند التحقق من release candidate مرر إلى `release gate` دليلي التدقيق BLS و VRF مع digest لكل منهما. استخدم على الأقل `--bls-audit` و `--bls-audit-sha256` و `--vrf-audit` و `--vrf-audit-sha256` و `--evidence-manifest`، وتحقق أن كل evidence file يطابق SHA-256 في manifest.
