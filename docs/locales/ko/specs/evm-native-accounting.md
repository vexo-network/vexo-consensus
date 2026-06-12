# EVM과 네이티브 회계

> Locale: ko · 한국어
> 이 문서는 영어 원문을 함께 읽기 위한 한국어 보조 문서입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

## 문서 개요

이 문서는 native coin과 EVM gas/accounting을 일관되게 연결하는 방식을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/ko/specs/evm-native-accounting.md`

## 이 문서를 읽는 이유

- native coin과 EVM gas/accounting을 일관되게 연결하는 방식
- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 이 지역화 문서는 이해를 돕기 위한 보조 문서이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.

## 읽고 나면 할 수 있어야 하는 것

- 이 문서가 어떤 구현·운영 결정을 돕는지 설명할 수 있어야 합니다.
- 영어 원문의 규범 문장과 현재 네트워크 설정을 연결해서 검토할 수 있어야 합니다.
- 예제 명령과 config 값을 복사하기 전에 chain ID, validator ID, fee/gas, peer 주소를 확인할 수 있어야 합니다.

## 핵심 개념

Vexo의 native coin과 EVM 계정 잔액은 서로 다른 자산이 아닙니다.

- 최소 단위는 `avxo`이고, 사람이 읽는 단위는 `gvxo`, `vexo`입니다.
- EVM의 `value`, gas fee, blob fee는 모두 native coin 회계와 연결됩니다.
- `eth_getBalance`로 보는 `0x` 계정 잔액과 native bank store의 해당 계정 잔액은 같은 경제 상태를 바라봅니다.
- 일반 Vexo tx는 ante layer에서 fee를 차감하고, raw Ethereum tx는 EVM state transition에서 gas/value 처리를 수행합니다.
- `value`, `gasPrice`, `maxFeePerGas`, `maxPriorityFeePerGas`, `maxFeePerBlobGas`, `effectiveGasPrice`는 `uint64`보다 커도 잘리지 않고 Ethereum hex quantity 형태로 Web3 RPC에 표시됩니다.
- EVM simulation/query 경로는 `value_hex`, `gas_price_hex`, `max_fee_per_gas_hex`, `max_priority_fee_per_gas_hex`를 지원합니다. 작은 값용 numeric field도 남아 있지만, hex field와 값이 충돌하면 안전하게 거절합니다.

## Web3 사용 시 주의점

Vexo는 Ethereum node가 아니라 Vexo consensus 위에 EVM 실행 환경을 얹은 네트워크입니다.

- Ethereum devp2p, Ethereum fork choice, Ethereum sync protocol은 사용하지 않습니다.
- Blob transaction은 sidecar가 필요하므로 `eth_sendRawBlobTransaction` 또는 `vexo_sendRawBlobTransaction`을 사용합니다.
- Blob hash만 들어 있는 `eth_sendRawTransaction` 요청은 sidecar가 없기 때문에 거절됩니다.
- Web3 block 응답과 `newHeads`는 해당 height의 retained EVM state root가 있어야만 응답합니다.
- app hash는 Web3의 Ethereum-style `stateRoot`를 대체하는 값으로 쓰면 안 되므로, RPC 서버는 app hash fallback을 하지 않습니다.
- `execution.strict_evm_state_root`는 config 호환성을 위해 남아 있지만, app-hash 대체 응답을 허용하지 않습니다.

## 운영자 체크포인트

- EVM 모듈을 켰다면 retained EVM snapshot 또는 replay evidence를 릴리즈 증거에 포함합니다.
- `base_fee`, `blob_base_fee`, `target_gas`, `target_blob_gas`가 실제 트래픽에 맞는지 장기 부하 테스트로 확인합니다.
- native bank balance와 `eth_getBalance`가 같은 계정에 대해 같은 자산 흐름을 보여주는지 테스트합니다.
- Web3 RPC를 공개한다면 filter snapshot 저장 경로, state root 보존 기간, blob sidecar 보존 기간을 운영 정책으로 정합니다.

## 안전 사용 체크리스트

- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 명령어, config key, RPC 이름, JSON 필드, 코드 식별자는 번역하지 않습니다.
- 예제 값은 그대로 복사하기 전에 자신의 chain ID, validator ID, fee/gas, peer 주소에 맞는지 확인합니다.
- 문서를 수정했다면 `make docs-check`로 locale tree와 번역 guard를 확인합니다.

## 주의할 점

- 이 지역화 문서는 이해를 돕기 위한 보조 문서이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.
- 구현이 바뀌면 영어 문서와 모든 locale 문서를 같은 변경에서 갱신해야 합니다.

## 원문 그대로 유지할 인터페이스

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `gasPrice`
- `maxFeePerGas`
- `maxPriorityFeePerGas`
- `maxFeePerBlobGas`
- `effectiveGasPrice`
- `value_hex`
- `gas_price_hex`
- `max_fee_per_gas_hex`
- `max_priority_fee_per_gas_hex`
- `eth_getBalance`
- `bank query balance`

## 영어 원문 구조

- EVM과 네이티브 회계
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM 실행
- State Root Policy
- Compatibility Boundary
- Failure Modes

## 규범 원문

- [영어 정본 문서](../../en/specs/evm-native-accounting.md)
