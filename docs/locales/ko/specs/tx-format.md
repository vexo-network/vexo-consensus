# 트랜잭션 형식

> Locale: ko · 한국어
> 이 문서는 영어 원문의 한국어 직역본입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.


## 먼저 읽을 순서

이 문서는 Tx Format의 규범 규격을 설명합니다. 처음이라면 아래 순서로 읽는 것이 좋습니다.

1. Scope
2. Canonical Payload
3. Address Format
4. Signed Envelope
5. Required Ante Metadata
6. CheckTx Requirements
7. Fee and Gas
8. Load Test Payloads
9. CLI Examples

이 순서는 먼저 범위와 상태를 이해하고, 그다음 메시지/안전/진행성 규칙을 확인한 뒤, 마지막으로 증거를 읽는 흐름과 같습니다.

## 문서 개요

이 문서는 transaction format, signing, fee, gas 규칙을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/ko/specs/tx-format.md`

## 이 문서를 읽는 이유

- transaction format, signing, fee, gas 규칙
- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 이 페이지는 영어 원문의 직역본이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.

## 읽고 나면 할 수 있어야 하는 것

- 이 문서가 어떤 구현·운영 결정을 돕는지 설명할 수 있어야 합니다.
- 영어 원문의 규범 문장과 현재 네트워크 설정을 연결해서 검토할 수 있어야 합니다.
- 예제 명령과 config 값을 복사하기 전에 chain ID, validator ID, fee/gas, peer 주소를 확인할 수 있어야 합니다.

## 안전 사용 체크리스트

- 영어 원문에서 MUST/SHOULD/MAY 문장을 먼저 확인합니다.
- 명령어, config key, RPC 이름, JSON 필드, 코드 식별자는 번역하지 않습니다.
- 예제 값은 그대로 복사하기 전에 자신의 chain ID, validator ID, fee/gas, peer 주소에 맞는지 확인합니다.
- 문서를 수정했다면 `make docs-check`로 로케일 문서 트리와 번역 검사를 확인합니다.

## 주의할 점

- 이 페이지는 영어 원문의 직역본이며, 감사·릴리즈·보안 판단은 영어 원문으로 확정합니다.
- 구현이 바뀌면 영어 문서와 모든 locale 문서를 같은 변경에서 갱신해야 합니다.

## 원문 그대로 유지할 인터페이스

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## 영어 원문 구조

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## 규범 원문

- [영어 정본 문서](../../en/specs/tx-format.md)

## 바이너리 서명 envelope

서명된 트랜잭션은 바이너리입니다(`txcodec.EncodeEnvelope`). 2바이트 prefix `0x89 'V'`(`txcodec.SignedPrefix`), 버전 바이트 `1`, 그리고 uvarint 길이가 앞에 붙은 네 필드가 이어집니다: chain ID(최대 `txcodec.MaxChainIDLength`, 128바이트), 공개키(최대 `txcodec.MaxPublicKeyLength`), 서명(최대 `txcodec.MaxSignatureLength`), payload. 디코더는 최소가 아닌 길이, 한도를 넘는 필드, 알 수 없는 버전, 뒤에 남는 바이트를 거부하므로 서명 트랜잭션의 인코딩은 하나뿐입니다. `0x89`는 유효한 UTF-8이 아니어서 텍스트 payload가 envelope로 오인되지 않습니다. 노드는 `txcodec.MaxChainIDLength`보다 긴 chain ID를 시작 시 거부합니다(`config.ErrChainIDTooLong`).

텍스트 인터페이스는 서명 트랜잭션을 base64로 전달합니다. `vexod keys sign-tx`는 `tx: <base64>`와 `encoding: base64`를 출력하고, `POST /v1/tx`는 요청이 `"encoding": "plain"`을 지정하지 않으면 `tx`를 base64로 읽으며, `vexod tx parse --encoding base64`는 서명 트랜잭션을, `--encoding plain`(기본값)은 텍스트 payload를 읽습니다. 셋 모두 `txcodec.DecodeTxEncoding`으로 디코딩합니다.

ante handler는 성공한 인증만 기억합니다(`app.VerifyCache`, 최대 `app.VerifyCacheCapacity`개). 키는 트랜잭션 바이트와 chain ID처럼 판정이 의존하는 모든 것의 digest이며, 실패는 기억하지 않습니다. 블록의 서명은 순서대로 처리되기 전에 병렬로 검증됩니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Scope — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Canonical Payload — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Address Format — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Signed Envelope — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Required Ante Metadata — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: CheckTx Requirements — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Fee and Gas — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Load Test Payloads — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: CLI Examples — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `gas_limit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm_chain_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `chain_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `base_fee` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `max(min_fee, base_fee * gas)` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blob_base_fee` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blob_gas` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blob_gas_fee_cap` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_sendRawBlobTransaction` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blob_hashes` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_getBlobSidecarByTxHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_getBlobSidecarByBlobHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_chainId` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `net_version` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_sendRawTransaction` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `dynamic_base_fee` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `target_gas` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `dynamic_blob_base_fee` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `target_blob_gas` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bank:send` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bank:send:<from>:<to>:<amount>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bank:mint:<to>:<amount>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_raw` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_hash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `max_txs` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `max_total_bytes` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `max_txs_per_sender` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
