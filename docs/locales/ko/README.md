> Locale: ko · 한국어

# 문서

이 디렉토리는 `vexo-consensus`의 실무 매뉴얼입니다.

소스 코드만 보고 추측하지 않고 네트워크를 이해, 구축, 운영, 검토, 릴리스해야 하는 사람을 위한 문서입니다. 좋은 문서는 아래 네 가지 질문에 바로 답해야 합니다.

1. 이 기능은 시스템에서 무엇을 담당하는가?
2. 어떤 파일, 명령어, config key, RPC, JSON 필드가 이를 구현하는가?
3. 안전하게 쓰려면 반드시 무엇이 맞아야 하는가?
4. 실제 네트워크에 올리기 전에 어떤 테스트와 운영 증거가 필요한가?

영어는 프로토콜, 보안, 릴리스, SDK, 명령어, config, RPC 동작의 규범 원문입니다. 로컬라이즈된 문서는 같은 트리를 같은 의미로 옮긴 번역본이며, 릴리스와 감사 판단은 항상 영어 원문을 기준으로 확인해야 합니다.

## 빠른 시작

시간이 많지 않다면 아래 순서로 읽으면 됩니다.

1. [`Node Initialization`](./operators/node-initialization.md) — 노드 홈을 만들고, 분리된 config 파일을 편집하고, validator 노드나 archive 노드를 시작하는 방법을 봅니다.
2. [`Docker Deployment`](../deployments/docker/README.md) — 단일 호스트 4노드 배포를 실행하거나 멀티 호스트 네트워크를 준비합니다.
3. [`Observability Guide`](./operators/observability.md) — 노드가 살아 있지만 비정상일 때 가장 먼저 봐야 할 신호를 확인합니다.
4. [`RPC API Versioning`](./sdk/rpc-api-versioning.md) — 지갑, Remix, Web3 도구를 Vexo RPC/Web3 엔드포인트에 연결하는 규칙을 확인합니다.

릴리스 후보를 보는 경우에는 자세한 사양보다 먼저 [`Production Readiness`](./production-readiness.md)와 [`Release Pipeline`](./release/release-pipeline.md)를 읽는 것이 좋습니다.

### 복사해서 쓰는 명령

| 작업 | 명령 경로 |
|---|---|
| 로컬 바이너리 빌드 | `make build` |
| 하나의 검증인 홈 만들기 | `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` |
| 홈 검증 | `vexod validate --home .vexo-validator-1` 및 `vexod config audit --home .vexo-validator-1 --strict` |
| 하나의 노드 실행 | `vexod start --home .vexo-validator-1` |
| 하나의 노드 쿼리 | `curl -s http://127.0.0.1:26657/v1/status` |
| Docker 4 validator 네트워크 실행 | `docker compose -f deployments/docker/compose.single-host-init.yml up` 다음에 `docker compose -f deployments/docker/compose.single-host.yml up` |
| Remix 연결 | Docker validator 1 Web3 URL `http://127.0.0.1:28657/web3` |
| Web3 chain ID 확인 | `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` |

## 이 문서 세트의 목적

`vexo-consensus`는 독립적인 PoS 네트워크를 만들기 위한 합의·런타임 프레임워크입니다. 이 문서 세트는 코드를 처음 보는 개발자, 노드를 운영하는 validator/operator, 릴리스 준비를 하는 maintainer, 감사를 준비하는 security reviewer가 같은 기준으로 프로젝트를 이해하도록 돕습니다.

명령어, JSON 필드, RPC 이름, config key, package path, 코드 식별자는 호환성을 위해 영어 원문 그대로 유지합니다. 설명과 읽는 순서, 운영상 주의점은 한국어로 풀어 적습니다.

좋은 문서는 단순히 “기능이 있다”라고만 말하지 않습니다. 각 문서는 다음 질문에 답해야 합니다.

