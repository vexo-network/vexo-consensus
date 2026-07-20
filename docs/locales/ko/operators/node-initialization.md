> Locale: ko · 한국어

# 노드 초기화

이 가이드에서는 검증인 및 아카이브 노드 홈을 초기화하고, 시작하고, 정상인지 확인하고, 클라이언트를 연결하는 방법을 설명합니다.

피어 연결은 `start` 명령줄에서 반복적으로 전달되지 않고 `network_config.json`에서 구성되어야 합니다.

합의, RPC, P2P, 로깅 또는 관리되는 Web3 계정에 영향을 미치는 런타임 동작은 구성 파일에만 해당됩니다. `vexod start`는 `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env` 및 `--evm-account-key`과 같은 플래그를 거부합니다. 대신 모든 운영자가 동일한 결정적 노드 동작을 검토할 수 있도록 분할 구성 파일을 편집하세요.

노드 모드 스위치가 없습니다. 노드 홈은 구성 파일, 기원, 키 자료, `validator_id` 및 서명자가 있는지 여부에 따라 정의됩니다.

## 당신이 만들고 있는 것

Vexo 노드 홈은 노드를 시작하는 데 필요한 모든 것을 포함하는 디렉터리입니다.
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
중요한 규칙은 간단합니다. 한 번 초기화하고 구성 파일을 편집한 다음 시작하는 것입니다. 쉘 플래그 내에 네트워크 동작을 숨기지 마십시오.

## 5분 로컬 런

다중 호스트 배포를 생각하기 전에 바이너리가 작동하는지 입증하려는 경우 이 흐름을 사용하십시오.
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
다른 터미널에서:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
예상되는 상태 형태:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
빈 블록 생성이 비활성화되면 단일 노드 또는 빈 메모리 풀 실행에서 최신 높이가 0으로 유지될 수 있습니다. 그렇다고 프로세스가 중단되었다는 의미는 아닙니다. 이는 노드가 빈 블록을 생성하지 않음을 의미합니다. 트랜잭션을 추가하거나 다중 검증기 테스트 네트워크를 실행하여 지속적인 커밋을 관찰하세요.

## 4개의 검증인 로컬 네트워크

피어 연결, 제안자 순환, 블록 커밋 로그 및 높이 증가를 원할 때 이 흐름을 사용하세요.
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
유용한 점검 사항:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
`log_config.json`에서 블록 커밋 로깅이 활성화된 경우 유효성 검사기 로그에는 다음과 같은 이벤트가 포함됩니다.
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
다음을 사용하여 생성된 로컬 네트워크를 중지합니다.
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 및 리믹스

Ethereum 스타일 JSON-RPC는 버전이 지정된 Vexo 운영 API 네임스페이스가 아닌 Web3 엔드포인트에 있습니다.

Docker 단일 호스트 유효성 검사기 1의 경우 Remix 사용자 정의 공급자 URL은 다음과 같습니다.
```text
http://127.0.0.1:28657/web3
```
기본 RPC 포트가 있는 직접 로컬 노드의 경우:
```text
http://127.0.0.1:26657/web3
```
Remix가 수행하는 동일한 호출을 테스트하십시오.
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
브라우저에 체인 ID 가져오기 실패 메시지가 표시되면 다음을 순서대로 확인하세요.

1. URL은 Web3 엔드포인트 경로로 끝납니다.
2. 브라우저가 호스트 포트에 도달할 수 있습니다. Docker 예제에서는 `28657`, `28667`, `28677` 및 `28687`을 노출합니다. 컨테이너 내부의 RPC 포트는 여전히 `26657`입니다.
3. RPC 서버가 실행 중입니다. 동일한 호스트 및 포트에서 상태 엔드포인트를 쿼리합니다.
4. CORS는 `network_config.json`/RPC 구성에 의해 허용됩니다. 기본 핸들러는 사용자 정의 CORS 목록이 설정되지 않은 경우 브라우저 프리플라이트를 허용합니다.
5. 체인의 `module_config.json`에 0이 아닌 EVM 체인 ID가 있습니다.

