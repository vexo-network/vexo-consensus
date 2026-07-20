> Locale: hi · हिन्दी

# नोड आरंभीकरण

यह मार्गदर्शिका बताती है कि सत्यापनकर्ता और संग्रह नोड घरों को कैसे आरंभ किया जाए, उन्हें कैसे शुरू किया जाए, सत्यापित किया जाए कि वे स्वस्थ हैं, और ग्राहकों को कैसे जोड़ा जाए।

पीयर कनेक्टिविटी को `network_config.json` में कॉन्फ़िगर किया जाना चाहिए, `start` कमांड लाइन पर बार-बार पास नहीं किया जाना चाहिए।

रनटाइम व्यवहार जो सर्वसम्मति, आरपीसी, पी2पी, लॉगिंग या प्रबंधित वेब3 खातों को प्रभावित करता है वह केवल कॉन्फिग-फ़ाइल है। `vexod start` `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env`, और `--evm-account-key` जैसे झंडों को अस्वीकार करता है; इसके बजाय विभाजित कॉन्फ़िगरेशन फ़ाइलों को संपादित करें ताकि प्रत्येक ऑपरेटर समान नियतात्मक नोड व्यवहार की समीक्षा करे।

कोई नोड-मोड स्विच नहीं है. एक नोड होम को उसकी कॉन्फ़िगरेशन फ़ाइलों, उत्पत्ति, मुख्य सामग्री और क्या `validator_id` प्लस एक हस्ताक्षरकर्ता मौजूद है, द्वारा परिभाषित किया गया है।

## आप क्या निर्माण कर रहे हैं

वेक्सो नोड होम एक निर्देशिका है जिसमें एक नोड को शुरू करने के लिए आवश्यक सभी चीजें शामिल हैं:
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
महत्वपूर्ण नियम सरल है: एक बार प्रारंभ करें, कॉन्फ़िगरेशन फ़ाइलों को संपादित करें, फिर प्रारंभ करें। शेल फ़्लैग के अंदर नेटवर्क व्यवहार को न छिपाएँ।

## पांच मिनट की लोकल दौड़

जब आप मल्टी-होस्ट परिनियोजन के बारे में सोचने से पहले बाइनरी कार्यों को साबित करना चाहते हैं तो इस प्रवाह का उपयोग करें।
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
दूसरे टर्मिनल में:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
अपेक्षित स्थिति आकार:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
खाली-ब्लॉक निर्माण अक्षम होने पर एकल-नोड या खाली-मेमपूल चलाने पर नवीनतम ऊंचाई शून्य पर रह सकती है। इसका मतलब यह नहीं है कि प्रक्रिया टूट गई है। इसका मतलब है कि नोड खाली ब्लॉक उत्पन्न नहीं कर रहा है। निरंतर प्रतिबद्धताओं का निरीक्षण करने के लिए लेनदेन जोड़ें या बहु-सत्यापनकर्ता परीक्षण नेटवर्क चलाएं।

## चार-सत्यापनकर्ता स्थानीय नेटवर्क

जब आप सहकर्मी कनेक्टिविटी, प्रस्तावक रोटेशन, ब्लॉक कमिट लॉग और ऊंचाई वृद्धि चाहते हैं तो इस प्रवाह का उपयोग करें।
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
उपयोगी जाँचें:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
यदि ब्लॉक कमिट लॉगिंग `log_config.json` में सक्षम है, तो सत्यापनकर्ता लॉग में निम्न घटनाएं शामिल हैं:
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
जनरेट किए गए स्थानीय नेटवर्क को इसके साथ रोकें:
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## वेब3 और रीमिक्स

एथेरियम-शैली JSON-RPC Web3 समापन बिंदु पर रहता है, संस्करणित Vexo परिचालन API नेमस्पेस के अंतर्गत नहीं।

डॉकर सिंगल-होस्ट सत्यापनकर्ता 1 के लिए, रीमिक्स कस्टम प्रदाता URL है:
```text
http://127.0.0.1:28657/web3
```
डिफ़ॉल्ट RPC पोर्ट वाले सीधे स्थानीय नोड के लिए:
```text
http://127.0.0.1:26657/web3
```
उसी कॉल का परीक्षण करें जो रीमिक्स करता है:
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
यदि कोई ब्राउज़र कहता है कि चेन-आईडी फ़ेच विफल रहा, तो इन्हें क्रम से जांचें:

