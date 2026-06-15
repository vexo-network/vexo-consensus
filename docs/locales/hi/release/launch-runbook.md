# लॉन्च रनबुक

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।


## पहले क्या पढ़ें

यह दस्तावेज़ Launch Runbook की release और operations प्रक्रिया बताता है। पहली बार पढ़ रहे हों तो इस क्रम में पढ़ें।

1. At a Glance
2. Prelaunch Gate
3. Release Candidate Gate
4. Genesis Gate
5. Launch Window
6. Postlaunch Archive

यह क्रम practical उपयोग से मेल खाता है: पहले goals और gates, फिर artifacts और evidence requirements, और अंत में execution steps।

## सारांश

यह दस्तावेज़ network launch से पहले operator checklist और execution procedure को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/hi/release/launch-runbook.md`

## यह दस्तावेज़ क्यों पढ़ें

- network launch से पहले operator checklist और execution procedure
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

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## अंग्रेज़ी source की संरचना

- लॉन्च रनबुक
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## EVM/Web3 अनुरूपता प्रमाण

सार्वजनिक रिलीज़ से पहले `--evm-web3-conformance-evidence` को `--sdk-conformance-evidence` से अलग archive करें। फ़ाइल में `evm_fixtures`, `evm_execution`, `web3_rpc` और `evm_corpus` होने चाहिए, ताकि `release gate` केवल अप्रमाणित summary को reject कर सके।

## VRF audit evidence SHA-256

release candidate जाँचते समय `release gate` को BLS और VRF दोनों audit evidence digest दें। कम से कम `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256` और `--evidence-manifest` साथ चलाएँ, फिर हर evidence file का SHA-256 manifest से मिलाएँ।

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../../en/release/launch-runbook.md)

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: Prelaunch Gate — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Release Candidate Gate — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Genesis Gate — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Launch Window — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Postlaunch Archive — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `release docs-quality` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `checksums.txt` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `sbom-go-modules.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `sbom-go-version.txt` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `release-manifest.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `release-audit-pack.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `release collect-evidence` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network analyze-longrun` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `longrun-evidence.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-default-fixtures` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-tx-fixtures` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-tx-fixtures-dir` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-execution-fixtures` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-execution-fixtures-dir` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-tx-fixtures-sha256` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-execution-fixtures-sha256` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-web3-conformance-evidence` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evm_fixtures` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evm_execution` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `web3_rpc` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evm_corpus` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod ops conformance` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `relayer soak-plan` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `chain_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evidence-manifest.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
