# Transaction Format

> Locale: ar · العربية
> هذا المستند دليل مترجم اعتماداً على الوثائق الإنجليزية المعتمدة. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## الغرض

يوضح هذا المستند قواعد transaction format وsigning وfee وgas. تبقى الأوامر وحقول JSON وأسماء RPC وconfig key ومعرّفات الكود المستخدمة في التنفيذ والتشغيل باللغة الإنجليزية للحفاظ على التوافق.

## النطاق الأساسي

- تحقق من العناصر التالية عند قراءة هذا المستند. تبقى الأوامر وحقول JSON وطرق RPC ومفاتيح الإعداد ومعرّفات الكود باللغة الإنجليزية للحفاظ على التوافق.
- للنصوص المعيارية التفصيلية، راجع الأصل الإنجليزي.
- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/ar/specs/tx-format.md`

## المعرّفات التي يجب الحفاظ عليها

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

## أقسام الأصل الإنجليزي

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

## ملاحظات تشغيلية

- تبقى `MUST` و `SHOULD` و `MAY` وأمثلة الأوامر وأمثلة JSON وأسماء RPC بالتهجئة الإنجليزية.
- بعد تعديل هذه الترجمة، شغّل `make docs-check`.
- إذا تعارضت هذه الصفحة مع المصدر الإنجليزي، فاعتمد المصدر الإنجليزي وحدّث ملف اللغة هذا في نفس التغيير.

## المصدر المعتمد

- [English canonical document](../../en/specs/tx-format.md)
