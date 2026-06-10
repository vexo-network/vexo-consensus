# Launch Runbook

> Locale: ko · 한국어
> 이 문서는 영어 원문을 함께 읽기 위한 한국어 보조 문서입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

## 문서 개요

이 문서는 네트워크 출시 전 운영자 체크리스트와 실행 절차을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/ko/release/launch-runbook.md`

## 이 문서를 읽는 이유

- 네트워크 출시 전 운영자 체크리스트와 실행 절차
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
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## 영어 원문 구조

- Launch Runbook
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Release candidate command updates

Use `vexod ops conformance --evm-default-fixtures` when a chain-specific Ethereum fixture corpus is not ready yet; add `--evm-tx-fixtures` later for chain-specific Web3/EVM cases. Use `vexod release gate --evidence-manifest dist/evidence-manifest.json` so long-run, chaos, signer, snapshot, P2P, state-sync, economics, governance, MEV, ops, formal-safety, SDK, external-audit, and BLS evidence are hash-bound before publication.

## VRF audit evidence SHA-256

릴리즈 후보를 검증할 때 `release gate` 명령에는 BLS와 VRF audit evidence digest를 모두 넣습니다. 최소한 `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256`, `--evidence-manifest`를 함께 사용하고, 모든 evidence 파일이 manifest의 SHA-256과 일치하는지 확인합니다.

## 규범 원문

- [English canonical document](../../en/release/launch-runbook.md)
