# دليل تحديث EVM

> Locale: ar · العربية
> هذا المستند ترجمة عربية للمصدر الإنجليزي. قرارات البروتوكول والأمن والإصدار تعتمد المصدر الإنجليزي.

يشرح هذا الدليل كيفية تحديث حزمة EVM المدمجة من دون كسر التعامل مع chain ID أو توافق Web3 أو أدلة الإصدار. وهو موجه للمشغلين والقائمين على الصيانة الذين يحتاجون إلى ترقية go-ethereum أو ضبط fork presets أو تغيير سلوك EVM ضمن إصدار مضبوط.

## ما الذي يُعد تحديثًا لـ EVM

اعتبر أي تغيير يمكن أن يؤثر في التنفيذ بأسلوب Ethereum أو في السلوك المرئي لـ Web3 تحديثًا حساسًا للإصدار:

- ترقية `go-ethereum` داخل `modules/evm/backend/geth`
- التغييرات في `modules/evm/ethcompat`
- التغييرات في `modules/evm`
- التغييرات في `execution.evm_fork_preset`
- التغييرات في `execution.evm_chain_config_json`
- التغييرات في قبول raw transactions أو gas accounting أو receipts أو traces أو proofs أو حقول ردود البلوكات
- التغييرات في معالجة حسابات Web3 المُدارة مثل `eth_accounts` و`eth_coinbase` و`eth_sign` و`eth_signTransaction` و`eth_sendTransaction`

## ترتيب التحديث الآمن

اتبع هذا الترتيب حتى يبقى الكود والإعدادات والوثائق متزامنة:

1. حدّث الـ geth-backed adapter المعزول أولاً.
2. حدّث بعد ذلك corpus fixtures واختبارات conformance.
3. إذا تغيّرت الدلالة، حدّث `docs/specs/evm-native-accounting.md` و`docs/specs/tx-format.md` و`docs/sdk/rpc-api-versioning.md`.
4. إذا تغيّر شكل release evidence، حدّث `docs/release/release-pipeline.md`.
5. إذا تغيّرت مفاتيح التحكم الظاهرة للمشغل، حدّث وثائق إعداد العقدة.
6. أعد تشغيل validation matrix قبل الدمج.

لا ترفع نسخة runtime الخاصة بـ EVM وتُصدرها في الوقت نفسه إلا إذا كانت conformance suites وRPC smoke checks وDocker deployment checks كلها قد نجحت.

## سير العمل

### 1. تثبيت نطاق التغيير

سجّل نية التحديث بدقة:

- fork behavior فقط
- transaction admission فقط
- execution semantics فقط
- RPC compatibility فقط
- معالجة blob / receipt / trace فقط
- سلوك الحسابات المُدارة أو المحافظ فقط

هذا التقسيم يحافظ على تركيز المراجعة ويمنع تحريك كود لا علاقة له بالتغيير.

### 2. التعديل في أضيق طبقة

فضّل هذه الحدود:

- `modules/evm/backend/geth` لتغييرات تكامل upstream go-ethereum
- `modules/evm/ethcompat` لـ raw transaction decoding وحفظ hash ومعالجة fixtures
- `modules/evm` لـ state transition وreceipts وlogs وstorage وsnapshots
- `rpc` لتغييرات سطح Web3 request/response
- `cmd/vexod` فقط عندما يجب على CLI أو release workflow إظهار السلوك الجديد

إذا وصل التغيير إلى application modules، فاحفظ حدود module واضحة وابقِ عمليات الكتابة إلى الحالة determinisitic.

### 3. تحديث الإعدادات الافتراضية

عندما تتغير الدلالة، حدّث الإعدادات الافتراضية في نفس patch:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- عند الحاجة حقول RPC الخاصة بالحسابات المُدارة في `network_config.json`
- EVM chain ID في `module_config.json`

لا تعتمد على CLI flag مخفي لشرح سلوك وقت التشغيل. يجب أن توضح الملفات وحدها سلوك العقدة.

### 4. تشغيل conformance stack

شغّل على الأقل:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

ثم تحقّق من المسارات التي يواجهها المستخدم أولًا وتتعطل عادة:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

في نشر Docker single-host تحقّق أيضًا من:

```text
http://127.0.0.1:28657/web3
```

