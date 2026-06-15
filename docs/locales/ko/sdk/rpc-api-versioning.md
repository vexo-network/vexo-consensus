# RPC API 버전 관리

> Locale: ko · 한국어
> 이 문서는 영어 원문을 함께 읽기 위한 한국어 보조 문서입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

## 문서 개요

이 문서는 RPC API 버전 관리, 호환성 별칭, 안정화 정책을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/ko/sdk/rpc-api-versioning.md`

## 이 문서를 읽는 이유

- RPC API 버전 관리, 호환성 별칭, 안정화 정책
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

- `/v1`
- `/v1/healthz`
- `/v1/readyz`
- `/v1/status`
- `/v1/diagnostics`
- `/v1/metrics`
- `/v1/metrics/text`
- `/v1/peers`
- `/v1/tx`
- `/v1/evidence`
- `/v1/recovery`
- `/v1/snapshot/latest`
- `/v1/snapshot/export`
- `/v1/snapshot/chunk?index=0&size=10000`
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`

## 영어 원문 구조

- RPC API Versioning
- 안정성 목표
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## 규범 원문

- [영어 정본 문서](../../en/sdk/rpc-api-versioning.md)

## RPC capability discovery

새 RPC capability discovery 인터페이스입니다. 운영자는 `/v1/capabilities`로 실제 연결된 provider 기능을 확인하고, SDK 임베더는 `rpc.Config.RequiredCapabilities` 또는 `rpc.Config.RequireAllCapabilities`로 누락된 기능을 시작 시점에 실패 처리할 수 있습니다.

다음 인터페이스 이름은 그대로 유지하세요: `/v1/capabilities`, `CapabilityResponse`, `CapabilitySnapshot`, `RequiredCapabilities`, `RequireAllCapabilities`, `metrics`, `blocks`, `finality`, `strict_replay`, `consensus_control`.

<!-- vexo-docs:technical-parity -->
- `admin_token` and `admin_tokens` are stable configuration keys and must remain unchanged when describing optional bearer-token enforcement.

## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Stability Goal — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Current Stable API — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Versioning Rules — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Capability Discovery — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Compatibility Aliases — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Error Format — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Query Proofs — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Event Queries — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: IBC Queries — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Web3 JSON-RPC Bridge — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Web3 EVM Configuration — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Operational Compatibility — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `/v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/healthz` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/readyz` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/status` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/diagnostics` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/capabilities` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/metrics` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/metrics/text` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/peers` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/tx` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/evidence` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/recovery` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/snapshot/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/snapshot/export` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/snapshot/chunk?index=0&size=10000` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/blocks` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/blocks/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/blocks/{height}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/state/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/state/{height}/{namespace}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/events?key={attribute_key}&value={attribute_value}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/proof?namespace={namespace}&key={key}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/proof?namespace={namespace}&key={key}&height=latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/finality/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/finality/{height}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/ibc/client/{client_id}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/ibc/connection/{connection_id}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/ibc/channel/{port_id}/{channel_id}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/validators/{height}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/committee/{height}/{round}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/prune` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/replay` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/consensus/start` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/consensus/stop` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `network_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `tls_cert_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `tls_key_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `tls_ca_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `tls_server_name` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod start` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `strict: true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_gasPrice` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_web3Capabilities` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `require_network_safety` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.NewNetworkSafeServer` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.NewNetworkSafeHandlerWithConfig` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.Config.RequiredCapabilities` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.Config.RequireAllCapabilities` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `pending_txs` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `state_by_height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `app_query` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `strict_replay` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_control` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/status` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/tx` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/blocks/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/*` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v2/*` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/proof` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `commit_chain` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/status.latest_height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/events` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Index: true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `{ "path": [...], "value": ... }` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `packets/{source_port}/{source_channel}/{sequence}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc_modules` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `web3_clientVersion` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `web3_sha3` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `net_version` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `net_listening` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `net_peerCount` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_chainId` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_protocolVersion` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_syncing` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_mining` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_hashrate` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_blockNumber` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_blobBaseFee` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_maxPriorityFeePerGas` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_feeHistory` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
