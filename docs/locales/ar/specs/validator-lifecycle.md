# Validator Lifecycle

> Locale: ar · العربية
> هذا المستند دليل مترجم اعتماداً على الوثائق الإنجليزية المعتمدة. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## الغرض

يوضح هذا المستند دورة حياة validator join وrotation وjail وslashing وleave. تبقى الأوامر وحقول JSON وأسماء RPC وconfig key ومعرّفات الكود المستخدمة في التنفيذ والتشغيل باللغة الإنجليزية للحفاظ على التوافق.

## النطاق الأساسي

- تحقق من العناصر التالية عند قراءة هذا المستند. تبقى الأوامر وحقول JSON وطرق RPC ومفاتيح الإعداد ومعرّفات الكود باللغة الإنجليزية للحفاظ على التوافق.
- للنصوص المعيارية التفصيلية، راجع الأصل الإنجليزي.
- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/ar/specs/validator-lifecycle.md`

## المعرّفات التي يجب الحفاظ عليها

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## أقسام الأصل الإنجليزي

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## ملاحظات تشغيلية

- تبقى `MUST` و `SHOULD` و `MAY` وأمثلة الأوامر وأمثلة JSON وأسماء RPC بالتهجئة الإنجليزية.
- بعد تعديل هذه الترجمة، شغّل `make docs-check`.
- إذا تعارضت هذه الصفحة مع المصدر الإنجليزي، فاعتمد المصدر الإنجليزي وحدّث ملف اللغة هذا في نفس التغيير.

## المصدر المعتمد

- [English canonical document](../../en/specs/validator-lifecycle.md)
