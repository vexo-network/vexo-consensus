# HotStuff تكيفي مع بوابة استرداد لشبكات Proof-of-Stake المعيارية

> Locale: ar · العربية  
> نوع الوثيقة: مسودة بحث وبروتوكول لإعادة الإنتاج  
> الحالة: مسودة مبنية على التنفيذ؛ أي ادعاء أداء يحتاج إلى قياسات وأدلة محفوظة.

## الملخص

تدرس هذه الوثيقة تكرار آلة حالة BFT بأسلوب HotStuff لشبكات Proof-of-Stake معيارية. يجمع التنفيذ بين three-chain finality ومجموعات validator ذات إصدارات مرتبطة بالارتفاع، وبين ثلاث آليات تشغيلية. تضبط وحدة تحكم تكيفية محدودة round timeout باستخدام زمن المعالجة p95 لكل من proposal وvote وcommit وحالة active peers. تؤجل recovery finality gate أي finalized application commit عندما يختلف سجل blocks الدائم عن سجل application state فوق ارتفاع آمن مشترك. كما تزيل deterministic transaction ordering أثر ترتيب وصول المعاملات إلى mempool المحلي عند تساوي مجموعة المعاملات، مع الحفاظ على اعتماد nonce لكل signer.

لا يدعي البحث أن PoS أو BFT أو HotStuff أو adaptive view synchronization أو order fairness اختراعات جديدة. السؤال أضيق: هل يقلل هذا التركيب المحدود للتحكم والاسترداد والترتيب timeouts غير الضرورية وعدم الاتساق أثناء recovery من دون تغيير قاعدة أمان HotStuff الأساسية؟ تفصل الوثيقة بين الحقائق المنفذة والفرضيات القابلة للدحض والنتائج التي ما زالت تحتاج إلى تجربة. لا يجوز نشر أرقام تحسن throughput أو latency قبل تكرار التجارب باستخدام binary وconfig وtopology وworkload مثبتة.

## أسئلة البحث

يقارن RQ1 السياسة التكيفية بالنظام نفسه مع fixed timeout عند تغير network delay، باستخدام عدد timeouts وp95 commit latency. يحقن RQ2 أخطاء storage وrestart ليتحقق من أن gate تمنع application state من تجاوز الارتفاع الدائم المشترك بين block وstate. يبدل RQ3 ترتيب مجموعة transaction واحدة ويشترط proposal order متطابقا وnonce متزايدا لكل signer. يقيس RQ4 كلفة CPU وmemory وnetwork وlatency في شبكة مستقرة بلا أعطال.

الفرضيات H1 إلى H4 اتجاهية وقابلة للدحض وليست نتائج. وجود code path لا يثبت منفعة. إذا لم يظهر فرق ذو دلالة فيجب نشر النتيجة السلبية أو حدود التطبيق بوضوح.

## الأعمال السابقة وحدود الجدة

قدم HotStuff سابقا leader-based BFT تحت partial synchrony وquorum certificates وchained commit واتصالا خطيا في happy path وresponsiveness. جمعت LibraBFT/DiemBFT وAptosBFT سابقا بروتوكولات مشتقة من HotStuff مع stake-weighted validator governance. يدرس Jolteon وDitto خفض latency والتكيف مع الشبكة وasynchronous fallback، بينما يدرس Fever responsive view synchronization. ينتمي Tendermint إلى مسار round-based PoS BFT مختلف. يفصل Narwhal/Tusk reliable transaction dissemination عن ordering. وتقدم Aequitas وWendy وThemis تعاريف order fairness أقوى من hash-based determinism المستخدمة هنا.

