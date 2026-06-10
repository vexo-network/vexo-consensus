# Custom Crypto Backend Guide

> Locale: ko · 한국어
> 이 문서는 영어 원문을 함께 읽기 위한 한국어 보조 문서입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

## 문서 개요

이 문서는 BLS, VRF, signer 등 custom crypto backend 연결 방식을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/ko/sdk/custom-crypto-backend.md`

## 이 문서를 읽는 이유

- BLS, VRF, signer 등 custom crypto backend 연결 방식
- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 이 지역화 문서는 이해를 돕기 위한 보조 문서이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.

## 읽고 나면 할 수 있어야 하는 것

- 이 문서가 어떤 구현·운영 결정을 돕는지 설명할 수 있어야 합니다.
- 영어 원문의 규범 문장과 현재 네트워크 설정을 연결해서 검토할 수 있어야 합니다.
- 예제 명령과 config 값을 복사하기 전에 chain ID, validator ID, fee/gas, peer 주소를 확인할 수 있어야 합니다.

## 구현자가 반드시 구분해야 할 것

Custom crypto backend는 “서명 라이브러리 하나 연결”이 아니라 consensus safety 경계입니다.

- BLS backend는 aggregate finality signature 검증에 쓰입니다.
- VRF backend는 committee/randomness 선택 경로에 쓰입니다.
- Remote signer는 validator private key를 노드 프로세스 밖으로 분리하기 위한 운영 경계입니다.
- 세 경로 모두 domain separation을 유지해야 하며, 서로 다른 메시지 타입을 같은 sign bytes로 서명하면 안 됩니다.

## BLS backend 체크리스트

- adapter metadata의 `Name`, `Version`, `Audited`, `AuditReport`, `DependencyAudit`, `DomainSeparation`, `RogueKeyDefense`를 채웁니다.
- validator credential에는 public key와 proof-of-possession을 포함합니다.
- subgroup check, malformed key rejection, rogue-key 방어, aggregate verification test vector를 통과해야 합니다.
- 릴리즈 전에는 audit evidence SHA-256을 release gate에 고정합니다.

## VRF backend 체크리스트

- deterministic VRF는 테스트용으로만 사용합니다.
- 네트워크 운영에서는 local encrypted key, KMS, remote VRF service 중 하나를 명확히 선택합니다.
- proof verification이 runtime/gossip/finality 경로에서 같은 domain으로 수행되는지 확인합니다.
- TLS 또는 인증 토큰 없이 remote VRF를 공개 네트워크에서 사용하지 않습니다.

## Remote signer 체크리스트

- `height`, `round`, `type` 기반 sign policy를 signer 쪽에서도 검증합니다.
- double-sign guard는 노드뿐 아니라 signer 저장소에도 두는 것이 안전합니다.
- client auth token, replay nonce, request audit log를 운영 기본값으로 둡니다.
- 기본 `Sign` wrapper보다 deadline이 있는 `SignWithContext` 또는 `SignWithPolicyContext`를 우선 사용합니다.

## 안전 사용 체크리스트

- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 명령어, config key, RPC 이름, JSON 필드, 코드 식별자는 번역하지 않습니다.
- 예제 값은 그대로 복사하기 전에 자신의 chain ID, validator ID, fee/gas, peer 주소에 맞는지 확인합니다.
- 문서를 수정했다면 `make docs-check`로 locale tree와 번역 guard를 확인합니다.

## 주의할 점

- 이 지역화 문서는 이해를 돕기 위한 보조 문서이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.
- 구현이 바뀌면 영어 문서와 모든 locale 문서를 같은 변경에서 갱신해야 합니다.

## 원문 그대로 유지할 인터페이스

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

## 영어 원문 구조

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## 규범 원문

- [English canonical document](../../en/sdk/custom-crypto-backend.md)
