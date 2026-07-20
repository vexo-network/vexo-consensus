> Locale: ar · العربية

# تهيئة العقدة

يشرح هذا الدليل كيفية تهيئة منازل عقدة التحقق والأرشفة وبدء تشغيلها والتحقق من صحتها وتوصيل العملاء.

يجب تكوين اتصال النظير في `network_config.json`، ولا يتم تمريره بشكل متكرر في سطر الأوامر `start`.

سلوك وقت التشغيل الذي يؤثر على الإجماع أو RPC أو P2P أو التسجيل أو حسابات Web3 المُدارة هو ملف تكوين فقط. `vexod start` يرفض العلامات مثل `--timeout-propose`، `--create-empty-blocks`، `--p2p-auth-token`، `--rpc-admin-token`، `--evm-account-key-env`، ​​و`--evm-account-key`؛ قم بتحرير ملفات التكوين المقسمة بدلاً من ذلك حتى يقوم كل مشغل بمراجعة نفس سلوك العقدة الحتمية.

لا يوجد مفتاح لوضع العقدة. يتم تعريف العقدة الرئيسية من خلال ملفات التكوين الخاصة بها، والنشأة، والمواد الرئيسية، وما إذا كان `validator_id` بالإضافة إلى المُوقع موجودًا.

## ما تقوم ببنائه

الصفحة الرئيسية لعقدة Vexo هي دليل يحتوي على كل ما تحتاجه العقدة للبدء:
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
القاعدة المهمة بسيطة: قم بالتهيئة مرة واحدة، ثم قم بتحرير ملفات التكوين، ثم ابدأ. لا تخفي سلوك الشبكة داخل أعلام الصدفة.

## تشغيل محلي لمدة خمس دقائق

استخدم هذا التدفق عندما تريد إثبات عمل الثنائي قبل التفكير في النشر متعدد المضيفين.
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
في محطة أخرى:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
شكل الحالة المتوقعة:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
قد يظل الارتفاع الأخير عند الصفر عند تشغيل عقدة واحدة أو مجموعة ذاكرة فارغة عند تعطيل إنشاء الكتلة الفارغة. هذا لا يعني أن العملية مكسورة. وهذا يعني أن العقدة لا تنتج كتل فارغة. أضف المعاملات أو قم بتشغيل شبكة اختبار متعددة المدقق لمراقبة الالتزامات المستمرة.

## شبكة محلية رباعية المدقق

استخدم هذا التدفق عندما تريد الاتصال بالأقران، وتناوب المقترح، وسجلات الالتزام بالحظر، وزيادة الارتفاع.
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
الشيكات المفيدة:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
إذا تم تمكين تسجيل التزام الحظر في `log_config.json`، فستتضمن سجلات المدقق أحداثًا مثل:
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
أوقف الشبكة المحلية التي تم إنشاؤها باستخدام:
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 وريميكس

يعيش JSON-RPC بنمط Ethereum عند نقطة نهاية Web3، وليس ضمن مساحة اسم واجهة برمجة التطبيقات التشغيلية Vexo التي تم إصدارها.

بالنسبة إلى أداة التحقق من مضيف واحد Docker 1، فإن عنوان URL المخصص لموفر Remix هو:
```text
http://127.0.0.1:28657/web3
```
بالنسبة للعقدة المحلية المباشرة بمنفذ RPC الافتراضي:
```text
http://127.0.0.1:26657/web3
```
اختبر نفس المكالمة التي يجريها Remix:
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
إذا أشار المتصفح إلى فشل جلب معرف السلسلة، فتحقق مما يلي بالترتيب:

1. ينتهي عنوان URL بمسار نقطة نهاية Web3.
2. يمكن للمتصفح الوصول إلى المنفذ المضيف. تعرض أمثلة عامل الإرساء `28657`، و`28667`، و`28677`، و`28687`؛ داخل الحاوية لا يزال منفذ RPC `26657`.
3. خادم RPC قيد التشغيل؛ الاستعلام عن نقطة نهاية الحالة على نفس المضيف والمنفذ.
4. يتم السماح بـ CORS بواسطة `network_config.json`/RPC config. يسمح المعالج الافتراضي بالاختبار المبدئي للمتصفح عند عدم تعيين قائمة CORS مخصصة.
5. تحتوي السلسلة على معرف سلسلة EVM غير صفري في `module_config.json`.

## عقدة التحقق من الصحة

استخدم `init validator` عندما تقترح العقدة، وتصوت، وتوقع رسائل الإجماع، وتشارك في تناوب المدقق.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
قم بتعيين `VEXO_KEY_PASSPHRASE` قبل تشغيل هذا الأمر، أو قم بتمرير `--passphrase` لإعداد محلي لمرة واحدة. `--encrypt-keys` يقوم بتشفير `validator.key.json`، `node.key.json`، و`validator.vrf.key.json`.

قاعدة الحضانة الرئيسية:

- `validator.key.json` يوقع على المقترحات المتفق عليها، والتصويتات، وتصويتات المهلة، والرسائل المتعلقة بالنهائية.
- `node.key.json` يوقع مصافحة P2P فقط؛ ولا يجب أبدًا إعادة استخدامه كمفتاح إجماع المدقق.
- `validator.vrf.key.json` يثبت عشوائية اللجنة ويجب معاملته كمواد حفظ صحة.
- يجب على المستمعين العموميين استخدام المستندات الرئيسية المحلية المشفرة أو المستندات الرئيسية على نمط KMS للموقع عن بعد. إذا كشفت العقدة عن RPC عام أو P2P عام تمت مصادقته أثناء `require_network_safety=true`، فإن بدء التشغيل يرفض مفاتيح التحقق المحلية ذات النص العادي.
- تتم كتابة المفاتيح التي تم إنشاؤها باستخدام وضع نظام الملفات `0600`؛ لا تزال تفضل المُوقع عن بُعد/KMS لأجهزة التحقق من الصحة طويلة الأمد.

للحصول على مفتاح إجماع BLS:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` يكتب مستند مفتاح `blst-bls12381-minpk-v1` BLS وينسخ إثبات الحيازة إلى `genesis.json` البيانات التعريفية للمدقق كـ `bls_pop`.

هذا يخلق:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` هو الموقع الإجماعي. `node.key.json` هو مُوقع المصافحة P2P المشار إليه بواسطة `network_config.json:p2p.node_key_path`. وهي منفصلة عن عمد بحيث يمكن لعقد الأرشيف والمدققين استخدام نفس النقل دون إعطاء كل نظير مفتاح توقيع المدقق.

ابدأ باستخدام الشبكات المعتمدة على التكوين:
```bash
vexod start --home .vexo-validator-1
```
بعد بدء التشغيل، اقرأ السجلات. يجب أن يقوم المدقق السليم بإصدار تشغيل العقدة، والاستماع إلى RPC، والاستماع إلى P2P، وبمجرد الالتزام بالكتل، يتم تنفيذ أحداث الكتلة. إذا تم تعطيل إنشاء الكتلة الفارغة، فإن فقدان السجلات المخصصة للكتلة يمكن أن يعني ببساطة عدم وجود معاملات.

## عقدة الأرشيف

استخدم `init archive` عندما يجب أن تحتفظ العقدة ببيانات السلسلة، وكشف RPC، والمزامنة من النظراء، وتجنب توقيع المدقق.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
هذا يخلق:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

إنه **لا** ينشئ `validator.key.json`.

ابدأ بـ:
```bash
vexod start --home .vexo-archive-1
```
لا توقع عقد الأرشيف على الأصوات المتفق عليها. إنها مفيدة لـ RPC، والفهرسة، ومزامنة الحالة، وخدمة الدليل التاريخي، والاحتفاظ بسجل استعلام أوسع من أدوات التحقق من الصحة.

