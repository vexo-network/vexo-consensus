# Release Pipeline

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।

## सारांश

यह दस्तावेज़ signed binaries, checksums और SBOM वाली release pipeline को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/hi/release/release-pipeline.md`

## यह दस्तावेज़ क्यों पढ़ें

- signed binaries, checksums और SBOM वाली release pipeline
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
- documentation बदलने के बाद `make docs-check` चलाकर locale tree और translation guards जाँचें।

## ध्यान रखने योग्य बातें

- यह स्थानीयकृत दस्तावेज़ समझने में मदद करता है; audit, release और security decisions अंग्रेज़ी source से तय होते हैं।
- implementation बदलने पर अंग्रेज़ी source और सभी स्थानीयकृत दस्तावेज़ को उसी change में update करें।

## ज्यों-का-त्यों रखने वाले interfaces

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

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## अंग्रेज़ी source की संरचना

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- लॉन्च रनबुक

## EVM/Web3 अनुरूपता प्रमाण

`--sdk-conformance-evidence` और `--evm-web3-conformance-evidence` अलग-अलग प्रमाण हैं। केवल “EVM passed” जैसा सारांश पर्याप्त नहीं है; EVM/Web3 प्रमाण में मशीन-पठनीय `evm_fixtures`, `evm_execution`, `web3_rpc` और `evm_corpus` अनुभाग होने चाहिए, और किसी भी सार्वजनिक compatibility दावे से पहले उसे SHA-256 के साथ `evidence-manifest.json` से बाँधना चाहिए।

## VRF audit evidence SHA-256

`release gate` केवल BLS audit evidence को नहीं, VRF audit evidence को भी SHA-256 से pin करता है। `--vrf-audit` file को `evidence-manifest.json` में होना चाहिए, और `--vrf-audit-sha256` file content से बिल्कुल मेल खाना चाहिए। config के साथ `vrf.audit_evidence_sha256` default digest pin है। यह नियम VRF service, KMS/HSM custody, TLS/mTLS या pinned CA, auth token और nonce replay defense को release evidence से जोड़ता है।

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../../en/release/release-pipeline.md)

## रिलीज़ evidence attestation शब्द

सार्वजनिक रिलीज़ में `evidence-manifest.json` की हर entry Ed25519 signature से verify होनी चाहिए। नीचे दिए गए CLI flags और JSON fields को translate न करें।

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
