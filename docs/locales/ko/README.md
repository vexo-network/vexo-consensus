# Vexo 문서

이 디렉토리는 Vexo 문서의 한국어 진입점입니다. 영어(`en`) 문서가 규범 원문이며, 한국어 문서는 운영자와 개발자가 빠르게 위치를 찾을 수 있도록 같은 디렉토리 구조를 유지합니다.

## 먼저 읽기

1. [합의 프로토콜 개요](./consensus-protocol.md)
2. [합의 명세](./specs/consensus-spec.md)
3. [트랜잭션 포맷](./specs/tx-format.md)
4. [검증자 생명주기](./specs/validator-lifecycle.md)
5. [보안 감사 준비](./security/audit-readiness.md)

## 문서 묶음

| 구분 | 경로 | 설명 |
|---|---|---|
| 운영자 | `operators/` | 노드 초기화, 검증자 추가, 설정 파일 관리 |
| 릴리즈 | `release/` | 릴리즈 파이프라인, 런북, 호환성, 게이트 |
| SDK | `sdk/` | 앱 모듈, 커스텀 crypto/storage/transport, RPC 버전 관리 |
| 보안 | `security/` | 위협 모델, 가정, 감사 준비 |
| 명세 | `specs/` | 합의, 네트워크, 스토리지, 트랜잭션, finality proof |

명령어, JSON 필드, RPC 메서드, 코드 식별자는 번역하지 않고 원문 그대로 유지합니다.
