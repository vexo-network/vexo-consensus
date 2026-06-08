# EVM and Native Accounting

> Locale: ar · العربية
> هذا المستند دليل مترجم اعتماداً على الوثائق الإنجليزية المعتمدة. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## الغرض

يوضح هذا المستند ربط native coin مع EVM gas/accounting بشكل متسق. تبقى الأوامر وحقول JSON وأسماء RPC وconfig key ومعرّفات الكود المستخدمة في التنفيذ والتشغيل باللغة الإنجليزية للحفاظ على التوافق.

## النطاق الأساسي

- تحقق من العناصر التالية عند قراءة هذا المستند. تبقى الأوامر وحقول JSON وطرق RPC ومفاتيح الإعداد ومعرّفات الكود باللغة الإنجليزية للحفاظ على التوافق.
- للنصوص المعيارية التفصيلية، راجع الأصل الإنجليزي.
- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/ar/specs/evm-native-accounting.md`

## المعرّفات التي يجب الحفاظ عليها

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`

## أقسام الأصل الإنجليزي

- EVM and Native Accounting
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM Execution
- Compatibility Boundary
- Failure Modes

## ملاحظات تشغيلية

- تبقى `MUST` و `SHOULD` و `MAY` وأمثلة الأوامر وأمثلة JSON وأسماء RPC بالتهجئة الإنجليزية.
- بعد تعديل هذه الترجمة، شغّل `make docs-check`.
- إذا تعارضت هذه الصفحة مع المصدر الإنجليزي، فاعتمد المصدر الإنجليزي وحدّث ملف اللغة هذا في نفس التغيير.

## المصدر المعتمد

- [English canonical document](../../en/specs/evm-native-accounting.md)
