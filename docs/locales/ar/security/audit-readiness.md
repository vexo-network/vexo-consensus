# Security Audit Readiness

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## نظرة عامة

يساعد هذا المستند على فهم نموذج التهديدات وافتراضات الأمان وأدلة التدقيق وربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/ar/security/audit-readiness.md`

## لماذا تقرأ هذا المستند

- نموذج التهديدات وافتراضات الأمان وأدلة التدقيق
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
- `/v1/*`
- `chain_id`
- `(height, round)`

## بنية المصدر الإنجليزي

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- أهداف الأمان
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## المصدر المعتمد

- [الوثيقة الإنجليزية المرجعية](../../en/security/audit-readiness.md)
- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`

## VRF audit evidence SHA-256

يجب أن تتضمن مواد التدقيق VRF adapter audit evidence إضافة إلى BLS. ثبت SHA-256 لملف مثل `docs/security/ecvrf-audit-evidence.json` في `vrf.audit_evidence_sha256` أو `--vrf-audit-sha256`، وراجع dependency audit و key custody و TLS/mTLS أو pinned CA و auth و replay defense و service availability كحد أمني واحد.
