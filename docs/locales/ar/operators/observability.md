> Locale: ar · العربية

# دليل الرصد

يشرح هذا الدليل تقييم صحة عقدة Vexo من RPC والمقاييس والسجلات وأدلة الإصدار. افحص أولا `running` و`latest_height`، ثم `latest_finalized_height` وpeers النشطة، ثم latency و`round_timeout`، وأخيرا signer وsnapshot وreplay والحظر. تشغيل العملية لا يثبت تقدم الإجماع بأمان.

## نقاط النهاية والحالة

استخدم `/v1/status` للارتفاع وapp hash والنهائية وpeers، و`/v1/metrics` لـ JSON، و`/metrics/text` لـ Prometheus، و`/v1/diagnostics` للجاهزية، و`/v1/finality/latest` و`/v1/state/latest` و`/v1/recovery/report` و`/v1/snapshot` للأدلة والاسترداد. يجب حماية admin endpoints عبر loopback أو شبكة المشغل أو mTLS أو gateway موثقة.

تعني `running=true` في `/v1/status` أن runtime بدأ فقط. يجب أن يتقدم `latest_height` و`latest_finalized_height`، وأن يتطابق `latest_app_hash` بين peers عند الارتفاع نفسه، وأن يستخدم `active_peer_count` لقياس الجلسات الفعلية بدلا من peers المهيأة أو المسجلة في score فقط.

| `vexo_peer_count` | الأقران المحظورون حاليًا بموجب سياسة الدرجات | تشير المسامير إلى الهجوم أو تكوين الأقران السيئ أو الحدود الصارمة للغاية |

## مقاييس Prometheus

راقب `vexo_node_running` و`vexo_latest_height` و`vexo_active_peer_count` و`vexo_configured_peer_count` و`vexo_quorum_health_ratio` و`vexo_height_rate_per_minute` و`vexo_round_timeouts` و`vexo_adaptive_round_timeout_nanos` وp95 للـ proposal/vote/commit و`vexo_mempool_size` و`vexo_snapshot_healthy` و`vexo_replay_healthy` و`vexo_validator_signing_failures` و`vexo_recovery_finality_deferrals`.

`vexo_peer_count` محفوظ للوحات المعلومات القديمة. يجب أن ترسم لوحات المعلومات الجديدة `vexo_active_peer_count` و`vexo_configured_peer_count` و`vexo_scored_peer_count` بشكل منفصل.

## قواعد التنبيه المقترحة

قم بضبط الأرقام لعدد المدقق الفعلي، والفاصل الزمني للكتلة، والكمون، والأجهزة. هذه هي نقاط البداية، وليست ثوابت عالمية.

| التنبيه | شرط البداية | الإجراء |
|---|---|---|
| توقف الارتفاع | لا تقدم خلال فترتين أو ثلاث | قارن validators وproposer وsigner وpeers |
| توقف النهائية | التنفيذ يتقدم وfinalized height لا يتقدم | افحص QC وproof وvalidator-set hash |
| لا peers نشطة | `vexo_active_peer_count == 0` لدقيقة | افحص العنوان والهوية وauth وchain ID |
| quorum ضعيف | `vexo_quorum_health_ratio < 0.75` لعدة نوافذ | افحص partition وlatency وفقد peers |
| timeout مرتفع | العداد أو timeout المتكيف فوق baseline | افحص الشبكة وproposer وCPU والقرص وsigner |
| تأجيل الاسترداد | زيادة `vexo_recovery_finality_deferrals` | صدر recovery report قبل تعديل البيانات |

## عتبات البدء المقترحة

استخدم هذه القيم كقيم تنبيه أولية، ثم اضبطها بعد خط أساس حقيقي طويل المدى:

| الإشارة | تحذير | حرج |
|---|---|---|
| معدل الارتفاع | أقل من 50% من baseline | لا نمو |
| peers النشطة | دون هدف quorum | صفر |
| latency p95 | أكثر من 50% من الميزانية | أكثر من 80% |
| signer | أي خطأ | أخطاء متكررة في ارتفاع واحد |
| snapshot أو replay | فشل فحص واحد | فشل متكرر أو divergence |

أهم قاعدة: التنبيه على **التغيير بمرور الوقت**. يمكن أن يكون الرقم الواحد مضللًا ؛ فمعدل الارتفاع، وتأخر النهاية، وزبد الأقران، ونمو الميمبول، وفشل الموقعين معًا تروي القصة الحقيقية.

## مصفوفة فرز الحوادث

| الحالة | الطبقة المحتملة | الخطوة الآمنة |
|---|---|---|
| توقف الارتفاع وpeers سليمة | consensus أو signer أو runtime | احفظ السجلات وافحص proposer/timeout |
| فقد peers بعد النشر | network أو config | احفظ config وتراجع عن address/auth |
| اختلاف app hashes | execution أو storage | أوقف العقد المتأثرة وشغل strict replay |
| رفض finality proof | finality أو validator set | تحقق من height وset hash وsignature domain |
| فشل استعادة snapshot | state sync أو storage | استعد إلى directory نظيف |
| رفض remote signer | custody أو policy | ميز رفض السياسة عن عطل النقل |

| ارتفاع الأقران المحظورين | نظير إلى نظير/الأمان | لقطات نتائج الأقران وأسباب الحظر | فحص النميمة المشوهة أو التكوين الخاطئ المشترك |

أثناء الحادث حافظ على WAL وaddrbook وsigner guard وdata directory وconfig والسجلات. حذفها يدمر الأدلة اللازمة للتمييز بين bug وخطأ المشغل.

## السجلات والاستجابة الأولى

يجب أن تحمل الأحداث المنظمة node ID وvalidator ID وchain ID وheight وround وblock hash وpeer ID. احتفظ بـ `peer_connected` و`peer_dial_failed` و`block_committed` و`round_timeout` و`validator_signing_failure` و`snapshot_exported` و`replay_checked` و`upgrade_halt` و`upgrade_applied`.

قارن `/v1/status` على validator اثنين على الأقل، ثم افحص `/v1/diagnostics` وسجلات peers وmempool ومقاييس fees وsigner وأخيرا `/v1/recovery/report`. أرشف metrics وpprof وconfigs وgenesis وbinary checksums وevidence manifests مع سجلات release candidate.
<!-- vexo-docs:technical-parity -->
## ملحق التكافؤ التقني

يحافظ هذا الملحق على الأسماء التقنية التي يجب أن تبقى مطابقة للإصدار المرجعي:

- `rpc_listening` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `p2p_listening` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `peer_configured` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `peer_connected` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `peer_disconnected` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `peer_dial_failed` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `peer_banned` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `consensus_loop_running` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `block_committed` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `round_timeout` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `validator_signing_failure` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `evidence_received` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `evidence_applied` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `snapshot_exported` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `replay_checked` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `upgrade_halt` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `upgrade_applied` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
- `dist/` — يُستخدم هذا الاسم كما هو في أمثلة التشغيل والتحقق من الإعدادات.
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