1. 이 기능은 시스템에서 어떤 책임을 맡는가?
2. 어떤 파일, 명령어, config key, RPC, JSON 필드가 이 기능을 구현하는가?
3. 안전하게 쓰려면 어떤 조건이 반드시 맞아야 하는가?
4. 실제 네트워크에 올리기 전에 어떤 테스트와 운영 증거가 필요한가?

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
| 프로토콜을 이해하려는 개발자 | [Consensus Spec](./specs/consensus-spec.md) | [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md) |
| app module을 붙이는 개발자 | [App Module Guide](./sdk/app-module-guide.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| EVM 기능을 붙이는 개발자 | [EVM and Native Accounting](./specs/evm-native-accounting.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| 노드를 운영하는 사람 | [Node Initialization](./operators/node-initialization.md) | [Adding a Validator](./operators/add-validator.md), [Observability Guide](./operators/observability.md) |
| 릴리스/감사를 준비하는 사람 | [Production Readiness](./production-readiness.md) | [Security Audit Readiness](./security/audit-readiness.md), [Release Pipeline](./release/release-pipeline.md) |

## 프로토콜 사양

| 문서 | 목적 |
|---|---|
| [Consensus Spec](./specs/consensus-spec.md) | 합의 상태 머신, 안전 규칙, liveness 가정, 증거 표면 |
| [Finality Proof Format](./specs/finality-proof-format.md) | 풀 노드와 라이트 클라이언트를 위한 증명 필드와 검증 규칙 |
| [Networking Spec](./specs/networking-spec.md) | 전송, handshake, peer scoring, backoff, DoS 방어 기대치 |
| [Storage Schema](./specs/storage-schema.md) | 내구성 있는 레코드, 인덱스, 복구 규칙, snapshot, schema migration 기대치 |
| [Transaction Format](./specs/tx-format.md) | 표준 트랜잭션 페이로드, signed envelope, nonce, fee, gas, CheckTx 요구 사항 |
| [EVM and Native Accounting](./specs/evm-native-accounting.md) | native/EVM 공유 잔액 모델, 256-bit amount, fee 처리, 호환성 경계 |
| [Validator Lifecycle](./specs/validator-lifecycle.md) | validator admission, rotation, evidence lifecycle, slashing, jailing, unbonding |

## SDK 및 확장 가이드

| 문서 | 목적 |
|---|---|
| [App Module Guide](./sdk/app-module-guide.md) | custom application module과 module CLI command 추가 방법 |
| [Custom Crypto Backend](./sdk/custom-crypto-backend.md) | signing/finality backend와 production BLS adapter metadata 추가 방법 |
| [Custom Storage and Transport](./sdk/custom-storage-transport.md) | custom store 또는 peer transport 구현 방법 |
| [RPC API Versioning](./sdk/rpc-api-versioning.md) | `/v1/*` compatibility rule과 endpoint stability 이해 |

## 운영 및 출시

| 문서 | 목적 |
|---|---|
| [Node Initialization](./operators/node-initialization.md) | validator/archive node 초기화와 분리된 subsystem config file 관리 |
| [Adding a Validator](./operators/add-validator.md) | validator를 추가하고 height별 validator-set update를 검증하는 운영 흐름 |
| [Observability Guide](./operators/observability.md) | health, metric, log, alert threshold, first-response playbook |
| [런칭 런북](./release/launch-runbook.md) | operator launch flow, halt 기준, monitoring, post-launch archive 요구 사항 |
| [Release Pipeline](./release/release-pipeline.md) | build, sign, package, release artifact gate |
| [Cosmos/Tendermint Comparison Gate](./release/cosmos-comparison-gate.md) | Tendermint/Cosmos의 성숙도 장점을 Vexo release evidence로 환산하는 기준 |
| [Version Compatibility Matrix](./release/version-compatibility.md) | binary, config, store, app, RPC, proof format 간 compatibility 기대치 |

## 보안

| 문서 | 목적 |
|---|---|
| [Security Audit Readiness](./security/audit-readiness.md) | threat model, assumption, limitation, safety argument, 필수 감사 증거 |

## 현지화된 문서

로케일 파일은 canonical tree에서 벗어나면 안 됩니다. 명령어, JSON 필드, RPC 이름, config key, 코드 식별자는 그대로 유지한 채 설명만 번역하는 방식이기 때문에, 예제를 언어별로 복사해도 동작과 의미가 바뀌지 않아야 합니다.

| 문서 | 목적 |
|---|---|
| [Documentation Locales](./locales/README.md) | locale 디렉터리 맵과 번역 정책 |
| [English Canonical Docs](./locales/en/README.md) | 규범적 영어 문서 트리 |
| [Korean Docs](./locales/ko/README.md) | 한국어 locale 트리 |
| [Chinese Docs](./locales/zh/README.md) | 중국어 locale 트리 |
| [Japanese Docs](./locales/ja/README.md) | 일본어 locale 트리 |
| [French Docs](./locales/fr/README.md) | 프랑스어 locale 트리 |
| [German Docs](./locales/de/README.md) | 독일어 locale 트리 |
| [Spanish Docs](./locales/es/README.md) | 스페인어 locale 트리 |
| [Portuguese Docs](./locales/pt/README.md) | 포르투갈어 locale 트리 |
| [Russian Docs](./locales/ru/README.md) | 러시아어 locale 트리 |
| [Arabic Docs](./locales/ar/README.md) | 아랍어 locale 트리 |
| [Hindi Docs](./locales/hi/README.md) | 힌디어 locale 트리 |
| [Indonesian Docs](./locales/id/README.md) | 인도네시아어 locale 트리 |
| [Vietnamese Docs](./locales/vi/README.md) | 베트남어 locale 트리 |

## 새 문서 작성

문서는 다음 기준을 따라야 합니다.

- 독자가 달성하려는 목표와 그 페이지가 지원하는 결정을 먼저 적습니다.
- 이 문서가 normative spec, implementation guide, operator guide, release/audit checklist 중 무엇인지 밝힙니다.
- 관련 명령어, package path, config key, RPC method, JSON field를 포함합니다.
- 안전 경계, 실패 모드, 피해야 할 shortcut을 설명합니다.
- 증거가 없는 상태에서 production-ready라고 주장하지 않습니다.
- 예제는 가능하면 바로 복사해 쓸 수 있게 유지하되, 반드시 바꿔야 하는 값은 분명히 표시합니다.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` 아래에 모든 Markdown 파일을 미러링합니다.
- `make docs-check`를 통과해 locale 트리와 canonical tree가 벌어지지 않게 합니다.

## 프로덕션 주장 규칙

기능이 있다고 해서 바로 production-ready라고 부르면 안 됩니다. production 주장에는 아래가 필요합니다.

- implementation code
- unit/property/adversarial tests
- 프로세스나 머신 경계를 넘는 기능이라면 운영 또는 E2E 증거
- 가정과 실패 모드에 대한 문서화
- BLS, VRF, Web3/EVM 호환성, slashing, state sync, upgrade, validator 경제처럼 보안에 민감한 영역에 대한 release-gate evidence

`vexod status --json`도 같은 규칙을 따릅니다. `features` map은 config로 해당 코드 경로가 켜져 있는지 알려 주고, `feature_assurance` map은 그 기능이 단순 구현인지, operator artifact가 필요한지, release evidence가 필요한지, 외부 감사까지 필요한지 알려 줍니다.

분리된 config file에는 운영자 안전 기본값이 들어갑니다. node home을 검토할 때는 아래를 먼저 확인하세요.

- restart-safe P2P handshake replay protection을 위한 `network_config.json:p2p.auth_replay_path`
- peer-authentication key를 위한 `network_config.json:p2p.node_key_path` (validator consensus custody와 분리)
- proposal spam 및 economic-friction policy를 위한 `module_config.json:governance.RequireDeposit`와 `module_config.json:governance.MinDeposit`
- execution/finality 경계를 위한 `consensus_config.json:consensus.execution_commit`
- restart-safe pending transaction recovery를 위한 `mempool_config.json:mempool.WALPath`

## 문서 검토 체크리스트

문서 변경을 merge하기 전에 다음을 확인하세요.

- 영어 문서가 릴리스/감사 원문으로 쓸 만큼 정확한지 확인합니다.
- 모든 locale 파일이 정확한 영어 canonical document를 가리키는지 확인합니다.
- 명령어, RPC 이름, config key, JSON field, package name은 그대로 유지합니다.
- `make docs-check`를 실행합니다.
- 명령어 예제, config schema, generated artifact가 바뀌었다면 더 넓은 프로젝트 검증도 실행합니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: How to Read This Set — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Start Here — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Protocol Specs — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: SDK and Extension Guides — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Operations and Release — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Security — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Localized Documentation — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Writing New Docs — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Production Claim Rule — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Documentation Review Checklist — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `vexo-consensus` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/*` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make docs-check` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod status --json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `feature_assurance` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `network_config.json:p2p.auth_replay_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `network_config.json:p2p.node_key_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `module_config.json:governance.RequireDeposit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `module_config.json:governance.MinDeposit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_config.json:consensus.execution_commit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `mempool_config.json:mempool.WALPath` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
