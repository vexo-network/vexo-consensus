> Locale: ko · 한국어

# 앱 모듈 가이드

## 목표

이 가이드에서는 Vexo에 애플리케이션 모듈을 추가하는 방법을 설명합니다.

## 여기서 시작하세요

처음으로 모듈을 추가하는 경우 다음 순서로 섹션을 읽으세요.

1. 모듈 인터페이스
2. 거래 라우팅
3. 모듈 구성
4. 상태 및 사건
5. 제네시스 및 앤티 처리
6. CLI 명령 및 테스트

이 순서는 일반적으로 수행해야 하는 작업과 일치합니다. 즉, 모듈 모양을 정의하고, 트랜잭션을 수신하는 방법을 결정하고, 소유한 상태를 결정한 다음, CLI를 가르치고 구동 방법을 테스트합니다.

## 모듈 인터페이스

`app.Module` 구현:
```go
type Module interface {
    Name() string
    InitGenesis(ctx app.Context, genesis app.GenesisState) error
    BeginBlock(ctx app.Context, header types.Header) error
    DeliverTx(ctx app.Context, tx types.Tx) types.Result
    EndBlock(ctx app.Context) error
}
```
선택적 인터페이스:

- 모듈 쿼리의 경우 `app.QueryHandler`
- 검증인 세트 업데이트를 위한 `app.ValidatorUpdateProvider`
- 결정론적 트랜잭션 이벤트 방출을 위한 `app.TxEventEmitter`
- 노드 보존 정리를 따라야 하는 모듈 소유 인덱스, 캐시 및 기록 스냅샷의 경우 `app.PruneHook`
- 앱 CLI 명령 트리를 통한 모듈 CLI 명령 제공자

## 거래 라우팅

기본 라우팅 모델은 접두사 기반입니다.
```text
<module>:<action>:<args...>:fee=<fee>:gas=<gas>:signer=<signer>:nonce=<nonce>
```
`bank`이라는 모듈은 `bank:`로 시작하는 페이로드를 수신합니다.

## 모듈 구성

