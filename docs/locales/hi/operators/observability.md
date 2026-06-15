> Locale: hi · हिन्दी

# अवलोकन संबंधी मार्गदर्शिका

यह मार्गदर्शिका बताती है कि आरपीसी, मेट्रिक्स, लॉग और रिलीज़ साक्ष्य से कैसे बताया जाए कि वेक्सो नोड स्वस्थ है या नहीं।

यह उन ऑपरेटरों के लिए लिखा गया है जिन्हें व्यावहारिक संकेतों की आवश्यकता है: क्या देखना है, प्रत्येक संख्या का क्या मतलब है, और कब किसी मान को खतरनाक माना जाना चाहिए।

## एक नज़र में

यदि कोई नोड गलत दिखता है, तो इन्हें क्रम से जांचें:

1. `running` और `latest_height` `/v1/status` में
2. `latest_finalized_height` और सहकर्मी गिनती
3. `round_timeout`, प्रस्ताव/वोट विलंबता, मेमपूल आकार, और प्रतिबद्ध विलंबता मेट्रिक्स
4. हस्ताक्षरकर्ता विफलताएं, स्नैपशॉट स्वास्थ्य, और रीप्ले स्वास्थ्य
5. सहकर्मी प्रतिबंध और सहकर्मी-डायल विफलताएँ

वह क्रम मायने रखता है क्योंकि यह "प्रक्रिया जीवित है" को "श्रृंखला वास्तव में सुरक्षित प्रगति कर रही है" से अलग करती है।

## कोर समापन बिंदु

| समापन बिंदु | उपयोग करें |
|---|---|
| `/v1/status` | तेज़ प्रक्रिया, ऊंचाई, ऐप हैश, अंतिमता, और सहकर्मी सारांश |
| `/v1/metrics` | डैशबोर्ड और स्वचालन के लिए JSON मेट्रिक्स |
| `/metrics/text` | प्रोमेथियस-संगत टेक्स्ट मेट्रिक्स |
| `/v1/diagnostics` | संयुक्त तैयारी, क्षमताएं, स्थिति, समकक्ष, भंडारण और मेट्रिक्स जांच |
| `/v1/finality/latest` | लाइट-क्लाइंट और सुरक्षा जांच के लिए नवीनतम अंतिम प्रमाण |
| `/v1/state/latest` | नवीनतम राज्य रूट और सत्यापनकर्ता-सेट बाइंडिंग |
| `/v1/recovery/report` | क्रैश/पुनरारंभ संगतता निदान |
| `/v1/snapshot` | स्नैपशॉट स्वास्थ्य और निर्यात मेटाडेटा |

व्यवस्थापक समापन बिंदु जैसे कि प्रून, रीप्ले और सर्वसम्मति नियंत्रण आमतौर पर केवल लूपबैक, एक ऑपरेटर नेटवर्क, एमटीएलएस या एक प्रमाणित गेटवे के माध्यम से पहुंच योग्य होना चाहिए। स्कोप्ड एडमिन टोकन वैकल्पिक रहते हैं और कॉन्फ़िगर होने पर लागू होते हैं।

## पढ़ना `/v1/status`

महत्वपूर्ण क्षेत्र:

| फ़ील्ड | मतलब | ऑपरेटर नोट |
|---|---|---|
| `running` | नोड प्रक्रिया शुरू हो गई है और रनटाइम स्थिति का स्वामी है | `true` अपने आप में सर्वसम्मत जीवंतता सिद्ध नहीं करता |
| `latest_height` | नवीनतम स्थानीय रूप से प्रतिबद्ध ऐप ऊंचाई | लाइव सत्यापनकर्ता नेटवर्क पर समय के साथ वृद्धि होनी चाहिए |
| `latest_finalized_height` | नवीनतम हॉटस्टफ तीन-श्रृंखला अंतिम ऊंचाई | निष्पादित/प्रतिबद्ध ऊंचाई से अनिश्चित काल तक पीछे नहीं रहना चाहिए |
| `latest_app_hash` | ऐप कमिट हैश | समान ऊंचाई पर साथियों से मेल खाना चाहिए |
| `peer_count` | बैकवर्ड-संगत कनेक्टेड/स्कोर किया गया सहकर्मी सारांश | नीचे अधिक विशिष्ट सहकर्मी फ़ील्ड को प्राथमिकता दें |
| `active_peer_count` | सक्रिय परिवहन सत्र, जब परिवहन उन्हें रिपोर्ट कर सकता है | लाइव पी2पी कनेक्टिविटी के लिए सर्वोत्तम त्वरित सिग्नल |
| `configured_peer_count` | कॉन्फ़िगर या सीखे गए सहकर्मी पते | पहुंच योग्यता की गारंटी नहीं है |
| `scored_peer_count` | साथियों को स्कोर तालिका में जाना जाता है | प्रतिबंध/दर-सीमा इतिहास के लिए उपयोगी, लाइव सत्रों के प्रमाण के लिए नहीं |
| `banned_peers` | साथियों को वर्तमान में स्कोर नीति द्वारा प्रतिबंधित कर दिया गया है | स्पाइक्स हमले, खराब सहकर्मी कॉन्फ़िगरेशन, या बहुत सख्त सीमाओं का संकेत देते हैं

4-सत्यापनकर्ता एकल-होस्ट नेटवर्क के लिए स्वस्थ उदाहरण: `running=true`, `latest_height` बढ़ रहा है, `latest_finalized_height` मौजूद है, `active_peer_count` `3` के पास है, और `banned_peers=0` है।

## प्रोमेथियस मेट्रिक्स