1. URL Web3 समापन बिंदु पथ के साथ समाप्त होता है।
2. ब्राउज़र होस्ट पोर्ट तक पहुंच सकता है। डॉकर उदाहरण `28657`, `28667`, `28677`, और `28687` को उजागर करते हैं; कंटेनर के अंदर RPC पोर्ट अभी भी `26657` है।
3. आरपीसी सर्वर चल रहा है; एक ही होस्ट और पोर्ट पर स्थिति समापन बिंदु को क्वेरी करें।
4. CORS को `network_config.json`/RPC कॉन्फ़िगरेशन द्वारा अनुमति दी जाती है। जब कोई कस्टम CORS सूची सेट नहीं होती है तो डिफ़ॉल्ट हैंडलर ब्राउज़र प्रीफ़्लाइट की अनुमति देता है।
5. श्रृंखला में `module_config.json` में एक गैर-शून्य ईवीएम श्रृंखला आईडी है।

## सत्यापनकर्ता नोड

`init validator` का उपयोग करें जब नोड प्रस्ताव देगा, वोट देगा, सर्वसम्मति संदेशों पर हस्ताक्षर करेगा और सत्यापनकर्ता रोटेशन में भाग लेगा।
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
इस कमांड को चलाने से पहले `VEXO_KEY_PASSPHRASE` सेट करें, या एक बार के स्थानीय सेटअप के लिए `--passphrase` पास करें। `--encrypt-keys` `validator.key.json`, `node.key.json`, और `validator.vrf.key.json` को एन्क्रिप्ट करता है।

अंगूठे का मुख्य अभिरक्षा नियम:

- `validator.key.json` आम सहमति प्रस्तावों, वोटों, टाइमआउट वोटों और अंतिमता से संबंधित संदेशों पर हस्ताक्षर करता है।
- `node.key.json` केवल P2P हैंडशेक पर हस्ताक्षर करता है; इसे कभी भी सत्यापनकर्ता सर्वसम्मति कुंजी के रूप में पुन: उपयोग नहीं किया जाना चाहिए।
- `validator.vrf.key.json` समिति की यादृच्छिकता को साबित करता है और इसे सत्यापनकर्ता हिरासत सामग्री की तरह माना जाना चाहिए।
- सार्वजनिक श्रोताओं को एन्क्रिप्टेड स्थानीय कुंजी दस्तावेज़ या रिमोट साइनर/केएमएस-शैली कुंजी दस्तावेज़ का उपयोग करना होगा। यदि कोई नोड `require_network_safety=true` के दौरान सार्वजनिक RPC या प्रमाणित सार्वजनिक P2P को उजागर करता है, तो स्टार्टअप प्लेनटेक्स्ट स्थानीय सत्यापनकर्ता कुंजियों को अस्वीकार कर देता है।
- जेनरेट की गई कुंजियाँ फ़ाइल सिस्टम मोड `0600` के साथ लिखी जाती हैं; लंबे समय तक जीवित रहने वाले सत्यापनकर्ताओं के लिए अभी भी एक दूरस्थ हस्ताक्षरकर्ता/केएमएस को प्राथमिकता देते हैं।

बीएलएस सर्वसम्मति कुंजी के लिए:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` एक `blst-bls12381-minpk-v1` BLS कुंजी दस्तावेज़ लिखता है और कब्जे के प्रमाण को `genesis.json` सत्यापनकर्ता मेटाडेटा में `bls_pop` के रूप में कॉपी करता है।

यह बनाता है:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` सर्वसम्मति हस्ताक्षरकर्ता है। `node.key.json` `network_config.json:p2p.node_key_path` द्वारा संदर्भित P2P हैंडशेक हस्ताक्षरकर्ता है। वे जानबूझकर अलग हैं ताकि संग्रह नोड्स और सत्यापनकर्ता प्रत्येक सहकर्मी को सत्यापनकर्ता हस्ताक्षर कुंजी दिए बिना एक ही परिवहन का उपयोग कर सकें।

इसे कॉन्फिग-संचालित नेटवर्किंग से प्रारंभ करें:
```bash
vexod start --home .vexo-validator-1
```
स्टार्टअप के बाद, लॉग पढ़ें। एक स्वस्थ सत्यापनकर्ता को नोड-रनिंग, आरपीसी-सुनना, पी2पी-सुनना, और, एक बार ब्लॉक प्रतिबद्ध होने के बाद, ब्लॉक-प्रतिबद्ध घटनाओं का उत्सर्जन करना चाहिए। यदि खाली-ब्लॉक निर्माण अक्षम है, तो ब्लॉक-प्रतिबद्ध लॉग गुम होने का मतलब यह हो सकता है कि कोई लेनदेन नहीं है।

## पुरालेख नोड