활성화된 모듈은 `config.json`이 아닌 노드 홈의 `module_config.json`에서 구성됩니다.
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  }
}
```
`config.json`은 `module_config_path`을 통해 사용자 정의 모듈 파일을 가리킬 수 있습니다. `module_config.json`에 모듈 기본값, 모듈 활성화, 실행 정책 및 거버넌스 정책을 유지하면 애플리케이션 개발자가 `network_config.json`, `consensus_config.json`, `mempool_config.json` 또는 `log_config.json`을 건드리지 않고도 모듈 동작을 변경할 수 있습니다.

## 상태

모듈은 네임스페이스 KV 저장소인 `app.Context.Store`을 수신합니다. 모듈에 그렇게 하지 말아야 할 더 강력한 이유가 없는 한 모듈 이름을 네임스페이스로 사용하십시오.

모든 저장소, 암호화 서명자, 원격 서명자, 쿼리 및 장기 실행 작업에 `ctx.GoContext()`을 사용하세요. 이제 런타임은 컨텍스트 인식 `CheckTx`, `PrepareProposal`, `ProcessProposal`, `FinalizeBlock` 및 `Query` 경로를 노출하므로 취소 및 차단/RPC 기한이 백그라운드에서 계속되는 대신 모듈 코드로 전파될 수 있습니다.

체인 전체 모듈 매개변수의 경우 임시 모듈 키 대신 `params` 키퍼를 선호합니다.
```go
keeper := params.NewKeeper(ctx.Store)
_, err := keeper.Set(ctx.GoContext(), params.Change{
    Authority: "governance",
    Module: "staking",
    Key: "max_validators",
    Value: []byte("100"),
})
```
내장된 `params` 모듈은 `params:set:<authority>:<module>:<key>:<base64-value>` 트랜잭션 및 `params/param/<module>/<key>` 쿼리를 지원합니다.

## 이벤트 및 쿼리 증명

모듈은 `app.TxEventEmitter`을 구현하여 색인 생성 가능한 이벤트를 내보낼 수 있습니다. 런타임은 성공적인 모듈 실행 결과가 생성된 후 이를 호출하고, 내보낸 이벤트를 복사하며, 런타임이 KV 스토어에서 지원될 때 `events.Indexer`를 통해 인덱싱된 속성을 유지합니다.
```go
func (module Module) Events(ctx app.Context, tx types.Tx, result types.Result) []events.Event {
    if result.Code != 0 {
        return nil
    }
    return []events.Event{{
        Type: "transfer",
        Attributes: []events.Attribute{
            {Key: "sender", Value: "alice", Index: true},
            {Key: "recipient", Value: "bob", Index: true},
        },
    }}
}
```
이벤트를 결정적으로 유지합니다. 동일한 블록과 트랜잭션은 모든 노드에서 동일한 이벤트 유형, 속성 및 인덱스 플래그를 내보내야 합니다.

상태 루트 바인딩 쿼리의 경우 `queryproof.Build` 및 `queryproof.Verify`을 사용하여 체인 ID, 높이, Merkle 상태 루트, 결정적 리프 해시 및 컴팩트 멤버십 경로 또는 컴팩트 왼쪽/오른쪽 이웃 부재 증명으로 네임스페이스/키/값 조회를 래핑합니다. 이는 Cosmos IAVL이 아닌 Vexo의 기본 상태 증명 형식입니다.

CLI는 동일한 Merkle 쿼리 방지 봉투를 노출합니다.
```bash
vexod proof query --home .vexo --namespace bank --key alice > proof.json
vexod proof verify --input proof.json --chain-id vexo-chain --height 10
```
데이터 가용성 약정은 정식 트랜잭션 청크를 사용합니다. 운영자 및 모듈 테스트 하네스는 DA 번들을 내보내고, 개별 청크 증명을 확인하고, 결정적 청크 샘플을 계획하고, 제한된 Reed-Solomon 스타일 복구를 테스트할 수 있습니다.
```bash
vexod proof da-export --tx-hex 68656c6c6f --tx-hex 776f726c64 --data-shards 4 --parity-shards 2 > da-bundle.json
vexod proof da-proof --tx-hex 68656c6c6f --tx-hex 776f726c64 --index 0 > da-proof.json
vexod proof da-verify --input da-proof.json
vexod proof da-sample --input da-bundle.json --chain-id vexo-chain --height 10 --samples 8 --min-samples 4 > da-samples.json
vexod proof da-recover --input da-bundle.json --drop 0 --drop 1
```
## IBC 및 계약 연장 지점

`ibc` 패키지는 IBC 호환 모듈 구축을 위한 클라이언트, 연결, 채널, 순서/비순서 채널 유효성 검사, 패킷 커밋, 승인, 시간 초과, 수신, 증명 확인, 클라이언트 동결 및 신뢰 기간 만료 기본 요소를 제공합니다. 완전한 타사 중계기 생태계 호환성은 체인 통합 작업으로 유지됩니다.

CLI에서 패킷 스캐폴드를 생성할 수 있으며 체인별 IBC 모듈은 패킷 약속을 다음 상태로 연결합니다.
```bash
vexod ibc tx client-create 07-vexo-0 counterparty 10 <validator-set-hash> <state-root> --signer relayer
vexod ibc tx client-update 07-vexo-0 11 <validator-set-hash> <state-root> [proof_json_base64] --fee 1 --gas 1000 --signer relayer --nonce 1
vexod relayer client-update --source-rpc 127.0.0.1:26657 --rpc 127.0.0.1:27657 --client-id 07-vexo-0 --fee 1 --gas 1000 --signer relayer --nonce 1 --submit
vexod ibc tx connection-open-init connection-0 07-vexo-0 connection-1 --fee 1 --gas 1000 --signer relayer --nonce 1
vexod ibc tx connection-open-ack connection-0 --fee 1 --gas 1000 --signer relayer --nonce 2
vexod ibc tx channel-open-init transfer channel-0 connection-0 channel-1 ordered --fee 1 --gas 1000 --signer relayer --nonce 3
vexod ibc tx channel-open-ack transfer channel-0 --fee 1 --gas 1000 --signer relayer --nonce 4
vexod ibc tx packet-send 1 transfer channel-0 transfer channel-1 payload --fee 1 --gas 1000 --signer relayer --nonce 1
vexod ibc tx packet-ack 1 transfer channel-0 transfer channel-1 payload ack --fee 1 --gas 1000 --signer relayer --nonce 2
vexod ibc tx packet-timeout 1 transfer channel-0 transfer channel-1 payload 100 --fee 1 --gas 1000 --signer relayer --nonce 3
vexod proof verify-ibc --home .vexo --client-id 07-vexo-0 --input ibc-proof.json
vexod relayer discover --rpc 127.0.0.1:26657 --json
vexod relayer packet-ack --rpc 127.0.0.1:26657 --proof-rpc 127.0.0.1:26657 --sequence 1 --source-port transfer --source-channel channel-0 --destination-port transfer --destination-channel channel-1 --data payload --ack ack --fee 1 --gas 1000 --signer relayer --nonce 2 --submit
vexod relayer loop --mode timeout --rpc 127.0.0.1:26657 --proof-rpc 127.0.0.1:26657 --sequence 1 --source-port transfer --source-channel channel-0 --destination-port transfer --destination-channel channel-1 --data payload --timeout-height 100 --interval 5s --continue-on-error --state relayer_state.json --submit
vexod relayer run --config relayer_config.json
vexod evm tx call evm 0xaaaa 0xbbbb transfer aabb 100000 --fee 1 --gas 100000 --signer 0xaaaa --nonce 1
vexod evm query storage 0xcontract 0x0
vexod evm query logs
vexod evm query logs 0xcontract
curl -s -X POST http://127.0.0.1:26657/ -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'
vexod ibc packet send \
  --sequence 1 \
  --source-port transfer \
  --source-channel channel-0 \
  --destination-port transfer \
  --destination-channel channel-1 \
  --data payload
