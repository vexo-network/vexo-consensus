# 최종성 증명 형식

> Locale: ko · 한국어
> 이 문서는 영어 원문의 한국어 직역본입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.


## 먼저 읽을 순서

이 문서는 Finality Proof Format의 규범 규격을 설명합니다. 처음이라면 아래 순서로 읽는 것이 좋습니다.

1. Scope
2. Proof Fields
3. Header Fields
4. Quorum Certificate Fields
5. Commit Chain Fields
6. Verification Algorithm
7. Accountable Safety Detection
8. Ed25519 Model
9. BLS Model

이 순서는 먼저 범위와 상태를 이해하고, 그다음 메시지/안전/진행성 규칙을 확인한 뒤, 마지막으로 증거를 읽는 흐름과 같습니다.

## 문서 개요

이 문서는 finality proof 필드, 검증 순서, validator set binding을 이해하고 실제 구현·운영 판단에 연결하도록 돕습니다. 예제와 식별자는 구현 호환성을 위해 영어 표기를 유지하지만, 읽는 흐름과 운영상 판단 기준은 한국어로 설명합니다.

- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/ko/specs/finality-proof-format.md`

## 이 문서를 읽는 이유

- finality proof 필드, 검증 순서, validator set binding
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

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## 헤더 높이 필드

헤더 해시는 chain ID, 높이, 시간, 이전 블록 해시, app hash, validator set hash, next validator set hash, data availability 루트, 트랜잭션 루트, round, justify round, `validator_set_height`, `state_height`를 모두 덮습니다.

- `state_height`: app hash가 커밋하는 state를 만든 실행 블록 높이입니다.
- `validator_set_height`: 이 블록을 인증하는 validator set의 registry 높이입니다.
- `next_validator_set_hash`: 이 블록의 자식을 인증하는 set, 즉 블록 `state_height` 이후 registry가 가진 set의 해시입니다.

이 높이들은 부모 헤더만으로 정해지므로 모든 노드와 라이트 클라이언트가 같은 값을 얻습니다. 첫 블록은 genesis set(`validator_set_height = 1`)과 genesis state(`state_height = 0`)를 가리킵니다. 부모가 자기 justify 인증서 바로 다음 view에서 제안되었으면 자식의 `state_height`는 `parent.height - 1`(자식이 담은 부모 인증서가 2-chain 규칙으로 조부모를 커밋)이고, 아니면 부모의 `state_height`를 그대로 씁니다. 자식의 `validator_set_height`는 부모의 `state_height + 1`이고, 자식의 validator set hash는 부모의 next validator set hash와 같아야 합니다. 따라서 인증된 헤더는 자식 set을 그 set이 서명하기 전에 고정하고, validator set 변경은 그것을 만든 state가 커밋된 다음에 적용됩니다.

증명 검증은 `Proof.ValidatorSetHeight == Header.ValidatorSetHeight`를 요구하고, registry verifier는 헤더 해시가 덮는 `Header.ValidatorSetHeight`의 set을 불러옵니다. commit 자식 C의 QC는 부모의 `next_validator_set_hash`가 가리키는 set으로 검증합니다.

## 라이트 클라이언트 검증

`finality.LightClient`는 이미 신뢰하는 최종 블록 T에서 더 뒤의 블록 B로 신뢰를 옮깁니다. 이때 쓰는 데이터는 신뢰하지 않는 풀노드가 `/v1/light/{height}`로 제공하는 finality proof와, 그 헤더와 commit 자식 헤더가 가리키는 두 validator set입니다. `vexod light verify`가 이 절차를 RPC에 대해 실행합니다.

`finality.VerifyLightBlock`은 다음을 모두 만족할 때 B를 받아들입니다.

클라이언트가 신뢰하는 것은 T와, T의 헤더가 `next_validator_set_hash`로 가리키는 다음 set(T의 자식을 인증하는 set)입니다.

1. T가 trusting period 안에 있고, B의 시간이 T보다 뒤이며 로컬 시계보다 clock drift 넘게 앞서지 않습니다.
2. B의 2-chain 증명이 제공된 두 set으로 검증되고, 각 set의 해시가 헤더가 가리키는 값과 같습니다.
3. QC(B) 서명자 중 T의 다음 set에 같은 키로 있는 검증자가 그 set voting power의 1/3을 넘습니다.

신뢰 가정은 신뢰한 set의 결함 power가 trusting period 동안 1/3 미만이라는 것입니다. 그러면 정직한 검증자가 B에 투표한 것이고, 정직한 검증자는 자기 복제 state의 set들을 가리키는 헤더에만 투표하므로 B의 set과 B가 가리키는 다음 set은 진짜이며, 그 다음 set이 서명해야 하는 QC(C)도 체인의 것입니다. trusting period는 검증자가 set을 떠난 뒤에도 슬래싱 가능한 기간보다 짧아야 합니다.

겹치는 power가 부족하면 클라이언트는 중간 블록을 먼저 신뢰하는 bisection으로 진행합니다. 모든 헤더가 자식 set을 가리키므로 T 바로 다음 블록은 T의 다음 set 자체가 인증하고, 검사 3이 full quorum으로 성립합니다. 따라서 블록 사이 set이 얼마나 바뀌어도 bisection은 항상 목표에 도달하며, 정직한 source에 대해서는 검증되지 않는 블록에서만 멈춥니다.

## 영어 원문 구조

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Light Client Verification
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## 규범 원문

- [영어 정본 문서](../../en/specs/finality-proof-format.md)

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Scope — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Proof Fields — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Header Fields — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Quorum Certificate Fields — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Commit Chain Fields — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Verification Algorithm — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Accountable Safety Detection — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Ed25519 Model — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: BLS Model — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `finality.Proof` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/finality/latest` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/finality/{height}` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `strict: true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/status.latest_height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `/v1/finality/*` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Proof.ValidatorSetHeight <= Header.Height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Proof.ValidatorSetHash == loaded_set.Hash()` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Header.ValidatorSetHash == loaded_set.Hash()` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `QuorumCert.Height == Header.Height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Header.TxRoot` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `HeaderHash(link.Header)` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `finality.AttackDetector` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--validator-set` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blst-bls12381-minpk-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `supranational/blst` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus.TwoChainCommitRule` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `1..Header.Height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `0 < Proof.ValidatorSetHeight <= Header.Height` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `link.BlockHash == HeaderHash(link.Header)` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `link.Header.Height == Header.Height + 1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `link.Header.PreviousBlockHash == Proof.BlockHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `link.QuorumCert` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `link.QuorumCert.Round == QuorumCert.Round + 1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RuntimeSuite.RegisterValidators` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `state_height = parent.height - 1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Proof.ValidatorSetHash == Header.ValidatorSetHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Header.Height - 2` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `link.Header.ValidatorSetHash == Header.NextValidatorSetHash` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `link.Header.ValidatorSetHeight` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
