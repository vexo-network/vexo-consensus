# मॉड्यूलर Proof-of-Stake नेटवर्क के लिए अनुकूली रिकवरी-गेटेड HotStuff

> Locale: hi · हिन्दी  
> दस्तावेज़ प्रकार: शोध पांडुलिपि और पुनरुत्पादन प्रोटोकॉल  
> स्थिति: कार्यान्वयन-आधारित मसौदा; प्रदर्शन दावों के लिए मापे गए artefacts आवश्यक हैं।

## सारांश

यह पांडुलिपि मॉड्यूलर Proof-of-Stake नेटवर्क के लिए HotStuff-शैली BFT state-machine replication का अध्ययन करती है। implementation three-chain finality और height-versioned validator set को तीन operational mechanisms के साथ जोड़ता है। सीमित adaptive controller proposal, vote और commit की p95 processing latency तथा active peer health के आधार पर round timeout को बदलता है। recovery finality gate durable block history और application state history के साझा सुरक्षित height से ऊपर अलग होने पर finalized application commit को रोकता है। deterministic transaction ordering एक ही transaction set के लिए local mempool arrival order का प्रभाव हटाता है, जबकि प्रत्येक signer की nonce dependency को बनाए रखता है।

योगदान यह दावा नहीं करता कि PoS, BFT, HotStuff, adaptive view synchronization या order fairness नए हैं। अधिक संकीर्ण प्रश्न यह है कि क्या control, recovery और ordering का यह bounded composition मूल HotStuff safety rule बदले बिना अनावश्यक timeout और recovery inconsistency को घटाता है। लागू तथ्य, खंडनीय hypotheses और अभी experiment मांगने वाले conclusions अलग रखे जाते हैं। pinned binary, config, topology और workload के repeated measurement के बिना throughput या latency improvement को result नहीं कहा जाएगा।

## शोध प्रश्न

RQ1 बदलती network delay में adaptive policy की तुलना उसी system के fixed timeout से करता है और timeout count तथा p95 commit latency मापता है। RQ2 storage/restart fault inject करके जाँचता है कि recovery gate application state को durable block/state की common height से आगे नहीं जाने देता। RQ3 समान transaction set की permutations से identical proposal order और प्रत्येक signer के बढ़ते nonce की माँग करता है। RQ4 stable fault-free network में CPU, memory, network और latency overhead मापता है।

H1 से H4 directional और falsifiable hypotheses हैं, results नहीं। code path की उपस्थिति लाभ सिद्ध नहीं करती। यदि meaningful improvement न मिले तो negative result या applicability boundary को ईमानदारी से report करना होगा।

## पूर्व कार्य और नवीनता की सीमा

HotStuff पहले ही partial synchrony में leader-based BFT, quorum certificate, chained commit, happy path पर linear communication और responsiveness प्रस्तुत कर चुका है। LibraBFT/DiemBFT तथा AptosBFT ने HotStuff-derived BFT को stake-weighted validator governance से जोड़ा है। Jolteon और Ditto lower latency, network adaptation और asynchronous fallback का अध्ययन करते हैं; Fever responsive view synchronization का अध्ययन करता है। Tendermint एक अलग round-based PoS BFT lineage है। Narwhal/Tusk reliable transaction dissemination को ordering से अलग करता है। Aequitas, Wendy और Themis यहाँ प्रयुक्त hash-based determinism से अधिक मजबूत order fairness परिभाषित करते हैं।

इसलिए “पहला PoS+BFT blockchain”, “HotStuff वाला पहला PoS network”, “AptosBFT के समान”, proof के बिना “asynchronous liveness” या “optimal communication”, “पूर्ण MEV protection”, या single-host test से “production-ready” जैसे कथन गलत हैं। candidate systems contribution अधिक सीमित है: Go में modular PoS node के भीतर bounded feedback controller, local durable-history commit gate और nonce-aware deterministic ordering को जोड़ना और fixed तथा gate-disabled baselines के विरुद्ध reproducible evaluation करना।

## सिस्टम मॉडल और mechanisms

height h पर Vh active validator set और Ph total voting power है। QC तभी valid है जब unique known signers कम से कम Ph के दो-तिहाई power दें। set और उसका hash height द्वारा versioned हैं। admission minimum stake के साथ permissionless, count-limited या config द्वारा restricted हो सकती है। यह layer Sybil resistance और governance संभालती है; BFT fault threshold नहीं बदलती।

network को partially synchronous माना जाता है। safety के लिए Byzantine voting power एक-तिहाई से कम, valid signatures, सही validator-set binding और reliable durable store आवश्यक हैं। liveness के लिए delay का अंततः bounded होना, honest quorum का reachable रहना, signers का available होना और पर्याप्त peer connectivity भी आवश्यक है। permanently asynchronous network में progress का दावा नहीं है।

EVM, Vexo consensus के नीचे application workload है। Ethereum bytecode execution और `/web3` tooling compatibility का अर्थ Ethereum fork choice या devp2p consensus लागू करना नहीं है।

base safety rule `locked_qc` और `high_qc` track करता है। proposal तभी safe है जब वह lock को extend करे या lock जितना नया justify QC लाए। validator समान height/round में अलग blocks के लिए vote नहीं कर सकता। height और hash से जुड़े तीन consecutive certified links grandparent को finalize करते हैं। adaptive controller इस predicate, quorum threshold, QC verification या three-chain rule को नहीं बदलता।

