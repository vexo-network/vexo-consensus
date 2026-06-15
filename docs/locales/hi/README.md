# Documentation

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।

## त्वरित शुरुआत

- `make build` से बाइनरी बनाइए.
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` से validator home बनाइए, फिर `vexod validate --home .vexo-validator-1` और `vexod config audit --home .vexo-validator-1 --strict` से जाँच करके `vexod start --home .vexo-validator-1` चलाइए.
- Docker नेटवर्क पहले `docker compose -f deployments/docker/compose.single-host-init.yml up` और फिर `docker compose -f deployments/docker/compose.single-host.yml up` से चलाइए.
- Remix के लिए `http://127.0.0.1:28657/web3` इस्तेमाल करें; chain ID जाँचने के लिए `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` चलाइए.
- दस्तावेज़ बदलने के बाद `make docs-check` चलाकर locale tree और translation guards जाँचिए.
## सारांश

यह दस्तावेज़ documentation index और सुझाया गया पढ़ने का क्रम को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/hi/README.md`

## यह दस्तावेज़ क्यों पढ़ें

- documentation index और सुझाया गया पढ़ने का क्रम
- पहले अंग्रेज़ी source में MUST/SHOULD/MAY statements जाँचें।
- यह स्थानीयकृत दस्तावेज़ समझने में मदद करता है; audit, release और security decisions अंग्रेज़ी source से तय होते हैं।

## पढ़ने के बाद क्या कर पाना चाहिए

- समझाना कि यह दस्तावेज़ किस implementation या operation decision में मदद करता है।
- अंग्रेज़ी source की normative requirements को वर्तमान network configuration से जोड़ना।
- examples copy करने से पहले chain ID, validator ID, fee/gas और peer addresses जाँचना।

## सुरक्षित उपयोग checklist

- पहले अंग्रेज़ी source में MUST/SHOULD/MAY statements जाँचें।
- commands, config key, RPC names, JSON fields और code identifiers का अनुवाद न करें।
- examples copy करने से पहले chain ID, validator ID, fee/gas और peer addresses अपनी network settings से मिलाएँ।
- documentation बदलने के बाद `make docs-check` चलाकर locale tree और translation guards जाँचें।

## ध्यान रखने योग्य बातें

- यह स्थानीयकृत दस्तावेज़ समझने में मदद करता है; audit, release और security decisions अंग्रेज़ी source से तय होते हैं।
- implementation बदलने पर अंग्रेज़ी source और सभी स्थानीयकृत दस्तावेज़ को उसी change में update करें।

## ज्यों-का-त्यों रखने वाले interfaces

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## अंग्रेज़ी source की संरचना

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../en/README.md)

## संरक्षित शब्द-सूची

नीचे दिए गए शब्द अनुवादित नहीं किए जाते।

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

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