`init archive` का उपयोग करें जब नोड को श्रृंखला डेटा रखना चाहिए, RPC को उजागर करना चाहिए, साथियों से सिंक करना चाहिए, और सत्यापनकर्ता हस्ताक्षर से बचना चाहिए।
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
यह बनाता है:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

यह `validator.key.json` नहीं** बनाता है।

इसे इससे प्रारंभ करें:
```bash
vexod start --home .vexo-archive-1
```
पुरालेख नोड्स आम सहमति वोटों पर हस्ताक्षर नहीं करते हैं। वे आरपीसी, इंडेक्सिंग, स्टेट सिंक, ऐतिहासिक प्रूफ सर्विंग और प्रूनिंग सत्यापनकर्ताओं की तुलना में व्यापक क्वेरी इतिहास रखने के लिए उपयोगी हैं।

## कॉन्फ़िगरेशन फ़ाइलों को विभाजित करें

नोड होम अलग-अलग कॉन्फ़िगरेशन फ़ाइलों का उपयोग करते हैं ताकि ऑपरेटर असंबंधित सेटिंग्स को मिश्रित किए बिना एक सबसिस्टम को संपादित कर सकें:

- `config.json` में नोड पहचान, चेन आईडी, डेटा पथ और विभाजित कॉन्फ़िगरेशन फ़ाइलों के पॉइंटर्स शामिल हैं।
- `module_config.json` में एप्लिकेशन मॉड्यूल चयन, निष्पादन/पूर्व नीति और मॉड्यूल-स्तरीय शासन नीति शामिल है।
- `network_config.json` में RPC, P2P नोड पहचान, श्रवण/पीयर/सीड सेटिंग्स, TLS/ऑथ सेटिंग्स और पीयर-स्कोरिंग नीति शामिल है।
- `consensus_config.json` में सर्वसम्मति लूप टाइमिंग, खाली-ब्लॉक नीति, क्रिप्टो बैकएंड, वीआरएफ, सत्यापनकर्ता प्रवेश और समिति नीति शामिल है।
- `mempool_config.json` में मेमपूल आकार, शुल्क, प्राथमिकता, वाल, डुप्लिकेट और टीटीएल नीति शामिल है।
- `log_config.json` में लॉग प्रारूप, स्तर, ब्लॉक कमिट इवेंट लॉगिंग और पीयर इवेंट लॉगिंग शामिल है।
- `genesis.json` में अपरिवर्तनीय उत्पत्ति सत्यापनकर्ता, सत्यापनकर्ता मेटाडेटा और उत्पत्ति मॉड्यूल स्थिति शामिल है।

`network_config.json` RPC सेटिंग्स में `shutdown_timeout`, `web3_max_subscriptions_per_connection`, और `web3_idle_timeout` भी शामिल हैं। `shutdown_timeout` सर्वसम्मति लूप, RPC सर्वर और नोड ट्रांसपोर्ट के लिए शानदार शटडाउन को सीमित करता है ताकि ऑपरेटर अटके हुए स्टॉप पथ पर हमेशा के लिए इंतजार न करें। उत्पन्न डिफ़ॉल्ट `10s` है; Web3 सदस्यताएँ `2m` निष्क्रिय समयबाह्य के साथ प्रति कनेक्शन 256 पर डिफ़ॉल्ट होती हैं, इसलिए सार्वजनिक RPC एंडपॉइंट असीमित निष्क्रिय सदस्यताएँ जमा नहीं कर सकते हैं।

`network_config.json` P2P सेटिंग्स में `auth_replay_path`, `require_auth_replay_store`, और `dial_timeout` शामिल हैं। उत्पन्न डिफ़ॉल्ट `data/p2p_auth_replay.jsonl` को नॉन रीप्ले साक्ष्य लिखता है और `10s` आउटबाउंड डायल टाइमआउट का उपयोग करता है। निजी लूपबैक परीक्षण के लिए रीप्ले स्टोर अधिकतर हानिरहित बहीखाता पद्धति है; सार्वजनिक रूप से प्रमाणित पी2पी के लिए यह एक सुरक्षा आवश्यकता है क्योंकि यह कैप्चर किए गए हस्ताक्षरित हैंडशेक को पुनरारंभ के बाद दोबारा चलाए जाने से रोकता है। `dial_timeout` टीएलएस, हस्ताक्षरित हैंडशेक सत्यापन और क्रॉस-रीजन विलंबता के लिए पर्याप्त लंबा होना चाहिए; इसे बहुत कम सेट करने से स्वस्थ साथी परतदार दिखते हैं और पुनः आरंभ करने के बाद जीवंतता धीमी हो सकती है।