## 검증인 노드

노드가 합의 메시지를 제안하고, 투표하고, 서명하고, 검증인 순환에 참여할 때 `init validator`을 사용하세요.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
이 명령을 실행하기 전에 `VEXO_KEY_PASSPHRASE`을 설정하거나 일회성 로컬 설정을 위해 `--passphrase`을 전달하세요. `--encrypt-keys`는 `validator.key.json`, `node.key.json` 및 `validator.vrf.key.json`을 암호화합니다.

주요 양육권 경험 법칙:

- `validator.key.json`은 합의 제안, 투표, 시간 제한 투표 및 최종 관련 메시지에 서명합니다.
- `node.key.json`은 P2P 핸드셰이크에만 서명합니다. 이는 검증인 합의 키로 재사용되어서는 안 됩니다.
- `validator.vrf.key.json`은 위원회 무작위성을 증명하며 검증인 보관 자료처럼 취급되어야 합니다.
- 퍼블릭 리스너는 암호화된 로컬 키 문서 또는 원격 서명자/KMS 스타일 키 문서를 사용해야 합니다. 노드가 `require_network_safety=true` 동안 공개 RPC 또는 인증된 공개 P2P를 노출하는 경우 시작 시 일반 텍스트 로컬 유효성 검사기 키를 거부합니다.
- 생성된 키는 파일 시스템 모드 `0600`로 작성됩니다. 여전히 수명이 긴 유효성 검사기에는 원격 서명자/KMS를 선호합니다.

BLS 합의 키의 경우:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls`은 `blst-bls12381-minpk-v1` BLS 키 문서를 작성하고 소유 증명을 `genesis.json` 유효성 검사기 메타데이터에 `bls_pop`로 복사합니다.

이로 인해 다음이 생성됩니다.

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json`은 합의 서명자입니다. `node.key.json`은 `network_config.json:p2p.node_key_path`에서 참조하는 P2P 핸드셰이크 서명자입니다. 아카이브 노드와 검증자는 모든 피어에게 검증자 서명 키를 제공하지 않고도 동일한 전송을 사용할 수 있도록 의도적으로 분리되어 있습니다.

구성 기반 네트워킹으로 시작하세요.
```bash
vexod start --home .vexo-validator-1
```
시작한 후 로그를 읽으십시오. 건강한 유효성 검사기는 노드 실행, RPC 청취, P2P 청취 및 블록이 커밋된 후 블록 커밋 이벤트를 방출해야 합니다. 빈 블록 생성이 비활성화된 경우 블록 커밋 로그가 누락되면 단순히 트랜잭션이 없음을 의미할 수 있습니다.

## 아카이브 노드

노드가 체인 데이터를 유지하고, RPC를 노출하고, 피어로부터 동기화하고, 유효성 검사기 서명을 방지해야 하는 경우 `init archive`을 사용하세요.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
이로 인해 다음이 생성됩니다.

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

`validator.key.json`을 생성하지 **않습니다**.

다음으로 시작하세요:
```bash
vexod start --home .vexo-archive-1
```
아카이브 노드는 합의 투표에 서명하지 않습니다. 이는 RPC, 인덱싱, 상태 동기화, 기록 증명 제공 및 정리 유효성 검사기보다 광범위한 쿼리 기록 유지에 유용합니다.

## 분할 구성 파일

노드 홈은 별도의 구성 파일을 사용하므로 운영자는 관련 없는 설정을 혼합하지 않고 하나의 하위 시스템을 편집할 수 있습니다.

