# Node Initialization

> Locale: ko · 한국어
> 이 문서는 영어 원문을 함께 읽기 위한 한국어 보조 문서입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

## 문서 개요

이 문서는 archive 노드와 validator 노드 초기화, 분리된 config 파일 운용을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/ko/operators/node-initialization.md`

## 이 문서를 읽는 이유

- archive 노드와 validator 노드 초기화, 분리된 config 파일 운용
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

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
- `--evm-account-key-env`
- `--evm-account-key`
- `validator_id`
- `init validator`
- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `--encrypt-keys`
- `validator.key.json`
- `validator.vrf.key.json`
- `--key-type bls`
- `genesis.json`
- `bls_pop`
- `config.json`
- `module_config.json`
- `consensus_config.json`
- `mempool_config.json`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## 영어 원문 구조

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## 규범 원문

- [영어 정본 문서](../../en/operators/node-initialization.md)
<!-- vexo-docs-ops-update-2026-06 -->

## 최신 운영 메모

새 네트워크 홈은 `network_config.json` 안의 `p2p.dial_timeout`, `p2p.auth_replay_path`, `p2p.require_auth_replay_store`를 함께 검토해야 합니다. 기본 `10s` dial timeout은 TCP 연결, TLS, signed handshake, replay-store 검사를 모두 포함합니다. 공개 네트워크에서는 이 값을 shell flag로 숨기지 말고 config review 대상에 포함하세요.

## 시작 시 State Sync

`network_config.json`의 `state_sync` 블록은 새 archive 노드, 교체 validator, 깨끗한 머신에 복구한 노드가 시작 전에 검증된 snapshot으로 상태를 채우는 경로입니다. `state_sync.enabled`가 true이면 `vexod start`는 `state_sync.snapshot_urls`를 순서대로 시도하고, chain ID, checksum, state root, KV namespace를 검증한 뒤 LevelDB에 복원하고 index를 재생성한 다음 노드를 시작합니다. 로컬 상태가 이미 `state_sync.min_height` 이상이고 `state_sync.trust_local_higher`가 true이면 로컬 store를 유지하며 `state_sync_skipped`를 기록합니다.

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

운영 로그에서 `state_sync_candidate_failed`, `state_sync_candidate_rejected`, `state_sync_applied`를 확인하세요. 공개 네트워크에서는 snapshot 제공자를 신뢰할 수 있는 운영 주체로 제한하고, finality/light-client evidence 없이 임의의 third-party snapshot을 사용하지 마세요.

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