`network_config.json` के पास स्टार्टअप स्टेट सिंक का भी स्वामित्व है। यह संग्रह नोड्स, प्रतिस्थापन सत्यापनकर्ताओं, या एक साफ मशीन पर पुनर्स्थापित नोड्स के लिए उपयोगी है। जब `state_sync.enabled` सत्य होता है, तो `vexod start` `state_sync.snapshot_urls` से पहला वैध स्नैपशॉट डाउनलोड करता है, चेन आईडी, चेकसम, स्टेट रूट्स और KV नेमस्पेस को सत्यापित करता है, इसे लेवलडीबी में पुनर्स्थापित करता है, इंडेक्स को फिर से बनाता है, और उसके बाद ही नोड शुरू करता है। यदि स्थानीय स्थिति पहले से ही `state_sync.min_height` को संतुष्ट करती है और `state_sync.trust_local_higher` सत्य है, तो स्टार्टअप `state_sync_skipped` को लॉग करता है और स्थानीय स्टोर रखता है।

उदाहरण `state_sync` ब्लॉक:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
स्टार्टअप फ़ेच त्रुटि के लिए `state_sync_candidate_failed`, अमान्य या पुराने स्नैपशॉट के लिए `state_sync_candidate_rejected` और सत्यापित पुनर्स्थापना के बाद `state_sync_applied` लॉग करता है। `max_snapshot_bytes` को आपके बुनियादी ढांचे द्वारा जानबूझकर पेश किए गए सबसे बड़े स्नैपशॉट के नीचे रखें, लेकिन सामान्य राज्य विकास के लिए पर्याप्त ऊंचा। किसी अप्रमाणित तृतीय-पक्ष स्नैपशॉट स्रोत पर सार्वजनिक नोड्स को इंगित न करें जब तक कि ऑपरेटर के पास उस स्रोत के लिए आउट-ऑफ़-बैंड ट्रस्ट नीति और अंतिम/लाइट-क्लाइंट साक्ष्य न हो।

यदि कोई फ़ील्ड नेटवर्क व्यवहार को बदलता है, तो विभाजित कॉन्फ़िगरेशन फ़ाइल को संपादित करें और उस समीक्षा की गई फ़ाइल को प्रतिबद्ध या वितरित करें। रनटाइम व्यवहार के लिए लंबे `vexod start` फ़्लैग पर भरोसा न करें। स्टार्ट कमांड जानबूझकर सर्वसम्मति समय, खाली-ब्लॉक, पी2पी ऑथ, आरपीसी एडमिन और प्रबंधित वेब3 कुंजी फ़्लैग को अस्वीकार कर देता है ताकि ऑपरेटर गलती से समीक्षा की गई कॉन्फ़िगरेशन से भिन्न व्यवहार न चलाएं।

## मैं कौन सी फ़ाइल संपादित करूं?

| लक्ष्य | फ़ाइल | फ़ील्ड |
|---|---|---|
| आरपीसी बाइंड पोर्ट बदलें | `network_config.json` | `rpc.address` |
| पी2पी बाइंड पोर्ट बदलें | `network_config.json` | `p2p.listen_address` |
| लगातार साथियों को जोड़ें | `network_config.json` | `p2p.peers` |
| बीज साथियों को जोड़ें | `network_config.json` | `p2p.seeds` |
| खाली ब्लॉक सक्षम/अक्षम करें | `consensus_config.json` | सर्वसम्मति खाली-ब्लॉक फ़ील्ड |
| सर्वसम्मति टाइमआउट ट्यून करें | `consensus_config.json` | प्रस्ताव, प्रीवोट, प्रीकमिट, और टाइमआउट फ़ील्ड प्रतिबद्ध करें |
| अंतिम निष्पादन की आवश्यकता है | `consensus_config.json` | सर्वसम्मति निष्पादन-प्रतिबद्ध क्षेत्र |
| मॉड्यूल सक्षम/अक्षम करें | `module_config.json` | एप्लिकेशन मॉड्यूल सूची |
| ईवीएम चेन आईडी बदलें | `module_config.json` | निष्पादन ईवीएम श्रृंखला आईडी फ़ील्ड |
| आधार शुल्क/गैस ट्यून करें | `module_config.json` | निष्पादन आधार-शुल्क, गतिशील-शुल्क, लक्ष्य-गैस, और अधिकतम-गैस फ़ील्ड |
| मेमपूल वाल कॉन्फ़िगर करें | `mempool_config.json` | मेमपूल वाल पथ |
| नियंत्रण ब्लॉक प्रतिबद्ध लॉग | `log_config.json` | लॉग कमिट-इवेंट फ़ील्ड |
| सहकर्मी लॉग को नियंत्रित करें | `log_config.json` | लॉग पीयर-इवेंट फ़ील्ड |