- `config.json`에는 노드 ID, 체인 ID, 데이터 경로 및 분할 구성 파일에 대한 포인터가 포함됩니다.
- `module_config.json`에는 애플리케이션 모듈 선택, 실행/전담 정책, 모듈 수준 거버넌스 정책이 포함됩니다.
- `network_config.json`에는 RPC, P2P 노드 ID, 수신/피어/시드 설정, TLS/인증 설정 및 피어 점수 정책이 포함됩니다.
- `consensus_config.json`에는 합의 루프 타이밍, 빈 블록 정책, 암호화 백엔드, VRF, 검증인 승인 및 위원회 정책이 포함됩니다.
- `mempool_config.json`에는 멤풀 크기, 수수료, 우선순위, WAL, 중복 및 TTL 정책이 포함됩니다.
- `log_config.json`에는 로그 형식, 레벨, 블록 커밋 이벤트 로깅, 피어 이벤트 로깅이 포함됩니다.
- `genesis.json`에는 불변의 제네시스 유효성 검사기, 유효성 검사기 메타데이터 및 제네시스 모듈 상태가 포함되어 있습니다.

`network_config.json` RPC 설정에는 `shutdown_timeout`, `web3_max_subscriptions_per_connection` 및 `web3_idle_timeout`도 포함됩니다. `shutdown_timeout`은 합의 루프, RPC 서버 및 노드 전송에 대한 정상적인 종료를 제한하므로 운영자는 정지 경로에서 영원히 기다리지 않습니다. 생성된 기본값은 `10s`입니다. Web3 구독은 기본적으로 `2m` 유휴 시간 제한을 사용하여 연결당 256개이므로 공용 RPC 엔드포인트는 무제한 유휴 구독을 누적할 수 없습니다.

`network_config.json` P2P 설정에는 `auth_replay_path`, `require_auth_replay_store` 및 `dial_timeout`이 포함됩니다. 생성된 기본값은 nonce 재생 증거를 `data/p2p_auth_replay.jsonl`에 기록하고 `10s` 아웃바운드 다이얼 시간 초과를 사용합니다. 개인 루프백 테스트의 경우 재생 저장소는 대부분 무해한 장부입니다. 공개 인증된 P2P의 경우 다시 시작한 후 캡처된 서명된 핸드셰이크 임시 값이 재생되는 것을 방지하기 때문에 안전 요구 사항입니다. `dial_timeout`은 TLS, 서명된 핸드셰이크 확인 및 지역 간 대기 시간을 위해 충분히 길어야 합니다. 너무 낮게 설정하면 건강한 피어가 불안정해 보이고 다시 시작한 후 활성 상태가 느려질 수 있습니다.

`network_config.json`은(는) 시작 상태 동기화도 소유합니다. 이는 아카이브 노드, 교체 유효성 검사기 또는 깨끗한 시스템에 복원된 노드에 유용합니다. `state_sync.enabled`가 true인 경우 `vexod start`은 `state_sync.snapshot_urls`에서 첫 번째 유효한 스냅샷을 다운로드하고 체인 ID, 체크섬, 상태 루트 및 KV 네임스페이스를 확인하고 이를 LevelDB로 복원하고 인덱스를 다시 빌드한 다음 노드를 시작합니다. 로컬 상태가 이미 `state_sync.min_height`을 충족하고 `state_sync.trust_local_higher`이 true인 경우 시작 시 `state_sync_skipped`을 기록하고 로컬 저장소를 유지합니다.

예 `state_sync` 블록:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
시작 시 가져오기 오류의 경우 `state_sync_candidate_failed`, 유효하지 않거나 오래된 스냅샷의 경우 `state_sync_candidate_rejected`, 확인된 복원 후에는 `state_sync_applied`을 기록합니다. `max_snapshot_bytes`을(를) 인프라가 의도적으로 제공하는 최대 스냅샷보다 낮게 유지하되 정상적인 상태 성장에는 충분히 높게 유지하세요. 운영자가 해당 소스에 대한 대역 외 신뢰 정책 및 최종성/라이트 클라이언트 증거를 갖고 있지 않는 한 인증되지 않은 타사 스냅샷 소스에서 공개 노드를 가리키지 마십시오.

필드가 네트워크 동작을 변경하는 경우 분할 구성 파일을 편집하고 검토된 파일을 커밋하거나 배포합니다. 런타임 동작에 긴 `vexod start` 플래그를 사용하지 마십시오. 시작 명령은 합의 타이밍, 빈 블록, P2P 인증, RPC 관리자 및 관리형 Web3 키 플래그를 의도적으로 거부하므로 운영자가 실수로 검토된 구성과 다른 동작을 실행하지 않습니다.

