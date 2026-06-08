# Launch Runbook

> Locale: ar · العربية
> هذا المستند دليل مترجم اعتماداً على الوثائق الإنجليزية المعتمدة. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## الغرض

يوضح هذا المستند قائمة تحقق المشغل وإجراءات ما قبل إطلاق الشبكة. تبقى الأوامر وحقول JSON وأسماء RPC وconfig key ومعرّفات الكود المستخدمة في التنفيذ والتشغيل باللغة الإنجليزية للحفاظ على التوافق.

## النطاق الأساسي

- تحقق من العناصر التالية عند قراءة هذا المستند. تبقى الأوامر وحقول JSON وطرق RPC ومفاتيح الإعداد ومعرّفات الكود باللغة الإنجليزية للحفاظ على التوافق.
- للنصوص المعيارية التفصيلية، راجع الأصل الإنجليزي.
- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/ar/release/launch-runbook.md`

## المعرّفات التي يجب الحفاظ عليها

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `chain_id`

## أقسام الأصل الإنجليزي

- Launch Runbook
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## ملاحظات تشغيلية

- تبقى `MUST` و `SHOULD` و `MAY` وأمثلة الأوامر وأمثلة JSON وأسماء RPC بالتهجئة الإنجليزية.
- بعد تعديل هذه الترجمة، شغّل `make docs-check`.
- إذا تعارضت هذه الصفحة مع المصدر الإنجليزي، فاعتمد المصدر الإنجليزي وحدّث ملف اللغة هذا في نفس التغيير.

## المصدر المعتمد

- [English canonical document](../../en/release/launch-runbook.md)
