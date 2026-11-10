# Vexo Lanes: 지분 가중 two-chain HotStuff를 위한 인증 batch 전파

> Locale: ko · 한국어  
> 문서 유형: 연구 논문 초안 및 재현성 프로토콜  
> 상태: 구현에 근거한 초안입니다. 아래 결과는 단일 호스트와 에뮬레이션 링크에서 측정했으며, 게재 전에 실제 네트워크에서 반복 측정해야 합니다.  
> 정본 구현: 각 실험과 함께 기록한 저장소 revision

## 초록

HotStuff 계열의 리더 기반 BFT 프로토콜은 리더 업링크에 묶입니다. 블록의 모든 트랜잭션이 한 머신에서 validator 수만큼 나가므로 처리량은 `1/(n−1)`로 떨어지고, 다른 validator의 링크는 놀게 됩니다. Vexo Lanes는 순서 결정 프로토콜을 교체하지 않고 전파를 리더에게서 떼어 냅니다. 모든 validator는 자기가 소유한 트랜잭션을 자기 lane의 서명된 batch로 봉인하고, 지분의 2/3를 가진 validator들이 저장했다고 ack하면 batch는 가용성 인증을 받습니다. 리더는 인증된 batch의 이름만으로 블록을 제안합니다. 블록 형식, 해시, 트랜잭션 루트, 서명 대상, 투표 규칙, two-chain 커밋 규칙은 그대로입니다. validator는 헤더가 커밋한 블록을 정확히 재구성하거나, 블록 전체를 받아 옵니다. 설계에는 sender→lane 소유권과 검열 대비 fallback, batch를 가져오기 전 proposal 인증, lane별 저장 몫, 지분 비례·view 전용 리더 스케줄이 더해집니다.

validator마다 100 Mbit/s, 25 ms로 에뮬레이션한 링크를 둔 단일 호스트에서, Lanes는 validator 4, 7, 10개일 때 60,080, 45,323, 37,099 tx/s를 커밋했습니다. 같은 노드가 블록 전체를 브로드캐스트하면 21,073, 10,909, 8,297 tx/s이므로 2.9×, 4.2×, 4.5×이고 `n`이 커질수록 격차가 커집니다. 가장 바쁜 validator의 업로드는 트랜잭션당 335–726바이트에서 82–218바이트로 줄었습니다. 용량보다 낮은 부하(4,000 tx/s)에서 Lanes는 validator 4개일 때 중앙 지연을 35 ms 늘리고, 7개와 10개일 때는 25 ms와 130 ms 줄였습니다. 이때 기준선의 p99는 0.8초와 1.7초에 이릅니다. 링크가 병목이 아니면(제한 없음, validator 3개) Lanes는 처리량을 4% 잃습니다. 평가를 만드는 과정에서 결함 9개가 드러났습니다. 짝수 크기 set에서 짝수 인덱스 validator만 블록을 제안하던 리더 스케줄, 그리고 10-validator 실행 네 번 중 한 번 처리량을 2,280 tx/s로 무너뜨린 proposal 판정 경쟁 상태가 그 예입니다.

## 1. 연구 질문

주된 질문은 다음과 같습니다.

> validator가 무엇에 투표하는지, 블록이 어떻게 커밋되는지를 바꾸지 않고, 기존 지분 가중 HotStuff 계열 프로토콜 아래에 인증 batch 전파를 넣어서 처리량이 리더의 업링크가 아니라 validator 전체 업링크의 합에 비례하게 만들 수 있는가?

- **RQ1, 리더 병목:** 기준선 처리량 한계 중 리더 업로드가 차지하는 몫은 얼마이고, Lanes는 그중 얼마를 없애는가?
- **RQ2, 확장성:** validator당 링크를 고정하고 validator 수를 늘리면 두 설계는 어떻게 변하는가?
- **RQ3, 보존:** 안전성, validator가 판정하는 블록, nonce 순서, 검열 저항이 유지되는가?
- **RQ4, 비용:** 대역폭이 충분할 때 인증과 간접 참조는 지연과 CPU를 얼마나 쓰는가?