```
계약 VM 어댑터는 `contract.Result`을 반환합니다. 기본 `evm` 어댑터는 go-ethereum의 EVM 인터프리터에 의해 지원됩니다. 서명된 Ethereum 원시 트랜잭션과 비지속적인 Web3 호출 시뮬레이션은 go-ethereum `ApplyMessage`를 통해 실행되는 반면, 명시적인 CREATE2 모듈 배포는 어댑터 VM 경계를 사용합니다. Geth 관련 해석기 코드는 `modules/evm/backend/geth` 아래에 격리되어 있고 서명된 Ethereum 트랜잭션 디코딩과 Ethereum 트랜잭션/수신/상태 트리 로직은 `modules/evm/ethcompat` 아래에 격리되어 있으므로 geth API 변경 사항은 해당 호환성 패키지 및 적합성 테스트에 포함되어야 합니다. `evm state-backend` 쿼리는 연결된 `github.com/ethereum/go-ethereum` 모듈 버전/체크섬을 노출하므로 운영자는 업그레이드 증거를 고정할 수 있습니다. 내장된 적합성 명령은 릴리스 증거가 전달되기 전에 호출 반환 데이터, 계약 생성, CREATE2, 되돌리기 동작, 영구 저장소 쓰기, 이벤트 로그, 값 전송, 사전 컴파일 실행, 액세스 목록 가스 의미 체계 및 blob-hash 의미 체계를 다루는 원시 트랜잭션 고정 장치와 실제 geth 실행 고정 장치를 모두 실행합니다. 외부 말뭉치는 `--evm-tx-fixtures-sha256` 또는 `--evm-execution-fixtures-sha256`로 고정되어야 합니다. `eth_sendRawTransaction`는 서명된 Ethereum 레거시/액세스 목록/동적 수수료/blob 유형 트랜잭션을 허용하고, 발신자 복구 및 체인 ID를 확인하고, `execution.allow_unprotected_legacy_tx`이 명시적으로 활성화되지 않은 한 보호되지 않은 Homestead 레거시 트랜잭션을 거부하고, Ethereum 트랜잭션 해시를 보존하고, 호출 또는 계약 생성을 Vexo 표준 `evm` 트랜잭션에 매핑합니다. Ethereum 계약 생성은 명시적인 솔트가 제공되지 않는 한 표준 발신자/nonce CREATE 주소를 사용합니다. 이 경우 geth 어댑터는 CREATE2를 사용합니다. EVM 값과 계정 잔액은 실행, 영수증, 계정 쿼리, 스냅샷 및 `eth_getProof` 전반에 걸쳐 서명되지 않은 256비트 Ethereum 의미를 보존합니다. 256비트보다 큰 값은 페일클로즈됩니다. EVM 모듈은 런타임 코드, VM `CodeWrites`, `StorageWrites` ~ `evm/storage/{address}/{slot}`, `NonceWrites`를 정식 계정 시퀀스 네임스페이스에 유지하고, `AccountDeletions`을 `SELFDESTRUCT`/빈 계정 마무리에서 유지하고, VM 잔액을 공유 `bank` 네임스페이스에 기록합니다. 트랜잭션 해시별 영수증, 트랜잭션 해시별 영수증 위치 인덱스, 높이/트랜잭션/로그 인덱스 + 주소별 로그, 보조 `evm_ethstate/{height}` 네임스페이스 아래 높이 인덱스 이더리움 계정 스냅샷. 다중 키 EVM 쓰기에는 `BatchKVStore`이 필요합니다. 원자 배치를 적용할 수 없는 사용자 지정 저장소는 코드, 저장소, 잔액, 영수증, Blob 사이드카 또는 인덱스를 부분적으로 작성하는 대신 페일클로즈됩니다. 정리는 기록 EVM 스냅샷과 정리 가능한 영수증 인덱스, 영수증, 로그 및 유지된 높이 아래의 Blob 사이드카 인덱스를 제거합니다. 영수증에는 실제 VM 쓰기에서 생성된 영수증 지원 `state_diff`이 포함되며 내장 geth 어댑터는 `vm_trace` 아래에 구조체 로거 opcode 추적을 저장합니다. Web3 재생 방법은 해당 값을 `stateDiff` 및 `vmTrace`로 노출합니다. 계정 삭제는 코드/스토리지/잔고/nonce 쓰기 후에 적용되므로 삭제된 계정이 오래된 시퀀스 상태를 남기지 않습니다. 이더리움 16진수 계정 키는 은행 읽기/쓰기 전에 정규화되므로 동일한 20바이트 주소의 체크섬 및 소문자 형식이 하나의 계정으로 확인됩니다. 모듈은 최신 `eth_getProof`에 대한 커밋된 은행/인증/EVM 상태와 기록 `eth_getProof`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getCode`, `eth_getStorageAt`에 대한 보관된 스냅샷에서 go-ethereum 호환 계정/스토리지 MPT를 재구성합니다. `eth_call` 및 Web3 `stateRoot`. 호출 쿼리는 블록 높이, 기본 수수료, Blob 기본 수수료, 호출자 제공 수수료 필드, EIP-7702 인증 목록 데이터, 유지된 스냅샷 상태, 엄격하게 검증된 선택적 상태 재정의 및 선택적 블록 재정의를 VM 호출에 전달하고, Ethereum 호출 의미 체계를 사용하여 geth 상태 전환 시뮬레이션 경로를 통해 실행하고, `STATICCALL`를 강제하는 대신 시뮬레이션 후 쓰기를 삭제합니다. 가스 가격과 수수료 한도를 생략한 통화는 가스 가격이 없고 기본 수수료 사전 확인 없이 시뮬레이션됩니다. 명시적인 수수료 필드를 제공하는 호출은 여전히 ​​geth 수수료 상한선 유효성 검사를 사용합니다. `eth_estimateGas`는 구성된 geth `params.ChainConfig` 또는 내장/플로어 데이터 가스 규칙에 대한 포크 사전 설정을 사용하므로 geth 버전을 변경하려면 앱 모듈을 다시 작성하는 대신 격리된 호환성 패키지 및 테스트를 업데이트해야 합니다. Txpool RPC는 연속 실행 가능한 발신자 nonce를 `pending`로 분할하고 향후 nonce 간격을 `queued`로 분할합니다. 이렇게 하면 `eth_getBalance`, `eth_getProof`, `eth_getCode`, `eth_getStorageAt`, `eth_call`, `eth_estimateGas`, `eth_createAccessList`, `eth_getTransactionReceipt`, `eth_getBlockReceipts`, 프로세스 메모리 대신 커밋된 모듈 상태로 지원되는 `eth_getTransactionByHash`, 주소 범위 `eth_getLogs` 및 전역 `eth_getLogs`입니다.