## 어떤 파일을 편집해야 하나요?

| 목표 | 파일 | 필드 |
|---|---|---|
| RPC 바인드 포트 변경 | `network_config.json` | `rpc.address` |
| P2P 바인드 포트 변경 | `network_config.json` | `p2p.listen_address` |
| 영구 피어 추가 | `network_config.json` | `p2p.peers` |
| 시드 피어 추가 | `network_config.json` | `p2p.seeds` |
| 빈 블록 활성화/비활성화 | `consensus_config.json` | 합의 빈 블록 필드 |
| 합의 시간 초과 조정 | `consensus_config.json` | 제안, 사전 투표, 사전 커밋 및 커밋 시간 제한 필드 |
| 최종 실행 필요 | `consensus_config.json` | 합의 실행-커밋 필드 |
| 모듈 활성화/비활성화 | `module_config.json` | 애플리케이션 모듈 목록 |
| EVM 체인 ID 변경 | `module_config.json` | 실행 EVM 체인 ID 필드 |
| 기본 수수료/가스 | `module_config.json` | 실행 기본 수수료, 동적 수수료, 대상 가스 및 최대 가스 분야 |
| mempool WAL 구성 | `mempool_config.json` | mempool WAL 경로 |
| 블록 커밋 로그 제어 | `log_config.json` | 커밋 이벤트 필드 기록 |
| 피어 로그 제어 | `log_config.json` | 피어 이벤트 필드 로그 |

의심스러운 경우 다음을 실행하세요.
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## 키 유형

네트워크 안전성 검증에는 감사된 BLS 집계 완결성이 필요하기 때문에 검증기 초기화의 기본값은 `--key-type bls`입니다. `--key-type ed25519`은(는) 네트워크 안전 게이트 외부의 비공개 실험 및 사용자 지정 배포에 계속 사용할 수 있습니다. `--encrypt-keys`는 일회용이 아닌 노드 홈에 사용해야 합니다. 독립형 키 생성은 VRF 키도 지원합니다.
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
VRF 키는 합의 서명자가 아닙니다. 이는 VRF 지원 위원회 선택에 사용되며 해당 백엔드가 활성화되면 `consensus_config.json`부터 `vrf_key_paths` 및 유효성 검사기 메타데이터 키 `vrf_public_key`까지 참조되어야 합니다.

`config.json`은 분할 구성 파일을 가리킵니다.
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
각 경로는 절대 경로이거나 노드 홈에 상대적인 경로일 수 있습니다. 생략하면 `vexod`은 기본 `<home>/<name>_config.json` 파일을 사용합니다.

