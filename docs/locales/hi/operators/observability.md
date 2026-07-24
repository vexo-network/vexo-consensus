> Locale: hi · हिन्दी

# Observability guide

यह guide RPC, metrics, logs और release evidence से Vexo node की health समझाती है। पहले `running` और `latest_height`, फिर `latest_finalized_height` और active peers, उसके बाद latency तथा `round_timeout`, और अंत में signer, snapshot, replay तथा bans जांचें। Process चलना सुरक्षित consensus progress का प्रमाण नहीं है।

## Endpoints और status

Height, app hash, finality और peers के लिए `/v1/status`; JSON metrics के लिए `/v1/metrics`; Prometheus के लिए `/metrics/text`; readiness के लिए `/v1/diagnostics`; proof और recovery के लिए `/v1/finality/latest`, `/v1/state/latest`, `/v1/recovery/report`, `/v1/snapshot` उपयोग करें। Admin endpoints को loopback, operator network, mTLS या authenticated gateway के पीछे रखें।

`/v1/status` में `running=true` केवल runtime start बताता है। `latest_height` और `latest_finalized_height` आगे बढ़ें, समान height पर `latest_app_hash` peers में समान हो, और वास्तविक sessions के लिए configured या scored peers की जगह `active_peer_count` देखें।

| `vexo_peer_count` | वर्तमान में स्कोर पॉलिसी द्वारा प्रतिबंधित सहकर्मी | स्पाइक्स हमले, खराब सहकर्मी कॉन्फ़िगरेशन या बहुत सख्त सीमाओं का संकेत देते हैं |

## Prometheus metrics

`vexo_node_running`, `vexo_latest_height`, `vexo_active_peer_count`, `vexo_configured_peer_count`, `vexo_quorum_health_ratio`, `vexo_height_rate_per_minute`, `vexo_round_timeouts`, `vexo_adaptive_round_timeout_nanos`, proposal/vote/commit p95, `vexo_mempool_size`, `vexo_snapshot_healthy`, `vexo_replay_healthy`, `vexo_validator_signing_failures` और `vexo_recovery_finality_deferrals` monitor करें।

`vexo_peer_count` पुराने डैशबोर्ड के लिए रखा गया है। नए डैशबोर्ड को `vexo_active_peer_count`, `vexo_configured_peer_count`, और `vexo_scored_peer_count` को अलग से चार्ट करना चाहिए।

## सुझाए गए अलर्ट नियम

वास्तविक सत्यापनकर्ता गणना, ब्लॉक अंतराल, विलंबता और हार्डवेयर के लिए संख्याओं को ट्यून करें। ये शुरुआती बिंदु हैं, सार्वभौमिक स्थिरांक नहीं।

| Alert | शुरुआती condition | Action |
|---|---|---|
| Height stalled | 2 या 3 intervals तक progress नहीं | validators, proposer, signer, peers compare करें |
| Finality stalled | execution बढ़े पर finalized height नहीं | QC, proof, validator-set hash जांचें |
| Active peer नहीं | `vexo_active_peer_count == 0` एक minute | address, identity, auth, chain ID जांचें |
| Quorum low | `vexo_quorum_health_ratio < 0.75` कई windows | partition, latency, peer loss देखें |
| Timeout high | counter या adaptive timeout baseline से ऊपर | network, proposer, CPU, disk, signer देखें |
| Recovery deferred | `vexo_recovery_finality_deferrals` बढ़े | data बदलने से पहले recovery report export करें |

## सुझाई गई शुरुआती सीमाएँ

इन्हें प्रारंभिक अलर्ट मानों के रूप में उपयोग करें, फिर एक वास्तविक लंबे समय तक चलने वाली बेसलाइन के बाद ट्यून करें:

| Signal | Warning | Critical |
|---|---|---|
| Height rate | baseline का 50% से कम | zero growth |
| Active peers | quorum target से कम | zero peers |
| p95 latency | budget का 50% से अधिक | 80% से अधिक |
| Signer | कोई भी error | एक height में repeated errors |
| Snapshot या replay | एक check fail | repeated failure या divergence |