가설은 8절의 측정으로 반증할 수 있습니다.

- H1: 기준선의 가장 바쁜 업로드는 트랜잭션당 `n`에 선형으로 늘고, Lanes는 트랜잭션 크기 근처에 머문다.
- H2: 대역폭이 제한된 링크에서 Lanes의 속도 향상은 `n`과 함께 커진다.
- H3: 제한 없는 링크에서 Lanes는 빠르지 않으며, 추가 비용은 왕복 한 번 이하다.

## 2. 선행 연구와 신규성 경계

전파와 순서 결정의 분리는 이미 확립되어 있습니다. Narwhal과 Tusk는 batch의 인증 DAG를 만들어 DAG 또는 HotStuff 합의로 정렬하고, Bullshark와 Shoal은 DAG 자체를 정렬합니다. Stratus는 HotStuff 아래에 가용성 인증서를 가진 공유 mempool batch를 둡니다. Mir-BFT와 ISS는 요청 버킷을 나눠 여러 리더를 돌리고, Autobahn은 데이터 lane을 부분 동기 합의 핵심에서 분리합니다. Aptos의 Quorum Store는 HotStuff 계열 합의 아래에 인증 batch를 배포했습니다.

따라서 이 원고는 다음을 주장하면 안 됩니다.

- 전파와 순서의 분리, 가용성 인증서, validator별 lane이 새롭다는 것;
- DAG 프로토콜, 비동기 liveness, 최적 통신 복잡도;
- 실제 광역 네트워크 결과. 이 문서의 모든 수치는 에뮬레이션 링크를 쓴 단일 호스트에서 나왔습니다;
- 리더 스케줄 자체가 새로운 아이디어라는 것. 결정적 지분 가중 순환은 표준입니다.

기여는 더 좁고 공학 중심입니다.

1. 기존 블록, 헤더, 서명, 커밋 규칙을 비트 단위로 유지하는 개조. 라이트 클라이언트, evidence, finality proof, state sync는 손대지 않습니다. lane proposal은 서명된 블록의 전송 인코딩일 뿐입니다.
2. 한 sender의 nonce 순서를 한 lane에 두는 lane 소유권과, 결함 있는 소유자의 검열을 막는 보유자 fallback.
3. batch를 하나라도 가져오기 전에 proposal의 리더와 서명을 인증해서, 위조 proposal이 네트워크에서 데이터를 끌어오게 만들 수 없게 한 것.
4. 링크 모델이 명시된 재현 가능한 측정 하네스, 측정 행렬, 그리고 평가가 드러낸 기반 노드의 결함(9절).

## 3. 시스템 모델

- `n`개 validator가 지분 가중 voting power를 가지며, Byzantine validator의 power는 전체의 1/3 미만입니다. quorum은 power의 `>= 2/3`입니다.
- 부분 동기: 알 수 없는 안정화 시점 이후 정직한 validator 사이의 메시지는 알려진 상한 안에 도착합니다.
- 인증된 점대점 채널과 높이별로 알려진 validator set을 가정하며, 서명은 도메인이 분리됩니다.
- validator는 멈추거나, 이중 서명하거나, batch를 숨기거나, 자기 lane을 범람시키거나, 위조 proposal과 batch를 보낼 수 있습니다.
- 애플리케이션은 트랜잭션을 결정적으로 판정하며, 리더는 애플리케이션 검사에서 거부된 트랜잭션을 뺄 수 있습니다.

## 4. 프로토콜

### 4.1 Lane과 batch