예 `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
거버넌스 정책도 `module_config.json`에 있습니다. 생성된 네트워크 안전 구성에는 제안 보증금이 필요합니다.
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
보증금은 제안서 제출자가 에스크로한 기본 잔액입니다. 통과 제안은 보증금을 환불합니다. 거부된 제안은 `RejectedDeposits`로 이동합니다. 거부된 예금이 기본 모듈 계정 대신 재무부에 자금을 지원해야 하는 경우 재무부/커뮤니티 풀 모듈에서 관리하는 주소를 사용하십시오.

예 `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` 및 `rpc.evm_account_private_keys`은 선택 사항이며 `eth_accounts`, `eth_sign`, `eth_signTransaction` 및 `eth_sendTransaction`와 같은 Web3 관리 계정 방법을 다시 지원합니다. 개인 키가 JSON에 저장되는 대신 프로세스 환경이나 비밀 관리자에 의해 삽입되도록 `evm_account_key_envs`을 선호합니다. 이 노드가 의도적으로 로컬 Web3 핫 지갑 엔드포인트로 작동하지 않는 한 일반적인 유효성 검사기 작업을 위해 두 목록을 모두 비워 두세요. 시작 안전은 공개 RPC 수신기에서 관리되는 EVM 단축키를 거부합니다.

예 `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths`은 `consensus_config.json`이 포함된 디렉터리를 기준으로 확인됩니다. 로컬 VRF 키 보관이 불가피한 경우 암호화된 키 문서를 사용하고 노드 프로세스에 `VEXO_KEY_PASSPHRASE`를 제공합니다. 운영자가 실행하는 네트워크의 경우 원시 VRF 개인 스칼라를 `consensus_config.json`에 직접 넣지 마십시오.

확인된 모든 경로를 검사하려면 `vexod config paths --home <home>`를 사용하세요.

아카이브 구성에는 다음이 포함됩니다.
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
`consensus_config.json` 아카이브는 로컬 합의 루프를 비활성화합니다.
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
생성된 검증자 주택은 기본적으로 `config.json`에 `"require_network_safety": true`로 설정됩니다. 이것은 모드가 아닙니다. 이는 결정적 암호화, 서명되지 않은/비시행된 거래, 수수료/가스 하한 누락, 내구성 있는 mempool WAL 누락, 동일한 서명자/임시 거래에 대한 대체 정책 누락, 안전하지 않은 위원회 무작위성 및 `finalized` 이외의 `execution_commit` 값을 거부하는 시작 안전 게이트입니다.

`require_network_safety`이 활성화되면 다음을 실행합니다.
```bash
vexod config audit --home <home> --strict
```
노드를 시작하기 전에. 동일한 네트워크에 참여하는 모든 검증자와 아카이브 홈에 대해 감사를 통과해야 합니다.

## 구성 기반 피어

피어 및 수신 주소는 `network_config.json`에 있습니다.
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start`은 이러한 피어를 자동으로 로드합니다.
```bash
vexod start --home .vexo-archive-1
```
영구 피어 및 시드는 `network_config.json`에 구성됩니다. `vexod start`은 피어 또는 시드 호스트 재정의를 허용하지 않습니다.

`vexod start` 명령줄에 수명이 긴 호스트 또는 `host:port` 설정을 입력하지 마세요. 대신 `network_config.json`의 `rpc.address`, `p2p.listen_address`, `p2p.peers` 및 `p2p.seeds`을(를) 편집하세요.

노드 홈의 수명 동안 `p2p.node_id`을 안정적으로 유지하세요. `p2p.node_key_path`은 `node.key.json` 또는 피어 핸드셰이크 서명에만 사용되는 다른 로컬/관리 키 문서를 가리켜야 합니다. 피어 맵은 의도적으로 동일하지 않은 한 계정 주소나 유효성 검사기 운영자 이름이 아닌 피어 노드 ID를 사용해야 합니다.

암호화되고 인증된 gRPC 피어 전송의 경우 `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path`를 설정하고 선택적으로 `p2p.tls_server_name`를 `network_config.json`에 설정합니다. 상대 TLS 경로는 노드 홈 디렉터리에서 확인됩니다. 모든 운영자가 동일한 재연결 동작을 사용할 수 있도록 `p2p.dial_timeout`을 동일한 파일에 유지하십시오. 쉘 스크립트에서 피어 타이밍을 숨기지 마십시오.

## 합의 타이밍