لذلك لا تصح عبارات مثل «أول blockchain تجمع PoS+BFT»، أو «أول شبكة PoS تستخدم HotStuff»، أو «مطابقة لـ AptosBFT»، أو ادعاء asynchronous liveness أو optimal communication بلا proof، أو «حماية كاملة من MEV»، أو «production-ready» اعتمادا على single-host test. المساهمة النظامية المحتملة أضيق: دمج bounded feedback controller وlocal durable-history commit gate وnonce-aware deterministic ordering في node PoS معياري بلغة Go، ثم تقييمها بصورة قابلة لإعادة الإنتاج أمام fixed وgate-disabled baselines.

## نموذج النظام والآليات

عند الارتفاع h نرمز إلى active validator set بـ Vh وإلى مجموع voting power بـ Ph. تكون QC صالحة عندما يقدم signers معروفون وفريدون ما لا يقل عن ثلثي Ph. تخضع المجموعة وhash الخاص بها لإصدارات حسب height. قد تكون admission permissionless مع minimum stake أو محدودة العدد أو restricted عبر config. تعالج هذه الطبقة Sybil resistance وgovernance ولا تغير BFT fault threshold.

يفترض أن network partially synchronous. يعتمد safety على أن Byzantine voting power أقل من الثلث، وعلى signatures صحيحة وvalidator-set binding صحيح وdurable store موثوق. وتحتاج liveness أيضا إلى أن يصبح delay محدودا في النهاية، وأن يبقى honest quorum قابلا للوصول، وأن تتوفر signers وpeer connectivity كافية. لا يوجد ادعاء تقدم في شبكة asynchronous إلى الأبد.

تعد EVM application workload تحت Vexo consensus. تنفيذ Ethereum bytecode وتوافق أدوات `/web3` لا يعني تنفيذ Ethereum fork choice أو devp2p consensus.

تتابع قاعدة الأمان `locked_qc` و`high_qc`. لا يكون proposal آمنا إلا إذا مدد lock أو حمل justify QC ليس أقدم منه. لا يجوز لـ validator التصويت إلى blocks مختلفة في height/round نفسها. تؤدي ثلاثة certified links متتالية مرتبطة بالارتفاع وhash إلى finalize للـ grandparent. لا تغير وحدة التحكم هذا الشرط أو quorum threshold أو QC verification أو three-chain rule.

تستخدم adaptive timeout ميزانية أساسية T0 وحالية Tt ومجموع proposal/vote/commit p95 latency وfloor مرتبطا بنقص peers. بعد timeout تزيد القيمة باتجاه 1.5×Tt، وبعد progress تنخفض باتجاه 0.8×Tt. يصبح ثلاثة أمثال observed latency candidate floor، وتحصر النتيجة بين T0 و8×T0. إذا لم يوجد active peer يكون peer floor مساويا 2×T0. لا تستهلك حالة idle بلا pending work ولا local execution/storage error أي round. هذه وحدة تحكم تشغيلية محدودة وليست pacemaker ثبت أنه مثالي.

تحسب recovery gate القيمة Hsafe=min(Hs,Hb) عندما يوجد durable state height Hs وblock-index height Hb. ما داما مختلفين تؤجل finalized application commits فوق Hsafe. هذا local persistence restriction وليس vote phase إضافية أو network certificate.

ينشئ deterministic ordering قيمة salt من chain ID وheight. تجمع transactions التي تحمل signer/nonce metadata في signer chains، وترتب داخل كل chain حسب nonce تصاعديا، ثم تدمج رؤوس chains حسب salted transaction hash. يختفي اعتماد arrival order لمجموعة مرشحة واحدة، لكن لا تضمن الآلية first-seen fairness أو censorship resistance أو confidentiality أو strong order-fairness لأن proposer ما زال يؤثر في inclusion.

يستخدم consensus vote path الحالي كامل height-versioned validator set وdeterministic proposer. يوجد ECVRF committee selector كـ component وquery لكنه غير مرتبط بـ quorum formation أو proposal eligibility. لذلك يبقى VRF committee consensus ضمن future work.

## منهج التجربة