최소 `relayer_config.json`:
```json
{
  "schema_version": "v1",
  "jobs": [
    {
      "name": "timeout-transfer",
      "mode": "timeout",
      "rpc": "127.0.0.1:26657",
      "proof_rpc": "127.0.0.1:26657",
      "submit": true,
      "state_path": "relayer_state.json",
      "interval": "5s",
      "failure_backoff": "30s",
      "continue_on_error": true,
      "packet": {
        "sequence": 1,
        "source_port": "transfer",
        "source_channel": "channel-0",
        "destination_port": "transfer",
        "destination_channel": "channel-1",
        "data": "payload",
        "timeout_height": 100
      }
    }
  ]
}
```
Relayer 쪽 읽기는 RPC를 통해 사용할 수 있습니다.
```bash
curl 'http://127.0.0.1:26657/v1/ibc/client/07-vexo-0'
curl 'http://127.0.0.1:26657/v1/ibc/packet/1/transfer/channel-0/transfer/channel-1'
curl 'http://127.0.0.1:26657/v1/ibc/proof/packet/1/transfer/channel-0/transfer/channel-1'
```
IBC 클라이언트는 상대방 최신 높이, 검증자 설정 해시 및 상태 루트로 업데이트될 수 있습니다. `ibc query capabilities`은 모듈이 Vexo 네이티브이고 `vexo-queryproof`를 사용하며 Cosmos ICS 유선 호환이 아님을 보고하는 `ibc/capabilities`을 반환합니다. `client-create`이 `--authority` 또는 `--signer`와 함께 제출된 경우 해당 값은 클라이언트 권한으로 저장되며 이후 `client-update` 트랜잭션은 동일한 권한/서명자를 전달해야 합니다. 승인되지 않은 릴레이어 업데이트는 거부됩니다. 클라이언트에 권한이 없는 경우 `client-update`에는 `proof_json_base64`이 포함되어야 하며 키퍼는 새 헤더를 수락하기 전에 신뢰할 수 있는 클라이언트에 대해 증명 네임스페이스, 키, 체인 ID, 높이, 상태 루트 및 값을 확인합니다. 중계자는 `relayer client-update --source-rpc`을 사용하여 상대방 `/v1/state/latest` 엔드포인트에서 해당 필드를 가져온 다음 생성된 업데이트를 대상 체인에 제출할 수 있습니다. 연결 및 채널은 패킷 흐름 전에 init/try/ack/confirm 핸드셰이크 상태를 지원합니다. 패킷 수신에는 확인 및 시간 제한 수명 주기 필드가 있으므로 중계자는 패킷 전송 제출, 수신 상태 관찰, RPC 이벤트 인덱스에서 패킷 전송 이벤트 검색, 패킷 승인 또는 패킷 시간 초과 제출, 특정 높이의 패킷 커밋 키에 대한 IBC 네임스페이스 증명 가져오기, 네임스페이스/키/값 확인을 통해 신뢰할 수 있는 로컬 IBC 클라이언트에 대한 증명 확인, 선택적으로 RPC를 통해 구축된 중계기 트랜잭션 제출, 제한된 또는 연속 폴링 루프 실행, 중계 체크포인트 유지 등을 수행할 수 있습니다. 다시 시작한 후 중복 제출을 방지하고 JSON 구성 파일에서 여러 릴레이 작업을 관리합니다. Relayer 루프는 반복, 증명 오류, 제출 오류, 제출된 개수, 체크포인트 건너뛰기를 포함한 작업별 측정항목을 인쇄합니다. `failure_backoff`을 사용하면 운영자는 일반적인 성공 폴링 간격을 변경하지 않고도 실패 증명 또는 제출 후 재시도 속도를 늦출 수 있습니다. Ack-with-proof에는 승인 바이트가 제출된 ack와 일치하는 상대방 영수증 증명이 필요합니다. timeout-with-proof는 부재 증명 또는 확인되지 않은 영수증 증명을 허용하고 확인된 영수증 증명을 거부합니다. 시간 초과 높이 스위핑은 높이 인덱스 패킷 시간 초과 인덱스를 사용하므로 현재 높이에서 만료되는 패킷만 검색합니다. 저장소는 전체 네임스페이스 스캔과 부분 시간 제한 인덱스 쓰기를 방지하기 위해 IBC 모듈에 대한 접두사 읽기 및 원자 배치를 노출해야 합니다.

