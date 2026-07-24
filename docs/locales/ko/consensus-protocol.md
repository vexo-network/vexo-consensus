> Locale: ko · 한국어

# 합의 프로토콜 개요

이 문서는 Vexo 합의를 이해하기 위한 상위 수준 진입점입니다. 규범적 세부 사항은 [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md), [Transaction Format](./specs/tx-format.md)을 따릅니다.

## 모델

Vexo는 proposal, vote, quorum certificate(QC), timeout certificate, locked-QC 안전 규칙, three-chain finality를 갖는 HotStuff 스타일 BFT 코어를 사용합니다. 블록은 locked QC를 확장하거나 lock보다 오래되지 않은 justify QC를 포함할 때만 안전하게 투표할 수 있습니다. 블록, 부모, 조부모 높이와 해시를 명시적으로 연결하지 못한 합성 QC 또는 높이를 건너뛴 QC 체인은 최종성 결정 전에 거부됩니다.

## 프로토콜 정체성과 연구 경계

Vexo는 수정하지 않은 HotStuff의 새 이름이 아니며 AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint, CometBFT와 동일한 프로토콜이나 구현도 아닙니다. 별도 Go 런타임에서 HotStuff 계열 안전 개념을 재사용하고, 적응형 라운드 시간, 내구성 복구, 결정적 트랜잭션 순서, 모듈 실행, 높이별 validator set 정책을 결합합니다.

현재 투표 경로는 높이별 전체 validator set과 결정적 proposer 선택을 사용합니다. 저장소의 VRF committee selector는 컴포넌트와 조회 경로에는 존재하지만 proposal 자격이나 quorum 구성에는 연결되지 않았습니다. 따라서 VRF committee 합의는 활성 기능이 아니라 후속 연구로만 기술해야 합니다. 기여 범위와 실험 절차는 [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md)를 참조하세요.

## 실행 및 복구 경계

QC 인증, HotStuff 최종화, 애플리케이션 실행, 상태 커밋은 서로 다른 사건입니다. 기본 `execution_commit=finalized`는 three-chain 규칙이 선택한 조상만 실행합니다. 적응형 pacemaker와 `recovery_finality_gate_enabled`는 지연 및 재시작 복구 정책일 뿐 proposer 선택, quorum power, safe-vote 규칙, three-chain finality를 변경하지 않습니다.

## 안전 경계

- 비잔틴 투표력의 1/3 미만
- 도메인으로 구분된 제안서, 투표, 시간 초과 투표 및 최종 서명
- 관련 증명 높이에서 검증인 설정 해시 바인딩
- QC 및 최종 증명에서 고유한 알려진 서명자
- 검증인 모호성에 대한 책임 있는 증거
- 동일한 최종 높이에서 상충되는 커밋 결정 거부

## 암호 경계

- `deterministic` backend는 시험 전용이며 network safety 검사를 통과하지 못합니다.
- 공개 네트워크 시험과 출시 준비에는 `ed25519`를 사용할 수 있습니다.
- `bls`는 기본적으로 `blst-bls12381-minpk-v1`을 사용하며 proof-of-possession, subgroup 검사, public-key 검증, 의존성 감사와 release-gate 증거가 필요합니다.
- VRF adapter metadata는 네트워크 안전성 검사에 필요하지만, 이는 VRF committee가 활성 합의 경로라는 뜻이 아닙니다.

- 모든 검증인 홈에 대한 엄격한 구성 감사
- 릴리스 게이트 증거
- 외부 보안 검토
- 멀티 호스트 장기 숙박 및 혼돈의 증거
- 서명자/KMS 정책 증거
- 체인별 경제 및 거버넌스 정책 검토

릴리스를 프로덕션 준비 상태로 처리하기 전에 [보안 감사 준비] (./security/audit-readiness.md) 및 [릴리스 파이프라인] (./release/release-pipeline.md) 을 참조하십시오.

<!-- vexo-docs:technical-parity -->
## 기술적 동등성 부록

이 부록은 번역본이 영어 원문과 같은 인터페이스와 운영 경계를 유지하는지 확인하기 위한 요약입니다. 명령, 설정 키, RPC 경로, 코드 식별자는 번역하지 않습니다. 대신 의미를 한국어로 풀어 쓰고, 실제 배포에서 바뀌면 안 되는 값은 그대로 적습니다.
`require_network_safety` 와 `block_committed` 는 특히 그대로 유지해야 하는 핵심 용어입니다.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### 섹션 추적
- section: Model - HotStuff 스타일 BFT, three-chain finality, QC, timeout certificate, locked-QC safety를 함께 읽어야 합니다.
- section: Execution Terms - QC certified, finalized, executed, state committed의 차이를 구분해야 합니다.
- section: Safety Boundary - 1/3 미만 Byzantine, domain separation, validator-set hash binding, accountable evidence를 확인해야 합니다.
- section: Crypto Boundary - `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1`, `ecvrf-p256-sha256-tai-v1` 를 동일한 기준으로 봐야 합니다.
- section: Operational Boundary - `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled` 와 snapshot/replay health를 함께 봐야 합니다.

### 유지해야 할 인터페이스
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## 운영 메모

검증인 홈을 새로 만들 때는 `config.json` 과 분리된 `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, `log_config.json` 을 함께 점검해야 합니다. 실제 운영에서는 `vexo_quorum_health_ratio` 와 `adaptive_round_timeout_enabled` 를 함께 보고, 피어 수만으로 건강도를 판단하지 않는 것이 좋습니다.

- `execution_commit=finalized` 를 우선으로 둡니다.
- `qc` 경로는 통제된 시험망에서만 사용합니다.
- `recovery_finality_gate_enabled` 와 snapshot/replay health를 함께 확인합니다.
