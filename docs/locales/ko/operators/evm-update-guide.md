# EVM 업데이트 가이드

> Locale: ko · 한국어
> 이 문서는 영어 원문의 한국어 번역본입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

이 문서는 내장 EVM 스택을 업데이트할 때 체인 ID 처리, Web3 호환성, 릴리즈 증거를 깨뜨리지 않으면서 바꾸는 방법을 설명합니다. go-ethereum 버전 변경, fork preset 조정, EVM 동작 수정처럼 릴리즈에 민감한 변경을 운영자와 유지보수자가 통제된 방식으로 수행할 때 참고합니다.

## EVM 업데이트로 보는 변경

다음 중 하나라도 바뀌면 단순 리팩터가 아니라 릴리즈 민감 기능 변경으로 취급해야 합니다.

- `modules/evm/backend/geth`의 `go-ethereum` 버전 변경
- `modules/evm/ethcompat` 변경
- `modules/evm` 변경
- `execution.evm_fork_preset` 변경
- `execution.evm_chain_config_json` 변경
- raw transaction 수락, gas accounting, receipts, traces, proofs, block response fields 변경
- `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction`, `eth_sendTransaction` 같은 managed Web3 account 처리 변경

## 안전한 업데이트 순서

코드, 설정, 문서가 동시에 맞물리도록 아래 순서로 진행합니다.

1. 먼저 geth-backed adapter를 독립적으로 수정합니다.
2. 그다음 fixture corpus와 conformance test를 갱신합니다.
3. 의미가 바뀌면 `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md`, `docs/sdk/rpc-api-versioning.md`를 수정합니다.
4. release evidence 형식이 바뀌면 `docs/release/release-pipeline.md`를 수정합니다.
5. 운영자 설정 스위치가 바뀌면 node configuration 문서를 고칩니다.
6. 머지 전에 validation matrix를 다시 돌립니다.

EVM runtime version을 올린 뒤 바로 배포하지 마세요. conformance suite, RPC smoke check, Docker deployment check가 모두 통과해야 합니다.

## 업데이트 절차

### 1. 변경 범위를 고정

업데이트 의도를 정확히 적습니다.

- fork behavior만 변경
- transaction admission만 변경
- execution semantics만 변경
- RPC compatibility만 변경
- blob / receipt / trace 처리만 변경
- managed account 또는 wallet behavior만 변경

이렇게 나눠야 리뷰가 집중되고, 무관한 코드가 같이 흔들리지 않습니다.

### 2. 가장 좁은 계층에서 수정

다음 경계를 우선합니다.

- `modules/evm/backend/geth`는 upstream go-ethereum 통합 변경
- `modules/evm/ethcompat`는 raw transaction decoding, hash 보존, fixture 처리
- `modules/evm`는 state transition, receipts, logs, storage, snapshot 동작
- `rpc`는 Web3 request/response surface 변경
- `cmd/vexod`는 CLI나 release workflow가 새 동작을 노출해야 할 때만 수정

변경이 application module까지 닿는다면 module boundary를 명시하고 deterministic state write를 유지해야 합니다.

### 3. 기본 설정 갱신

의미가 바뀌면 같은 패치에서 기본 config도 같이 바꿔야 합니다.

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- 필요하면 `network_config.json`의 managed account RPC 필드
- `module_config.json`의 EVM chain ID

숨겨진 CLI flag로 runtime behavior를 설명하려고 하지 마세요. config 파일만 봐도 노드 동작이 보이게 해야 합니다.

### 4. conformance stack 실행

최소한 아래는 돌립니다.

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

그다음 실제 사용자가 가장 먼저 깨뜨리는 경로를 확인합니다.

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Docker single-host 배포라면 아래 주소도 확인합니다.

```text
http://127.0.0.1:28657/web3
```

다음 동작은 꼭 확인합니다.

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

그다음 단순 contract deploy, proxy contract deploy, UUPS upgrade 경로를 실제 production에서 쓸 RPC endpoint로 시험합니다.

### 5. proxy와 upgrade 동작 확인

아래가 모두 참이어야 업데이트가 끝난 것입니다.

- 일반 contract deploy 성공
- proxy deploy 성공
- UUPS upgrade 호출 성공
- upgrade 후 storage와 code가 기대값으로 읽힘
- nonce tracking이 monotonic하게 유지됨
- block producer가 unsafe proposal 오류 없이 트랜잭션을 받아들임

proxy deploy는 되는데 upgrade가 실패하면 아직 배포하면 안 됩니다. 이건 경고가 아니라 release blocker입니다.

### 6. 증거 갱신

EVM surface가 바뀌면 release evidence bundle도 갱신합니다.

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- pinned SHA-256 fixture reference

release evidence에는 무엇이 바뀌었는지, 무엇을 테스트했는지, 어떤 commit 또는 version을 검증했는지가 들어가야 합니다. 실제로 실행한 코드와 증거가 맞지 않으면 EVM 업데이트가 끝났다고 말하면 안 됩니다.

## 검증 매트릭스

이 표를 머지 게이트로 사용합니다.

| Check | 왜 중요한가 |
| --- | --- |
| `make evm-conformance` | fork rule과 execution regression을 잡음 |
| `go test ./modules/evm -count=1` | receipts, logs, storage, balances, snapshots 검증 |
| `go test ./rpc -count=1` | Web3 request/response 호환성 검증 |
| `make network-e2e` | 노드가 여전히 시작되고 peer를 맺고 commit 하는지 확인 |
| Docker single-host smoke | Remix와 브라우저 도구가 쓰는 경로 확인 |
| Contract deploy | transaction admission과 receipt generation 확인 |
| Proxy deploy | ABI와 storage layout 가정 확인 |
| UUPS upgrade | upgrade semantics와 upgrade 후 read 확인 |

어느 하나라도 빨간색이면 업데이트 완료라고 하면 안 됩니다.

## 롤백 기준

다음 중 하나라도 생기면 EVM 업데이트를 되돌립니다.

- `eth_chainId`가 예상치 않게 바뀜
- `eth_sendRawTransaction`이 유효한 트랜잭션을 거부하기 시작함
- `eth_call` 또는 `eth_estimateGas`가 예상 fork rule과 달라짐
- receipts, logs, proofs가 committed state와 더 이상 맞지 않음
- proxy 또는 upgrade 트랜잭션이 실패하기 시작함
- release evidence가 현재 코드 경로와 맞지 않음

롤백은 마지막으로 정상 확인된 adapter version, config default, fixture set을 함께 복구해야 합니다.

## 기술 동등성 부록

이 부록은 업데이트 가이드를 다른 문서와 같은 기준에 맞추기 위한 것입니다.

- `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc`, `cmd/vexod`는 안정적인 구현 경계로 유지합니다.
- `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction`, `eth_sendTransaction`의 철자는 바꾸지 않습니다.
- `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures`, `--evm-web3-conformance-evidence`의 철자도 그대로 유지합니다.
- 운영 질문은 단순해야 합니다. 이번 업데이트가 Ethereum-style execution을 유지하면서도 Vexo consensus와 release safety에 맞는가?

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