`contract` 패키지는 EVM/WASM 호환 모듈에 대한 VM 레지스트리 및 호출 경계를 제공합니다. `evm` 모듈은 계약 코드, 실행 영수증, 스토리지 슬롯, 로그, VM 코드 쓰기, VM nonce 쓰기, VM 계정 삭제 및 VM 잔액 쓰기를 저장하고 RPC 서버는 `rpc_modules`, `vexo_web3Capabilities`, `web3_clientVersion`, `web3_sha3`과 같은 Web3 JSON-RPC 브리지 메서드를 노출합니다. `net_version`, `net_listening`, `net_peerCount`, `eth_chainId`, `eth_protocolVersion`, `eth_syncing`, `eth_mining`, `eth_hashrate`, `eth_accounts`, `eth_coinbase`, `eth_blockNumber`, `eth_getBlockByNumber`, `eth_getBlockByHash`, `eth_getBlockTransactionCountByNumber`, `eth_getBlockTransactionCountByHash`, `eth_getTransactionByBlockNumberAndIndex`, `eth_getTransactionByBlockHashAndIndex`, `eth_getUncleCountByBlockNumber`, `eth_getUncleCountByBlockHash`, `eth_getUncleByBlockNumberAndIndex`, `eth_getUncleByBlockHashAndIndex`, `eth_gasPrice`, `eth_blobBaseFee`, `eth_maxPriorityFeePerGas`, `eth_feeHistory`, `eth_getBalance`, `eth_getTransactionCount`, `eth_getProof`, `eth_getCode`, `eth_getStorageAt`, `eth_sendRawTransaction`, `eth_sendTransaction`, `eth_signTransaction`, `eth_sign`, `personal_sign`, `eth_getTransactionReceipt`, `eth_getBlockReceipts`, `eth_getTransactionByHash`, `eth_getRawTransactionByHash`, `eth_getRawTransactionByBlockNumberAndIndex`, `eth_getRawTransactionByBlockHashAndIndex`, `eth_pendingTransactions`, `eth_getLogs`, `eth_newFilter`, `eth_getFilterChanges`, `eth_getFilterLogs`, `eth_uninstallFilter`, `eth_call`, `eth_estimateGas`, `eth_createAccessList`, `txpool_status`, `txpool_content`, `txpool_contentFrom`, `txpool_inspect`, `debug_traceTransaction`, `debug_traceCall`, `debug_traceBlockByNumber`, `debug_traceBlockByHash`, `trace_call`, `trace_transaction`, `trace_get`, `trace_block`, `trace_filter`, `trace_replayTransaction` 및 `trace_replayBlockTransactions`. 체인은 `evm`이라는 내장 geth 지원 어댑터를 사용하거나, `modules/evm/backend/geth` 및 `modules/evm/ethcompat`을 검증하여 geth를 업데이트하거나, 실행 측면을 사용자 정의 `contract.VM` 어댑터로 바꿀 수 있습니다. Web3 조회 경로는 보류/대기 중인 트랜잭션, 유지된 블록 트랜잭션, 유지된 기록 계정/코드/스토리지 스냅샷, 영수증, 결정적 영수증 지원 추적, 폴링 필터 및 WebSocket 구독을 정렬하여 Ethereum 도구가 커밋된 블록 레코드에서 보조 영수증 인덱스를 다시 작성해야 하는 경우에도 커밋 전후에 트랜잭션을 검색할 수 있도록 합니다.

