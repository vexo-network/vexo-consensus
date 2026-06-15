# नेटवर्किंग विनिर्देश

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।


## पहले क्या पढ़ें

यह दस्तावेज़ Networking Spec की normative specification बताता है। पहली बार पढ़ रहे हों तो इस क्रम में पढ़ें।

1. Scope
2. Transport
3. Topics
4. Handshake
5. Wire Compatibility
6. Address Roles
7. Peer Scoring
8. Reconnect and Backoff
9. DoS/DDOS Defenses
10. Operational Signals

यह क्रम पढ़ने के सही तरीके से मेल खाता है: पहले scope और state, फिर message, safety और liveness नियम, और अंत में evidence।

## सारांश

यह दस्तावेज़ P2P handshake, gossip, peer scoring और ban policy को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/specs/networking-spec.md`
- Locale path: `docs/locales/hi/specs/networking-spec.md`

## यह दस्तावेज़ क्यों पढ़ें

- P2P handshake, gossip, peer scoring और ban policy
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

- `consensus`
- `tx`
- `commit`
- `evidence`
- `network_config.json`
- `rpc.address`
- `p2p.listen_address`
- `p2p.peers`
- `p2p.seeds`
- `p2p_address`
- `rpc_address`
- `host:port`
- `0.0.0.0:26656`
- `[::]:26656`
- `0`
- `p2p.tls_cert_path`
- `p2p.tls_key_path`
- `p2p.tls_ca_path`
- `p2p.tls_server_name`
- `start`
- `BanThreshold`
- `MaxScore`

- `validator_id`
- `p2p.node_id`
- `node.key.json`
- `p2p.node_key_path`
- `signature_nonce`
- `node_public_key`
- `signature`
- `Wire Compatibility`
## अंग्रेज़ी source की संरचना

- Networking Spec
- Scope
- Transport
- Topics
- Handshake
- Address Roles
- Transport TLS
- Peer Scoring
- Reconnect and Backoff
- DoS/DDOS Defenses
- Operational Signals

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../../en/specs/networking-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Peer timing और permanent peers

सिर्फ अस्थायी dial failure से configured peer या seed ban नहीं होता। failure backoff और diagnostics में दिखता है; ban malicious gossip, auth failure, या rate-limit abuse जैसे व्यवहार प्रमाण से होना चाहिए। `p2p.dial_timeout` multi-region latency और TLS/auth cost देखकर सेट करें।

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: Scope — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Transport — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Topics — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Handshake — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Wire Compatibility — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Address Roles — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Peer Scoring — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Reconnect and Backoff — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: DoS/DDOS Defenses — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Operational Signals — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `validator_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json:p2p.node_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `node_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `node.key.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json:p2p.auth_replay_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json:p2p.node_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.dial_timeout` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `signature_nonce` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `node_public_key` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.listen_address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.peers` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.seeds` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p_address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc_address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `host:port` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `0.0.0.0:26656` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `[::]:26656` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_cert_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_ca_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_server_name` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.tls_cert_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.tls_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.tls_ca_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.tls_server_name` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.admin_token` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.admin_tokens` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