모든 validator는 lane 하나를 작성합니다. 자기가 소유한 트랜잭션을 `lanes.Batch`(체인, 작성자, 순번, 트랜잭션)로 봉인하는데, 512 KiB에 이르거나 첫 트랜잭션 후 5 ms가 지나면 봉인하고 `lanes.MaxBatchTxs`개, `lanes.MaxBatchBytes`바이트로 제한합니다. 직전 batch가 16 KiB 미만인 부하 낮은 lane은 그 batch의 인증서도 기다리며(봉인 후 최대 50 ms), 그래서 한 batch가 인증 왕복 한 번 동안 도착한 트랜잭션을 모읍니다. batch마다 서명 하나, validator당 ack 하나, 모든 validator의 인증서 검증이 필요하므로, 이 pacing이 없으면 400 tx/s인 lane은 트랜잭션 두 개짜리 batch를 봉인합니다. 부하가 높은 lane은 기다리지 않으므로 batch가 파이프라인으로 흐릅니다. batch id는 이 필드 전부를 해시하고, 작성자가 서명해서 모든 validator에게 보냅니다. batch는 독립적이며 batch 체인이나 DAG는 없습니다.

### 4.2 가용성 인증서

올바른 batch를 저장한 validator는 `DomainLaneBatchAck` 도메인으로 그 id에 대한 ack에 서명합니다. 작성자는 power `>= 2/3`의 ack를 `BatchCert`로 모아 알립니다. Byzantine validator의 power가 1/3 미만이므로 인증자 중 정직한 쪽이 power의 1/3 초과를 가지며, 인증된 batch는 그것을 제공할 정직한 validator가 적어도 하나 저장하고 있습니다. 작성자는 ack하지 않은 validator에게 지수 backoff(1초에서 두 배씩, 최대 네 번)로 batch를 재전송하므로, 혼잡한 링크가 아직 나르고 있는 사본으로 범람하지 않습니다.

### 4.3 소유권과 fallback

트랜잭션은 id 순으로 정렬한 validator 위에서 sender를 해시한 lane `lanes.Owner(members, sender)`에 속합니다. 한 sender의 트랜잭션은 모두 한 lane에서 작성자가 받아들인 순서로 이동하고 리더는 lane의 batch를 순번 순서로 제안하므로, lane이 nonce 순서를 뒤바꾸지 않습니다. 다른 lane 소유의 트랜잭션을 받은 validator는 그것을 보관하고, `lanes.FallbackDelay`(3초) 뒤에도 저장한 batch에 없으면 자기 lane에 batch로 만듭니다. 결함 있는 소유자는 sender를 fallback 지연만큼 늦출 수 있을 뿐 검열할 수 없습니다.

### 4.4 Lane proposal

리더는 인증된 batch만, lane마다 순번 순서로 첫 미인증 batch 직전까지, lane을 번갈아 가며 블록 바이트 한도까지 고릅니다. 평소의 애플리케이션 검사와 결정적 순서 결정을 거쳐 블록에 그대로 서명하고 `lane_proposal`을 브로드캐스트합니다. 내용은 트랜잭션을 뺀 서명된 proposal, batch id들, 애플리케이션 검사에서 거부된 batch 트랜잭션의 증가하는 위치(`Excluded`), batch에 없는 트랜잭션(`Extra`)입니다. 리더의 validator당 업로드는 블록 크기와 무관하게 batch당 몇 바이트입니다.

### 4.5 재구성과 가져오기 전 인증

받는 validator는 먼저 헤더 해시를 기준으로 proposal의 리더와 서명을 판정합니다. 위조 proposal은 무엇도 가져오기 전에 거부되고 보낸 peer의 점수를 깎습니다. validator set이 아직 실행하지 않은 블록에 의존하는 proposal은 최대 16개까지 보관했다가, 블록을 하나 실행하거나 proposal을 받아들이는 즉시 아무것도 가져오지 않고 다시 판정합니다. two-chain 규칙에서는 블록을 확정하는 proposal이 받는 쪽이 그 블록을 실행하기 전에 도착하는 일이 흔합니다. 인증된 proposal은 저장한 batch로 재구성하며, 없는 batch는 작성자에게, 그다음 다른 인증자에게 요청합니다. 재구성한 블록은 해시와 트랜잭션 루트가 서명된 헤더와 같을 때만 판정합니다. 그렇지 않거나 아직 판정할 수 없으면(validator set 대기) 보낸 peer에게 `proposal_request`를 보내 전체 `proposal`을 기다리며, 노드는 peer당 최대 두 번 제공합니다. validator가 판정하는 바이트는 언제나 리더가 서명한 바이트입니다.

