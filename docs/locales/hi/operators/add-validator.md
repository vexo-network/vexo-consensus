> Locale: hi · हिन्दी

# एक सत्यापनकर्ता जोड़ना

यह मार्गदर्शिका वेक्सो नेटवर्क में एक सत्यापनकर्ता जोड़ने के लिए ऑपरेटर प्रवाह का वर्णन करती है।

सटीक प्रवेश पथ श्रृंखला की हिस्सेदारी और शासन नीति पर निर्भर करता है। कम से कम, सत्यापनकर्ता को श्रृंखला स्थिति में प्रस्तुत किया जाना चाहिए, वैध क्रेडेंशियल होना चाहिए, और ऊंचाई-संस्करण वाले सत्यापनकर्ता सेट अपडेट का हिस्सा बनना चाहिए।

## 1. वैलिडेटर होम आरंभ करें
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
बीएलएस सत्यापनकर्ता कुंजी के लिए:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
इन आदेशों को चलाने से पहले `VEXO_KEY_PASSPHRASE` सेट करें, या एकबारगी स्थानीय सेटअप के लिए `--passphrase` पास करें।

मौजूदा श्रृंखला में बीएलएस सत्यापनकर्ता को स्वीकार करते समय, सत्यापनकर्ता अद्यतन प्रस्ताव में उत्पन्न `bls_pop` मेटाडेटा शामिल करें।
डिफ़ॉल्ट BLS कुंजी पथ `blst-bls12381-minpk-v1` का उपयोग करता है; केवल संदर्भ/संगतता परीक्षण के लिए `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` का उपयोग करें।

उत्पन्न सार्वजनिक कुंजी को संग्रहीत करें:
```bash
vexod keys show --home .vexo-validator-new --json
```
जनरेट किए गए `node.key.json` को भी रखें। यह `network_config.json:p2p.node_id` के लिए P2P हैंडशेक पर हस्ताक्षर करता है; यह एक सत्यापनकर्ता सर्वसम्मति कुंजी नहीं है और इसे खाता कुंजी के रूप में पुन: उपयोग नहीं किया जाना चाहिए।

## 2. नेटवर्क पते और साथियों को कॉन्फ़िगर करें

`.vexo-validator-new/network_config.json` को संपादित करें और स्थानीय श्रवण पते और लगातार साथियों को सेट करें:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
उत्पादन सत्यापनकर्ताओं के लिए लंबे समय तक चलने वाली कमांड-लाइन नेटवर्किंग ओवरराइड पर भरोसा न करें। लगातार सहकर्मी पते `network_config.json` में रखें।

अलग-अलग पता भूमिकाओं का उपयोग करें:

- `p2p.listen_address` और `rpc.address` इस मशीन या कंटेनर के लिए स्थानीय बाइंड पते हैं।
- `p2p.node_id` इस नोड की सहकर्मी पहचान है। साथियों द्वारा इसे सीख लेने के बाद इसे स्थिर रखें।
- `p2p.node_key_path` उस सहकर्मी की पहचान के लिए स्थानीय हैंडशेक हस्ताक्षर कुंजी की ओर इशारा करता है।
- `p2p.peers` में डायल लक्ष्य शामिल हैं जिनका उपयोग यह नोड अन्य साथियों तक पहुंचने के लिए करता है; मानचित्र कुंजियाँ दूरस्थ नोड्स के `p2p.node_id` मान होनी चाहिए।
- सत्यापनकर्ता मेटाडेटा `p2p_address` और `rpc_address` में सार्वजनिक विज्ञापित पते होने चाहिए, न कि केवल डॉकर सेवा नाम, जब तक कि नेटवर्क जानबूझकर निजी न हो।

## 3. सत्यापनकर्ता प्रवेश जमा करें

उदाहरण के लिए स्टेकिंग प्रवाह, एक स्टेकिंग लेनदेन बनाएं:
```bash
vexod staking --help
```
सत्यापनकर्ता प्रवेश लेनदेन में शामिल होना चाहिए:

- सत्यापनकर्ता आईडी
- सत्यापनकर्ता पता
- सर्वसम्मति सार्वजनिक कुंजी
- मतदान शक्ति या हिस्सेदारी संदर्भ
- सत्यापनकर्ता कमीशन आधार अंक, यदि श्रृंखला स्व-सेवा कमीशन अपडेट की अनुमति देती है
- P2P `node_id` मेटाडेटा यदि श्रृंखला सहकर्मी मानचित्रों को पूर्वनिर्धारित करने के लिए उत्पत्ति/सत्यापनकर्ता मेटाडेटा का उपयोग करती है
- सार्वजनिक पी2पी पता मेटाडेटा
- सार्वजनिक आरपीसी पता मेटाडेटा, यदि सार्वजनिक है
- बीएलएस सक्षम होने पर बीएलएस कब्जे का सबूत मेटाडेटा

सत्यापनकर्ता अद्यतन एक विशिष्ट ऊंचाई पर प्रभावी होना चाहिए और एक नया सत्यापनकर्ता-सेट हैश उत्पन्न करना चाहिए।

सत्यापनकर्ता सक्रिय होने के बाद, ऑपरेटर स्टेकिंग मॉड्यूल के माध्यम से इनाम स्थिति को उजागर कर सकते हैं:
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. सत्यापनकर्ता सेट अद्यतन सत्यापित करें

अद्यतन ऊंचाई के बाद:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
जांचें:

- सत्यापनकर्ता ऊंचाई-विशिष्ट सेट में दिखाई देता है
-मतदान शक्ति सही है
- सत्यापनकर्ता-सेट हैश अपेक्षा के अनुरूप बदल गया
- अंतिम प्रमाण सही सत्यापनकर्ता-सेट ऊंचाई का संदर्भ देते हैं

## 5. सत्यापनकर्ता कुंजी रोटेशन की योजना बनाएं

गैर-अतिव्यापी `active_from` और `active_until` मेटाडेटा के साथ अगला कुंजी दस्तावेज़ तैयार करके सत्यापनकर्ता कुंजियों को घुमाया जा सकता है, फिर अतिरिक्त रोटेशन कुंजी के साथ नोड शुरू किया जा सकता है:
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
हस्ताक्षर करते समय, नोड उस कुंजी का उपयोग करता है जिसकी सक्रिय विंडो में सर्वसम्मति की ऊंचाई होती है। दूरस्थ हस्ताक्षरकर्ता कुंजी दस्तावेज़ समान नीति, ऑथ-टोकन और डबल-साइन गार्ड आवश्यकताओं को रखते हैं।

## 6. सत्यापनकर्ता प्रारंभ करें
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
स्टार्टअप में कोई नेटवर्क मोड स्विच नहीं है। स्टार्टअप से पहले `config audit --strict` का उपयोग करें जब नेटवर्क से सार्वजनिक-नेटवर्क सुरक्षा मान्यताओं को पूरा करने की उम्मीद की जाती है।

## 7. मॉनिटर

देखें:

- प्रस्ताव/वोट विलंबता
- राउंड टाइमआउट
- सत्यापनकर्ता के हस्ताक्षर विफल
- सहकर्मी प्रतिबंध
- मेमपूल आकार
- विलंबता प्रतिबद्ध
- स्नैपशॉट/रीप्ले स्वास्थ्य

उपयोग करें:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## सुरक्षा नोट

- स्वतंत्र श्रृंखलाओं में कभी भी सत्यापनकर्ता कुंजियों का पुन: उपयोग न करें।
- उत्पादन सत्यापनकर्ताओं के लिए दूरस्थ हस्ताक्षरकर्ता नीति सक्षम रखें।
- कब्जे के सबूत या समकक्ष दुष्ट-कुंजी बचाव के बिना बीएलएस सत्यापनकर्ता को स्वीकार न करें।
- सही साक्ष्य-ऊंचाई सत्यापनकर्ता सेट से जुड़े सत्यापित साक्ष्य के बिना किसी सत्यापनकर्ता को न काटें या जेल न भेजें।

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: 1. Initialize Validator Home — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: 2. Configure Network Addresses and Peers — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: 3. Submit Validator Admission — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: 4. Verify Validator Set Update — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: 5. Plan Validator Key Rotation — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: 6. Start Validator — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: 7. Monitor — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Safety Notes — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `VEXO_KEY_PASSPHRASE` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--passphrase` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `bls_pop` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `blst-bls12381-minpk-v1` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `node.key.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json:p2p.node_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `.vexo-validator-new/network_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.listen_address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.node_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.node_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.peers` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p_address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc_address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `node_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `active_from` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `active_until` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `config audit --strict` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
