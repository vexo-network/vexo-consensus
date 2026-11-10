# 합의 규격

> Locale: ko · 한국어
> 이 문서는 영어 원문의 한국어 직역본입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.


## 먼저 읽을 순서

이 문서는 Consensus Spec의 규범 규격을 설명합니다. 처음이라면 아래 순서로 읽는 것이 좋습니다.

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

이 순서는 먼저 범위와 상태를 이해하고, 그다음 메시지/안전/진행성 규칙을 확인한 뒤, 마지막으로 증거를 읽는 흐름과 같습니다.

## 문서 개요

이 문서는 합의 state machine의 규범 규격을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/ko/specs/consensus-spec.md`

## 이 문서를 읽는 이유

- 합의 state machine의 규범 규격
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

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## 영어 원문 구조

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Block Time
- Leader Schedule
- Lanes
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## 규범 원문

- [영어 정본 문서](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## 헤더 app hash

블록 헤더의 `AppHash`는 블록 `StateHeight`를 실행한 뒤의 state를 커밋합니다(그 값이 `0`이면 genesis state). 이는 헤더를 검증하는 모든 validator에게 체인이 커밋을 증명한 가장 최근 블록입니다. 헤더의 `NextValidatorSetHash`는 그 state 이후 registry가 가진 set, 즉 자식 블록을 인증할 set을 가리킵니다. 자식의 `ValidatorSetHeight`는 부모의 `StateHeight + 1`이고 자식의 `ValidatorSetHash`는 부모의 `NextValidatorSetHash`와 같아야 하며, validator는 다음 set이 자기 registry와 일치하는 proposal에만 투표합니다. 값은 application app hash와 그 블록 이후 `validator_registry` 네임스페이스 루트의 `HeaderAppHash`이므로, evidence 판정에 쓰이는 과거 높이의 validator set도 application state와 함께 커밋됩니다. proposer가 이 값을 채우고, validator는 자기 state의 같은 높이 app hash와 일치하는 proposal에만 투표하며, 노드는 `AppHash`가 자기 state와 다른 인증 블록을 실행하지 않습니다. 따라서 인증서는 그 높이의 복제 state 전체를 보증하고, state sync 노드는 snapshot을 이 값과 대조합니다.

다른 app hash를 담은 서명된 proposal을 거부한 validator는 proposer를 `app_hash` 사유의 invalid-proposal evidence로 신고합니다. slashing 모듈은 각 블록 헤더가 커밋한 app hash를 state 높이별로 기록해 두고(evidence 유효 기간 동안 유지), evidence를 이 인증된 값과 대조해 판정합니다. 따라서 모든 노드가 같은 결론을 내리며, 자기 state가 어긋난 노드는 정직한 proposer를 처벌할 수 없습니다.

노드는 복원한 snapshot 아래나 보관 기간(retention) 아래의 state를 갖고 있지 않지만, 헤더는 체인이 커밋을 증명할 때까지 자기 validator set의 state를 계속 가리키므로 그런 state를 가리킬 수 있습니다. 이때 노드는 slashing 모듈이 그 state에 대해 인증된 값으로 기록한 `AppHash`를 사용해, state를 가진 노드와 똑같이 그 헤더에 투표하고 실행합니다.

## 블록 시간

블록의 `TimeUnixNano`는 부모 블록의 시간보다 엄격히 커야 하고, 첫 블록의 시간은 양수여야 합니다. 이를 어긴 제안은 서명이 올바르더라도 무효입니다. 제안자는 자기 시계로 시간을 찍고, 시계가 부모보다 뒤처져 있으면 부모 시간 + 1ns로 올립니다. 검증자는 블록 시간이 자기 시계보다 `MaxBlockTimeDrift`(2초) 넘게 앞서 있으면 투표하지 않습니다. 따라서 인증된 블록의 시간은 정직한 검증자 시계의 drift 범위 안에 있고, 라이트 클라이언트의 trusting period와 EVM `TIMESTAMP`는 이 값을 기준으로 합니다.

## 블록 크기

블록의 트랜잭션은 `consensus.MaxBlockBytes`(2 MiB)를 넘을 수 없으며, 크기는 `consensus.BlockTxBytes`로 셉니다. 이 함수는 트랜잭션마다 `types.TxCost`, 즉 길이에 `types.TxOverheadBytes`(16, 트랜잭션을 싣는 인코딩의 framing)를 더한 값을 부과합니다. 그래서 작은 트랜잭션이 많은 블록도 큰 트랜잭션 몇 개인 블록과 같은 상한 안에서 인코딩되며, 가장 큰 합의 메시지(한도 크기의 전체 제안이나 인증서가 붙은 커밋 블록)는 `transport.MinMessageBytes`(4 MiB)에 들어갑니다. transport는 그보다 작은 메시지 한도를 거부하며 이 값이 기본값입니다. 더 작은 한도의 노드는 유효한 블록을 받을 수 없기 때문입니다. 한도를 넘는 제안은 서명이 올바르더라도 무효이고, 가리키는 batch가 한도를 넘는 lane 제안은 블록으로 재구성되지 않습니다. 노드는 한도보다 큰 `max_block_bytes` 설정을 시작 시 거부합니다.

## Invalid-proposal evidence 사유

모든 사유는 모든 노드가 똑같이 가진 데이터로 판정됩니다.

- `da_mismatch`, `missing_data`: proposal 자신의 트랜잭션으로 판정합니다.
- `validator_set_hash`: 헤더가 가리키는 registry 높이의 set으로 판정합니다. `next_validator_set_hash`: 헤더가 가리키는 state 이후의 set으로 판정합니다. 높이가 0이거나 헤더 높이보다 크면 거부합니다. 정직한 proposer는 자기 복제 state의 set을 가리키므로 누명을 쓸 수 없습니다.
- `app_hash`: slashing 모듈이 기록한, 인증 블록이 커밋한 app hash로 판정합니다.
- `parent_linkage`: 증명이 부모 헤더를 담고, 부모는 proposal의 `PreviousBlockHash`로 해시되어야 합니다. proposal이 부모만으로 정해지는 규칙(높이 = 부모 + 1, 시간 > 부모 시간, 부모가 정한 set/state 높이)을 어겼음을 보입니다. 첫 블록은 부모 없이 previous hash 0, 양의 시간, genesis set/state 높이를 검사합니다.

다른 체인 데이터, 로컬 시계, 재실행이 필요한 결함(시계보다 앞선 블록 시간, 무효 트랜잭션 등)은 노드마다 판단이 다를 수 있으므로 evidence가 아닙니다. 증명은 모두 proposer가 proposal domain으로 서명한 proposal이어야 합니다.

## 리더 스케줄

view의 리더는 그 view에 제안된 블록의 header와 그 블록을 인증하는 validator set으로 정해집니다(`consensus.Leader`). 기본은 stake 스케줄 `validator.SelectProposer(validators, round)`입니다. 활성 validator를 id 순으로 정렬해 각자 `VotingPower`개의 연속 슬롯을 `[0, W)`에서 가지며(`W`는 총 voting power), view `v`는 슬롯 `v·S mod W`의 주인이 이끕니다. stride `S`는 `ceil(W·(φ−1))`부터 찾은, `W`와 서로소인 첫 정수이므로 view `0..W−1`은 모든 슬롯을 한 번씩 지납니다. 따라서 연속한 `W`개 view의 어떤 창에서도 각 validator는 정확히 자기 power만큼 view를 이끌고, 황금비 stride가 한 validator의 view를 서로 떨어뜨려 놓아 죽은 validator는 고립된 view만 잃게 합니다. stake 스케줄은 view에만 의존하고 높이에는 의존하지 않습니다. 총 power가 64비트를 넘으면 리더를 고르지 않습니다.

timeout certificate 뒤의 view(`Round != JustifyRound+1`)는 전체 set의 stake 스케줄이 이끕니다. 그래서 어떤 인증 블록을 가졌든 모든 validator가 같은 리더에 합의합니다. 부모 블록의 인증서 바로 다음 view는 header가 bench하지 않은 validator들의 stake 스케줄이 이끕니다.

`Header.Benched`는 id 순으로 정렬된 validator와 각자 bench되는 마지막 view를 담으며, 부모가 정합니다(`consensus.ChildBench(parent, round, set)`). 부모의 목록에서 view가 지난 항목과 set에서 power가 없는 validator를 빼고, 부모의 view와 `round` 사이의 모든 view의 리더를 그 view + `consensus.BenchViews`(1000)까지 추가합니다. 그런 view는 체인이 잇는 블록을 만들지 못했으므로 그 리더가 실패한 것입니다. 첫 view의 리더는 부모의 bench로 정해진 리더이고, 이후 view의 리더는 전체 set 스케줄의 리더입니다. `round − BenchViews` 이후의 view만 세며, bench된 power가 전체의 1/3에 이르게 하는 실패는 아무도 bench하지 않으므로 스케줄은 항상 2/3 넘는 power를 유지합니다. 첫 블록은 아무도 bench하지 않습니다. 부모가 정한 것과 다른 목록을 담은 header는 무효이지만, 판단에 validator set이 필요하므로 증거가 아니라 거부로 처리합니다.

bench는 liveness 규칙입니다. 누가 제안하는지만 바꾸고 무엇에 투표해도 안전한지는 바꾸지 않습니다. 죽은 validator는 실패한 view 하나와, 이후에는 timeout 뒤 전체 set 스케줄이 그에게 떨어지는 view만 잃게 하며, 모든 view의 stake 비율만큼 잃게 하지 않습니다.

## Lanes

트랜잭션 전파는 순서 결정과 분리되어 있습니다(패키지 `lanes`). 블록 형식, 해시, 트랜잭션 루트, validator가 서명·실행하는 대상은 바뀌지 않고, lanes는 블록이 전달되는 방식만 바꿉니다.

- 모든 validator는 자기 lane을 가집니다. 자기가 소유한 트랜잭션을 서명된 `lanes.Batch`(체인, 작성자, 순번, 트랜잭션; 최대 `lanes.MaxBatchTxs`개, `lanes.MaxBatchBytes`바이트)로 봉인해 모든 validator에게 보냅니다. 올바른 batch를 저장한 validator는 `DomainLaneBatchAck` 도메인으로 ack에 서명하고, voting power `>= 2/3`의 ack가 모이면 `BatchCert`가 됩니다. 따라서 인증된 batch는 power의 1/3 초과를 가진 validator들이 저장하고 있으며 그중 적어도 하나는 정직합니다. 직전 batch가 16 KiB 미만인 lane은 그 batch가 인증될 때까지, 최대 봉인 후 50 ms까지 다음 batch를 기다려서, 부하가 낮은 lane이 트랜잭션 몇 개마다 서명과 인증서 비용을 치르지 않게 합니다.
- 트랜잭션은 `lanes.Owner(members, sender)` lane에 속합니다. 한 sender의 트랜잭션은 한 lane에서 작성자가 받아들인 순서로 이동하므로 lane이 nonce 순서를 쪼개지 않습니다. 다른 lane 소유 트랜잭션을 가진 validator는 `lanes.FallbackDelay` 동안 그것을 담은 batch가 없으면 직접 batch로 만들기 때문에, 결함 있는 소유자가 sender를 검열할 수 없습니다.
- 리더는 인증된 batch만, lane마다 순번 순서로 첫 미인증 batch 직전까지, lane을 번갈아 가며 제안하고 `lane_proposal`을 보냅니다. 이는 트랜잭션을 뺀 서명된 proposal, batch id들, 애플리케이션 검사에서 거부된 batch 트랜잭션의 위치(`Excluded`), batch에 없는 트랜잭션(`Extra`)입니다. 리더의 validator당 업로드는 블록 크기와 무관하게 batch당 몇 바이트이고, 전파 부하는 모든 validator에게 분산됩니다.
- validator는 무엇이든 가져오기 전에 proposal 서명과 리더를 먼저 판정합니다. 위조된 `lane_proposal`은 아무에게도 업로드 비용을 지우지 못하고 보낸 peer의 점수를 깎습니다. 저장한 batch로 블록을 재구성하고, 없는 batch는 작성자나 다른 인증자에게서 받아오며, 재구성한 블록이 헤더가 커밋한 블록과 정확히 같을 때만 판정합니다. validator set이 아직 실행하지 않은 블록에 의존하는 proposal(전체든 lanes든)은 최대 16개까지 보관했다가, 블록을 하나 실행하거나 다른 proposal을 받아들이면 무엇도 가져오기 전에 다시 판정합니다. 재구성한 블록이 서명된 블록과 다르거나 batch가 제때 오지 않으면 보낸 peer에게 `proposal_request`를 보내 전체 `proposal`을 기다립니다. 노드는 보관한 proposal을 peer마다 최대 두 번 제공하고 최근 32개를 보관합니다.
- 작성자마다 아직 소비되지 않은 batch는 저장소의 바이트 몫으로 제한되므로, lane을 범람시키는 validator는 자기 몫만 채웁니다. 커밋된 블록이 소비하지 않은 batch는 만료되고, 소비된 batch는 따라잡는 validator를 위해 몇 초 더 제공됩니다.

lanes 없이 동작하는 것은 측정 기준선(`DisseminateByProposal`)뿐이며, 이때 validator는 받은 트랜잭션을 gossip하고 리더는 블록 전체를 보냅니다.

## 최종성 규칙

Vexo는 global view 위의 two-chain commit rule(`consensus.TwoChainCommitRule`)을 씁니다. 블록 `B`는 `QC(B)`가 view `b`에서 만들어지고, 자식 `C`가 `QC(B)`를 justify certificate로 담고, `QC(C)`가 바로 다음 view `b + 1`에서 만들어졌을 때 최종화됩니다. 최종 블록의 모든 조상도 최종입니다.

## 빈 블록과 라운드 회복

`create_empty_blocks=false`이면 mempool이 비어 있을 때 height가 멈춰 보일 수 있습니다. 이는 정상 idle 상태입니다. 거래가 들어와도 노드는 현재 `(height, round)`의 결정적 proposer일 때만 제안하며, proposer가 아니라고 로컬에서 다른 round로 건너뛰지 않습니다. round는 유효한 timeout certificate 또는 certified finality 전환으로만 진행되고, 블록 실행·저장 오류는 timeout으로 처리되지 않습니다.

## 적응형 라운드 timeout

`adaptive_round_timeout_enabled = true`이면 base timeout, proposal/vote/commit 처리 시간의 rolling p95, 진행 성공 여부, active peer 부족을 사용해 timeout을 계산합니다. timeout 발생 시 1.5배로 증가하고, 진행 성공 시 0.8배로 감소하며, 관측 처리 시간의 3배를 안전 예산으로 반영해 base와 base의 8배 사이로 제한합니다. 이 정책은 safe-vote, proposer, quorum power, QC, two-chain commit rule을 바꾸지 않습니다.

## 복구 최종성 gate

`recovery_finality_gate_enabled = true`이고 내구성 application state 높이와 block index 높이가 다르면 두 높이의 최솟값을 safe recovery height로 사용합니다. 그보다 높은 finalized application commit은 일치가 복구될 때까지 지연됩니다. 현재 timeout과 복구 지연은 `/v1/metrics`와 `/metrics/text`에서 관측할 수 있습니다.

## 결정적 트랜잭션 순서

Proposal은 `chain_id`와 height로 salt를 만들고 signer별 transaction chain을 nonce 오름차순으로 정렬한 뒤 salted transaction hash로 각 chain head를 병합합니다. 동일 candidate set에서는 mempool 도착 순서를 제거하지만 first-seen fairness, censorship resistance, confidentiality 또는 형식적 order fairness를 보장하지 않습니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Scope — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Roles — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: State — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Message Types — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Safety Rules — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Finality Rule — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Execution Commit Policy — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Liveness Assumptions — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Empty Blocks and Round Recovery — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Evidence — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `chain_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `validator_set_hash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `locked_qc` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `high_qc` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `last_timeout_cert` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `last_finalized` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `>= 2/3` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `B3.height = B2.height + 1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `B2.height = B1.height + 1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `execution_commit = "qc"` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `execution_commit = "finalized"` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `block_committed` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `create_empty_blocks = false` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `latest_height = 0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `latest_height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `actual_hash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `actual_time_unix_nano` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `parity_shards` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `C.PreviousBlockHash = hash(B)` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `C.height = B.height + 1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `da_mismatch` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `missing_data` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
