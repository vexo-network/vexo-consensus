# 보안 감사 준비

> Locale: ko · 한국어
> 이 문서는 영어 원문을 함께 읽기 위한 한국어 보조 문서입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

## 문서 개요

이 문서는 threat model, 보안 가정, 감사 제출 증거을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/ko/security/audit-readiness.md`

## 이 문서를 읽는 이유

- threat model, 보안 가정, 감사 제출 증거
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

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## 영어 원문 구조

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- 보안 목표
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

감사 제출물에는 BLS뿐 아니라 VRF adapter audit evidence도 포함해야 합니다. `docs/security/ecvrf-audit-evidence.json` 같은 evidence 파일의 SHA-256을 `vrf.audit_evidence_sha256` 또는 `--vrf-audit-sha256`에 고정하고, dependency audit, key custody, TLS/mTLS 또는 pinned CA, auth, replay 방어, service availability를 한 묶음으로 검토합니다.

## 규범 원문

- [영어 정본 문서](../../en/security/audit-readiness.md)
## 감사 준비 추가 설명

감사자는 코드 존재 여부보다 증거의 재현성을 먼저 봅니다. 따라서 위협 모델, 알려진 한계, BLS/VRF 감사 자료, EVM conformance 결과, longrun/chaos 결과, KMS 서명 증거, snapshot/replay 증거가 동일한 바이너리와 동일한 설정에서 생성되었는지 확인해야 합니다. 운영자는 예외를 숨기지 말고 release gate가 거절한 항목을 그대로 기록한 뒤, 담당자와 재검증 조건을 runbook에 남겨야 합니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Scope — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Threat Model — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Security Assumptions — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Known Limitations — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Formal-ish Safety Argument — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Required Evidence for Audit — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Auditor Focus Areas — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Practical Audit Walkthrough — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Remote Signer Audit Notes — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: EVM/Web3 Audit Notes — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Snapshot and WAL Audit Notes — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `docs/security/blst-audit-evidence.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `remote-vrf-http-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod keys serve-vrf` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `release collect-evidence` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/*` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `chain_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `go.mod` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/recovery/report` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
