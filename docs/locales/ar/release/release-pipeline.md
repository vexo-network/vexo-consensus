# Release Pipeline

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## نظرة عامة

يساعد هذا المستند على فهم مسار الإصدار مع binaries موقعة وchecksums وSBOMوربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/ar/release/release-pipeline.md`

## لماذا تقرأ هذا المستند

- مسار الإصدار مع binaries موقعة وchecksums وSBOM
- تحقق أولاً من عبارات MUST/SHOULD/MAY في المصدر الإنجليزي.
- هذا المستند المحلي يساعد على الفهم؛ قرارات audit وrelease وsecurity تُحسم بالمصدر الإنجليزي.

## ما الذي يجب أن تستطيع فعله بعد القراءة

- شرح قرار التنفيذ أو التشغيل الذي يدعمه هذا المستند.
- ربط المتطلبات المعيارية في المصدر الإنجليزي بإعدادات الشبكة الحالية.
- التحقق من chain ID وvalidator ID وfee/gas وعناوين peer قبل نسخ الأمثلة.

## قائمة تحقق للاستخدام الآمن

- تحقق أولاً من عبارات MUST/SHOULD/MAY في المصدر الإنجليزي.
- لا تترجم الأوامر أو config key أو أسماء RPC أو حقول JSON أو معرّفات الكود.
- قبل نسخ الأمثلة، طابق chain ID وvalidator ID وfee/gas وعناوين peer مع شبكتك.
- بعد تعديل الوثائق، شغّل `make docs-check` للتحقق من locale tree وحراس الترجمة.

## نقاط يجب الانتباه لها

- هذا المستند المحلي يساعد على الفهم؛ قرارات audit وrelease وsecurity تُحسم بالمصدر الإنجليزي.
- عند تغير التنفيذ، حدّث المصدر الإنجليزي وكل المستندات المحلية في نفس التغيير.

## واجهات يجب الحفاظ عليها كما هي

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

## بنية المصدر الإنجليزي

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- دليل الإطلاق

## دليل توافق EVM/Web3

يجب أن يبقى دليل `--sdk-conformance-evidence` منفصلاً عن دليل `--evm-web3-conformance-evidence`. لا يكفي نص عام يقول إن EVM يعمل؛ يجب أن يحتوي دليل EVM/Web3 على أقسام آلية قابلة للفحص هي `evm_fixtures` و`evm_execution` و`web3_rpc` و`evm_corpus`، وأن يكون مربوطاً بـ `evidence-manifest.json` عبر SHA-256 قبل أي ادعاء توافق عام.

## المصدر المعتمد

- [الوثيقة الإنجليزية المرجعية](../../en/release/release-pipeline.md)

## مصطلحات attestation لأدلة الإصدار

في الإصدارات العامة يجب التحقق من كل عنصر داخل `evidence-manifest.json` بتوقيع Ed25519. اترك أعلام CLI وحقول JSON التالية كما هي من دون ترجمة.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`

## VRF audit evidence SHA-256

يجب أن يثبت `release gate` أدلة تدقيق VRF عبر SHA-256 كما يفعل مع BLS. يجب أن يكون ملف `--vrf-audit` داخل `evidence-manifest.json`، وأن يطابق `--vrf-audit-sha256` محتوى الملف بدقة. عند استخدام config يكون `vrf.audit_evidence_sha256` هو digest pin الافتراضي. هذه القاعدة تربط VRF service و KMS/HSM custody و TLS/mTLS أو pinned CA و auth token ودفاع nonce replay بأدلة الإصدار.