### 4.6 지분 비례 리더 스케줄

view `v`의 리더는 `validator.SelectProposer(validators, v)`입니다. id 순으로 정렬한 validator는 `[0, W)`에서 `VotingPower`개의 연속 슬롯을 가지고, view `v`는 슬롯 `v·S mod W`에 대응합니다. stride `S`는 `ceil(W·(φ−1))`부터 찾은, `W`와 서로소인 첫 정수입니다. view `0..W−1`은 모든 슬롯을 정확히 한 번씩 지나므로, `W`개 view의 어떤 창에서도 각 validator는 정확히 자기 power만큼 view를 받고, 황금비 stride가 한 validator의 view를 서로 떨어뜨립니다. 스케줄은 view에만 의존합니다.

### 4.7 자원 한도

- 작성자별로 아직 소비되지 않은 batch는 256 MiB 저장소 중 64 MiB로 제한되므로, lane을 범람시키면 범람시킨 쪽의 몫만 찹니다.
- 소비되지 않은 batch는 30초 뒤 만료되고, 소비된 batch는 따라잡는 validator를 위해 5초 더 제공됩니다.
- 노드는 한 batch를 한 peer에게 최대 두 번 제공하고, lane 요청은 id 256개까지, lane proposal은 batch `consensus.MaxLaneRefs`개까지 담으며, 아직 판정할 수 없는 proposal은 최대 16개까지 보관합니다.
- wire 형식은 버전이 붙은 이진 인코딩이며 잘리거나 남는 바이트를 거부합니다.

## 5. 정당성 논증

- **안전성.** 투표 규칙, quorum, lock, 커밋 규칙은 그대로이고, validator는 재구성한 해시와 트랜잭션 루트가 리더가 서명한 헤더와 같은 블록에만 투표합니다. Lanes는 투표를 늦출 수는 있어도 대상을 바꿀 수 없으므로 안전성은 기반 two-chain 프로토콜의 안전성으로 환원됩니다.
- **가용성.** 리더는 인증된 batch만 지명하며 각 batch는 그것을 제공할 정직한 validator가 적어도 하나 저장합니다. 그래도 가져오기에 실패하면 proposal을 가진 보낸 peer에게서 전체를 받습니다.
- **Liveness.** 안정화 이후 정직한 작성자는 유한 시간 안에 batch를 인증받고 정직한 리더가 그것을 제안합니다. 결함 있는 리더는 기반 pacemaker가 처리합니다. validator set이 의존하는 블록을 실행하기 전에 proposal을 받은 validator는 그 실행 직후 판정하므로, 한 블록 뒤처져도 자기가 이끄는 view를 잃지 않습니다. 죽은 validator는 자기 view와 lane을 잃지만, 다른 validator가 fallback 지연 뒤 그 소유 트랜잭션을 batch로 만듭니다(4개 중 1개를 내린 상태로 테스트).
- **순서와 검열.** 한 sender의 트랜잭션은 한 lane을 공유하고 lane 순서로 제안됩니다. fallback이 소유자의 검열 능력을 제한하고, 리더 스케줄은 각 validator에게 정확히 지분만큼의 view를 줍니다.
- **자원 고갈.** 위조 proposal은 가져오기를 일으키지 않습니다. lane 몫, 만료, 제공 횟수 제한이 Byzantine validator가 남에게 저장·업로드시킬 수 있는 양을 제한합니다.

이것은 논증이지 증명이 아닙니다. 이 합성에 대한 기계 검증 증명은 후속 과제입니다.

## 6. 구현 대응

- `lanes/batch.go`: batch, ack, 인증서, `VerifyBatch`, `VerifyCert`.
- `lanes/lanes.go`: 봉인, 전파, 재전송, 저장 몫, 후보, fallback.
- `lanes/wire.go`: 이진 lane 메시지.
- `consensus/lane_proposal.go`: `consensus.LaneProposal`, 재구성, `consensus.MaxLaneRefs`.
- `consensus/transport_reactor.go`: 가져오기 전 인증, batch 가져오기, 보류 판정, 전체 proposal fallback.
- `validator/proposer.go`: 리더 스케줄.
- `node/lanes.go`: 노드 연결, 소유권 라우팅, proposal용 lane 후보.
- `node/throughput_test.go`, `node/shaped_transport_test.go`: 측정 하네스와 링크 모델.

