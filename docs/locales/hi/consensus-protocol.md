> Locale: hi · हिन्दी

# सहमति प्रोटोकॉल का अवलोकन

यह पृष्ठ Vexo consensus दस्तावेजों का उच्च-स्तरीय प्रवेश बिंदु है। मानक विवरण [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md) और [Transaction Format](./specs/tx-format.md) में हैं।

## मॉडल

Vexo proposal, vote, quorum certificate(QC), timeout certificate, locked-QC सुरक्षा और three-chain finality वाला HotStuff शैली का BFT core उपयोग करता है। किसी block को vote देना तभी सुरक्षित है जब वह locked QC को बढ़ाए या lock जितना नया justify QC रखे। block, parent और grandparent की height तथा hash को स्पष्ट रूप से न बांधने वाली synthetic या height-skipping QC chain finality निर्णय से पहले अस्वीकार होती है।

## प्रोटोकॉल पहचान और शोध सीमा

Vexo अपरिवर्तित HotStuff का नया नाम नहीं है और AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint या CometBFT जैसा ही protocol या implementation भी नहीं है। स्वतंत्र Go runtime HotStuff सुरक्षा अवधारणाओं के साथ adaptive round timing, durable recovery, deterministic transaction ordering, modular execution और height-versioned validator sets जोड़ता है।

सक्रिय vote path पूरी height-versioned validator set और deterministic proposer उपयोग करता है। VRF committee selector component और query के रूप में उपलब्ध है, लेकिन proposal eligibility या quorum formation से अभी जुड़ा नहीं है। इसलिए इसे enabled consensus property नहीं, future work लिखा जाना चाहिए। योगदान और प्रयोग विधि के लिए [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) देखें।

## निष्पादन और recovery सीमा

QC certification, HotStuff finalization, application execution और state commit अलग घटनाएं हैं। डिफॉल्ट `execution_commit=finalized` केवल three-chain rule द्वारा चुने ancestor को execute करता है। adaptive pacemaker और `recovery_finality_gate_enabled` latency और restart recovery नियंत्रित करते हैं; वे proposer, quorum power, safe-vote या finality rule नहीं बदलते।

## सुरक्षा सीमा

- एक - तिहाई से भी कम बीजान्टिन मतदान शक्ति
- डोमेन से अलग किए गए प्रस्ताव, वोट, टाइमआउट - वोट और अंतिम हस्ताक्षर
- प्रासंगिक प्रमाण ऊंचाई पर सत्यापनकर्ता - सेट हैश बाइंडिंग
- क्यूसी और फ़ाइनल प्रूफ़ में विशिष्ट ज्ञात हस्ताक्षरकर्ता
- सत्यापनकर्ता के इक्विवोकेशन के लिए जवाबदेह सबूत
- एक ही अंतिम ऊंचाई पर परस्पर विरोधी प्रतिबद्ध निर्णयों की अस्वीकृति

## क्रिप्टो सीमा

- `deterministic` backend केवल परीक्षण के लिए है और network safety validation में विफल होता है।
- `ed25519` public-network परीक्षण और launch तैयारी के लिए समर्थित है।
- `bls` डिफॉल्ट रूप से `blst-bls12381-minpk-v1` उपयोग करता है और proof-of-possession, subgroup check, public-key validation, dependency audit तथा release-gate evidence मांगता है।
- validation को VRF adapter metadata चाहिए, पर इससे VRF committee सक्रिय consensus path नहीं बन जाता।

- हर वैलिडेटर घर के लिए सख्त कॉन्फ़िगरेशन ऑडिट
- रिलीज़ - गेट सबूत
- बाहरी सुरक्षा समीक्षा
- लंबे समय तक चलने वाले कई मेज़बान और अराजकता के सबूत
- हस्ताक्षरकर्ता/केएमएस नीति साक्ष्य
- चेन - विशिष्ट आर्थिक और शासन नीति की समीक्षा

रिलीज़ को प्रोडक्शन के लिए तैयार मानने से पहले [Security Audit Readiness](./security/audit-readiness.md) और [Release Pipeline](./release/release-pipeline.md) देखें।
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