## تقسيم ملفات التكوين

تستخدم منازل العقدة ملفات تكوين منفصلة حتى يتمكن المشغلون من تحرير نظام فرعي واحد دون خلط الإعدادات غير ذات الصلة:

- `config.json` يحتوي على هوية العقدة ومعرف السلسلة ومسار البيانات ومؤشرات لملفات التكوين المقسمة.
- `module_config.json` يحتوي على اختيار وحدة التطبيق، وسياسة التنفيذ/الانتظار، وسياسة الإدارة على مستوى الوحدة.
- `network_config.json` يحتوي على RPC، وهوية عقدة P2P، وإعدادات الاستماع/النظير/البذور، وإعدادات TLS/auth، وسياسة تسجيل النظير.
- `consensus_config.json` يحتوي على توقيت حلقة الإجماع، وسياسة الكتلة الفارغة، والواجهة الخلفية للتشفير، وVRF، وقبول المدقق، وسياسة اللجنة.
- `mempool_config.json` يحتوي على حجم مجمع الذاكرة، والرسوم، والأولوية، وWAL، والتكرار، وسياسة TTL.
- `log_config.json` يحتوي على تنسيق السجل، والمستوى، وتسجيل أحداث التزام الحظر، وتسجيل أحداث النظير.
- `genesis.json` يحتوي على أدوات التحقق من صحة التكوين غير القابلة للتغيير، وبيانات تعريف أداة التحقق من الصحة، وحالة وحدة التكوين.

تتضمن إعدادات `network_config.json` RPC أيضًا `shutdown_timeout`، و`web3_max_subscriptions_per_connection`، و`web3_idle_timeout`. `shutdown_timeout` حدود إيقاف التشغيل السلس لحلقة الإجماع وخادم RPC ونقل العقدة حتى لا ينتظر المشغلون إلى الأبد على مسار توقف عالق. الإعداد الافتراضي الذي تم إنشاؤه هو `10s`؛ اشتراكات Web3 الافتراضية هي 256 لكل اتصال مع مهلة خاملة `2m` بحيث لا يمكن لنقاط نهاية RPC العامة تجميع اشتراكات خاملة غير محدودة.

تتضمن إعدادات `network_config.json` P2P `auth_replay_path`، و`require_auth_replay_store`، و`dial_timeout`. يقوم الإعداد الافتراضي الذي تم إنشاؤه بكتابة دليل إعادة التشغيل مرة واحدة إلى `data/p2p_auth_replay.jsonl` ويستخدم مهلة الاتصال الصادر `10s`. بالنسبة لاختبار الاسترجاع الخاص، يكون متجر إعادة التشغيل في الغالب عبارة عن مسك دفاتر غير ضار؛ بالنسبة إلى P2P المصادق عليه بشكل عام، يعد هذا أحد متطلبات السلامة لأنه يمنع إعادة تشغيل المصافحة الموقعة التي تم التقاطها بعد إعادة التشغيل. يجب أن يكون `dial_timeout` طويلاً بما يكفي لـ TLS، والتحقق من المصافحة الموقعة، وزمن الاستجابة عبر المناطق؛ إن ضبطه على مستوى منخفض للغاية يجعل الأقران الأصحاء يبدون هشين ويمكن أن يؤدي إلى إبطاء الحيوية بعد إعادة التشغيل.

يمتلك `network_config.json` أيضًا مزامنة حالة بدء التشغيل. يعد هذا مفيدًا لعقد الأرشيف أو أدوات التحقق من الصحة البديلة أو العقد التي تمت استعادتها على جهاز نظيف. عندما يكون `state_sync.enabled` صحيحًا، يقوم `vexod start` بتنزيل أول لقطة صالحة من `state_sync.snapshot_urls`، والتحقق من معرف السلسلة، والمجموع الاختباري، وجذور الحالة، ومساحات أسماء KV، واستعادتها إلى LevelDB، وإعادة بناء الفهارس، وعندها فقط يبدأ تشغيل العقدة. إذا كانت الحالة المحلية تفي بالفعل بـ `state_sync.min_height` وكان `state_sync.trust_local_higher` صحيحًا، فإن بدء التشغيل يسجل `state_sync_skipped` ويحتفظ بالمتجر المحلي.