## 7. 실험 방법

### 7.1 처리군

- **Lanes:** 기본 노드.
- **기준선:** 같은 바이너리에 `DisseminateByProposal`을 켠 것. validator는 받은 트랜잭션을 gossip하고 리더는 블록 전체를 브로드캐스트합니다. 합의, 애플리케이션, mempool, 순서 결정, 실행은 모두 공유합니다.

### 7.2 링크 모델

모든 validator는 Apple M4 한 대의 한 프로세스에서 돕니다. 각 validator의 업로드는 고정 속도와 단방향 지연을 가진 가상 링크로 제한되며, 합의·커밋·evidence 메시지를 트랜잭션·batch 같은 대량 트래픽보다 먼저 보내서 기준선 proposal이 gossip 뒤에 밀리지 않게 합니다. 손실, 지터, 교차 트래픽은 모델에 없습니다.

### 7.3 워크로드와 지표

클라이언트는 nonce 없는 89바이트 트랜잭션을 모든 validator에 번갈아 제출합니다. 처리량 실행은 150,000개(제한 없는 실행은 300,000개)를 한꺼번에 제출하고, 처리량은 커밋된 트랜잭션 수를 첫 제출부터 마지막 커밋까지의 시간으로 나눈 값입니다. 지연 실행은 20,000개를 4,000 tx/s 개방 루프(`VEXO_RATE`)로 넣어 두 처리군의 용량보다 낮게 유지합니다. 지연은 트랜잭션이 받아들여진 때부터 첫 validator가 그 블록의 커밋을 관측할 때까지이며 p50/p95/p99로 보고합니다. 포화 상태의 지연은 적체만 재므로 처리량 실행에서는 보고하지 않습니다. 업로드는 각 validator 링크가 커밋된 트랜잭션당 나른 바이트이며, 가장 바쁜 validator 값과 proposal topic 값을 따로 보고합니다. 각 칸은 세 번 실행했습니다.

round timeout 기본값은 1초 + 단방향 지연 8배(1.2초)입니다. 10-validator 처리량 칸은 두 처리군 모두 5초를 씁니다. 기본 timeout에서는 기준선이 높이 6 이후 진행하지 못합니다. 1 MiB 블록을 큐가 쌓인 링크로 validator 9개에게 올리는 데 timeout과 비슷한 시간이 걸려 모든 view가 timeout되기 때문입니다. 운영 설정은 적응형 timeout을 켜지만, 이 하네스는 고정된 처리군을 비교하려고 끕니다.

## 8. 결과

### 8.1 대역폭 제한 처리량 (100 Mbit/s, 25 ms)

| Validator | Lanes tx/s, 평균 (최소–최대) | 기준선 tx/s, 평균 (최소–최대) | 향상 | Lanes 최대 업로드 B/tx | 기준선 최대 업로드 B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

모든 실행이 150,000개를 전부 커밋했습니다. Lanes는 round timeout이 없었고, 기준선은 7-validator 실행 한 번에서 두 번 있었습니다. 기준선의 proposal topic만으로 리더는 트랜잭션 크기 `s`에 대해 트랜잭션당 `s·(n−1)`바이트를 씁니다(validator 4개 266 B/tx, 7개 580 B/tx). Lanes는 0.2–0.4 B/tx입니다. 7-validator 실행 세 번 중 두 번은 실행 초반에 일부 validator가 proposal 전체를 받아 왔습니다(53 B/tx). 한꺼번에 제출된 부하가 512 KiB batch로 봉인되었고, 리더가 그 batch를 제안한 지 300 ms가 지나도 마지막 1/3 validator에게 아직 도착하지 않았기 때문입니다. H1과 H2를 지지합니다.

### 8.2 용량 이하 지연 (4,000 tx/s 제시)