모듈이 자체 모듈 네임스페이스 외부에 쓰는 경우 `app.ReplayNamespaceProvider`을 구현하세요. 기록 격리 재생은 이후 블록을 다시 실행하기 전에 유지된 기본 높이에서 모듈 이름과 선언된 모든 재생 네임스페이스를 가져옵니다. 내장된 EVM 모듈은 `evm`, `evm_ethstate`, `bank` 및 `auth`을 선언하므로 계약 상태, 보유된 이더리움 스냅샷, 기본 잔고 및 이더리움 계정 논스가 함께 재생됩니다.

내장된 스테이킹 모듈에는 위임, 위임 취소, 성숙된 본딩 해제 철회, 감옥 해제, 검증인 수수료, 수수료 보상 분배, 보상 쿼리, 보상 청구 및 스테이킹 원장 슬래싱이 포함됩니다. 앤티 레이어에 의해 구성된 수수료 수집기로 수집된 수수료는 엔드 블록에서 검증인 전력에 의해 분배된 다음 검증인 커미션 이후 위임자 스테이크에 의해 분배됩니다. 다중 키 쓰기를 스테이킹하려면 원자적 배치 저장소가 필요합니다. 이를 통해 잔액, 지분, 유효성 검사기 권한, 보상 상태 및 결합 해제된 보관권이 사용자 지정 스토리지 백엔드에서 부분적으로 업데이트되는 것을 방지할 수 있습니다. 은행 잔고는 EVM/네이티브 256비트 형식으로 저장될 수 있지만 스테이킹 금액과 검증인 투표권은 의도적으로 `uint64`에 제한됩니다. 위임은 해당 스테이킹 도메인에서 표시할 수 없는 잔액을 자르는 대신 거부합니다. 위임 취소는 항목 기반 보관 기록으로 추적되므로 동일한 위임자/검증인에 대한 여러 위임 취소는 독립적으로 성숙될 수 있으며 `withdraw-unbonded`은 릴리스 높이가 지난 항목만 릴리스합니다. 합의 삭감이 페널티 영수증을 적용하는 경우 런타임은 스테이킹에 해당 검증자에 대한 위임을 비례적으로 줄이도록 요청하고 멱등성 표시를 작성하므로 조정 재개 시 동일한 증거가 두 번 삭감되지 않습니다.
```bash
vexod staking tx set-commission validator-1 500 --signer validator-1
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
vexod staking tx claim-rewards alice validator-1 --fee 1 --gas 1000 --signer alice --nonce 2
vexod staking query unbonding alice validator-1
vexod staking query unbonding-balance alice validator-1
vexod staking tx withdraw-unbonded alice validator-1 --fee 1 --gas 1000 --signer alice --nonce 3
vexod governance tx submit-json '{"submitter":"alice","title":"multi-change","description":"raise throughput safely","metadata_uri":"ipfs://proposal","type":"parameter_change","deposit":"100avxo","changes":[{"module":"execution","key":"max_gas","value":"20000000"},{"module":"mempool","key":"max_txs","value":"50000"}]}'
```
`deposit`은 스토어 지원 런타임의 제안 메타데이터만이 아닙니다. 거버넌스 모듈은 이를 `module_config.json:governance`에 대해 검증하고, 제출자로부터 기본 은행 잔액을 에스크로하고, 제안이 성공적으로 실행되면 이를 환불하고, 실행이 거부된 제안을 해결하면 구성된 거부 예금 모듈 계정으로 이동합니다. 일반적인 블록 실행은 단계적 저장소를 사용하기 때문에 제안서 상태와 은행 보관소는 블록과 함께 원자적으로 커밋을 씁니다.

