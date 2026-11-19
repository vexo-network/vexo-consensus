# स्टोरेज स्कीमा

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source का सीधा हिन्दी अनुवाद है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।


## पहले क्या पढ़ें

यह दस्तावेज़ Storage Schema की normative specification बताता है। पहली बार पढ़ रहे हों तो इस क्रम में पढ़ें।

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

यह क्रम पढ़ने के सही तरीके से मेल खाता है: पहले scope और state, फिर message, safety और liveness नियम, और अंत में evidence।

## सारांश

यह दस्तावेज़ durable storage namespace, key schema और recovery marker को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/hi/specs/storage-schema.md`

## यह दस्तावेज़ क्यों पढ़ें

- durable storage namespace, key schema और recovery marker
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
- दस्तावेज़ बदलने के बाद `make docs-check` चलाकर स्थानीय दस्तावेज़ वृक्ष और अनुवाद जाँचें।

## ध्यान रखने योग्य बातें

- यह स्थानीयकृत दस्तावेज़ समझने में मदद करता है; audit, release और security decisions अंग्रेज़ी source से तय होते हैं।
- implementation बदलने पर अंग्रेज़ी source और सभी स्थानीयकृत दस्तावेज़ को उसी change में update करें।

## ज्यों-का-त्यों रखने वाले interfaces

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## अंग्रेज़ी source की संरचना

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../../en/specs/storage-schema.md)

## Pebble बैकएंड और बाइनरी रिकॉर्ड

डिफ़ॉल्ट बैकएंड Pebble (`github.com/cockroachdb/pebble/v2`) है, जिसे 256 MiB block cache और 64 MiB memtable के साथ खोला जाता है। रिकॉर्ड `types` पैकेज की canonical बाइनरी एन्कोडिंग में संग्रहीत होते हैं: न्यूनतम uvarint, लंबाई-प्रीफ़िक्स वाली बाइट स्ट्रिंग और सूचियाँ, अपने 32 बाइट के रूप में हैश, `0` या `1` presence byte के बाद वैकल्पिक मान, और घोषित करने वाले struct के क्रम में फ़ील्ड। डिकोडर बची हुई बाइट सहित बाकी सब कुछ अस्वीकार करता है, इसलिए क्षतिग्रस्त रिकॉर्ड दूसरे मान के रूप में पढ़ा नहीं जाता बल्कि रिपोर्ट होता है, और हर रिकॉर्ड की एक ही एन्कोडिंग है। पुराने बिल्ड (LevelDB, JSON रिकॉर्ड) द्वारा लिखी गई डेटा डायरेक्टरी पढ़ी नहीं जा सकती और बदली नहीं जाती: Pebble `format major version 1` त्रुटि के साथ उसे खोलने से मना करता है। ऐसे नोड को snapshot से पुनर्स्थापित करें या खाली डेटा डायरेक्टरी में sync करें।

संबंधित पहचानकर्ता:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## State history (undo log) और tree cache

state history प्रति key संस्करणों का undo log है। जब कोई key किसी ऊँचाई पर बदलती है (ब्लॉक के लेखन उसकी ऊँचाई पर, पहली committed state के बाद का सीधा लेखन (`Set`, `Delete`, `SetBatch`) अगली committed ऊँचाई पर), store उस key का पहले वाला मान, अनुपस्थिति सहित, उस ऊँचाई के संस्करण के रूप में हर key और ऊँचाई के लिए एक बार दर्ज करता है; उस ऊँचाई को commit करने वाले ब्लॉक से पहले के सीधे लेखन उनमें से पहले द्वारा दर्ज संस्करण रखते हैं (`hdirect` वह ऊँचाई बताता है)। किसी committed ऊँचाई पर key का मान वह है जो उस ऊँचाई के ऊपर उसका सबसे निचला संस्करण दर्ज करता है, और ऐसा संस्करण न हो तो उसका वर्तमान मान। बदलाव दर्ज करना केवल key का वर्तमान मान पढ़ता है, history नहीं, और ऐतिहासिक पढ़ना key के संस्करणों में एक seek है। `PruneBelow` उस ऊँचाई पर या उससे नीचे के सभी संस्करण हटाता है जहाँ से वह रखता है। tree node भी इसी तरह संस्करण रखते हैं। स्मृति की दो संरचनाएँ ब्लॉक का tree काम घटाती हैं; कोई भी स्थायी नहीं है और कोई परिणाम नहीं बदलती: वर्तमान tree के अधिकतम 2^18 encoded node रखने वाला tree node cache (store का हर batch इसे अद्यतन करता है, और ऐसे batch के commit होते समय विषम रहने वाली generation के कारण reader cache का उपयोग केवल अपने snapshot की स्थिति पर करता है), और हर namespace के लिए पिछली root गणना का root और node बदलाव, जो उस generation और namespace के लेखनों के SHA-256 digest से जुड़े हैं, ताकि app hash, state roots और commit एक जैसे लेखन एक ही बार गणना करें।

## Mempool WAL compaction और genesis batch

commit mempool WAL को तभी compact करता है जब उसमें कम से कम 4 MiB हो और पिछले compaction या खोलने के बाद के आकार का दोगुना हो। इसलिए जोड़ा गया हर byte औसतन लगभग एक बार ही दोबारा लिखा जाता है, और restart जिस log को replay करता है वह live सामग्री के दोगुने या 4 MiB के भीतर रहता है। स्पष्ट compaction (`Node.Compact`, या start जो application द्वारा अस्वीकृत replay tx हटाता है) हमेशा उसे दोबारा लिखता है। `InitChain` सभी module की genesis state एक batch में लिखता है, इसलिए कई खातों वाला genesis एक durable write में होता है और विफल genesis store में कुछ नहीं छोड़ता।

## Commit batch में finality proof और mempool से विलंबित हटाना

Node का commit पथ block के QC द्वारा पूर्ण की गई finality proofs को उसी block/state/app-write batch में लिखता है (`CommitBlockStateWithWrites` उन्हें अंतिम argument के रूप में लेता है)। इसलिए एक commit में हर proof के लिए एक और durable write के बजाय केवल एक write लगता है। जिस proof को store अस्वीकार करेगा वह batch से बाहर रहता है और commit के बाद का चरण उसे सहेजते हुए विफलता की रिपोर्ट करता है। Commit अपने transactions को pool से हटाता है और हटाने का record sync की प्रतीक्षा किए बिना queue में डालता है। अगला commit अपना block execute करने से पहले उस record के durable होने की प्रतीक्षा करता है, क्योंकि restart केवल अंतिम committed block का हटाना दोहराता है। आमतौर पर यह record बाद के admission के sync में साथ लिख जाता है।

## State tree की विलंबित hashing

State tree की गणना hashing को टालती है: write केवल उन हिस्सों को चिह्नित करता है जिन्हें उसने बदला, और root, proof या changes माँगे जाने पर हर बदला हुआ node नीचे से ऊपर एक बार hash होता है। इसलिए root के पास के nodes, जिन्हें block का हर write फिर से लिखता है, हर write पर नहीं बल्कि हर block में एक बार hash होते हैं। Hashes, nodes और roots वही रहते हैं जो हर write पर hash करने से मिलते।

## Mempool WAL का आवधिक sync

Mempool WAL हर बदलाव को log में लिखता है और हर 100 ms पर sync करता है (`walSyncInterval`)। Admission केवल write की प्रतीक्षा करता है, sync की नहीं। Process crash में हर record बचा रहता है; machine crash में अधिकतम अंतिम अंतराल के admissions खोते हैं, जिन्हें client फिर से भेजते हैं। Committed block का हटाने वाला record अगले commit से पहले sync होता है (`WaitDurable`)।

## History record के लिए read का पुनः उपयोग

Commit बदले गए key का पिछला मान version के रूप में दर्ज करता है, अक्सर उसे फिर से पढ़े बिना: store पिछले batch commit के बाद `Get` द्वारा पढ़े गए मान, अधिकतम 2^16, याद रखता है, और block का execution जिन keys को बदलता है उनमें से अधिकांश को पहले पढ़ता है। मान का उपयोग तभी होता है जब पढ़ने के बाद कोई batch commit न हुआ हो, इसलिए वह वर्तमान मान है; अनुपस्थित key याद नहीं रखी जाती और पढ़ी जाती है।

## State tree nodes का संस्करण-आधारित भंडारण

Tree nodes संस्करण के अनुसार रखे जाते हैं और बाद की कोई height उन्हें दोबारा नहीं लिखती। Batch हर बदले node को `tree: | namespace | height | node key` में लिखता है, और internal node अपने दोनों halves के संस्करण भी रखता है, जिससे हर node अपने parent से मिलता है। Root record (`troot:`) उस हर height को चिह्नित करता है जिस पर tree बदला; किसी भी height का tree उससे कम या बराबर अंतिम root record से पढ़ा जाता है। बदला गया node नीचे की heights के लिए रहता है और बदलने वाली height की orphan list (`torph:`, प्रति batch एक) में जाता है, जिसे `PruneBelow` हटाता है। संस्करण केवल बढ़ते हैं, इसलिए block के writes मौजूदा keys में नहीं बिखरते और Pebble के flush और compaction छोटे रहते हैं। Keys का version index (`hidx:`) भी प्रति batch एक entry है।

संबंधित पहचानकर्ता:

- `troot: | namespace | height`
- `torph: | height | first entry`
- `hidx: | height | first group`

## Tree node cache में decoded nodes

Tree node cache वर्तमान trees के अधिकतम 2^18 nodes को decoded रूप में, उनके halves के संस्करणों के साथ रखता है, इसलिए read न decode करता है न memory allocate करता है।

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: Scope — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Backend — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Records — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Indexes — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: EVM Records — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Recovery Rules — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Snapshot Validation — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Schema Migration — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `store.Store` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `eth_getBalance` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `eth_getProof` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `bank/{0x_address}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `auth/nonce/{0x_address}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evm/code/{0x_address}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evm/storage/{0x_address}/{slot}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `seen_ttl` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `code/{address}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `storage/{address}/{slot}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `receipts/{tx_hash}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `logs/by_height/{height}/{tx_hash}/{log_index}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `logs/{address}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evm_receipts` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `receipt_index/{tx_hash}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `blob_sidecars/by_tx/{tx_hash}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `blob_sidecars/by_blob/{blob_hash}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `tree:` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `receipt_height/{height}/{tx_hash}` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `receipt_pruned_below` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