تحقق على الأقل من السلوكيات التالية:

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

بعد ذلك اختبر نشر عقد بسيط، ونشر proxy contract، ومسار UUPS upgrade باستخدام نفس RPC endpoint الذي سيستخدمه wallet أو الأداة في الإنتاج.

### 5. تأكيد سلوك proxy وupgrade

لا يكتمل تحديث EVM إلا إذا كانت كل هذه النقاط صحيحة:

- نجاح نشر contract عادي
- نجاح نشر proxy
- نجاح استدعاء UUPS upgrade
- بعد الترقية تعود قراءات storage وcode كما هو متوقع
- يبقى nonce tracking تصاعديًا
- يقبل block producer النتائج من دون أخطاء unsafe proposal

إذا نجح نشر proxy لكن فشل upgrade، فلا يزال غير صالح للنشر. اعتبر ذلك release blocker وليس مجرد تحذير.

### 6. تحديث الأدلة

عندما تتغير واجهة EVM، حدّث أيضًا release evidence bundle:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- أي مراجع SHA-256 مثبتة للـ fixtures

يجب أن تقول release evidence ما الذي تغيّر، وما الذي اختُبر، وأي commit أو version تم التحقق منه. لا تصف تحديث EVM بأنه مكتمل إذا كانت الأدلة لا تطابق الكود الذي نُفِّذ فعليًا.

## مصفوفة التحقق

استخدم هذا الجدول كـ merge gate.

| Check | لماذا يهم |
| --- | --- |
| `make evm-conformance` | يلتقط regressions الخاصة بـ fork rule والتنفيذ |
| `go test ./modules/evm -count=1` | يتحقق من receipts وlogs وstorage وbalances وsnapshots |
| `go test ./rpc -count=1` | يتحقق من توافق Web3 request/response |
| `make network-e2e` | يؤكد أن العقدة ما زالت تبدأ وتجد peers وتُجري commit |
| Docker single-host smoke | يؤكد المسار الذي يستخدمه Remix وأدوات المتصفح |
| Contract deploy | يؤكد قبول المعاملات وتوليد receipts |
| Proxy deploy | يؤكد افتراضات ABI وstorage layout |
| UUPS upgrade | يؤكد دلالة الترقية والقراءة بعدها |

إذا كان أي فحص أحمر، فلا تقل إن التحديث انتهى.

## معايير التراجع

قم بالرجوع عن تحديث EVM إذا حدث أي مما يلي:

- تغير `eth_chainId` بشكل غير متوقع
- بدأ `eth_sendRawTransaction` برفض معاملات صحيحة
- انحرف `eth_call` أو `eth_estimateGas` عن fork rules المتوقعة
- توقفت receipts أو logs أو proofs عن التطابق مع committed state
- بدأت معاملات proxy أو upgrade بالفشل
- لم تعد release evidence تطابق مسار الكود الحالي

يجب أن يستعيد rollback آخر نسخة جيدة من adapter، وإعدادات config الافتراضية، ومجموعة fixtures معًا.

## ملحق التكافؤ التقني

يحافظ هذا الملحق على اتساق الدليل مع بقية شجرة الوثائق.

- احتفظ بـ `modules/evm/backend/geth` و`modules/evm/ethcompat` و`modules/evm` و`rpc` و`cmd/vexod` كحدود تنفيذ مستقرة.
- احتفظ بتهجئة `execution.evm_fork_preset` و`execution.evm_chain_config_json` و`execution.allow_unprotected_legacy_tx` و`eth_chainId` و`eth_call` و`eth_estimateGas` و`eth_sendRawTransaction` و`eth_getTransactionReceipt` و`eth_getProof` و`eth_getStorageAt` و`eth_accounts` و`eth_coinbase` و`eth_signTransaction` و`eth_sendTransaction` كما هي.
- احتفظ أيضًا بتهجئة `make evm-conformance` و`make network-e2e` و`--evm-default-fixtures` و`--evm-tx-fixtures` و`--evm-execution-fixtures` و`--evm-web3-conformance-evidence` كما هي.
- السؤال التشغيلي يبقى بسيطًا: هل يحافظ هذا التحديث على التنفيذ بأسلوب Ethereum مع بقائه منسجمًا مع أمان Vexo consensus وrelease safety؟

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