مثال على كتلة `state_sync`:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
يسجل بدء التشغيل `state_sync_candidate_failed` لخطأ في الجلب، `state_sync_candidate_rejected` للقطة غير صالحة أو قديمة، و`state_sync_applied` بعد الاستعادة التي تم التحقق منها. احتفظ بـ `max_snapshot_bytes` أسفل أكبر لقطة تخدمها بنيتك الأساسية عن عمد، ولكنها مرتفعة بما يكفي لنمو الحالة الطبيعية. لا تقم بتوجيه العقد العامة إلى مصدر لقطة خارجي غير مصادق عليه ما لم يكن لدى المشغل سياسة ثقة خارج النطاق ودليل نهائي/عميل خفيف لهذا المصدر.

إذا قام أحد الحقول بتغيير سلوك الشبكة، فقم بتحرير ملف التكوين المقسم وقم بتنفيذ هذا الملف الذي تمت مراجعته أو توزيعه. لا تعتمد على إشارات `vexod start` الطويلة لسلوك وقت التشغيل. يرفض أمر البدء عمدًا توقيت الإجماع، والكتلة الفارغة، ومصادقة P2P، ومسؤول RPC، وعلامات مفاتيح Web3 المُدارة حتى لا يقوم المشغلون عن طريق الخطأ بتشغيل سلوك مختلف عن التكوين الذي تمت مراجعته.

## ما هو الملف الذي أقوم بتحريره؟

| الهدف | ملف | المجال |
|---|---|---|
| تغيير منفذ ربط RPC | `network_config.json` | `rpc.address` |
| تغيير منفذ ربط P2P | `network_config.json` | `p2p.listen_address` |
| إضافة أقرانهم المستمرين | `network_config.json` | `p2p.peers` |
| إضافة أقرانهم البذور | `network_config.json` | `p2p.seeds` |
| تمكين/تعطيل الكتل الفارغة | `consensus_config.json` | حقل الكتلة الفارغة المتفق عليه |
| ضبط مهلة الإجماع | `consensus_config.json` | حقول الاقتراح والتصويت المسبق والالتزام المسبق والالتزام |
| تتطلب التنفيذ النهائي | `consensus_config.json` | مجال الالتزام بالتنفيذ بالإجماع |
| تمكين/تعطيل الوحدات | `module_config.json` | قائمة وحدات التطبيق |
| تغيير معرف سلسلة EVM | `module_config.json` | حقل معرف سلسلة تنفيذ EVM |
| لحن الرسوم الأساسية / الغاز | `module_config.json` | رسوم التنفيذ الأساسية، والرسوم الديناميكية، والغاز المستهدف، وحقول الغاز الأقصى |
| تكوين mempool WAL | `mempool_config.json` | مسار ميمبول وول |
| سجلات التزام كتلة التحكم | `log_config.json` | حقل تسجيل الأحداث |
| التحكم في سجلات الأقران | `log_config.json` | حقل سجل أحداث النظير |

عندما تكون في شك، قم بتشغيل:
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## أنواع المفاتيح