합의 루프 타이밍은 `consensus_config.json`에 있습니다.
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose`은 라운드가 제안을 기다리는 시간을 제어합니다.
- `timeout_prevote`은 투표 수집 창을 제어합니다.
- `timeout_precommit`는 커밋 인증서 수집 창을 제어합니다.
- `timeout_commit`은 커밋된 블록 이후의 최소 지연을 제어합니다.
- `create_empty_blocks: false`는 트랜잭션이 가능할 때만 노드가 제안한다는 의미입니다.
- `execution_commit: "finalized"`는 최종 조상을 실행하기 전에 HotStuff 3체인 최종 결정을 기다리며 생성된 검증인 기본값입니다. `execution_commit: "qc"`은 QC 인증 블록을 즉시 실행하고 유지하지만 안전 게이트에서는 이를 거부합니다.

`round_timeout`은 호환성 집계로만 유지됩니다. 위의 Tendermint 스타일 타임아웃 필드를 선호하세요.

`create_empty_blocks`이 false인 경우 mempool이 비어 있는 동안 높이는 변경되지 않고 유지될 수 있습니다. 이는 예상된 결과입니다. 체인은 빈 블록을 커밋하는 대신 유용한 작업을 기다리고 있습니다. 트랜잭션이 나타나고 로컬 합의 라운드 상태가 다른 제안자를 지나 표류하면 노드는 검증자가 제안자가 되고 멤풀에서 구축되는 다음 라운드로 진행됩니다. 이 복구 경로는 빈 블록 스팸을 다시 활성화하지 않고도 트랜잭션으로 인해 트리거된 활성 상태를 유지합니다.

## 다중 검증인 네트워크

생성된 네트워크의 경우:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
생성된 각 검증인 홈은 다음을 수신합니다.

- 자체 `validator.key.json`
- 자체 분할 구성 파일: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` 및 `log_config.json`
- 공유된 `genesis.json`
- 다른 검증인을 위한 `network_config.json` 피어 항목

`vexod network up` 및 `make network-e2e`은 모든 유효성 검사기가 시작되기를 기다리는 동안 프로세스 수준 시간 초과를 사용하고, 연기 트랜잭션을 제출하고, 높이 증가를 관찰합니다. 기본 명령 제한 시간은 프로세스 시작, LevelDB 열기, P2P 서명 핸드셰이크, TLS/인증 확인, 트랜잭션 허용 및 최종성을 포함하므로 합의 간격보다 의도적으로 더 깁니다. 합의 시간 초과를 적극적으로 낮추는 경우 하네스를 너무 일찍 종료하는 대신 시작 오류를 진단할 수 있을 만큼 네트워크 가동 시간 초과를 크게 유지하십시오.