중요한 거버넌스 정책 분야:

- `RequireDeposit`: true인 경우 보증금이 없는 제안을 거부합니다.
- `MinDeposit`: 최소 기본 금액(예: `1avxo` 또는 `0.01vexo`).
- `DepositDenom`: 사람이 읽을 수 있는 예금에 대해 허용되는 접미사; 원시 숫자는 원자 단위로 해석됩니다.
- `DepositEscrow`: 투표/집행이 보류되는 동안 예금을 보유하는 모듈 계정입니다.
- `RejectedDeposits`: 거부된 제안서에 첨부된 보증금의 대상입니다.

두 거래 형태 모두 예금을 지원합니다.
```bash
vexod governance tx submit alice max-gas execution max_gas 20000000 100avxo
vexod governance tx submit-json '{"submitter":"alice","title":"multi-change","deposit":"100avxo","changes":[{"module":"execution","key":"max_gas","value":"20000000"}]}'
```
맞춤형 모듈은 제안서 입금액 자체를 인출해서는 안 됩니다. 거버넌스 모듈이 에스크로/환불/슬래시를 처리하도록 하고, 집계 및 시간 잠금 확인 후 거버넌스가 실행할 수 있는 결정론적 매개변수 변경 또는 모듈 메시지만 노출합니다.

