# 모듈형 지분증명 네트워크를 위한 적응형 복구 게이트 HotStuff

> Locale: ko · 한국어  
> 문서 유형: 연구 논문 초안 및 재현성 프로토콜  
> 상태: 현재 구현에 근거한 초안이며 성능 주장은 측정 자료가 있어야 성립합니다.

## 초록

이 문서는 모듈형 Proof-of-Stake 네트워크를 위한 HotStuff 계열 BFT 상태 기계 복제를 연구합니다. 현재 구현은 3-chain finality와 높이별 validator set에 세 가지 운영 메커니즘을 결합합니다. 첫째, proposal·vote·commit의 p95 처리 지연과 활성 peer 상태를 이용해 round timeout을 제한된 범위에서 늘리거나 줄입니다. 둘째, 영속 block history와 application state history가 서로 다른 경우 안전한 복구 높이보다 앞선 finalized commit을 보류합니다. 셋째, 동일한 transaction 집합이면 mempool 도착 순서와 무관하게 같은 순서를 만들면서 한 signer의 nonce 순서는 보존합니다.

이 연구의 신규성은 PoS, BFT, HotStuff, 적응형 view synchronization 또는 transaction order fairness 자체를 처음 만들었다는 데 있지 않습니다. 연구 질문은 이 구체적인 제어 정책과 복구 정책의 결합이 기존 HotStuff 안전 규칙을 바꾸지 않으면서 불필요한 timeout과 복구 중 상태 불일치를 줄이는가입니다. 따라서 구현된 사실, 검증할 가설, 아직 실험이 필요한 주장을 분리합니다. 처리량이나 지연 개선 수치는 고정된 binary, config, topology, workload로 반복 실험하기 전에는 논문 결과로 적지 않습니다.

## 연구 질문과 가설

핵심 질문은 지연·peer 상태를 반영하는 bounded pacemaker, durable-history finality gate, nonce-preserving deterministic ordering이 fixed timeout 기준선보다 안정성과 속도를 개선하는지입니다.

- RQ1: 네트워크 지연이 바뀔 때 adaptive policy가 fixed policy보다 round timeout 횟수와 p95 commit latency를 줄이는가?
- RQ2: storage/restart fault를 주입했을 때 recovery gate가 안전 복구 높이보다 application state가 앞서 나가는 것을 막는가?
- RQ3: 동일 transaction 집합의 모든 입력 순열에서 같은 proposal order가 나오고 signer별 nonce가 증가 순서를 지키는가?
- RQ4: 정상 네트워크에서 controller가 추가하는 CPU, memory, network, latency 비용은 얼마인가?

H1부터 H4까지는 검증 가능한 방향성 가설입니다. 현재 코드가 존재한다는 사실만으로 가설이 참이라고 쓰지 않습니다. 유의한 이점이 나오지 않으면 그것도 유효한 부정적 결과 또는 적용 범위 결과로 보고합니다.

## 선행연구와 신규성 경계

HotStuff는 partial synchrony 모델에서 leader 기반 BFT, quorum certificate, chained commit, 정상 경로의 선형 통신과 responsiveness를 제시했습니다. LibraBFT/DiemBFT와 AptosBFT는 HotStuff 계열 BFT가 stake 기반 validator governance와 실제 blockchain에 결합될 수 있음을 보여줍니다. Jolteon과 Ditto는 latency와 network adaptation 및 asynchronous fallback을 연구했고, Fever는 responsive view synchronization을 연구했습니다. Tendermint는 다른 round 기반 PoS BFT 계열입니다. Narwhal/Tusk는 transaction dissemination과 ordering을 분리했습니다. Aequitas, Wendy, Themis는 이 구현의 hash ordering보다 강한 order-fairness 정의를 다룹니다.

그러므로 다음 표현은 사용하지 않습니다.

- “PoS와 BFT를 결합한 최초 blockchain”
- “HotStuff를 사용하는 최초 PoS network”
- “AptosBFT와 동일하거나 이를 대체하는 protocol”
- formal proof 없이 “asynchronous liveness”, “optimal communication”, “완전한 MEV 방지”
- single-host Docker test만으로 “production-ready”