افتراضيات Validator init هي `--key-type bls` لأن التحقق من صحة الشبكة يتطلب نهائية تجميع BLS المدققة. يظل `--key-type ed25519` متاحًا للتجارب الخاصة وعمليات النشر المخصصة خارج بوابة أمان الشبكة. يجب استخدام `--encrypt-keys` لأي عقدة منزلية غير قابلة للرمي. يدعم إنشاء المفاتيح المستقلة أيضًا مفاتيح VRF:
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
مفاتيح VRF ليست موقعة بالإجماع. يتم استخدامها لاختيار اللجنة المدعومة من VRF ويجب الرجوع إليها من `consensus_config.json` حتى `vrf_key_paths` بالإضافة إلى مفتاح بيانات تعريف المدقق `vrf_public_key` عند تمكين هذه الواجهة الخلفية.

يشير `config.json` إلى ملفات التكوين المقسمة:
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
قد يكون كل مسار مطلقًا أو متعلقًا بالعقدة الرئيسية. إذا تم حذفه، فإن `vexod` يستخدم الملف `<home>/<name>_config.json` الافتراضي.

مثال `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
توجد سياسة الحوكمة أيضًا في `module_config.json`. تتطلب التكوينات الآمنة للشبكة التي تم إنشاؤها إيداع الاقتراح:
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
الوديعة عبارة عن رصيد أصلي مضمون من مقدم الاقتراح. اجتياز المقترحات استرداد الودائع؛ المقترحات المرفوضة تنقلها إلى `RejectedDeposits`. استخدم عنوانًا يتم التحكم فيه بواسطة وحدة الخزانة/مجمع المجتمع الخاص بك إذا كانت الودائع المرفوضة يجب أن تمول الخزانة بدلاً من حساب الوحدة الافتراضية.

مثال `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` و`rpc.evm_account_private_keys` هما أساليب اختيارية ومرجعية للحساب المُدار على Web3 مثل `eth_accounts`، و`eth_sign`، و`eth_signTransaction`، و`eth_sendTransaction`. تفضل `evm_account_key_envs` بحيث يتم إدخال المفتاح الخاص بواسطة بيئة العملية أو المدير السري بدلاً من تخزينه في JSON. احتفظ بالقائمتين فارغتين لتشغيل أداة التحقق العادية ما لم تكن هذه العقدة تعمل عمدًا كنقطة نهاية لمحفظة Web3 الساخنة المحلية. ترفض سلامة بدء التشغيل مفاتيح التشغيل السريع المُدارة لـ EVM على مستمعي RPC العامين.

مثال `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
يتم حل `vrf_key_paths` بالنسبة إلى الدليل الذي يحتوي على `consensus_config.json`. استخدم المستندات الرئيسية المشفرة وقم بتوفير `VEXO_KEY_PASSPHRASE` لعملية العقدة عندما يكون حفظ مفتاح VRF المحلي أمرًا لا مفر منه. لا تضع كميات قياسية خاصة من VRF مباشرة في `consensus_config.json` للشبكات التي يديرها المشغل.

استخدم `vexod config paths --home <home>` لفحص جميع المسارات التي تم حلها.

يحتوي تكوين الأرشيف على:
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
الأرشيف `consensus_config.json` يعطل حلقة الإجماع المحلية:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
تم تعيين منازل المدقق التي تم إنشاؤها `"require_network_safety": true` في `config.json` بشكل افتراضي. هذا ليس وضعا. إنها بوابة أمان لبدء التشغيل ترفض التشفير الحتمي، والمعاملات غير الموقعة/غير المتوقفة، وأرضيات الرسوم/الغاز المفقودة، وتفتقد mempool WAL الدائم، وتفتقد سياسة الاستبدال لنفس معاملات الموقع/nonce، وعشوائية اللجنة غير الآمنة، وقيم `execution_commit` بخلاف `finalized`.

عند تمكين `require_network_safety`، قم بتشغيل:
```bash
vexod config audit --home <home> --strict
```
قبل البدء بالعقدة. يجب أن تتم عملية التدقيق لكل مدقق ومنزل أرشيف يشارك في نفس الشبكة.

## الأقران القائمون على التكوين