जब संदेह हो, तो दौड़ें:
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## कुंजी प्रकार

सत्यापनकर्ता init डिफ़ॉल्ट रूप से `--key-type bls` पर होता है क्योंकि नेटवर्क-सुरक्षा सत्यापन के लिए ऑडिटेड BLS समग्र अंतिमता की आवश्यकता होती है। `--key-type ed25519` नेटवर्क-सुरक्षा गेट के बाहर निजी प्रयोगों और कस्टम परिनियोजन के लिए उपलब्ध रहता है। `--encrypt-keys` का उपयोग किसी भी नॉन-थ्रोअवे नोड होम के लिए किया जाना चाहिए। स्टैंडअलोन कुंजी पीढ़ी भी वीआरएफ कुंजी का समर्थन करती है:
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
वीआरएफ कुंजियाँ आम सहमति हस्ताक्षरकर्ता नहीं हैं। उनका उपयोग वीआरएफ-समर्थित समिति चयन के लिए किया जाता है और बैकएंड सक्षम होने पर उन्हें `consensus_config.json` से `vrf_key_paths` प्लस सत्यापनकर्ता मेटाडेटा कुंजी `vrf_public_key` तक संदर्भित किया जाना चाहिए।

`config.json` विभाजित कॉन्फ़िगरेशन फ़ाइलों की ओर इंगित करता है:
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
प्रत्येक पथ नोड होम से निरपेक्ष या सापेक्ष हो सकता है। यदि छोड़ दिया जाए, तो `vexod` डिफ़ॉल्ट `<home>/<name>_config.json` फ़ाइल का उपयोग करता है।

उदाहरण `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
शासन नीति भी `module_config.json` में रहती है। जेनरेट की गई नेटवर्क-सुरक्षित कॉन्फ़िगरेशन के लिए प्रस्ताव जमा की आवश्यकता होती है:
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
जमा राशि प्रस्ताव प्रस्तुतकर्ता से प्राप्त मूल शेष राशि है। पारित प्रस्ताव जमा राशि वापस कर दें; अस्वीकृत प्रस्ताव इसे `RejectedDeposits` पर ले जाते हैं। यदि अस्वीकृत जमा को डिफ़ॉल्ट मॉड्यूल खाते के बजाय खजाने को निधि देनी चाहिए तो अपने ट्रेजरी/सामुदायिक-पूल मॉड्यूल द्वारा नियंत्रित पते का उपयोग करें।

उदाहरण `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` और `rpc.evm_account_private_keys` वैकल्पिक हैं और Web3 प्रबंधित-खाता विधियां जैसे `eth_accounts`, `eth_sign`, `eth_signTransaction`, और `eth_sendTransaction` हैं। `evm_account_key_envs` को प्राथमिकता दें ताकि निजी कुंजी JSON में संग्रहीत होने के बजाय प्रक्रिया वातावरण या गुप्त प्रबंधक द्वारा इंजेक्ट की जाए। सामान्य सत्यापनकर्ता संचालन के लिए दोनों सूचियों को खाली रखें जब तक कि यह नोड जानबूझकर स्थानीय वेब3 हॉट-वॉलेट एंडपॉइंट के रूप में कार्य नहीं कर रहा हो। स्टार्टअप सुरक्षा सार्वजनिक आरपीसी श्रोताओं पर प्रबंधित ईवीएम हॉट कुंजियों को अस्वीकार कर देती है।

उदाहरण `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` को `consensus_config.json` वाली निर्देशिका के सापेक्ष हल किया जाता है। एन्क्रिप्टेड कुंजी दस्तावेज़ों का उपयोग करें और स्थानीय वीआरएफ कुंजी अभिरक्षा अपरिहार्य होने पर नोड प्रक्रिया को `VEXO_KEY_PASSPHRASE` प्रदान करें। ऑपरेटर द्वारा संचालित नेटवर्क के लिए कच्चे वीआरएफ निजी स्केलर को सीधे `consensus_config.json` में न डालें।

सभी हल किए गए पथों का निरीक्षण करने के लिए `vexod config paths --home <home>` का उपयोग करें।

पुरालेख कॉन्फ़िगरेशन में है:
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
संग्रह `consensus_config.json` स्थानीय सर्वसम्मति लूप को अक्षम कर देता है:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
जेनरेटेड वैलिडेटर होम डिफ़ॉल्ट रूप से `"require_network_safety": true` को `config.json` में सेट करते हैं। यह कोई विधा नहीं है; यह एक स्टार्टअप सुरक्षा द्वार है जो नियतात्मक क्रिप्टो, अहस्ताक्षरित/गैर-बंद लेनदेन, लापता शुल्क/गैस फर्श, लापता टिकाऊ मेमपूल वाल, समान हस्ताक्षरकर्ता/गैर लेनदेन के लिए लापता प्रतिस्थापन नीति, असुरक्षित समिति यादृच्छिकता, और `finalized` के अलावा `execution_commit` मानों को अस्वीकार करता है।

जब `require_network_safety` सक्षम हो, तो चलाएँ:
```bash
vexod config audit --home <home> --strict
```
नोड शुरू करने से पहले. एक ही नेटवर्क में भाग लेने वाले प्रत्येक सत्यापनकर्ता और संग्रह गृह के लिए ऑडिट पास होना चाहिए।

## कॉन्फिग-आधारित सहकर्मी

सहकर्मी और सुनने वाले पते `network_config.json` में रहते हैं:
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` इन साथियों को स्वचालित रूप से लोड करता है:
```bash
vexod start --home .vexo-archive-1
```
लगातार समकक्ष और बीज `network_config.json` में कॉन्फ़िगर किए गए हैं; `vexod start` पीयर या सीड होस्ट ओवरराइड को स्वीकार नहीं करता है।