후보 기여는 더 좁습니다. bounded feedback controller, local durable-history commit gate, nonce-aware deterministic ordering을 하나의 Go 기반 modular PoS node에 통합하고, fixed/gate-disabled baseline과 비교 가능한 재현 실험을 정의한 systems contribution입니다.

## 시스템 모델과 안전 가정

높이 h에서 활성 validator set을 Vh, 총 voting power를 Ph라고 둡니다. 알려진 고유 signer의 power가 Ph의 3분의 2 이상이어야 QC가 유효합니다. validator set과 hash는 높이별로 versioning됩니다. admission은 minimum stake 기반 permissionless, 최대 validator 수 제한, 또는 whitelist 방식으로 설정할 수 있습니다. 이 admission은 Sybil resistance와 governance 계층이며 BFT fault threshold를 바꾸지 않습니다.

network는 partially synchronous하다고 가정합니다. Byzantine voting power가 3분의 1 미만이고 signature, validator-set binding, durable store 가정이 유지되면 safety를 기대합니다. liveness에는 결국 지연이 bounded되고 honest quorum, signer, peer connectivity가 확보되어야 합니다. 영구 asynchronous network에서 진행을 보장한다고 주장하지 않습니다.

EVM은 Vexo consensus 아래의 application workload입니다. Ethereum bytecode 실행과 `/web3` tooling compatibility를 제공하지만 Ethereum fork choice나 devp2p consensus를 구현한다는 뜻은 아닙니다.

## 제안 메커니즘

기본 안전 규칙은 `locked_qc`와 `high_qc`를 추적합니다. proposal은 lock을 연장하거나 lock 이상으로 새로운 justify QC를 가져야 안전합니다. 같은 height와 round에서 서로 다른 block에 투표할 수 없습니다. 세 개의 연속된 height/hash 인증 link가 있을 때 grandparent가 finalized됩니다. controller는 이 predicate, quorum threshold, commit rule을 바꾸지 않습니다.

adaptive timeout은 base budget T0, current budget Tt, proposal/vote/commit p95 latency 합, active peer deficit을 사용합니다. timeout이면 current를 1.5배 방향으로 증가시키고, progress가 있으면 0.8배 방향으로 감소시킵니다. 관측 latency 합에는 3배 budget을 두며 최종값은 T0와 8×T0 사이로 제한합니다. peer가 전혀 없으면 최소 floor가 2×T0가 됩니다. pending work가 없는 idle 시간과 execution/storage error는 round timeout으로 계산하지 않습니다.

recovery gate는 durable state height Hs와 block-index height Hb가 모두 존재할 때 Hsafe=min(Hs,Hb)를 계산합니다. 두 기록이 다르면 Hsafe를 초과하는 finalized application commit을 복구가 끝날 때까지 보류합니다. 이것은 local persistence restriction이며 새로운 vote phase나 network certificate가 아닙니다.

deterministic ordering은 chain ID와 height로 salt를 만들고 signer/nonce가 있는 transaction을 signer chain으로 묶습니다. 각 chain 내부는 nonce 오름차순이고, chain head는 salted transaction hash로 merge됩니다. 이 방식은 같은 후보 집합의 결과를 동일하게 만들지만 first-seen fairness, censorship resistance, confidentiality 또는 strong order fairness를 증명하지 않습니다. proposer가 후보 집합 자체를 선택할 여지는 남습니다.

현재 consensus vote path는 전체 height-versioned validator set과 deterministic proposer를 사용합니다. repository의 ECVRF committee selector는 component와 query surface에 존재하지만 quorum formation과 proposal eligibility에 연결되어 있지 않습니다. 따라서 “VRF committee consensus가 구현 완료되었다”는 주장은 하지 않고 future work로 둡니다.

## 구현 근거와 실험 설계

