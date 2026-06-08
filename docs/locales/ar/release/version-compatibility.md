# Version Compatibility Matrix

> Locale: ar · العربية
> هذا المستند دليل مترجم اعتماداً على الوثائق الإنجليزية المعتمدة. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## الغرض

يوضح هذا المستند مصفوفة توافق الإصدارات ومعايير قرار الترقية. تبقى الأوامر وحقول JSON وأسماء RPC وconfig key ومعرّفات الكود المستخدمة في التنفيذ والتشغيل باللغة الإنجليزية للحفاظ على التوافق.

## النطاق الأساسي

- تحقق من العناصر التالية عند قراءة هذا المستند. تبقى الأوامر وحقول JSON وطرق RPC ومفاتيح الإعداد ومعرّفات الكود باللغة الإنجليزية للحفاظ على التوافق.
- للنصوص المعيارية التفصيلية، راجع الأصل الإنجليزي.
- Canonical path: `docs/release/version-compatibility.md`
- Locale path: `docs/locales/ar/release/version-compatibility.md`

## المعرّفات التي يجب الحفاظ عليها

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `/v1/*`
- `vexod upgrade plan --json`
- `vexod upgrade apply`
- `rollback_required`
- `make release-candidate`

## أقسام الأصل الإنجليزي

- Version Compatibility Matrix
- Current Matrix
- Upgrade Compatibility Checklist
- Rollback Drill

## ملاحظات تشغيلية

- تبقى `MUST` و `SHOULD` و `MAY` وأمثلة الأوامر وأمثلة JSON وأسماء RPC بالتهجئة الإنجليزية.
- بعد تعديل هذه الترجمة، شغّل `make docs-check`.
- إذا تعارضت هذه الصفحة مع المصدر الإنجليزي، فاعتمد المصدر الإنجليزي وحدّث ملف اللغة هذا في نفس التغيير.

## المصدر المعتمد

- [English canonical document](../../en/release/version-compatibility.md)