عناوين الزملاء والاستماع موجودة في `network_config.json`:
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
يقوم `vexod start` بتحميل هؤلاء النظراء تلقائيًا:
```bash
vexod start --home .vexo-archive-1
```
يتم تكوين الأقران والبذور المستمرة في `network_config.json`؛ `vexod start` لا يقبل تجاوزات المضيف النظير أو الأساسي.

لا تضع مضيفًا طويل العمر أو إعدادات `host:port` في سطر الأوامر `vexod start`. قم بتحرير `rpc.address`، و`p2p.listen_address`، و`p2p.peers`، و`p2p.seeds` في `network_config.json` بدلاً من ذلك.

حافظ على `p2p.node_id` مستقرًا طوال عمر العقدة الرئيسية. يجب أن يشير `p2p.node_key_path` إلى `node.key.json` أو مستند رئيسي محلي/مُدار آخر يُستخدم فقط لتوقيع المصافحة بين الزملاء. يجب أن تستخدم الخرائط النظيرة معرفات العقدة النظيرة، وليس عناوين الحساب أو أسماء مشغلي أداة التحقق ما لم تكن هذه هي نفسها عن قصد.

بالنسبة لنقل نظير gRPC المشفر والمصادق، قم أيضًا بتعيين `p2p.tls_cert_path`، `p2p.tls_key_path`، `p2p.tls_ca_path`، واختياريًا `p2p.tls_server_name` في `network_config.json`. يتم حل مسارات TLS النسبية من الدليل الرئيسي للعقدة. احتفظ بـ `p2p.dial_timeout` في نفس الملف بحيث يستخدم كل مشغل نفس سلوك إعادة الاتصال؛ لا تخفي توقيت الأقران في البرامج النصية لـ Shell.

## توقيت الإجماع

توقيت حلقة الإجماع موجود في `consensus_config.json`:
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` يتحكم في مدة انتظار الجولة للاقتراح.
- `timeout_prevote` يتحكم في نافذة جمع الأصوات.
- `timeout_precommit` يتحكم في نافذة تجميع شهادة الالتزام.
- `timeout_commit` يتحكم في الحد الأدنى من التأخير بعد الحظر الملتزم به.
- `create_empty_blocks: false` يعني أن العقدة تقترح فقط عندما تكون المعاملات متاحة.
- `execution_commit: "finalized"` ينتظر قرار HotStuff ثلاثي السلاسل قبل تنفيذ السلف النهائي وهو الإعداد الافتراضي للمدقق الذي تم إنشاؤه. `execution_commit: "qc"` ينفذ ويستمر في تنفيذ الكتل المعتمدة من QC على الفور، لكن بوابة الأمان ترفضها.

يتم الاحتفاظ بـ `round_timeout` فقط كمجموع توافق. تفضل حقول المهلة بنمط Tendermint أعلاه.

عندما يكون `create_empty_blocks` خطأ، يمكن أن يظل الارتفاع بدون تغيير عندما يكون مجمع الذاكرة فارغًا. هذا متوقع: تنتظر السلسلة عملاً مفيدًا بدلاً من ارتكاب كتل فارغة. عندما تظهر معاملة وتنتقل حالة جولة الإجماع المحلي عبر مُقترح آخر، تتقدم العقدة إلى الجولة التالية حيث يكون المدقق الخاص بها هو المُقترح ويتم البناء من مجمع الذاكرة. يحافظ مسار الاسترداد هذا على حيوية المعاملات دون إعادة تمكين البريد العشوائي الفارغ.

## شبكة متعددة المصادقة

بالنسبة للشبكة التي تم إنشاؤها:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
يتلقى كل منزل مدقق تم إنشاؤه:

- `validator.key.json` الخاص به
- ملفات التكوين المقسمة الخاصة بها: `module_config.json`، `network_config.json`، `consensus_config.json`، `mempool_config.json`، و`log_config.json`
- `genesis.json` مشترك
- `network_config.json` إدخالات النظير للمدققين الآخرين

يستخدم `vexod network up` و`make network-e2e` مهلة على مستوى العملية أثناء انتظار بدء جميع أدوات التحقق من الصحة، وإرسال معاملة الدخان، ومراقبة نمو الارتفاع. تكون مهلة الأمر الافتراضية أطول عمدًا من الفاصل الزمني المتفق عليه لأنها تغطي بدء العملية وفتح LevelDB والمصافحة الموقعة من P2P وفحوصات TLS/auth وقبول المعاملة والنهائية. إذا قمت بتخفيض مهلة الإجماع بشكل كبير، فاحتفظ بمهلة تشغيل الشبكة كبيرة بما يكفي لتشخيص أخطاء بدء التشغيل بدلاً من إيقاف أداة التسخير مبكرًا.

بالنسبة للشبكات الحاوية أو متعددة المضيفين، ضع قيم الهيكل في ملف JSON:
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` و`rpc_host_template` هي أهداف طلب مكتوبة في قائمة نظيرات `network_config.json` لكل عقدة. في Docker، يمكن أن تكون هذه أسماء خدمات مثل `validator-%d`.
- `p2p_advertise_host_template` و`rpc_advertise_host_template` عبارة عن عناوين عامة مكتوبة في البيانات التعريفية للمدقق في `genesis.json`. استخدم أسماء DNS أو عناوين IP العامة هنا للشبكات العامة.
- `p2p_listen_host` و`rpc_listen_host` هما مضيفان محليان للربط. استخدم `0.0.0.0` للحاويات أو الخوادم التي يجب أن تستمع على جميع الواجهات.
- لا تعيد استخدام أسماء خدمات Docker فقط كعناوين عامة مُعلن عنها ما لم تكن الشبكة خاصة عن قصد.