| Validator | Lanes p50 / p99 | 기준선 p50 / p99 | Lanes proposal B/tx | 기준선 proposal B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

validator 4개에서는 인증 때문에 Lanes 중앙 지연이 약 35 ms, 즉 왕복 한 번 늘어납니다. 7개부터는 이 부하에서도 기준선 리더 업링크가 proposal을 늦춥니다. Lanes의 중앙값이 더 낮고 꼬리는 2–3배 짧으며, 10개에서는 Lanes만 제시 부하를 따라갔습니다(실행 전체 기준 3,655–3,677 대 3,082–3,261 tx/s 커밋). H3는 작은 set에서만 성립합니다.

### 8.3 제한 없는 링크 (validator 3개)

| 처리군 | tx/s, 평균 (최소–최대) | 최대 업로드 B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| 기준선 | 114,522 (113,095–116,408) | 147.4–148.3 |

대역폭이 충분하면 기준선이 4% 빠릅니다. Lanes의 추가 서명과 ack는 H3가 예측한 대로 순수 비용입니다. 그래도 Lanes가 올리는 바이트는 절반 미만입니다.

### 8.4 원인 귀속

기준선은 리더 링크에 묶여 있습니다. 측정한 proposal 바이트가 `s·(n−1)`과 일치하고 처리량은 `n`에 따라 떨어집니다. Lanes는 이 행렬에서 링크가 아니라 호스트에 묶입니다. 가장 바쁜 validator도 링크를 다 쓰지 않았고, 모든 validator가 한 머신의 코어 10개를 나눠 쓰므로 validator를 늘리면 같은 코어에 서명 검증과 실행이 더해집니다. 따라서 Lanes 수치는 validator마다 머신이 따로 있는 배포의 하한이지만, 기준선 수치는 병목이 리더 링크이므로 하한이 아닙니다. 이 귀속은 주장하기 전에 별도 머신에서 다시 측정해야 합니다. 번갈아 돌린 A/B 실행 두 쌍으로 코드 효과와 호스트 변동을 분리했습니다. proposal 판정 수정은 validator 7개 기준선을 바꾸지 않았고(어느 쪽이든 11,203–11,894 tx/s), 16 KiB 임계값의 lane pacing은 validator 4개 포화 처리량을 2% 미만 바꾸면서(59,295–60,402 대 60,592–61,449 tx/s) 10-validator 저부하 중앙 지연을 1.43초에서 0.44초로 줄였습니다.

## 9. 평가가 드러낸 결함

각 결함에는 수정을 되돌리면 실패하는 회귀 테스트가 있습니다.