تستخدم كل treatments binary وapplication config نفسيهما. تقارن fixed مع adaptive off وgate on، وadaptive مع كليهما on، وgate-disabled ablation داخل شبكة بحث معزولة قابلة للحذف فقط. عند توفر الموارد تستخدم 4 و7 و16 و31 validator؛ أما single-host فهو smoke test فقط.

تشمل الظروف latency بقيم 10 و50 و100 و250 ms، وstep delay وjitter وloss بنسبة 0/1/5/10%، وrestart لـ validator عادي ولـ current proposer، وعدم توفر أقل قليلا من ثلث voting power، وminority partition ثم healing، وsigner delay، وinjected durable-history mismatch. وتشمل workloads native transfer وEVM transfer وcontract creation وevent log وproxy deployment وUUPS upgrade.

تجمع committed/finalized height وproposal/vote/commit p50/p95/p99 وend-to-end finality latency وعدد timeouts وتوزيع rounds وcurrent adaptive timeout وpeer count وrecovery deferral وthroughput وgas وCPU وRSS وdisk/network bytes وأحداث rejection وdouble-sign وinvalid nonce. لا تدخل performance run في التحليل إلا إذا اتفقت كل validators على app hash وfinalized block hash، وتطابقت transaction/receipt/block locations، ووجد deployed code، وبقي proxy state صحيحا بعد upgrade.

بعد warm-up تنفذ ثلاثون إعادة مستقلة على الأقل لكل حالة ما لم يبرر عدد أصغر مسبقا بـ power analysis. يرتب treatments عشوائيا وتحفظ seeds. ينشر median وIQR وp95 وconfidence interval وeffect size. لا يختار أفضل run فقط، وتحدد exclusion rules قبل رؤية النتائج.

## الصحة وإعادة الإنتاج والأخلاق

تغير السياسة التكيفية وقت محاولة timeout vote فقط، ولا تغير تعريف vote أو QC الآمن. تقيد gate commits ولا تستطيع السماح بـ commit رفضته القاعدة الأساسية. تساعد deterministic ordering في توحيد execution input لكنها لا تستبدل proof ضد conflicting finality.

يجب أن تصوغ proof صالحة للنشر stake-weighted quorum intersection وlock monotonicity ووحدانية finalized block في كل height وvalidator-set transition وvote WAL crash recovery وكون controller/gate محايدين تجاه safety. تعد unit tests وadversarial simulations evidence لكنها لا تستبدل formal proof أو independent audit.

يحفظ كل experiment قيمة commit وdirty-tree status ومعلومات Go/OS/CPU/memory/container وtopology وgenesis وsplit configs وbinary SHA-256 وworkload seed وraw JSON/JSONL/CSV وvalidator logs وfinal app hashes وanalysis scripts وfailed-run ledger. لا يعاد تسمية آلية معروفة لتقديمها كاختراع، ولا تصنع أرقام، ويفصل بين hypothesis وobservation وinterpretation.

يفصح عن AI assistance وفق سياسة venue، ويبقى المؤلفون مسؤولين عن كل claim وcitation وexperiment وproof. ينفذ fault injection فقط على isolated system مملوك أو مصرح به. لا تنشر private keys أو operator tokens أو participant data أو production endpoints. تتبع اكتشافات الأمن coordinated vulnerability disclosure.

قبل submission يجب أن تتطابق المخطوطة مع pinned source revision، ويحفظ prior-art search، وتكون baselines قابلة لإعادة الإنتاج، وتكتمل multi-host fault measurements، ويمكن إعادة إنشاء كل table/figure من raw data وscripts. تبقى negative results وlimitations وproof wording المناسب وexternal methodology review ضمن النسخة. قبل ذلك الوصف الصحيح هو «مسودة بحث مبنية على التنفيذ»، لا «consensus جديد مثبت».

<!-- vexo-docs:technical-parity -->

## ملحق التكافؤ التقني

تبقى الأسماء التالية بلا ترجمة:

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