## 제네시스

`InitGenesis`은(는) `app.GenesisState`에서 모듈별 생성 값을 받습니다. 기존 은행 제네시스 키는 `bank:<address>`를 사용합니다.

## 앤티 처리

모듈은 수수료 단위, 기본 수수료, nonce, 가스 한도 분석 또는 서명 확인을 다시 구현해서는 안 됩니다. 이는 앤티 레이어에 속합니다.
트랜잭션을 실행하는 모듈은 `EstimateGas(ctx, tx)`을 구현하고 `DeliverTx` 내에서 `ctx.ConsumeGas(amount)`을 호출해야 제안 수락 전과 실행 중에 언더가스 트랜잭션이 실패합니다.

## CLI 명령

모듈은 다음을 사용하여 구조화된 CLI 명령을 노출해야 합니다.

- 명령 이름
- 사용법
- 설명
- 인수
- 깃발
- 예
- 자식 명령

CLI는 로컬 상태 변경을 직접 실행하지 않고 트랜잭션 페이로드를 생성해야 합니다.

## 테스트

모든 모듈은 다음을 테스트해야 합니다.

- 제네시스 초기화
- 유효한 거래와 유효하지 않은 거래
- 쿼리 응답
- 앤티 호환성
- 결정론적 상태 뿌리
- 유효성 검사기 업데이트(있는 경우)

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Goal — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Module Interface — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Transaction Routing — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Module Configuration — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: State — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Events and Query Proofs — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: IBC and Contract Extension Points — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Genesis — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Ante Handling — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: CLI Commands — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Tests — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `app.Module` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `app.QueryHandler` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `app.ValidatorUpdateProvider` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `app.TxEventEmitter` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `app.PruneHook` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bank:` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `module_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `module_config_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `network_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `mempool_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `log_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `app.Context.Store` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `ctx.GoContext()` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `params:set:<authority>:<module>:<key>:<base64-value>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `params/param/<module>/<key>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `events.Indexer` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `queryproof.Build` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `queryproof.Verify` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `contract.Result` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `modules/evm/backend/geth` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `modules/evm/ethcompat` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm state-backend` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `github.com/ethereum/go-ethereum` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--evm-tx-fixtures-sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--evm-execution-fixtures-sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_sendRawTransaction` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `execution.allow_unprotected_legacy_tx` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getProof` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm/storage/{address}/{slot}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm_ethstate/{height}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `state_diff` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vm_trace` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getBalance` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getTransactionCount` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getCode` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getStorageAt` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_call` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_estimateGas` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `params.ChainConfig` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_createAccessList` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getTransactionReceipt` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getBlockReceipts` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getTransactionByHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getLogs` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `relayer_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `ibc/capabilities` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo-queryproof` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `client-create` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--authority` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--signer` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `client-update` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `proof_json_base64` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/state/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `relayer client-update --source-rpc` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `failure_backoff` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc_modules` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo_web3Capabilities` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
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
- `eth_accounts` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_coinbase` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_blockNumber` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getBlockByNumber` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getBlockByHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getBlockTransactionCountByNumber` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getBlockTransactionCountByHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getTransactionByBlockNumberAndIndex` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getTransactionByBlockHashAndIndex` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getUncleCountByBlockNumber` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getUncleCountByBlockHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