1. **짝수 리더 스케줄.** 이전 스케줄 `(height−1+round) mod n`은 높이와 view를 함께 올렸으므로, 결함 없는 실행에서 `n`이 짝수면 짝수 인덱스 validator만 리더가 되었습니다. validator 절반이 한 번도 제안하지 못했고 그 지분은 제안 몫을 받지 못했습니다. 4.6절의 지분 비례 view 전용 스케줄로 교체했습니다(`node/leader_schedule_test.go`).
2. **커밋된 트랜잭션 재생.** 늦게 도착한 gossip이 이미 커밋된 트랜잭션을 다시 받아들였고, 기준선이 이를 두 번 커밋했습니다(150,000개에 커밋 152,629개). 이제 커밋된 트랜잭션을 seen cache에 기록합니다.
3. **이차 시간 seen-cache 만료.** 트랜잭션을 받아들일 때마다 seen cache 전체를 훑었습니다. 이제 오래된 것부터 만료하고 크기를 제한합니다.
4. **재전송 폭주.** 큐가 쌓인 링크에서 200 ms 고정 재전송이 아직 전송 중인 batch를 다시 보냈습니다. 이제 지수 backoff를 씁니다.
5. **무한히 중복되는 evidence.** 충돌 투표가 반복될 때마다 같은 evidence 사본이 한도 없이 추가되었습니다. 이제 위반마다 한 번, 1,024개 창 안에 기록합니다.
6. **커밋 본문 요청.** 커밋을 놓친 노드가 알린 모든 validator에게 같은 본문을 요청했습니다. 이제 한 곳에 요청하고 1초 뒤 재시도합니다.
7. **재전송 때만 판정되는 proposal.** validator set이 받는 쪽이 아직 실행하지 않은 블록에 의존하는 proposal은 거부된 뒤 제안자가 재전송할 때, 즉 round timeout의 1/4 뒤에야 다시 판정되었습니다. 뒤처진 validator는 view를 놓쳤고, 그 validator가 다음 view의 리더면 모두가 round timeout 하나를 통째로 기다렸습니다. 10-validator 실행 네 번 중 한 번이 timeout 12번과 함께 2,280 tx/s로 무너졌습니다. 이제 그런 proposal을 보관했다가 블록을 실행하는 즉시 판정합니다. 수정 후 10-validator 연속 10회 실행에서 첫 validator 기준 round timeout이 없었습니다(`consensus/transport_reactor_test.go`).
8. **저부하 batch당 비용.** 5 ms 봉인 지연 때문에 lane당 400 tx/s에서 트랜잭션 두 개짜리 batch가 만들어졌고, 초당 2,000개 batch의 서명·ack·인증서 검증이 호스트를 포화시켰습니다. validator 10개에서 Lanes는 제시된 4,000 tx/s 중 2,900 tx/s만 커밋했고 중앙 지연은 1.4초였습니다. lane pacing(4.1절)으로 고쳤습니다.
9. **잘못 표기된 지연 지표.** 하네스가 마지막 블록의 로컬 실행 시간을 "commit latency p50"으로 보고했습니다. 이제 트랜잭션마다 제출부터 커밋까지의 지연과 그 백분위수를 잽니다.

## 10. 타당성 위협

- **에뮬레이션 네트워크.** 단일 호스트, 손실 없는 shaped 링크, 공유 CPU입니다. 실제 WAN의 손실, 지터, 비대칭 링크는 모델링하지 않았습니다.
- **공유 CPU.** 큰 `n`의 Lanes 결과는 호스트에 묶여 있으므로, 별도 머신에서는 10-validator 향상이 과소 또는 과대 평가될 수 있습니다.
- **timeout 민감도.** 기준선은 validator 10개에서 5초 timeout이 필요합니다. 조정된 기준선이나, short-id compact block 같은 다른 proposal 인코딩을 쓰는 기준선은 작은 `n`에서 격차를 줄일 것입니다.
- **전체 proposal fallback.** validator가 전체 proposal을 요청하기 전 기다리는 300 ms는 혼잡한 100 Mbit/s 링크에서 512 KiB batch가 모든 validator에게 닿는 시간보다 짧아서, 7-validator 실행 두 번의 초반에 리더가 블록 전체를 업로드했습니다.
- **워크로드.** 작은 nonce 없는 송금뿐입니다. EVM 호출처럼 실행이 무거우면 두 처리군 모두 병목이 실행으로 옮겨 갑니다.
- **반복 세 번.** validator 10개의 변동(34.9k–39.4k tx/s)이 보이므로 신뢰 구간을 내려면 반복을 늘려야 합니다.
- **인증서.** Ed25519 인증서는 서명을 이어 붙입니다. BLS 집계를 쓰면 ack와 인증서가 줄어듭니다.

## 11. 재현 절차

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS`는 round timeout을 바꾸고, `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`은 지연 칸을 실행합니다. `VEXO_UPLINK_MBIT`와 `VEXO_LATENCY_MS`를 빼면 제한 없는 링크입니다. 공개하는 모든 수치에는 저장소 revision, 호스트, Go 버전, 각 실행의 전체 로그 줄을 함께 기록해야 합니다.

<!-- vexo-docs:technical-parity -->

## 기술 동등성 부록

다음 구현·실험 이름은 번역하지 않습니다.

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`
- `validator/proposer.go`, `node/lanes.go`
- `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `validator.SelectProposer(validators, v)`
- `lane_proposal`, `proposal_request`, `0..W−1`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