ثم قم بإنشاء منازل العقدة من هذا الملف:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## استكشاف الأخطاء وإصلاحها

| العَرَض | على الأرجح السبب | ما يجب التحقق منه |
|---|---|---|
| `latest_height` لا يزيد | تم تعطيل الكتل الفارغة ولا توجد رسائل نصية، أو عدم وجود عدد كافٍ من أدوات التحقق عبر الإنترنت، أو عدم توفر المُوقع | `consensus_config.json`، سجلات المدقق، `/v1/diagnostics` |
| `peer_count` هو `0` | لا يمكن الوصول إلى عناوين النظراء أو تم إنشاء `network_config.json` لأسماء المضيفين الخاطئة | `p2p.peers`، منافذ مضيف الحاوية، DNS، جدار الحماية |
| `p2p auth replay store` خطأ | يتطلب P2P العام/المصادق تخزين إعادة تشغيل متين | `p2p.auth_replay_path` واكتب الإذن تحت المنزل |
| `eth_chainId` فشل في الريمكس | عنوان URL خاطئ، أو منفذ مضيف خاطئ، أو تم حظر CORS/الاختبار المبدئي للمتصفح بواسطة التكوين المخصص | استخدم عنوان URL لنقطة نهاية Web3، ثم قم بلف نفس نقطة النهاية مباشرة |
| `config audit --strict` فشل | عثرت بوابة الأمان على خاصية تكوين غير آمنة | اقرأ عملية التحقق الفاشلة، ثم قم بتحرير ملف التكوين المقسم الذي يطلق عليه |
| `no block_committed logs` | تم تعطيل التسجيل أو لم يتم إنشاء أي كتل | `log_config.json`، `create_empty_blocks`، محتويات مجمع الذاكرة |
| `managed EVM key rejected` | يتم تكوين المفاتيح الخاصة الساخنة على مستمع RPC عام | قم بإزالة `evm_account_private_keys` أو احتفظ بـ RPC خاصًا |

## قائمة التحقق الدنيا للمشغل

قبل تسليم العقدة الرئيسية إلى جهاز أو مشغل آخر:

- `vexod validate --home <home>` يمر.
- `vexod config audit --home <home> --strict` يمر بهذا المنزل بالضبط.
- `config.json`، تتم مراجعة ملفات التكوين المقسمة، `genesis.json`، والبيانات التعريفية للمدقق العام.
- `validator.key.json`، `node.key.json`، و`validator.vrf.key.json` يتم تشفيرها أو استبدالها بمستندات مفتاح الموقع/KMS عن بعد.
- `network_config.json:p2p.peers` يحتوي على عناوين يمكن الاتصال بها من الجهاز الهدف، وليس أسماء Docker فقط ما لم يتم تشغيل العقدة فعليًا داخل شبكة Docker تلك.
- `network_config.json` مستمعي RPC/P2P العامين لديهم مادة TLS عند تمكين `require_network_safety`.
- يتم تعيين `module_config.json:execution.EVMChainID` قبل محافظ Web3 أو اتصال Remix.
- `mempool_config.json` لديه مسار WAL إذا كان يجب على العقدة استرداد الرسائل النصية المعلقة بعد إعادة التشغيل.
- `log_config.json` يمكّن الالتزام بالحظر وسجلات الأقران أثناء تشغيل الشبكة.

<!-- vexo-docs:technical-parity -->
## ملحق التكافؤ التقني

يساعد هذا الملحق على ضمان أن الترجمة تحتفظ بالواجهات القابلة للتنفيذ والأقسام الأساسية من الوثيقة الإنجليزية المعتمدة. تبقى الأوامر ومفاتيح الإعداد وطرق RPC وأسماء الحزم كما هي في كل اللغات.

### تتبع الأقسام
- section: Validator Node — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Archive Node — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Split Configuration Files — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Key Types — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Config-Based Peers — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Consensus Timing — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.
- section: Multi-Validator Network — يجب قراءة هذا القسم مع قيم الإعداد وأدلة التحقق وشروط الفشل والإجراءات المطلوبة من المشغل.

### واجهات تبقى دون تغيير
- `network_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexod start` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--timeout-propose` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--create-empty-blocks` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--p2p-auth-token` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--rpc-admin-token` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--evm-account-key-env` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--evm-account-key` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `validator_id` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `VEXO_KEY_PASSPHRASE` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--passphrase` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--encrypt-keys` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `validator.key.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `node.key.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `validator.vrf.key.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `require_network_safety=true` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--key-type bls` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `blst-bls12381-minpk-v1` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `genesis.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `bls_pop` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `module_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `consensus_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `mempool_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `log_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `data/` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `network_config.json:p2p.node_key_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `shutdown_timeout` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `web3_max_subscriptions_per_connection` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `web3_idle_timeout` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `auth_replay_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `require_auth_replay_store` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `dial_timeout` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `data/p2p_auth_replay.jsonl` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `--key-type ed25519` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vrf_key_paths` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vrf_public_key` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexod` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `<home>/<name>_config.json` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc.evm_account_key_envs` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc.evm_account_private_keys` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `eth_accounts` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `eth_sign` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `eth_signTransaction` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `eth_sendTransaction` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `evm_account_key_envs` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexod config paths --home <home>` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `"require_network_safety": true` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `execution_commit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `require_network_safety` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `host:port` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc.address` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.listen_address` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.peers` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.seeds` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.node_id` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.node_key_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.tls_cert_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.tls_key_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.tls_ca_path` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.tls_server_name` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p.dial_timeout` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_propose` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_prevote` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_precommit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `timeout_commit` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `create_empty_blocks: false` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `execution_commit: "finalized"` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `execution_commit: "qc"` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `round_timeout` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `create_empty_blocks` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `vexod network up` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `make network-e2e` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p_host_template` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc_host_template` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `validator-%d` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p_advertise_host_template` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc_advertise_host_template` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `p2p_listen_host` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.
- `rpc_listen_host` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعداد، لذلك لا يُترجم.

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
