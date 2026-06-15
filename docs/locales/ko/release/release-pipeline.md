# 릴리즈 파이프라인

> Locale: ko · 한국어
> 이 문서는 영어 원문을 함께 읽기 위한 한국어 보조 문서입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.


## 먼저 읽을 순서

이 문서는 Release Pipeline의 릴리즈·운영 절차를 설명합니다. 처음이라면 아래 순서로 읽는 것이 좋습니다.

1. Goals
2. Release Commands
3. CI Gates
4. Evidence Quality Rules
5. Artifacts
6. Reproducibility Notes
7. Signed Binaries
8. SBOM
9. Audit Pack
10. Release Candidate Targets
11. Launch Runbook

이 순서는 먼저 목표와 게이트를 이해하고, 그다음 아티팩트와 증거 요구사항을 확인한 뒤, 마지막으로 실행 순서를 따르는 흐름과 같습니다.

## 문서 개요

이 문서는 서명된 바이너리, checksums, SBOM을 포함한 릴리즈 파이프라인을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/ko/release/release-pipeline.md`

## 이 문서를 읽는 이유

- 서명된 바이너리, checksums, SBOM을 포함한 릴리즈 파이프라인
- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 이 지역화 문서는 이해를 돕기 위한 보조 문서이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.

## 읽고 나면 할 수 있어야 하는 것

- 이 문서가 어떤 구현·운영 결정을 돕는지 설명할 수 있어야 합니다.
- 영어 원문의 규범 문장과 현재 네트워크 설정을 연결해서 검토할 수 있어야 합니다.
- 예제 명령과 config 값을 복사하기 전에 chain ID, validator ID, fee/gas, peer 주소를 확인할 수 있어야 합니다.

## 안전 사용 체크리스트

- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 명령어, config key, RPC 이름, JSON 필드, 코드 식별자는 번역하지 않습니다.
- 예제 값은 그대로 복사하기 전에 자신의 chain ID, validator ID, fee/gas, peer 주소에 맞는지 확인합니다.
- 문서를 수정했다면 `make docs-check`로 locale tree와 번역 guard를 확인합니다.

## 주의할 점

- 이 지역화 문서는 이해를 돕기 위한 보조 문서이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.
- 구현이 바뀌면 영어 문서와 모든 locale 문서를 같은 변경에서 갱신해야 합니다.

## 원문 그대로 유지할 인터페이스

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
- `RELEASE_CGO_ENABLED=1`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `release-candidate-smoke`
- `release-candidate-plan`
- `make release-portable RELEASE_REQUIRE_BLS=0`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## 영어 원문 구조

- Release Pipeline
- 목표
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- 출시 런북

## 릴리즈 게이트 증거 바인딩

`release gate`는 이제 릴리즈 증거 산출물을 `evidence-manifest.json`으로 묶어서 검증합니다. Manifest는 각 증거의 `name`, `path`, `sha256`을 기록하고, 실제 파일 내용이 manifest 해시와 다르면 gate가 실패합니다. 운영자가 그대로 복사할 수 있어야 하므로 `--evidence-manifest`, `--sdk-conformance-evidence`, `--evm-web3-conformance-evidence`, `--external-audit`, `--bls-audit` 같은 CLI 플래그와 JSON 필드명은 번역하지 않습니다. SDK evidence와 EVM/Web3 evidence는 분리되며, EVM/Web3 evidence에는 `evm_fixtures`, `evm_execution`, `web3_rpc`, `evm_corpus`가 포함되어야 합니다.

## 릴리즈 후보 정책

공개 릴리즈 후보는 기본 `make release-candidate`를 사용합니다. 이 target은 실제 gate이며 `release-candidate-real`로 연결되고, BLS가 들어간 artifact를 만들기 위해 `RELEASE_CGO_ENABLED=1`을 요구합니다. `make release-candidate-plan`은 PR smoke와 운영 계획 검토용입니다. 내장 fixture와 dry-run plan을 쓰므로 최종 릴리즈 evidence로 제출하면 안 됩니다. no-cgo artifact가 필요하면 `make release-portable RELEASE_REQUIRE_BLS=0`으로 만들 수 있지만, 이 artifact는 BLS-capable release로 발표하면 안 됩니다. `RELEASE_CGO_ENABLED=1`이고 `RELEASE_TARGETS`를 지정하지 않으면 Makefile은 현재 host target만 빌드합니다. 여러 OS/architecture artifact가 필요하면 각 target에 맞는 cgo cross-compiler가 있는 runner에서 `RELEASE_TARGETS`를 명시하세요.

## VRF audit evidence SHA-256

`release gate`는 BLS audit evidence뿐 아니라 VRF audit evidence도 SHA-256으로 고정해야 합니다. `--vrf-audit` 파일은 `evidence-manifest.json`에 포함되어야 하며, `--vrf-audit-sha256` 값은 파일 내용과 정확히 일치해야 합니다. config를 사용하는 경우 `vrf.audit_evidence_sha256`이 기본 digest pin으로 쓰입니다. 이 규칙은 VRF service, KMS/HSM custody, TLS/mTLS 또는 pinned CA, auth token, nonce replay 방어가 실제 릴리즈 증거에 묶였는지 확인하기 위한 것입니다.

## 규범 원문

- [영어 정본 문서](../../en/release/release-pipeline.md)

## 릴리즈 증거 attestation 용어

공개 릴리즈에서는 `evidence-manifest.json` 항목이 Ed25519 서명으로 검증되어야 합니다. 아래 CLI 플래그와 JSON 필드는 번역하지 말고 그대로 유지합니다.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
<!-- vexo-docs-ops-update-2026-06 -->

## 네트워크 E2E 해석

`make network-e2e`는 단순 빌드 테스트가 아니라 실제 바이너리로 4개 validator를 띄우고 signed-shape smoke transaction, peer 연결, height 증가, clean stop까지 확인합니다. `NETWORK_E2E_GO_TIMEOUT`은 외부 Go test 제한이고, 내부 network timeout보다 충분히 커야 실제 실패 원인이 로그에 남습니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Goals — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Release Commands — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: CI Gates — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Evidence Quality Rules — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Artifacts — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Reproducibility Notes — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Signed Binaries — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: SBOM — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Audit Pack — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Release Candidate Targets — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Launch Runbook — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `network analyze-longrun` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release collect-evidence` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `ops-runbook` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p-scale` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `state-sync-light-client` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `snapshot-replay` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make check` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make fuzz-smoke` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod consensus adversarial` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod ops conformance` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod network longrun` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod network chaos-plan` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make network-e2e` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make race` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `NETWORK_E2E_GO_TIMEOUT` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make test` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make vet` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make docs-check` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make build` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make release-candidate-smoke VERSION=ci`
- `make release-candidate-plan VERSION=ci` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make release-candidate VERSION=<rc> RELEASE_CGO_ENABLED=1 RC_EVM_CONFORMANCE_FLAGS=...` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evidence-manifest.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--allow-external-pending` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--private-rc` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo-release-evidence-attestation-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release evidence-manifest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--signing-key` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--signing-key-env` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `<evidence-file>.sig` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `<evidence-file>.sig.pub` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `<evidence-file>.pub` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `dist/` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod-<version>-<os>-<arch>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `checksums.txt` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `checksums.txt.asc` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `sbom-go-modules.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `sbom-go-version.txt` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release-manifest.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release-audit-pack.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `longrun-analysis.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `docs-quality.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RELEASE_CGO_ENABLED=1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `supranational/blst` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `go build -trimpath` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `BUILD_DATE` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make release-candidate` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make release-portable RELEASE_REQUIRE_BLS=0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RELEASE_TARGETS` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release-candidate` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release-candidate-real` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod ops conformance --strict` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RC_EVM_CONFORMANCE_FLAGS` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RC_LONGRUN_DURATION` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release-candidate-plan` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RELEASE_REQUIRE_BLS=0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `allow_noop_migrations=true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod upgrade apply --allow-empty-migrations` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--bls-audit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--bls-audit-sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--config <path>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `crypto.audit_evidence_sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--vrf-audit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--vrf-audit-sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.audit_evidence_sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `docs/security/blst-audit-evidence.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `docs/security/ecvrf-audit-evidence.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
