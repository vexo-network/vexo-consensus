# Custom Crypto Backend Guide

> Locale: ar · العربية
> هذا المستند مستنداً مساعداً بالعربية يُقرأ مع المصدر الإنجليزي. تبقى قرارات البروتوكول والأمان والإصدار معيارية في النص الإنجليزي.

## نظرة عامة

يساعد هذا المستند على فهم ربط custom crypto backend مثل BLS وVRF وsignerوربط ذلك بقرارات التنفيذ والتشغيل.

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/ar/sdk/custom-crypto-backend.md`

## لماذا تقرأ هذا المستند

- ربط custom crypto backend مثل BLS وVRF وsigner
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

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

## بنية المصدر الإنجليزي

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## المصدر المعتمد

- [English canonical document](../../en/sdk/custom-crypto-backend.md)
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `ecvrf-p256-sha256-tai-v1`
- `remote-vrf-http-v1`

## VRF audit evidence SHA-256

يجب أن يعلن VRF backend حدود التدقيق بوضوح مثل BLS. املأ `vrf.adapter_name` و `vrf.audit_report` و `vrf.dependency_audit` و `vrf.audit_evidence_sha256` و `vrf.key_source`؛ إذا اختلفت adapter metadata عن config فيجب أن يفشل runtime بشكل fail closed. يتحقق ECVRF adapter المدمج من go.mod dependency pin و audit evidence digest، بينما يستخدم remote VRF adapter مرجع تدقيق خارجي لـ KMS/HSM.

## Remote VRF service

`vexod keys serve-vrf` يوفّر `POST /prove` و `POST /verify` باستخدام ECVRF key، و `vexod keys verify-vrf` يتحقق من remote prover من البداية للنهاية. أبقِ `VEXO_REMOTE_VRF_TOKEN` و `remote-vrf-http-v1` و `vexo.remote_vrf.prove.v1` و `vexo.remote_vrf.verify.v1` كما هي.

Keep these interface names unchanged: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.