लंबे समय तक चलने वाले होस्ट या `host:port` सेटिंग्स को `vexod start` कमांड लाइन पर न रखें। इसके बजाय `rpc.address`, `p2p.listen_address`, `p2p.peers`, और `p2p.seeds` को `network_config.json` में संपादित करें।

नोड होम के जीवनकाल के लिए `p2p.node_id` को स्थिर रखें। `p2p.node_key_path` को `node.key.json` या किसी अन्य स्थानीय/प्रबंधित कुंजी दस्तावेज़ को इंगित करना चाहिए जिसका उपयोग केवल सहकर्मी हैंडशेक हस्ताक्षर के लिए किया जाता है। पीयर मैप्स में पीयर नोड आईडी का उपयोग किया जाना चाहिए, खाता पते या सत्यापनकर्ता ऑपरेटर के नामों का नहीं, जब तक कि वे जानबूझकर समान न हों।

एन्क्रिप्टेड और प्रमाणित जीआरपीसी पीयर ट्रांसपोर्ट के लिए, `network_config.json` में `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path` और वैकल्पिक रूप से `p2p.tls_server_name` भी सेट करें। सापेक्ष टीएलएस पथ नोड होम निर्देशिका से हल किए जाते हैं। `p2p.dial_timeout` को एक ही फ़ाइल में रखें ताकि प्रत्येक ऑपरेटर समान पुन: कनेक्ट व्यवहार का उपयोग करे; शेल स्क्रिप्ट में पीयर टाइमिंग को न छिपाएं।

## आम सहमति का समय