컨테이너화된 네트워크 또는 다중 호스트 네트워크의 경우 JSON 파일에 토폴로지 값을 입력합니다.
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` 및 `rpc_host_template`은 각 노드의 `network_config.json` 피어 목록에 기록된 다이얼 대상입니다. Docker에서는 `validator-%d`과 같은 서비스 이름이 될 수 있습니다.
- `p2p_advertise_host_template` 및 `rpc_advertise_host_template`는 `genesis.json`의 유효성 검사기 메타데이터에 기록된 공개 주소입니다. 공용 네트워크의 경우 여기에서 DNS 이름 또는 공용 IP를 사용하십시오.
- `p2p_listen_host` 및 `rpc_listen_host`은 로컬 바인드 호스트입니다. 모든 인터페이스를 수신해야 하는 컨테이너 또는 서버에는 `0.0.0.0`을 사용하세요.
- 네트워크가 의도적으로 비공개인 경우를 제외하고 Docker 전용 서비스 이름을 광고된 공용 주소로 재사용하지 마십시오.

그런 다음 해당 파일에서 노드 홈을 생성합니다.
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## 문제 해결

| 증상 | 원인일 가능성이 가장 높음 | 확인해야 할 사항 |
|---|---|---|
| `latest_height`이 증가하지 않습니다 | 빈 블록이 비활성화되고 tx가 없으며 온라인 유효성 검사기가 충분하지 않거나 서명자를 사용할 수 없습니다 | `consensus_config.json`, 유효성 검사기 로그, `/v1/diagnostics` |
| `peer_count`은(는) `0`입니다 | 피어 주소에 연결할 수 없거나 잘못된 호스트 이름에 대해 `network_config.json`이 생성되었습니다 | `p2p.peers`, 컨테이너 호스트 포트, DNS, 방화벽 |
| `p2p auth replay store` 오류 | 공개/인증된 P2P에는 내구성 있는 재생 스토리지가 필요합니다 | `p2p.auth_replay_path` 및 집 아래 쓰기 권한 |
| `eth_chainId`이 리믹스에서 실패함 | 잘못된 URL, 잘못된 호스트 포트 또는 브라우저 CORS/프리플라이트가 사용자 정의 구성에 의해 차단됨 | Web3 엔드포인트 URL을 사용한 후 동일한 엔드포인트를 직접 컬링 |
| `config audit --strict` 실패 | 안전 게이트가 안전하지 않은 구성 속성을 발견했습니다 | 실패한 검사를 읽은 다음 이름이 지정된 분할 구성 파일을 편집하십시오. |
| `no block_committed logs` | 로깅이 비활성화되었거나 블록이 생성되지 않습니다 | `log_config.json`, `create_empty_blocks`, 메모리풀 콘텐츠 |
| `managed EVM key rejected` | 핫 개인 키는 공개 RPC 수신기에 구성됩니다. | `evm_account_private_keys` 제거 또는 RPC 비공개 유지 |

## 최소 운영자 체크리스트

다른 기계나 운영자에게 노드 홈을 넘기기 전에 다음을 수행하십시오.

- `vexod validate --home <home>`이 통과되었습니다.
- `vexod config audit --home <home> --strict`이(가) 해당 집에 해당됩니다.
- `config.json`, 분할 구성 파일, `genesis.json` 및 공개 유효성 검사기 메타데이터가 검토됩니다.
- `validator.key.json`, `node.key.json`, `validator.vrf.key.json`는 암호화되거나 원격 서명자/KMS 키 문서로 대체됩니다.
- `network_config.json:p2p.peers`에는 노드가 실제로 Docker 네트워크 내에서 실행되지 않는 한 Docker 전용 이름이 아닌 대상 시스템에서 전화를 걸 수 있는 주소가 포함되어 있습니다.
- `network_config.json` 공용 RPC/P2P 수신기는 `require_network_safety`이 활성화된 경우 TLS 자료를 갖습니다.
- `module_config.json:execution.EVMChainID`은 Web3 지갑이나 Remix 연결 전에 설정됩니다.
- 노드가 다시 시작한 후 보류 중인 tx를 복구해야 하는 경우 `mempool_config.json`에는 WAL 경로가 있습니다.
- `log_config.json`은 네트워크가 가동되는 동안 블록 커밋 및 피어 로그를 활성화합니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Validator Node — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Archive Node — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Split Configuration Files — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Key Types — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Config-Based Peers — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Consensus Timing — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Multi-Validator Network — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `network_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod start` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--timeout-propose` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--create-empty-blocks` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--p2p-auth-token` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--rpc-admin-token` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--evm-account-key-env` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--evm-account-key` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `validator_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `VEXO_KEY_PASSPHRASE` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--passphrase` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--encrypt-keys` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `validator.key.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `node.key.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `validator.vrf.key.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `require_network_safety=true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--key-type bls` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blst-bls12381-minpk-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `genesis.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bls_pop` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `module_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `mempool_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `log_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `data/` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `network_config.json:p2p.node_key_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `shutdown_timeout` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `web3_max_subscriptions_per_connection` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `web3_idle_timeout` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `auth_replay_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `require_auth_replay_store` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `dial_timeout` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `data/p2p_auth_replay.jsonl` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--key-type ed25519` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf_key_paths` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf_public_key` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `<home>/<name>_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.evm_account_key_envs` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.evm_account_private_keys` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_accounts` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_sign` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_signTransaction` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_sendTransaction` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm_account_key_envs` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod config paths --home <home>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `"require_network_safety": true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `execution_commit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `require_network_safety` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `host:port` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.address` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.listen_address` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.peers` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.seeds` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.node_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.node_key_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.tls_cert_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.tls_key_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.tls_ca_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.tls_server_name` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.dial_timeout` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `timeout_propose` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `timeout_prevote` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `timeout_precommit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `timeout_commit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `create_empty_blocks: false` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `execution_commit: "finalized"` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `execution_commit: "qc"` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `round_timeout` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `create_empty_blocks` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod network up` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make network-e2e` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p_host_template` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc_host_template` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `validator-%d` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p_advertise_host_template` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc_advertise_host_template` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p_listen_host` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc_listen_host` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
