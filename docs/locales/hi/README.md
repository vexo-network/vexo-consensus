> Locale: hi · हिन्दी

# दस्तावेज

यह directory `vexo-consensus` की व्यावहारिक manual है। यह developers, operators, release maintainers और reviewers के लिए है जिन्हें केवल source code से अनुमान लगाए बिना network समझना है।

हर page को component की जिम्मेदारी, implementation files, commands, config keys और APIs, safety conditions तथा real network से पहले जरूरी evidence समझाना चाहिए। Protocol, security, release, SDK, command, config और RPC behavior के लिए English normative source है; यह translation पढ़ने में सहायता देती है, audit निर्णय में English source को नहीं बदलती।

शुरू करने के लिए नीचे के commands चलाएं और फिर `Node Initialization`, `Docker Deployment`, `Observability Guide` तथा `RPC API Versioning` पढ़ें।

| टास्क | कमांड पाथ |
|---|---|
| स्थानीय बाइनरी बनाएँ | __ VEXO_CODE_0 __ |
| एक वैलिडेटर होम बनाएँ | __ VEXO_CODE_1 __ |
| एक घर की पुष्टि करें | __ VEXO_CODE_2 __और __ VEXO_CODE_3 __ |
| एक नोड चलाएं | __ VEXO_CODE_4 __ |
| क्वेरी वन नोड |' curl - s __ VEXO_URL_0 __ |
| डॉकर फोर - वैलिडेटर नेटवर्क चलाएं | __ VEXO_CODE_5 __ इसके बाद __ VEXO_CODE_6 __ |
| कनेक्ट रीमिक्स | डॉकर सत्यापनकर्ता 1 वेब 3 यूआरएल `__VEXO_URL_1 __ का उपयोग करें |
| वेब3 चेन आईडी की जाँच करें | __ VEXO_CODE_7__ |

## त्वरित शुरुआत

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## यहाँ से शुरू करें

| दस्तावेज़ | उद्देश्य |
|---|---|
| [उत्पादन तत्परता मार्गदर्शिका](./production-readiness.md) | प्रोटोकॉल, रनटाइम, संचालन, साक्ष्य और रिलीज तत्परता का एकल मानचित्र |

## प्रोटोकॉल ऐनक

- [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md) और [Validator Lifecycle](./specs/validator-lifecycle.md) safety, finality और validator set changes बताते हैं।
- [Networking Spec](./specs/networking-spec.md), [Storage Schema](./specs/storage-schema.md) और [Transaction Format](./specs/tx-format.md) transport, durable recovery और transaction admission बताते हैं।
- [EVM and Native Accounting](./specs/evm-native-accounting.md) native और EVM accounting की सीमा तय करता है।

## SDK और extensions

[App Module Guide](./sdk/app-module-guide.md), [Custom Crypto Backend](./sdk/custom-crypto-backend.md), [Custom Storage and Transport](./sdk/custom-storage-transport.md) और `RPC API Versioning` runtime को consensus या RPC contract तोड़े बिना बढ़ाने का तरीका बताते हैं।

## Operations, release और security

`Node Initialization`, [Adding a Validator](./operators/add-validator.md), `Observability Guide`, [लॉन्च रनबुक](./release/launch-runbook.md), `Release Pipeline` और [Version Compatibility Matrix](./release/version-compatibility.md) operator path बनाते हैं। [Security Audit Readiness](./security/audit-readiness.md) threat model और अनिवार्य evidence देता है।

## Maturity rule

केवल code होना production readiness सिद्ध नहीं करता। Unit, adversarial और E2E tests, operational artifacts, assumptions, failure modes और release gate results जरूरी हैं। Commands, RPC methods और config keys हर translation में समान रहते हैं।

## शोध और प्रकाशन

शोध-पत्र तैयार करते समय [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md) से शुरू करें। यह दस्तावेज लागू किए गए adaptive round timeout, recovery finality gate और deterministic transaction ordering को पूर्व कार्य से अलग करता है तथा शोध प्रश्न, परिकल्पनाएं, प्रयोग प्रक्रिया, पुनरुत्पाद्य artefacts और शोध नैतिकता एक स्थान पर देता है। बिना मापे प्रदर्शन को परिणाम नहीं बताया गया है और PoS, BFT या HotStuff को स्वयं नई खोज नहीं माना गया है।

भाषाओं के बीच समान दस्तावेज पहचानने के लिए `Node Initialization`, `Docker Deployment`, `Observability Guide`, `RPC API Versioning`, `Production Readiness`, `Release Pipeline` और `Adaptive Recovery-Gated HotStuff Research Draft` नाम अपरिवर्तित रखे गए हैं।

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: How to Read This Set — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Start Here — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Protocol Specs — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: SDK and Extension Guides — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Operations and Release — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Security — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Localized Documentation — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Writing New Docs — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Production Claim Rule — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Documentation Review Checklist — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `vexo-consensus` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/v1/*` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `make docs-check` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod status --json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `feature_assurance` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json:p2p.auth_replay_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json:p2p.node_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `module_config.json:governance.RequireDeposit` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `module_config.json:governance.MinDeposit` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `consensus_config.json:consensus.execution_commit` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `mempool_config.json:mempool.WALPath` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