सर्वसम्मति लूप टाइमिंग `consensus_config.json` में रहती है:
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` यह नियंत्रित करता है कि एक दौर किसी प्रस्ताव के लिए कितनी देर तक प्रतीक्षा करता है।
- `timeout_prevote` वोट संग्रह विंडो को नियंत्रित करता है।
- `timeout_precommit` कमिट-सर्टिफिकेट संग्रह विंडो को नियंत्रित करता है।
- `timeout_commit` प्रतिबद्ध ब्लॉक के बाद न्यूनतम विलंब को नियंत्रित करता है।
- `create_empty_blocks: false` का अर्थ है कि नोड केवल तभी प्रस्तावित करता है जब लेनदेन उपलब्ध हो।
- `execution_commit: "finalized"` अंतिम पूर्वज को निष्पादित करने से पहले HotStuff तीन-श्रृंखला अंतिम निर्णय की प्रतीक्षा करता है और जेनरेट किया गया सत्यापनकर्ता डिफ़ॉल्ट है। `execution_commit: "qc"` QC-प्रमाणित ब्लॉक को तुरंत निष्पादित और जारी रखता है, लेकिन सुरक्षा द्वार इसे अस्वीकार कर देता है।

`round_timeout` को केवल अनुकूलता समुच्चय के रूप में रखा गया है। उपरोक्त टेंडरमिंट-शैली टाइमआउट फ़ील्ड को प्राथमिकता दें।

जब `create_empty_blocks` गलत होता है, तो मेमपूल खाली होने पर ऊंचाई अपरिवर्तित रह सकती है। यह अपेक्षित है: श्रृंखला खाली ब्लॉक करने के बजाय उपयोगी कार्य की प्रतीक्षा कर रही है। जब कोई लेन-देन प्रकट होता है और स्थानीय सर्वसम्मति दौर की स्थिति किसी अन्य प्रस्तावक से आगे बढ़ जाती है, तो नोड अगले दौर में आगे बढ़ता है जहां उसका सत्यापनकर्ता प्रस्तावक होता है और मेमपूल से बनता है। यह पुनर्प्राप्ति पथ खाली-ब्लॉक स्पैम को पुन: सक्षम किए बिना लेनदेन-ट्रिगर जीवंतता बनाए रखता है।

## मल्टी-वैलिडेटर नेटवर्क

उत्पन्न नेटवर्क के लिए:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
प्रत्येक उत्पन्न सत्यापनकर्ता घर को प्राप्त होता है:

- इसका अपना `validator.key.json`
- इसकी अपनी विभाजित कॉन्फ़िगरेशन फ़ाइलें: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, और `log_config.json`
- एक साझा `genesis.json`
- `network_config.json` अन्य सत्यापनकर्ताओं के लिए सहकर्मी प्रविष्टियाँ

`vexod network up` और `make network-e2e` सभी सत्यापनकर्ताओं के शुरू होने, स्मोक लेनदेन सबमिट करने और ऊंचाई वृद्धि का निरीक्षण करने की प्रतीक्षा करते समय एक प्रक्रिया-स्तरीय टाइमआउट का उपयोग करते हैं। डिफ़ॉल्ट कमांड टाइमआउट जानबूझकर आम सहमति अंतराल से अधिक लंबा है क्योंकि इसमें प्रक्रिया स्टार्टअप, लेवलडीबी ओपन, पी2पी हस्ताक्षरित हैंडशेक, टीएलएस/ऑथ चेक, लेनदेन प्रवेश और अंतिमता शामिल है। यदि आप आम सहमति टाइमआउट को आक्रामक तरीके से कम करते हैं, तो हार्नेस को बहुत जल्दी खत्म करने के बजाय स्टार्टअप त्रुटियों का निदान करने के लिए नेटवर्क-अप टाइमआउट को पर्याप्त बड़ा रखें।

कंटेनरीकृत या मल्टी-होस्ट नेटवर्क के लिए, JSON फ़ाइल में टोपोलॉजी मान डालें:
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` और `rpc_host_template` प्रत्येक नोड की `network_config.json` सहकर्मी सूची में लिखे गए डायल लक्ष्य हैं। डॉकर में, ये `validator-%d` जैसे सेवा नाम हो सकते हैं।
- `p2p_advertise_host_template` और `rpc_advertise_host_template` सार्वजनिक पते `genesis.json` में सत्यापनकर्ता मेटाडेटा में लिखे गए हैं। सार्वजनिक नेटवर्क के लिए यहां DNS नाम या सार्वजनिक आईपी का उपयोग करें।
- `p2p_listen_host` और `rpc_listen_host` स्थानीय बाइंड होस्ट हैं। कंटेनर या सर्वर के लिए `0.0.0.0` का उपयोग करें जिन्हें सभी इंटरफेस पर सुनना चाहिए।
- विज्ञापित सार्वजनिक पते के रूप में केवल डॉकर सेवा नामों का पुन: उपयोग न करें जब तक कि नेटवर्क जानबूझकर निजी न हो।

फिर उस फ़ाइल से नोड होम जेनरेट करें:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## समस्या निवारण

| लक्षण | सबसे संभावित कारण | क्या जांचें |
|---|---|---|
| `latest_height` नहीं बढ़ता | खाली-ब्लॉक अक्षम और कोई टीएक्स नहीं, ऑनलाइन पर्याप्त सत्यापनकर्ता नहीं, या हस्ताक्षरकर्ता अनुपलब्ध | `consensus_config.json`, सत्यापनकर्ता लॉग, `/v1/diagnostics` |
| `peer_count` `0` है | सहकर्मी पते पहुंच योग्य नहीं हैं या गलत होस्टनाम के लिए `network_config.json` उत्पन्न किया गया था | `p2p.peers`, कंटेनर होस्ट पोर्ट, डीएनएस, फ़ायरवॉल |
| `p2p auth replay store` त्रुटि | सार्वजनिक/प्रमाणीकृत पी2पी को टिकाऊ रीप्ले स्टोरेज की आवश्यकता होती है | `p2p.auth_replay_path` और होम के नीचे अनुमति लिखें |
| `eth_chainId` रीमिक्स में विफल | गलत यूआरएल, गलत होस्ट पोर्ट, या ब्राउज़र सीओआरएस/प्रीफ्लाइट कस्टम कॉन्फ़िगरेशन द्वारा अवरुद्ध | Web3 एंडपॉइंट URL का उपयोग करें, फिर उसी एंडपॉइंट को सीधे कर्ल करें |
| `config audit --strict` विफल | सुरक्षा द्वार को एक असुरक्षित कॉन्फ़िगरेशन गुण मिला | असफल जांच को पढ़ें, फिर स्प्लिट कॉन्फ़िगरेशन फ़ाइल को संपादित करें जिसका नाम | है
| `no block_committed logs` | लॉगिंग अक्षम है या कोई ब्लॉक नहीं बनाया जा रहा है | `log_config.json`, `create_empty_blocks`, मेमपूल सामग्री |
| `managed EVM key rejected` | हॉट प्राइवेट कुंजियाँ सार्वजनिक RPC श्रोता पर कॉन्फ़िगर की जाती हैं | `evm_account_private_keys` हटाएं या RPC को निजी रखें |