पाठ समापन बिंदु गेज को उजागर करता है जैसे:

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`

`vexo_peer_count` पुराने डैशबोर्ड के लिए रखा गया है। नए डैशबोर्ड को `vexo_active_peer_count`, `vexo_configured_peer_count` और `vexo_scored_peer_count` को अलग-अलग चार्ट करना चाहिए।

## सुझाए गए चेतावनी नियम

वास्तविक सत्यापनकर्ता गणना, ब्लॉक अंतराल, विलंबता और हार्डवेयर के लिए संख्याओं को ट्यून करें। ये प्रारंभिक बिंदु हैं, सार्वभौमिक स्थिरांक नहीं।

| चेतावनी | आरंभिक स्थिति | क्यों |
|---|---|---|
| नोड नीचे | 1 मिनट के लिए `vexo_node_running == 0` | प्रक्रिया/रनटाइम रोक दिया गया |
| ऊंचाई रुकी हुई | `latest_height` 2-3 अपेक्षित ब्लॉक अंतरालों के लिए अपरिवर्तित | आम सहमति या क्रियान्वयन रुका हुआ |
| अंतिम रूप से रुका हुआ | `latest_finalized_height` अपरिवर्तित रहता है जबकि ब्लॉक निष्पादित होते रहते हैं | अंतिम पथ या कोरम मुद्दा |
| कोई सक्रिय साथी नहीं | `vexo_active_peer_count == 0` एक गैर-पृथक नोड पर 1 मिनट के लिए | पी2पी आउटेज, प्रमाणीकरण बेमेल, या पता समस्या |
| साथियों की संख्या बहुत कम है | कोरम कनेक्टिविटी लक्ष्य से नीचे सक्रिय सहकर्मी | विभाजन या बूटस्ट्रैप समस्या |
| राउंड टाइमआउट स्पाइक | टाइमआउट काउंटर सामान्य बेसलाइन की तुलना में तेजी से बढ़ता है | विलंबता, प्रस्तावक विफलता, या नेटवर्क विभाजन |
| विलंबता उच्च प्रतिबद्ध | p95/p99 सर्वसम्मत टाइमआउट बजट के करीब पहुंचता है | स्टोर/रनटाइम अधिभार |
| मेमपूल दबाव | मेमपूल का आकार कई मिनट तक बढ़ता है | शुल्क नीति, स्पैम, या ब्लॉक क्षमता समस्या |
| स्नैपशॉट अस्वस्थ | `vexo_snapshot_healthy == 0` | सिंक/पुनर्प्राप्ति जोखिम बताएं |
| दोबारा खेलना अस्वस्थ | `vexo_replay_healthy == 0` | नियतिवाद या राज्य स्थिरता जोखिम |
| हस्ताक्षरकर्ता विफलताएं | `vexo_validator_signing_failures > 0` | केएमएस/दूरस्थ हस्ताक्षरकर्ता/नीति विफलता |
| सुलह विफलताएं | `vexo_post_commit_reconciliation_failures > 0` | टिकाऊ साक्ष्य या प्रतिबद्ध मरम्मत की आवश्यकता |
| प्रतिबंधित सहकर्मी स्पाइक | प्रतिबंधित सहकर्मी अचानक बढ़ जाते हैं | हमला, ग़लत कॉन्फ़िगर किए गए सहकर्मी, या स्कोरिंग सीमा समस्या |

## सुझाई गई आरंभिक सीमाएँ

इन्हें शुरुआती अलर्ट मानों के रूप में उपयोग करें, फिर वास्तविक दीर्घकालिक आधार रेखा के बाद ट्यून करें:

| सिग्नल | चेतावनी | गंभीर | पहली कार्रवाई |
|---|---:|---:|---|
| ऊंचाई दर | 2 विंडोज़ के लिए अपेक्षा से 50% कम | 2-3 ब्लॉक अंतराल के लिए शून्य वृद्धि | सभी सत्यापनकर्ताओं की तुलना करें, प्रस्तावक/हस्ताक्षरकर्ता/सहकर्मी लॉग की जांच करें |
| अंतिम ऊंचाई अंतराल | 5 मिनट तक बढ़ता है | निष्पादित होने पर ऊंचाई 10 मिनट तक बढ़ती रहती है | QC/अंतिम प्रमाण लॉग और सत्यापनकर्ता-सेट हैश का निरीक्षण करें |
| सक्रिय सहकर्मी | कोरम कनेक्टिविटी लक्ष्य से नीचे | शून्य सक्रिय सहकर्मी | विज्ञापित पता, टीएलएस/ऑथ, जेनेसिस/चेन आईडी बेमेल की जांच करें |
| राउंड टाइमआउट | 3x सामान्य आधार रेखा | निरंतर टाइमआउट लूप | टाइमआउट बजट बढ़ाएं या विलंबता/विभाजन की जांच करें |
| प्रस्ताव विलंबता p95 | `timeout_propose` के 50% से ऊपर | `timeout_propose` के 80% से ऊपर | प्रोफ़ाइल प्रस्तावक, मेमपूल, डीए प्रतिबद्धता, डिस्क |
| वोट विलंबता p95 | प्रीवोट/प्रीकमिट बजट का 50% से ऊपर | बजट का 80% से ऊपर | सीपीयू, हस्ताक्षरकर्ता, परिवहन, गपशप बैकप्रेशर का निरीक्षण करें |
| विलंबता p95 प्रतिबद्ध करें | ब्लॉक अंतराल के 50% से ऊपर | ब्लॉक अंतराल के 80% से ऊपर | लेवलडीबी, राज्य की जड़ें, ईवीएम निष्पादन, स्नैपशॉट का निरीक्षण करें |
| मेमपूल आकार | 5 मिनिट तक बढ़ रहा है | `max_txs` या निरंतर प्रतिस्थापन मंथन के निकट | आधार शुल्क, न्यूनतम शुल्क, टीएक्स वैधता, स्पैम का निरीक्षण करें |
| हस्ताक्षरकर्ता विफलताएं | कोई भी गैर-शून्य मान | एक ऊंचाई वाली विंडो में बार-बार विफलता | यदि डबल-साइन गार्ड या कुंजी बेमेल दिखाई दे तो सत्यापनकर्ता को रोकें |
| स्नैपशॉट स्वास्थ्य | एक असफल जांच | बार-बार विफल निर्यात/सत्यापन/पुनर्स्थापना | राज्य-सिंक सेवा रोकें और पुनर्प्राप्ति रिपोर्ट चलाएँ |
| फिर से खेलना स्वास्थ्य | एक सख्त रीप्ले विफलता | नवीनतम सुरक्षित ऊंचाई पर बेमेल को फिर से चलाएं | डेटा डीआईआर को सुरक्षित रखें और असुरक्षित अपग्रेड/रिलीज़ को रोकें |
| प्रतिबंधित साथियों | अचानक उछाल | कॉन्फिग रोलआउट के बाद कई साथियों पर प्रतिबंध लगा दिया गया | स्कोर कैप, टीएलएस सीए, सहकर्मी पहचान, वैकल्पिक प्रमाणीकरण प्रमाण, और घड़ी तिरछा जांचें |

सबसे महत्वपूर्ण नियम: **समय के साथ परिवर्तन** पर सतर्क रहें। एक एकल संख्या भ्रामक हो सकती है; ऊंचाई दर, अंतिम अंतराल, सहकर्मी मंथन, मेमपूल वृद्धि, और हस्ताक्षरकर्ता विफलताएं मिलकर वास्तविक कहानी बताती हैं।

## घटना ट्राइएज मैट्रिक्स

| स्थिति | संभावित परत | क्या सुरक्षित रखें | सुरक्षित अगला कदम |
|---|---|---|---|
| ऊंचाई रुकी, साथी स्वस्थ | सर्वसम्मति/हस्ताक्षरकर्ता/रनटाइम | सर्वसम्मति लॉग, हस्ताक्षरकर्ता लॉग, मेमपूल नमूना | प्रस्तावक कुंजी और राउंड टाइमआउट लॉग सत्यापित करें |
| तैनाती के बाद साथियों को हटा दिया गया | नेटवर्किंग/कॉन्फ़िगरेशन | नेटवर्क कॉन्फ़िगरेशन, टीएलएस प्रमाणपत्र, एड्रबुक, पीयर लॉग | विज्ञापित पता/टीएलएस/प्रमाणीकरण परिवर्तन वापस लें |
| ऐप हैश समान ऊंचाई पर भिन्न होते हैं | निष्पादन/भंडारण | डेटा डायर, ब्लॉक रिकॉर्ड, ऐप लॉग, रीप्ले आउटपुट | प्रभावित नोड्स को रोकें और सख्त रीप्ले चलाएं |
| अंतिम प्रमाण अस्वीकृत | अंतिमता/सत्यापनकर्ता सेट | प्रूफ़ JSON, वैलिडेटर प्रूफ़ ऊंचाई पर सेट | सत्यापनकर्ता-सेट हैश और साइन बाइट्स डोमेन सत्यापित करें |
| स्नैपशॉट पुनर्स्थापना विफल | राज्य सिंक/भंडारण | स्नैपशॉट फ़ाइल, चेकसम, राज्य जड़ें, लॉग पुनर्स्थापित करें | लाइव डेटा के विरुद्ध पुनः प्रयास न करें; साफ़ डीआईआर में पुनर्स्थापित करें |
| दूरस्थ हस्ताक्षरकर्ता अनुरोधों को अस्वीकार कर देता है | कुंजी हिरासत | हस्ताक्षरकर्ता ऑडिट लॉग, गार्ड फ़ाइल, नॉन फ़ाइल, नोड लॉग | नीति अस्वीकृति को परिवहन आउटेज से अलग करें |
| प्रतिबंधित सहकर्मी स्पाइक | पी2पी/सुरक्षा | सहकर्मी स्कोर स्नैपशॉट और प्रतिबंध के कारण | विकृत गपशप या साझा की गई गलत कॉन्फ़िगरेशन का निरीक्षण करें |

घटनाओं के दौरान, "सफाई" के बजाय डेटा को संरक्षित करने को प्राथमिकता दें। वाल, एड्रबुक, साइनर गार्ड या लेवलडीबी निर्देशिकाओं को हटाने से बग को ऑपरेटर त्रुटि से अलग करने के लिए आवश्यक सबूत नष्ट हो सकते हैं।

## रखने के लिए ईवेंट लॉग करें

जहां प्रासंगिक हो, संरचित लॉग को नोड आईडी, सत्यापनकर्ता आईडी, चेन आईडी, ऊंचाई, गोल, ब्लॉक हैश और पीयर आईडी के साथ बनाए रखा जाना चाहिए।

महत्वपूर्ण घटनाएँ:

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

रिलीज़ उम्मीदवारों के लिए, मेट्रिक्स नमूने, पीप्रोफ़ नमूने, कॉन्फ़िगरेशन फ़ाइलें, उत्पत्ति, बाइनरी चेकसम और साक्ष्य मैनिफ़ेस्ट के साथ संग्रहित लॉग।

## पहली प्रतिक्रिया प्लेबुक

जब कोई ऑपरेटर कोई समस्या देखता है:

1. कम से कम दो सत्यापनकर्ताओं पर `/v1/status` जांचें।
2. `latest_height`, `latest_finalized_height`, `latest_app_hash` और साथियों की संख्या की तुलना करें।
3. अनुपलब्ध क्षमताओं या अस्वास्थ्यकर स्टोरेज/रीप्ले/स्नैपशॉट जांच के लिए `/v1/diagnostics` की जांच करें।
4. प्रमाणीकरण, टीएलएस, जेनेसिस, चेन आईडी, या बैकऑफ़ त्रुटियों के लिए सहकर्मी ईवेंट लॉग का निरीक्षण करें।
5. यदि टीएक्स शामिल नहीं हैं तो मेमपूल और बेस-फी मेट्रिक्स का निरीक्षण करें।
6. यदि सत्यापनकर्ता के हस्ताक्षर विफल हो जाते हैं तो हस्ताक्षरकर्ता और दूरस्थ हस्ताक्षरकर्ता लॉग को सत्यापित करें।
7. डेटा को हटाने या संशोधित करने से पहले पुनर्प्राप्ति रिपोर्ट निर्यात करें।
8. यदि अंतिम संघर्ष का संदेह है, तो स्वचालन रोकें, लॉग/सबूत को संरक्षित करें, और अंतिम संघर्ष का पता लगाएं।

## डैशबोर्ड लेआउट

एक उपयोगी डैशबोर्ड में आमतौर पर पाँच पंक्तियाँ होती हैं:

1. **लाइवनेस**: नोड रनिंग, नवीनतम ऊंचाई, अंतिम ऊंचाई, ऊंचाई दर।
2. **सर्वसम्मति विलंबता**: राउंड टाइमआउट, प्रस्ताव/वोट/प्रतिबद्धता p95 और p99।
3. **नेटवर्क**: सक्रिय/कॉन्फ़िगर/स्कोर किए गए पीयर, प्रतिबंधित पीयर, पीयर विंडो संदेश।
4. **निष्पादन**: मेमपूल आकार, गैस/बेस शुल्क, टीएक्स गिनती, प्रतिबद्ध विलंबता।
5. **पुनर्प्राप्ति और सुरक्षा**: स्नैपशॉट स्वास्थ्य, रीप्ले स्वास्थ्य, हस्ताक्षरकर्ता विफलताएँ, सामंजस्य विफलताएँ।

डैशबोर्ड को उबाऊ रखें. लक्ष्य हर आंतरिक काउंटर को दिखाना नहीं है; इससे पहले कि सत्यापनकर्ता अलग हो जाएं या उपयोगकर्ताओं को रुके हुए लेनदेन का पता चले, खतरनाक स्थिति स्पष्ट हो जाए।

## अवलोकन से साक्ष्य जारी करें

एक रिलीज़ उम्मीदवार के लिए, अवलोकनशीलता केवल लाइव निगरानी नहीं है। यह प्रमाण बन जाता है:

1. प्रत्येक सत्यापनकर्ता से बेसलाइन `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest`, और `/v1/recovery/report` एकत्रित करें।
2. चुनी गई अवधि और दर के लिए लोड चलाएं।
3. कम से कम एक पुनरारंभ, एक सहकर्मी व्यवधान, और एक स्नैपशॉट निर्यात/सत्यापित/पुनर्स्थापित ड्रिल इंजेक्ट करें।
4. प्रत्येक सत्यापनकर्ता से अंतिम मेट्रिक्स एकत्र करें।
5. पहले/बाद के नमूने, लॉग, पीप्रोफ़ नमूने, हस्ताक्षरकर्ता ऑडिट लॉग और साक्ष्य प्रकट को `dist/` में संग्रहीत करें।

एक अच्छा साक्ष्य बंडल एक समीक्षक को उत्तर देता है: क्या ऊंचाई बढ़ी, क्या अंतिम प्रगति हुई, क्या साथियों ने ठीक किया, क्या टीएक्स ने प्रतिबद्ध किया, क्या स्नैपशॉट सत्यापित किया, क्या रीप्ले स्वस्थ रहा, क्या हस्ताक्षरकर्ताओं ने डबल-हस्ताक्षर करने से परहेज किया, और क्या सटीक रिलीज बाइनरी ने परिणाम उत्पन्न किए?

<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट सुनिश्चित करता है कि अनुवाद अंग्रेज़ी canonical दस्तावेज़ के चलाने योग्य इंटरफ़ेस और मुख्य अनुभागों को न खोए। commands, config keys, RPC methods और package names सभी भाषाओं में अपरिवर्तित रहते हैं।

### अनुभाग ट्रैकिंग
- section: Core Endpoints — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Reading `/v1/status` — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Prometheus Metrics — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Suggested Alert Rules — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Suggested Starting Thresholds — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Incident Triage Matrix — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Log Events to Keep — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: First Response Playbook — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Dashboard Layout — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।
- section: Release Evidence From Observability — इस अनुभाग को configuration values, verification evidence, failure conditions और operator actions के साथ पढ़ना चाहिए।

### ज्यों का त्यों रखे गए इंटरफ़ेस
- `/v1/status` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/v1/metrics` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/metrics/text` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/v1/diagnostics` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/v1/finality/latest` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/v1/state/latest` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/v1/recovery/report` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `/v1/snapshot` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `latest_height` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `latest_finalized_height` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `latest_app_hash` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `active_peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `configured_peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `scored_peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `banned_peers` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `banned_peers=0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_node_running` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_latest_height` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_active_peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_configured_peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_scored_peer_count` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_banned_peers` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_height_rate_per_minute` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_round_timeouts` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_proposal_latency_p95_nanos` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_vote_latency_p95_nanos` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_commit_latency_p95_nanos` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_mempool_size` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_snapshot_healthy` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_replay_healthy` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_validator_signing_failures` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_post_commit_reconciliation_failures` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_node_running == 0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_active_peer_count == 0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_snapshot_healthy == 0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_replay_healthy == 0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_validator_signing_failures > 0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `vexo_post_commit_reconciliation_failures > 0` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `timeout_propose` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `max_txs` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `node_running` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `rpc_listening` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `p2p_listening` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `peer_configured` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `peer_connected` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `peer_disconnected` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `peer_dial_failed` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `peer_banned` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `consensus_loop_running` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `block_committed` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `round_timeout` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `validator_signing_failure` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evidence_received` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `evidence_applied` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `snapshot_exported` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `replay_checked` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `upgrade_halt` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `upgrade_applied` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
- `dist/` — यह नाम executable examples और configuration validation में ज्यों का त्यों उपयोग होता है, इसलिए इसका अनुवाद न करें।
