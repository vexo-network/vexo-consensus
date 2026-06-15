> Locale: ko · 한국어

# 유효성 검사기 추가

이 가이드는 Vexo 네트워크에 검증인을 추가하기 위한 운영자 흐름을 설명합니다.

정확한 승인 경로는 체인의 스테이킹 및 거버넌스 정책에 따라 다릅니다. 최소한 유효성 검사기는 체인 상태로 표시되고 유효한 자격 증명이 있어야 하며 높이 버전의 유효성 검사기 집합 업데이트의 일부가 되어야 합니다.

## 1. 검증인 홈 초기화
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
BLS 유효성 검사기 키의 경우:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
이러한 명령을 실행하기 전에 `VEXO_KEY_PASSPHRASE`을 설정하거나 일회성 로컬 설정을 위해 `--passphrase`을 전달하세요.

BLS 검증자를 기존 체인에 허용할 때 생성된 `bls_pop` 메타데이터를 검증자 업데이트 제안에 포함하세요.
기본 BLS 키 경로는 `blst-bls12381-minpk-v1`을 사용합니다. 참조/호환성 테스트에만 `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1`를 사용하세요.

생성된 공개 키를 보관합니다.
```bash
vexod keys show --home .vexo-validator-new --json
```
또한 생성된 `node.key.json`을 유지하세요. `network_config.json:p2p.node_id`에 대한 P2P 핸드셰이크에 서명합니다. 이는 검증인 합의 키가 아니므로 계정 키로 재사용해서는 안 됩니다.

## 2. 네트워크 주소 및 피어 구성

`.vexo-validator-new/network_config.json`를 편집하고 로컬 수신 주소와 영구 피어를 설정합니다.
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
프로덕션 유효성 검사기에 대해 수명이 긴 명령줄 네트워킹 재정의에 의존하지 마십시오. `network_config.json`에 영구 피어 주소를 유지하세요.

별도의 주소 역할을 사용하십시오.

- `p2p.listen_address` 및 `rpc.address`는 이 머신 또는 컨테이너의 로컬 바인드 주소입니다.
- `p2p.node_id`은 이 노드의 피어 ID입니다. 동료가 배운 후에도 안정적으로 유지하십시오.
- `p2p.node_key_path`는 해당 피어 ID에 대한 로컬 핸드셰이크 서명 키를 가리킵니다.
- `p2p.peers`에는 이 노드가 다른 피어에 연결하는 데 사용하는 다이얼 대상이 포함되어 있습니다. 맵 키는 원격 노드의 `p2p.node_id` 값이어야 합니다.
- 네트워크가 의도적으로 비공개인 경우를 제외하고 유효성 검사기 메타데이터 `p2p_address` 및 `rpc_address`에는 Docker 전용 서비스 이름이 아닌 공개 광고 주소가 포함되어야 합니다.

## 3. 검증인 입학 허가서 제출

스테이킹 흐름의 예를 들어 스테이킹 트랜잭션을 구축합니다.
```bash
vexod staking --help
```
검증인 승인 거래에는 다음이 포함되어야 합니다.

- 검증자 ID
- 검증인 주소
- 합의 공개 키
- 투표권 또는 스테이크 참조
- 체인이 셀프 서비스 커미션 업데이트를 허용하는 경우 검증자 커미션 기준 포인트
- 체인이 피어 맵을 미리 설정하기 위해 제네시스/검증기 메타데이터를 사용하는 경우 P2P `node_id` 메타데이터
- 공개 P2P 주소 메타데이터
- 공개 RPC 주소 메타데이터(공개인 경우)
- BLS가 활성화된 경우 BLS 소유 증명 메타데이터

유효성 검사기 업데이트는 특정 높이에서 유효해야 하며 새로운 유효성 검사기 세트 해시를 생성해야 합니다.

검증인이 활성화된 후 운영자는 스테이킹 모듈을 통해 보상 상태를 노출할 수 있습니다.
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. 검증인 세트 업데이트 확인

업데이트 후 높이:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
확인:

- 키별 세트에 유효성 검사기가 나타납니다.
- 투표권이 정확하다
- 유효성 검사기 세트 해시가 예상대로 변경되었습니다.
- 최종 증명은 올바른 유효성 검사기 설정 높이를 참조합니다.

## 5. 계획 유효성 검사기 키 순환

겹치지 않는 `active_from` 및 `active_until` 메타데이터를 사용하여 다음 키 문서를 준비한 후 추가 회전 키로 노드를 시작하여 유효성 검사기 키를 회전할 수 있습니다.
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
서명 시 노드는 활성 창에 합의 높이가 포함된 키를 사용합니다. 원격 서명자 키 문서는 동일한 정책, 인증 토큰 및 이중 서명 보호 요구 사항을 유지합니다.

## 6. 유효성 검사기 시작
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
시작에는 네트워크 모드 스위치가 없습니다. 네트워크가 공용 네트워크 안전 가정을 충족할 것으로 예상되는 경우 시작하기 전에 `config audit --strict`을(를) 사용하세요.

## 7. 모니터

시청:

- 제안/투표 지연
- 라운드 타임아웃
- 검증인 서명 실패
- 동료 금지
- 멤풀 크기
- 커밋 대기 시간
- 스냅샷/재생 상태

사용:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## 안전 참고사항

- 독립 체인 전체에서 유효성 검사기 키를 재사용하지 마세요.
- 프로덕션 유효성 검사기에 대해 원격 서명자 정책을 활성화된 상태로 유지합니다.
- 소유 증명이나 이에 상응하는 불량 키 방어 없이 BLS 검증자를 인정하지 마십시오.
- 올바른 증거 높이 검증자 세트에 연결된 검증된 증거 없이 검증자를 베거나 감옥에 가두지 마십시오.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: 1. Initialize Validator Home — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: 2. Configure Network Addresses and Peers — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: 3. Submit Validator Admission — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: 4. Verify Validator Set Update — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: 5. Plan Validator Key Rotation — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: 6. Start Validator — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: 7. Monitor — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Safety Notes — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `VEXO_KEY_PASSPHRASE` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--passphrase` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bls_pop` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blst-bls12381-minpk-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `node.key.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `network_config.json:p2p.node_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `.vexo-validator-new/network_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `network_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.listen_address` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc.address` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.node_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.node_key_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p.peers` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `p2p_address` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `rpc_address` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `node_id` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `active_from` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `active_until` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `config audit --strict` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