## न्यूनतम ऑपरेटर चेकलिस्ट

किसी अन्य मशीन या ऑपरेटर को नोड होम सौंपने से पहले:

- `vexod validate --home <home>` गुजरता है।
- `vexod config audit --home <home> --strict` ठीक उसी घर के लिए गुजरता है।
- `config.json`, विभाजित कॉन्फ़िगरेशन फ़ाइलें, `genesis.json`, और सार्वजनिक सत्यापनकर्ता मेटाडेटा की समीक्षा की जाती है।
- `validator.key.json`, `node.key.json`, और `validator.vrf.key.json` को दूरस्थ हस्ताक्षरकर्ता/KMS कुंजी दस्तावेज़ों द्वारा एन्क्रिप्ट या प्रतिस्थापित किया जाता है।
- `network_config.json:p2p.peers` में ऐसे पते शामिल हैं जो लक्ष्य मशीन से डायल किए जा सकते हैं, केवल डॉकर नाम नहीं जब तक कि नोड वास्तव में उस डॉकर नेटवर्क के अंदर नहीं चलता।
- `network_config.json` सार्वजनिक RPC/P2P श्रोताओं के पास `require_network_safety` सक्षम होने पर TLS सामग्री होती है।
- `module_config.json:execution.EVMChainID` को Web3 वॉलेट या रीमिक्स कनेक्ट से पहले सेट किया गया है।
- `mempool_config.json` के पास एक WAL पथ है यदि नोड को पुनरारंभ के बाद लंबित txs को पुनर्प्राप्त करना चाहिए।
- `log_config.json` नेटवर्क चालू होने के दौरान ब्लॉक कमिट और पीयर लॉग को सक्षम बनाता है।

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: Validator Node — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Archive Node — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Split Configuration Files — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Key Types — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Config-Based Peers — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Consensus Timing — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Multi-Validator Network — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `network_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod start` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--timeout-propose` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--create-empty-blocks` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--p2p-auth-token` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--rpc-admin-token` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-account-key-env` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--evm-account-key` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `validator_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `VEXO_KEY_PASSPHRASE` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--passphrase` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--encrypt-keys` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `validator.key.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `node.key.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `validator.vrf.key.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `require_network_safety=true` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--key-type bls` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `blst-bls12381-minpk-v1` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `genesis.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `bls_pop` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `module_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `consensus_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `mempool_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `log_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `data/` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `network_config.json:p2p.node_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `shutdown_timeout` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `web3_max_subscriptions_per_connection` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `web3_idle_timeout` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `auth_replay_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `require_auth_replay_store` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `dial_timeout` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `data/p2p_auth_replay.jsonl` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `--key-type ed25519` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vrf_key_paths` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vrf_public_key` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `<home>/<name>_config.json` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.evm_account_key_envs` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.evm_account_private_keys` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `eth_accounts` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `eth_sign` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `eth_signTransaction` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `eth_sendTransaction` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evm_account_key_envs` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod config paths --home <home>` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `"require_network_safety": true` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `execution_commit` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `require_network_safety` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `host:port` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc.address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.listen_address` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.peers` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.seeds` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.node_id` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.node_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_cert_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_key_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_ca_path` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.tls_server_name` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p.dial_timeout` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `timeout_propose` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `timeout_prevote` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `timeout_precommit` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `timeout_commit` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `create_empty_blocks: false` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `execution_commit: "finalized"` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `execution_commit: "qc"` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `round_timeout` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `create_empty_blocks` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexod network up` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `make network-e2e` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p_host_template` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc_host_template` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `validator-%d` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p_advertise_host_template` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc_advertise_host_template` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p_listen_host` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc_listen_host` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
