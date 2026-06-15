# सहमति विनिर्देश

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।


## पहले क्या पढ़ें

यह दस्तावेज़ Consensus Spec की normative specification बताता है। पहली बार पढ़ रहे हों तो इस क्रम में पढ़ें।

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

यह क्रम पढ़ने के सही तरीके से मेल खाता है: पहले scope और state, फिर message, safety और liveness नियम, और अंत में evidence।

## सारांश

यह दस्तावेज़ consensus state machine की normative specification को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/hi/specs/consensus-spec.md`

## यह दस्तावेज़ क्यों पढ़ें

- consensus state machine की normative specification
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

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## अंग्रेज़ी source की संरचना

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Empty blocks और round recovery

`create_empty_blocks=false` होने पर खाली mempool में height स्थिर दिखना सामान्य idle अवस्था है। transaction आने पर node अगले local proposer round तक आगे बढ़कर transaction block बना सकता है, लेकिन QC/finality नियम वही रहते हैं।

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: Scope — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Roles — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: State — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Message Types — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Safety Rules — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Finality Rule — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Execution Commit Policy — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Liveness Assumptions — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Empty Blocks and Round Recovery — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Evidence — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `chain_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `validator_set_hash` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `locked_qc` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `high_qc` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `last_timeout_cert` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `last_finalized` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `>= 2/3` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `B3.height = B2.height + 1` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `B2.height = B1.height + 1` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `execution_commit = "qc"` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `execution_commit = "finalized"` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `block_committed` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `create_empty_blocks = false` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `latest_height = 0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `latest_height` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `actual_hash` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `actual_time_unix_nano` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `parity_shards` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
