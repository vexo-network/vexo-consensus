# 스토리지 스키마

> Locale: ko · 한국어
> 이 문서는 영어 원문의 한국어 직역본입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.


## 먼저 읽을 순서

이 문서는 Storage Schema의 규범 규격을 설명합니다. 처음이라면 아래 순서로 읽는 것이 좋습니다.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

이 순서는 먼저 범위와 상태를 이해하고, 그다음 메시지/안전/진행성 규칙을 확인한 뒤, 마지막으로 증거를 읽는 흐름과 같습니다.

## 문서 개요

이 문서는 durable storage namespace, key schema, recovery marker을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/ko/specs/storage-schema.md`

## 이 문서를 읽는 이유

- durable storage namespace, key schema, recovery marker
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

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## 영어 원문 구조

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## 규범 원문

- [영어 정본 문서](../../en/specs/storage-schema.md)

## State tree

store는 namespace마다 state tree(`stateproof`)를 KV 쌍 옆 `tree:` 아래에 보관합니다. key의 경로는 `sha256(key)`이고, 빈 subtree는 0, key가 하나뿐인 subtree는 그 key의 leaf, 나머지는 두 절반의 internal node입니다. 모양은 key 집합에만 의존하므로 모든 replica가 같은 root를 얻습니다. 쓰기는 같은 원자 batch 안에서 그 key 경로의 노드만 다시 쓰므로, 블록의 state root 비용은 namespace 크기가 아니라 `O(쓰기 수 × 깊이)`입니다. `ImportNamespace`는 그 namespace의 tree를 다시 만듭니다.

## Pebble backend와 바이너리 레코드

기본 backend는 Pebble(`github.com/cockroachdb/pebble/v2`)이며 256 MiB block cache와 64 MiB memtable로 엽니다. 레코드는 `types` 패키지의 canonical 바이너리 인코딩으로 저장됩니다: 최소 uvarint, 길이가 앞에 붙은 바이트열과 리스트, 32바이트 그대로의 해시, `0` 또는 `1` presence byte 뒤의 선택 값, 그리고 선언한 구조체 순서의 필드. 디코더는 뒤에 남는 바이트를 포함해 그 밖의 모든 것을 거부하므로 손상된 레코드는 다른 값으로 읽히지 않고 오류로 보고되며, 레코드의 인코딩은 하나뿐입니다. 이전 빌드(LevelDB, JSON 레코드)가 쓴 데이터 디렉터리는 읽을 수 없고 변환되지 않습니다. Pebble이 `format major version 1` 오류로 열기를 거부하므로, 그런 노드는 snapshot에서 복원하거나 빈 데이터 디렉터리로 동기화하세요.

관련 식별자:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## State history(undo log)와 tree cache

state history는 key별 버전의 undo log입니다. key가 어떤 높이에서 바뀌면(블록의 쓰기는 그 높이, 첫 committed state 이후의 직접 쓰기(`Set`, `Delete`, `SetBatch`)는 다음 committed 높이) store는 key가 그 전에 가졌던 값(없음 포함)을 그 높이의 버전으로 key와 높이마다 한 번 기록합니다. 그 높이를 commit하는 블록보다 먼저 한 직접 쓰기는 첫 번째 것이 기록한 버전을 유지합니다(`hdirect`가 그 높이를 가리킵니다). committed 높이에서 key의 값은 그 높이보다 위의 가장 낮은 버전이 기록한 값이며, 그런 버전이 없으면 현재 값입니다. 변경 기록은 key의 현재 값만 읽고 history는 읽지 않으며, 과거 조회는 key 버전에 대한 seek 한 번입니다. `PruneBelow`는 유지 시작 높이 이하의 버전을 모두 지웁니다. tree node도 같은 방식으로 버전을 남깁니다. 메모리 구조 두 가지가 블록의 tree 작업을 줄이며, 둘 다 영속되지 않고 결과를 바꾸지 않습니다: 현재 tree의 인코딩된 node를 최대 2^18개 담는 tree node cache(store가 commit하는 모든 batch가 갱신하고, batch가 commit되는 동안 홀수인 generation 덕분에 reader는 자기 snapshot의 상태에서만 cache를 씁니다), 그리고 namespace마다 마지막 root 계산의 root와 node 변경을 그 generation과 namespace 쓰기의 SHA-256 digest로 보관해 app hash, state root, commit이 같은 쓰기를 한 번만 계산하게 하는 구조입니다.

## Mempool WAL compaction과 genesis batch

commit은 mempool WAL이 4 MiB 이상이면서 마지막 compaction(또는 열기) 직후 크기의 두 배 이상이 되었을 때만 log를 compaction합니다. 그래서 append된 바이트는 평균 한 번 정도만 다시 쓰이고, 재시작 때 replay하는 log는 live 내용의 두 배 또는 4 MiB 이내로 유지됩니다. 명시적 compaction(`Node.Compact`, 또는 시작 시 application이 거부한 replay tx를 버리는 경우)은 항상 다시 씁니다. `InitChain`은 모든 module의 genesis state를 하나의 batch로 쓰므로 계정이 많은 genesis도 durable write 한 번이면 되고, 실패한 genesis는 store에 아무것도 남기지 않습니다.

## Commit batch 안의 finality proof와 지연된 mempool 제거

node commit 경로는 block의 QC가 완성하는 finality proof를 block/state/app-write batch에 함께 씁니다(`CommitBlockStateWithWrites`의 마지막 인자). 그래서 commit은 proof마다 durable write를 하나씩 더 하지 않고 한 번으로 끝납니다. store가 거부할 proof는 batch에서 빠지고, commit 이후 단계가 저장하면서 실패를 보고합니다. commit은 자기 tx를 pool에서 제거하고 제거 record를 sync를 기다리지 않고 queue에 넣습니다. 다음 commit은 자기 block을 실행하기 전에 그 record가 durable해질 때까지 기다립니다. 재시작은 마지막으로 commit된 block의 제거만 다시 하기 때문입니다. 제거 record는 보통 뒤따르는 admission의 sync에 함께 실립니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Scope — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Backend — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Records — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Indexes — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: EVM Records — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Recovery Rules — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Snapshot Validation — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Schema Migration — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `store.Store` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getBalance` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `eth_getProof` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bank/{0x_address}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `auth/nonce/{0x_address}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm/code/{0x_address}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm/storage/{0x_address}/{slot}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `seen_ttl` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `code/{address}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `storage/{address}/{slot}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `receipts/{tx_hash}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `logs/{address}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `evm_receipts` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `receipt_index/{tx_hash}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blob_sidecars/by_tx/{tx_hash}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blob_sidecars/by_blob/{blob_hash}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `receipt_height/{height}/{tx_hash}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `receipt_pruned_below` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