सबसे ज़रूरी नियम: समय के साथ **बदलाव के बारे में अलर्ट **। एक एकल संख्या भ्रामक हो सकती है; ऊंचाई दर, अंतिम अंतराल, सहकर्मी मंथन, मेमपूल विकास, और हस्ताक्षरकर्ता विफलताएं एक साथ वास्तविक कहानी बताती हैं।

## इंसिडेंट ट्राइज मैट्रिक्स

| Situation | संभावित layer | सुरक्षित कदम |
|---|---|---|
| Healthy peers पर height रुकी | consensus, signer, runtime | logs बचाएं, proposer/timeout जांचें |
| Deploy के बाद peers गए | network या config | config बचाएं, address/auth rollback करें |
| App hashes अलग | execution या storage | प्रभावित nodes रोकें, strict replay चलाएं |
| Finality proof rejected | finality या validator set | height, set hash, signature domain जांचें |
| Snapshot restore fail | state sync या storage | clean directory में restore करें |
| Remote signer reject | custody या policy | policy rejection और transport outage अलग करें |

| प्रतिबंधित सहकर्मी स्पाइक | P2P/सुरक्षा | सहकर्मी स्कोर स्नैपशॉट और प्रतिबंध के कारण | विकृत गपशप या साझा गलत कॉन्फ़िगरेशन का निरीक्षण करें |

Incident में WAL, addrbook, signer guard, data directory, configs और logs सुरक्षित रखें। इन्हें हटाने से bug और operator error में अंतर करने वाला evidence नष्ट होता है।

## Logs और first response

Structured events में node ID, validator ID, chain ID, height, round, block hash और peer ID रखें। `peer_connected`, `peer_dial_failed`, `block_committed`, `round_timeout`, `validator_signing_failure`, `snapshot_exported`, `replay_checked`, `upgrade_halt` और `upgrade_applied` सुरक्षित रखें।

कम से कम दो validators पर `/v1/status` compare करें, फिर `/v1/diagnostics`, peer logs, mempool और fee metrics, signer, और अंत में `/v1/recovery/report` देखें। Release candidate logs के साथ metrics, pprof, configs, genesis, binary checksums और evidence manifests archive करें।
<!-- vexo-docs:technical-parity -->
## तकनीकी समानता परिशिष्ट

यह परिशिष्ट उन तकनीकी नामों को सुरक्षित रखता है जो कैनॉनिकल संस्करण के साथ समान रहेंगे:

- `rpc_listening` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `p2p_listening` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `peer_configured` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `peer_connected` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `peer_disconnected` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `peer_dial_failed` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `peer_banned` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `consensus_loop_running` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `block_committed` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `round_timeout` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `validator_signing_failure` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `evidence_received` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `evidence_applied` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `snapshot_exported` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `replay_checked` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `upgrade_halt` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `upgrade_applied` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `dist/` — यह नाम रन उदाहरणों और कॉन्फ़िगरेशन सत्यापन में बिना बदलाव के उपयोग होता है।
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `/v1/snapshot`
- `configured_peer_count`
- `scored_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `latest_app_hash`
- `banned_peers=0`
- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
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
- `vexo_adaptive_round_timeout_enabled`
- `vexo_adaptive_round_timeout_nanos`
- `vexo_quorum_health_ratio`
- `vexo_recovery_finality_gate_enabled`
- `vexo_recovery_finality_deferrals`
- `vexo_node_running == 0`
- `vexo_active_peer_count == 0`
- `vexo_adaptive_round_timeout_enabled == 0`
- `vexo_quorum_health_ratio < 0.75`
- `vexo_recovery_finality_gate_enabled == 0`
- `vexo_snapshot_healthy == 0`
- `vexo_replay_healthy == 0`
- `vexo_validator_signing_failures > 0`
- `vexo_post_commit_reconciliation_failures > 0`
- `timeout_propose`
- `max_txs`
