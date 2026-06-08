# Documentation

> Locale: ko · 한국어
> 이 문서는 `vexo-consensus` 문서 세트를 한국어로 읽기 위한 지역화 문서입니다. 프로토콜 규칙, 보안 판단, 릴리즈 판단, 명령어 의미, config key, RPC 이름은 영어 원문이 규범입니다.

## 이 문서 세트의 목적

`vexo-consensus`는 독립적인 PoS 네트워크를 만들기 위한 합의·런타임 프레임워크입니다. 이 문서 세트는 코드를 처음 보는 개발자, 노드를 운영하는 validator/operator, 릴리즈를 준비하는 maintainer, 감사를 준비하는 security reviewer가 같은 기준으로 프로젝트를 이해하도록 돕습니다.

명령어, JSON 필드, RPC 이름, config key, package path, 코드 식별자는 호환성을 위해 영어 원문 그대로 유지합니다. 설명과 읽는 순서, 운영상 주의점은 한국어로 풀어 적습니다.

## 처음 읽는 순서

1. [Consensus Protocol Overview](./consensus-protocol.md)
2. [Consensus Spec](./specs/consensus-spec.md)
3. [Transaction Format](./specs/tx-format.md)
4. [Validator Lifecycle](./specs/validator-lifecycle.md)
5. [Node Initialization](./operators/node-initialization.md)
6. [Security Audit Readiness](./security/audit-readiness.md)

## 역할별 읽기 경로

| 역할 | 먼저 볼 문서 | 같이 봐야 할 문서 |
|---|---|---|
| 합의 프로토콜을 이해하려는 개발자 | [Consensus Spec](./specs/consensus-spec.md) | [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md) |
| app module을 붙이는 개발자 | [App Module Guide](./sdk/app-module-guide.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| EVM 기능을 붙이거나 검토하는 개발자 | [EVM and Native Accounting](./specs/evm-native-accounting.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| validator/operator | [Node Initialization](./operators/node-initialization.md) | [Adding a Validator](./operators/add-validator.md), [Networking Spec](./specs/networking-spec.md) |
| 릴리즈 담당자 | [Release Pipeline](./release/release-pipeline.md) | [Launch Runbook](./release/launch-runbook.md), [Version Compatibility Matrix](./release/version-compatibility.md) |
| 보안 검토자 | [Security Audit Readiness](./security/audit-readiness.md) | [Consensus Spec](./specs/consensus-spec.md), [Storage Schema](./specs/storage-schema.md) |

## Protocol Specs

| 문서 | 다루는 내용 |
|---|---|
| [Consensus Spec](./specs/consensus-spec.md) | 합의 state machine, safety rule, liveness assumption, evidence surface |
| [Finality Proof Format](./specs/finality-proof-format.md) | full node와 light client가 finality proof를 검증하는 방식 |
| [Networking Spec](./specs/networking-spec.md) | handshake, gossip, peer scoring, ban, backoff, DoS 방어 |
| [Storage Schema](./specs/storage-schema.md) | durable record, index, recovery rule, snapshot, schema migration |
| [Transaction Format](./specs/tx-format.md) | signed envelope, nonce, fee, gas, CheckTx 요구사항 |
| [EVM and Native Accounting](./specs/evm-native-accounting.md) | native coin과 EVM balance/gas/accounting 연결 방식 |
| [Validator Lifecycle](./specs/validator-lifecycle.md) | validator admission, rotation, slashing, jailing, unbonding |

## SDK and Extension Guides

| 문서 | 다루는 내용 |
|---|---|
| [App Module Guide](./sdk/app-module-guide.md) | custom app module 추가, module CLI command 연결 |
| [Custom Crypto Backend](./sdk/custom-crypto-backend.md) | signing/finality backend, BLS/VRF adapter 연결 |
| [Custom Storage and Transport](./sdk/custom-storage-transport.md) | store 또는 peer transport adapter 구현 |
| [RPC API Versioning](./sdk/rpc-api-versioning.md) | `/v1/*` endpoint 안정성, compatibility alias, Web3/EVM RPC 경계 |

## Operations and Release

| 문서 | 다루는 내용 |
|---|---|
| [Node Initialization](./operators/node-initialization.md) | validator/archive node home 생성, split config 운용 |
| [Adding a Validator](./operators/add-validator.md) | validator 추가 흐름, height-specific validator set 검증 |
| [Launch Runbook](./release/launch-runbook.md) | 네트워크 출시 전후 체크리스트, halt 기준, postlaunch archive |
| [Release Pipeline](./release/release-pipeline.md) | signed binary, checksum, SBOM, reproducible artifact |
| [Cosmos/Tendermint Comparison Gate](./release/cosmos-comparison-gate.md) | Tendermint/Cosmos 스타일 기대치와 Vexo release evidence 비교 |
| [Version Compatibility Matrix](./release/version-compatibility.md) | binary, config, store, app, RPC, proof format 호환성 |

## Security

| 문서 | 다루는 내용 |
|---|---|
| [Security Audit Readiness](./security/audit-readiness.md) | threat model, security assumption, known limitation, audit evidence |

## 안전하게 읽는 법

- 한국어 문서는 이해를 돕기 위한 지역화 문서입니다.
- 릴리즈, 감사, 보안 판단은 반드시 영어 원문과 대조합니다.
- 예제 명령어는 그대로 복사하기 전에 자신의 `chain_id`, `validator_id`, fee/gas, peer 주소, key path에 맞는지 확인합니다.
- 문서를 수정했다면 `make docs-check`를 실행합니다.
- 영어 원문과 한국어 문서가 충돌하면 영어 원문을 기준으로 하고, 같은 변경에서 한국어 문서도 갱신합니다.

## 규범 원문

- [English canonical document](../en/README.md)