adaptive timeout base budget T0, current budget Tt, proposal/vote/commit p95 latency sum और peer deficit floor का उपयोग करता है। timeout के बाद value 1.5×Tt की ओर बढ़ती है; progress के बाद 0.8×Tt की ओर घटती है। observed latency का तीन गुना candidate floor बनता है और result T0 तथा 8×T0 के बीच bounded रहता है। active peer न होने पर peer floor 2×T0 है। pending work रहित idle समय और local execution/storage error round consume नहीं करते। यह bounded operational controller है, theoretically optimal pacemaker का proof नहीं।

recovery gate durable state height Hs और block-index height Hb उपलब्ध होने पर Hsafe=min(Hs,Hb) निकालता है। दोनों अलग रहने तक Hsafe से ऊपर finalized application commits defer होते हैं। यह local persistence restriction है, extra vote phase या network certificate नहीं।

deterministic ordering chain ID और height से salt बनाता है। signer/nonce metadata वाली transactions को signer chains में group करके nonce ascending क्रम में रखा जाता है, फिर chain heads को salted transaction hash से merge किया जाता है। समान candidate set के लिए arrival-order dependence हटती है। यह first-seen fairness, censorship resistance, confidentiality या strong order-fairness guarantee नहीं करता, क्योंकि proposer inclusion को प्रभावित कर सकता है।

वर्तमान consensus vote path full height-versioned validator set और deterministic proposer इस्तेमाल करता है। ECVRF committee selector component और query के रूप में है, पर quorum formation या proposal eligibility से जुड़ा नहीं है। VRF committee consensus future work है।

## प्रयोग विधि

सभी treatments समान binary और application config उपयोग करते हैं। तुलना में adaptive off/gate on वाला fixed baseline, दोनों on वाली adaptive policy और केवल isolated disposable research network में gate-disabled ablation शामिल हैं। resource होने पर 4, 7, 16 और 31 validators उपयोग किए जाते हैं; single-host केवल smoke test है।

conditions में 10, 50, 100 और 250 ms latency, step delay, jitter, 0/1/5/10% loss, साधारण validator और current proposer restart, एक-तिहाई से थोड़ा कम voting power unavailability, minority partition/heal, signer delay और injected durable-history mismatch शामिल हैं। workload में native transfer, EVM transfer, contract creation, event log, proxy deployment और UUPS upgrade शामिल हैं।

metrics में committed/finalized height, proposal/vote/commit p50/p95/p99, end-to-end finality latency, timeout count, round distribution, current adaptive timeout, peer count, recovery deferral, throughput, gas, CPU, RSS, disk/network bytes, rejection, double-sign और invalid nonce शामिल हैं। performance run तभी valid है जब सभी validators समान app hash और finalized block hash पर सहमत हों, transaction/receipt/block locations consistent हों, deployed code मौजूद हो और upgrade के बाद proxy state सुरक्षित रहे।

warm-up के बाद प्रत्येक condition कम से कम तीस independent repetitions चलाती है, जब तक छोटी संख्या को पहले power analysis से justify न किया गया हो। treatment order randomize और seeds preserve किए जाते हैं। median, IQR, p95, confidence interval और effect size report किए जाते हैं। केवल best run चुनना निषिद्ध है; exclusion rules results देखने से पहले तय होते हैं।

## correctness, reproduction और ethics

adaptive policy केवल timeout vote की कोशिश का समय बदलती है, safe vote या QC की परिभाषा नहीं। gate commits को और restrict करती है और base rule द्वारा rejected commit को allow नहीं कर सकती। deterministic ordering common execution input में सहायता करती है, पर conflicting finality के विरुद्ध proof का स्थान नहीं लेती।

publication-quality proof को stake-weighted quorum intersection, lock monotonicity, प्रति height finalized block uniqueness, validator-set transition, vote WAL crash recovery और controller/gate की safety-neutral property formalize करनी होगी। unit tests और adversarial simulations evidence हैं, formal proof या independent audit का substitute नहीं।

हर experiment commit, dirty-tree status, Go/OS/CPU/memory/container, topology, genesis, split configs, binary SHA-256, workload seed, raw JSON/JSONL/CSV, validator logs, final app hashes, analysis scripts और failed-run ledger archive करता है। known mechanism का नाम बदलकर invention नहीं कहा जाता, figures fabricate नहीं की जातीं और hypothesis, observation, interpretation अलग रखे जाते हैं।

AI assistance को venue policy के अनुसार disclose किया जाता है; authors हर claim, citation, experiment और proof के उत्तरदायी रहते हैं। fault injection केवल owned या authorized isolated system पर किया जाता है। private keys, operator tokens, participant data और production endpoints artifacts में नहीं जाते। security findings coordinated vulnerability disclosure का पालन करते हैं।

submission से पहले manuscript pinned source revision से मेल खाए, prior-art search archived हो, baselines reproducible हों, multi-host fault measurements पूर्ण हों और हर table/figure raw data तथा scripts से फिर बने। negative results, limitations, उचित proof wording और external methodology review final version में बने रहते हैं। तब तक सही विवरण “implementation-based research draft” है, “नया सिद्ध consensus” नहीं।

<!-- vexo-docs:technical-parity -->

## तकनीकी समानता परिशिष्ट

निम्न नाम बिना अनुवाद सुरक्षित रखे जाते हैं:

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
