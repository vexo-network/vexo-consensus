> Locale: ko · 한국어

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- 비잔틴 투표력의 1/3 미만
- 도메인으로 구분된 제안서, 투표, 시간 초과 투표 및 최종 서명
- 관련 증명 높이에서 검증인 설정 해시 바인딩
- QC 및 최종 증명에서 고유한 알려진 서명자
- 검증인 모호성에 대한 책임 있는 증거
- 동일한 최종 높이에서 상충되는 커밋 결정 거부

# # 암호화폐 경계

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

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
