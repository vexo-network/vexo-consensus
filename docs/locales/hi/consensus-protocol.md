> Locale: hi · हिन्दी

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- एक - तिहाई से भी कम बीजान्टिन मतदान शक्ति
- डोमेन से अलग किए गए प्रस्ताव, वोट, टाइमआउट - वोट और अंतिम हस्ताक्षर
- प्रासंगिक प्रमाण ऊंचाई पर सत्यापनकर्ता - सेट हैश बाइंडिंग
- क्यूसी और फ़ाइनल प्रूफ़ में विशिष्ट ज्ञात हस्ताक्षरकर्ता
- सत्यापनकर्ता के इक्विवोकेशन के लिए जवाबदेह सबूत
- एक ही अंतिम ऊंचाई पर परस्पर विरोधी प्रतिबद्ध निर्णयों की अस्वीकृति

## क्रिप्टो सीमा

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- हर वैलिडेटर घर के लिए सख्त कॉन्फ़िगरेशन ऑडिट
- रिलीज़ - गेट सबूत
- बाहरी सुरक्षा समीक्षा
- लंबे समय तक चलने वाले कई मेज़बान और अराजकता के सबूत
- हस्ताक्षरकर्ता/केएमएस नीति साक्ष्य
- चेन - विशिष्ट आर्थिक और शासन नीति की समीक्षा

रिलीज़ को प्रोडक्शन के लिए तैयार मानने से पहले [Security Audit Readiness ](./ security/audit-readiness.md) और [Release Pipeline ](./ release/release-pipeline.md) देखें।
<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट उन तकनीकी शब्दों और इंटरफेस को सुरक्षित रखता है जो कैनॉनिकल संस्करण और अनुवाद के बीच नहीं बदलने चाहिए।

### सेक्शन ट्रैकिंग
- section: Model - HotStuff, three-chain finality, QC, timeout certificate और locked-QC safety को साथ पढ़ना चाहिए।
- section: Execution Terms - qc certified, finalized, executed और state committed के बीच का अंतर स्पष्ट रहना चाहिए।
- section: Safety Boundary - एक-तिहाई से कम byzantine सीमा, domain separation, validator-set hash binding और accountable evidence की जाँच करें।
- section: Crypto Boundary - `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1` और `ecvrf-p256-sha256-tai-v1` पहचानकर्ताओं को स्थिर रखें।
- section: Operational Boundary - `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled` और snapshot/replay संकेतों को साथ देखें।
- `require_network_safety` और `block_committed` को अनुवाद में भी जैसा है वैसा ही दिखना चाहिए।
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### बनाए रखने योग्य इंटरफेस
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## संचालन नोट्स

नया validator home बनाते समय `config.json` के साथ `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` और `log_config.json` को भी जाँचें।
production में `vexo_quorum_health_ratio` और `adaptive_round_timeout_enabled` को साथ में देखें।

- `execution_commit=finalized` को प्राथमिकता दें।
- `qc` केवल नियंत्रित testnet में चालू करें।
- `recovery_finality_gate_enabled` को snapshot और replay प्रमाणों के साथ सत्यापित करें।