state-machine safety와 locked QC는 consensus package, three-chain binding은 commit rule, adaptive controller는 node loop, recovery gate는 recovery/commit path, ordering은 fairordering package에 있습니다. validator registry와 staking module은 height-versioned PoS set을 제공하며 consensus WAL은 crash 이후 double vote를 막는 영속 기록을 제공합니다. EVM workload는 EVM module, geth adapter, RPC integration으로 검증합니다.

실험 treatment는 같은 binary와 app config를 사용합니다.

| Treatment | Adaptive timeout | Recovery gate | 목적 |
| --- | --- | --- | --- |
| Fixed | off | on | RQ1 baseline |
| Adaptive | on | on | 제안 정책 |
| Gate ablation | on | off | 격리된 test network에서 RQ2 비교 |

가능하면 4, 7, 16, 31 validator를 사용하고 single-host는 smoke 용도로만 씁니다. 10/50/100/250 ms latency, stepped jitter, 0/1/5/10% loss, proposer restart, minority partition/heal, signer delay, durable history mismatch를 포함합니다. workload는 native transfer와 EVM transfer, contract creation, event log, proxy deployment, UUPS upgrade를 포함해야 합니다.

metric은 height rate, proposal/vote/commit p50·p95·p99, end-to-end finality latency, timeout count, round distribution, current adaptive timeout, peer count, recovery deferral, throughput, gas, CPU, memory, disk/network 사용량, rejection·double-sign·invalid nonce count입니다. 모든 validator가 같은 app hash와 finalized block hash에 동의해야 성능 run을 유효하게 인정합니다.

각 조건은 warm-up 뒤 원칙적으로 30회 이상 독립 반복하고 treatment 순서를 무작위화합니다. median, IQR, p95, confidence interval, effect size를 보고하며 가장 좋은 run만 골라서는 안 됩니다. outlier 제외 규칙은 결과를 보기 전에 정합니다.

## 정확성 범위와 연구 윤리

adaptive policy는 언제 timeout vote를 시도하는지만 바꾸고 무엇이 안전한 vote/QC인지는 바꾸지 않습니다. recovery gate는 허용 범위를 더 제한할 뿐 기본 규칙이 거부한 commit을 허가할 수 없습니다. deterministic ordering은 deterministic execution input에 기여하지만 conflicting finality 방지 proof의 대체물이 아닙니다.

publication 수준에서는 quorum intersection, lock monotonicity, 한 height에서 finalized block의 유일성, validator-set transition, vote WAL crash recovery, controller와 gate의 safety-neutral 성질을 formalize해야 합니다. unit test와 adversarial simulation은 evidence이지만 formal proof와 external audit를 대신하지 않습니다.

연구자는 알려진 메커니즘을 이름만 바꾸어 발명으로 주장하지 않고 실패 run과 limitation을 보존해야 합니다. throughput/latency/validator count를 만들지 말고 hypothesis, observation, interpretation을 분리합니다. AI assistance는 venue policy에 따라 공개하며 모든 claim과 citation 책임은 저자에게 있습니다. fault experiment는 소유하거나 허가받은 isolated system에서만 실행하고 key, token, participant data, production endpoint를 artifact에 포함하지 않습니다.

## 재현과 제출 기준

experiment마다 commit, dirty-tree status, Go/OS/CPU/memory/container version, topology, genesis, split config, binary SHA-256, workload seed, raw JSON/JSONL/CSV, validator log, final app hash, analysis script, failed-run ledger를 보관합니다. plain contract와 proxy/UUPS flow 모두 transaction/receipt/block lookup까지 검증합니다.

제출 전에는 source revision과 paper 설명이 일치하고, prior-art search가 기록되어야 합니다. fixed/adaptive treatment, multi-host fault injection, raw data 재생, figure 재생, negative result 포함, proof wording, external methodology review가 모두 완료되어야 합니다. 그 전의 정확한 표현은 “구현 기반 연구 초안”이며 “새롭게 증명된 합의”가 아닙니다.

<!-- vexo-docs:technical-parity -->

## 기술 동등성 부록

아래 구현·설정·검증 이름은 번역하지 않습니다.

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
