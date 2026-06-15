> Locale: ko · 한국어

# 맞춤형 암호화 백엔드 가이드

## 목표

이 가이드에서는 감사된 BLS 및 VRF 어댑터를 포함하여 사용자 지정 암호화 백엔드를 추가하는 방법을 설명합니다.

## 여기서 시작하세요

최단 경로만 필요한 경우:

1. 합의 서명자, 최종성 검증자, BLS 어댑터, VRF 어댑터 또는 원격 서명자가 필요한지 결정합니다.
2. 서명 코드를 연결하기 전에 도메인 분리 규칙을 확인하세요.
3. 사용하려는 백엔드에 대한 프로덕션 요구 사항을 읽어보세요.
4. 원격 서명자 정책 및 테스트 백엔드 경고를 검토하여 마무리합니다.

이 순서를 따르면 배포에 가장 중요한 안전 경계를 건너뛰지 않게 됩니다.

`vexo-consensus`은 어댑터 계약, 레지스트리 후크, 메타데이터 유효성 검사, 런타임 연결, `supranational/blst` BLS12-381 min-pk 어댑터, CIRCL 지원 BLS12-381 참조 어댑터 및 ECVRF P-256 어댑터를 제공합니다. 운영자는 가치 있는 배포를 위해 감사된 어댑터를 등록할 수 있으며 감사 증거, 키 보관 및 릴리스 게이트 검증은 배포 책임으로 남아 있습니다.

## 인터페이스

구현:
```go
type Signer interface {
    PublicKey() types.PublicKey
    Sign(message []byte) (types.Signature, error)
    Verify(publicKey types.PublicKey, message []byte, signature types.Signature) bool
}

type AggregateSigner interface {
    Aggregate(signatures []types.Signature) (types.AggregateSignature, error)
    VerifyAggregate(publicKeys []types.PublicKey, message []byte, signature types.AggregateSignature) bool
}
```
## 런타임 스위트

백엔드는 다음을 제공해야 합니다.

- 합의 서명자
- 최종 검증자
- 합의 수집자
- 키 검증
- 결정론적 직렬화

## 도메인 분리

모든 서명은 명시적인 도메인을 사용해야 합니다.

- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`

프로덕션 경로에서 직접 원시 메시지에 서명하지 마십시오.

## 프로덕션 BLS 요구 사항

BLS 어댑터에는 다음이 포함되어야 합니다.

- 감사된 라이브러리 종속성
- 공개 키 검증
- 서명 검증
- 하위 그룹 확인
- 소유 증명 또는 이와 동등한 불량 키 방어
- 도메인 분리
- 결정론적 집계 인코딩
- 어댑터 및 전이적 암호화 종속성에 대한 종속성 감사
- 테스트 벡터
- 잘못된 키/서명에 대한 퍼지 테스트

프로덕션 BLS는 `BLSAdapter`을 통해 등록되며 서명자 또는 런타임 최종성 백엔드로 사용되기 전에 `ValidateBLSAdapter`을 전달해야 합니다. 어댑터 메타데이터는 감사 상태, 감사 보고서 ID, 종속성 감사 ID, 공개 키 유효성 검사, 하위 그룹 검사, 불량 키 방어, 결정론적 인코딩, 잘못된 입력 퍼즈 범위 및 소유 증명 지원을 선언해야 합니다. 이제 네트워크 안전 게이트에는 공개/가치 보유 구성을 위한 BLS가 필요합니다. Ed25519는 집계 최종성 확인을 제공할 수 없기 때문에 의도적으로 해당 게이트 외부에 있습니다.

메타데이터 등록은 실제 감사된 구현을 대체할 수 없습니다. 어댑터 패키지는 실제 하위 그룹 검사, 키 유효성 검사, 소유 증명 확인, 서명 확인, 집계 확인 및 잘못된 형식의 입력 거부를 수행해야 합니다.

어댑터 패키지는 `init()`의 구현을 등록해야 합니다.
```go
func init() {
    crypto.RegisterBLSAdapter("audited-bls-v1", func() (crypto.BLSAdapter, error) {
        return NewAuditedBLSAdapter()
    })
}
```
`crypto.adapter_name`은(는) `BLSAdapter.Metadata().Name`과(와) 일치해야 합니다. 그렇지 않으면 런타임 시작이 실패합니다. 이렇게 하면 감사된 구현이 실제로 바이너리에 연결되지 않은 구성 전용 "BLS 활성화" 상태가 방지됩니다. `crypto.audit_evidence_sha256`는 릴리스 도구가 구성을 외부 BLS 감사 아티팩트에 바인딩할 수 있도록 32바이트 16진수 다이제스트여야 합니다.

유효성 검사기 공개 키는 `BLSValidatorCredential` 레코드 또는 유효성 검사기 메타데이터 키 `bls_pop`를 통해 허용되어야 합니다. `ValidateBLSValidatorCredentials`은(는) 누락된 ID, 누락된 키, 중복된 공개 키, 유효하지 않은 키, 유효하지 않은 소유 증명 값을 거부합니다. `NewBLSAggregateVerifier`은 감사된 어댑터를 래핑하므로 최종성 확인은 등록된 유효성 검사기 키만 허용합니다.

기본 프로덕션 지향 어댑터는 `blst-bls12381-minpk-v1`입니다. `github.com/supranational/blst`, 압축된 min-pk 인코딩, G1의 공개 키, G2의 서명, 소유 증명 확인, 하위 그룹 유효성 검사, 도메인 분리 서명 및 동일한 메시지 최종 투표를 위한 빠른 집계 확인을 사용합니다. `blst` 바인딩은 cgo 기반입니다. 따라서 공개/가치 포함 릴리스 아티팩트에는 각 대상에 적합한 C 도구 체인인 `RELEASE_CGO_ENABLED=1`, 유효성 검사기 키/PoP 증거 및 릴리스 후보에 첨부된 어댑터 감사 증거가 필요합니다. `RELEASE_REQUIRE_BLS=1` 및 cgo가 비활성화되면 Makefile이 닫히지 않으므로 BLS가 없는 휴대용 연기 아티팩트를 BLS 지원 네트워크 바이너리로 착각할 수 없습니다. 개인 연기 유물에만 `make release-portable RELEASE_REQUIRE_BLS=0`을 사용하세요. 내장된 CIRCL 어댑터는 참조 및 호환성 테스트를 위해 여전히 `circl-bls12381-g1sigg2-basic-v1`로 등록되어 있지만 의도적으로 네트워크 안전 게이트에서 프로덕션 BLS 어댑터로 허용되지 않습니다. 구성 메타데이터는 안전하지 않은 어댑터를 감사된 어댑터로 승격할 수 없습니다. `NewBLSTBLSKeyDocument` 및 `NewCIRCLBLSKeyDocument`는 둘 다 `bls_proof_of_possession` 메타데이터를 작성하므로 검증자 생성 메타데이터는 악성 키 방어 증명을 전달할 수 있습니다.

CLI 도우미:
```bash
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
vexod init validator --home .vexo-validator --validator validator-1 --key-type bls
```
초기화 흐름은 키 문서의 `bls_proof_of_possession`을 제네시스 메타데이터 키 `bls_pop`로 복사합니다.

## 프로덕션 VRF 요구 사항

VRF 지원 위원회 선택은 동일한 등록 패턴을 사용합니다.
```go
func init() {
    crypto.RegisterVRFAdapter("audited-vrf-v1", func(cfg config.VRFConfig) (crypto.VRFAdapter, error) {
        return NewAuditedVRFAdapter(cfg)
    })
}
```
`vrf.adapter_name`, `vrf.audit_report`, `vrf.dependency_audit`, `vrf.audit_evidence_sha256` 및 `vrf.key_source`는 어댑터 메타데이터 및 릴리스 증거와 일치해야 합니다. `committee.backend`이 `vrf`인 경우 자동으로 결정적 VRF로 돌아가는 대신 일치하는 어댑터가 연결되지 않으면 런타임 시작이 실패합니다. 결정적 VRF로 대체되어서는 안 되는 SDK 통합은 `production_adapter: true`이 없는 구성을 거부하는 `crypto.NewProductionVRF`을 호출해야 합니다. 위원회 선택이 결정적인 경우 런타임은 VRF 어댑터를 로드하지 않습니다.

내장된 ECVRF 어댑터는 `ecvrf-p256-sha256-tai-v1`로 등록됩니다. P-256/SHA-256 시도 및 증분 ECVRF 증명을 사용합니다. 유효성 검사기는 메타데이터 키 `vrf_public_key`에 base64 VRF 공개 키를 넣을 수 있습니다. 그렇지 않으면 위원회 선택이 검증인 합의 공개 키로 대체됩니다.

내장된 원격 VRF 어댑터는 `remote-vrf-http-v1`로 등록됩니다. `vrf.adapter_name`를 해당 값으로 설정하고 `vrf.key_source`을 `remote-http:<base-url>` 또는 일반 HTTPS 기본 URL로 설정합니다. 어댑터는 base64 `public_key`, `seed`, 임의의 `nonce`, `issued_at_unix_nano`, `deadline_unix_nano` 및 도메인 `vexo.remote_vrf.prove.v1`을 사용하여 `POST /prove`을 호출합니다. 응답은 동일한 `nonce`를 에코하고 base64 `output`과 `proof`를 반환해야 합니다. 동일한 챌린지 필드와 도메인 `vexo.remote_vrf.verify.v1`을 사용하여 `POST /verify`을 호출합니다. 서비스가 `{ "valid": true, "nonce": "<same nonce>" }`을 반환하는 경우에만 확인이 성공합니다. `VEXO_REMOTE_VRF_TOKEN` 환경 변수가 설정된 경우 요청에는 `Authorization: Bearer <token>`이 포함됩니다. `vrf.tls_cert_path`, `vrf.tls_key_path`, `vrf.tls_ca_path` 및 `vrf.tls_server_name`은 원격 증명자에 대해 mTLS 또는 고정된 CA 검증을 활성화합니다. 인증서/키는 함께 구성해야 합니다. 유효하지 않은 TLS 자료는 자동으로 인증되지 않은 HTTP로 돌아가는 대신 어댑터 구성에 실패합니다. 이는 KMS/HSM 지원 VRF 보관을 위해 선호되는 통합 지점이지만 원격 서비스에는 여전히 `vrf.audit_report`와 일치하는 독립적인 감사 증거와 가용성, 재생 보호, nonce/감사 로깅, TLS/mTLS, 권한 부여 및 키 액세스 정책에 대한 운영 증거가 필요합니다.

원격 VRF 구현은 합의 또는 RPC 기한에 따라 선택이 이루어질 때마다 상황 인식 `ProveWithContext` 및 `VerifyWithContext` 메서드를 사용해야 합니다. 레거시 `Prove` 및 `Verify` 메서드는 편의 래퍼입니다. 생산 경로는 취소를 전파해야 느린 원격 증명자가 블록 또는 요청 시간 초과보다 오래 지속될 수 없습니다.

암호화된 로컬 ECVRF 키로 지원되는 내장 참조 서비스의 경우 `keys serve-vrf`를 사용하세요.
```bash
VEXO_KEY_PASSPHRASE='change-me' \
vexod keys serve-vrf \
  --home /var/lib/vexo/validator-1 \
  --listen 127.0.0.1:9100 \
  --auth-token-env VEXO_REMOTE_VRF_TOKEN
```
이 서비스는 `POST /prove` 및 `POST /verify`만 노출합니다. `--auth-token` 또는 `--auth-token-env`이 설정된 경우 누락된 전달자 토큰을 거부하고, 챌린지 도메인/기한 필드의 유효성을 검사하고, `vexod keys serve-vrf`을 통해 실행될 때 지속 가능한 재생/감사 증거를 기록합니다. 임베디드 서비스는 `crypto.NewRemoteVRFService`를 통해 자체 감사 싱크 및 재생 저장소를 계속 제공할 수 있습니다. 기본 제공 명령은 단일 호스트 검증 및 제어된 배포에 유용합니다. 공개 검증인 관리는 동일한 HTTP 계약과 내구성 있는 nonce/감사 저장소를 갖춘 HSM/KMS 지원 서비스를 사용해야 합니다.

기본적으로 `vexod keys serve-vrf`은(는) 이제 `--home`: `remote-vrf-nonces.jsonl` 및 `remote-vrf-audit.jsonl`에서 내구성 있는 재생 및 감사 파일을 사용합니다. VRF 서비스가 별도의 서비스 계정 또는 마운트된 볼륨으로 실행되는 경우 `--nonce-path` 및 `--audit-log`로 재정의합니다. 내장된 KMS/HSM 서비스는 `crypto.RemoteVRFServiceConfig.ReplayStore`를 제공하고 `RequireDurableReplayStore: true`을 설정해야 합니다. 내장된 `crypto.NewFileRemoteVRFReplayStore`는 참조 구현입니다. 이렇게 하면 재시작 시 동일한 챌린지 nonce를 다시 수락하는 것을 방지할 수 있습니다.

원격 증명자를 `consensus_config.json`에 연결하기 전에 처음부터 끝까지 확인하십시오.
```bash
vexod keys verify-vrf \
  --url https://vrf.example.internal \
  --public-key <base64-vrf-public-key> \
  --seed release-candidate-check \
  --auth-token-env VEXO_REMOTE_VRF_TOKEN
```
`consensus_config.json`에서 참조된 암호화된 VRF 키 문서를 선호합니다.
```json
{
  "vrf_key_paths": ["validator.vrf.key.json"],
  "vrf": {
    "adapter_name": "ecvrf-p256-sha256-tai-v1",
    "audit_report": "operator-audit-reference",
    "dependency_audit": "github.com/vechain/go-ecvrf@v0.0.0-20251211112124-5d5a3ef70fc9",
    "audit_evidence_sha256": "<sha256-of-vrf-audit-evidence>",
    "key_source": "config.vrf.keys",
    "production_adapter": true
  }
}
```
원격 VRF 증명자의 경우 HTTPS와 mTLS/고정 CA를 선호합니다.
```json
{
  "vrf": {
    "adapter_name": "remote-vrf-http-v1",
    "audit_report": "operator-remote-vrf-audit",
    "dependency_audit": "external:remote-vrf-service-audit-2026",
    "audit_evidence_sha256": "<sha256-of-remote-vrf-audit-evidence>",
    "key_source": "remote-http:https://vrf.example.internal",
    "production_adapter": true,
    "tls_cert_path": "vrf-client.crt",
    "tls_key_path": "vrf-client.key",
    "tls_ca_path": "vrf-ca.pem",
    "tls_server_name": "vrf.example.internal"
  }
}
```
다음을 사용하여 암호화된 VRF 키 문서를 생성합니다.
```bash
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
시작 시 `vexod`은(는) `consensus_config.json`이 포함된 디렉터리에서 상대 `vrf_key_paths`을 확인하고, `VEXO_KEY_PASSPHRASE`을 통해 암호화된 키 문서를 해독하고 개인 키를 런타임 VRF 어댑터에 삽입합니다. 직접 `vrf.keys`은 테스트 또는 사용자 정의 로더에 계속 사용할 수 있지만 운영자는 구성 파일에 원시 개인 스칼라를 저장하지 않아야 합니다. 공개 가치 보유 네트워크의 경우 원격 서명자/KMS 지원 VRF 증명자는 여전히 로컬 키 보관보다 선호됩니다.

## 원격 서명자 요구 사항

원격 서명자는 자체 정책 튜플을 시행해야 합니다.
```text
(chain_id, height, round, type, domain)
```
노드 프로세스가 다시 시작되거나 손상된 경우에도 동일한 튜플에 대해 충돌하는 메시지를 거부해야 합니다.

`vexo-consensus`은(는) 노드 측 및 HTTP KMS/HSM `DoubleSignGuard` 도우미와 내구성 있는 원격 서명자 nonce 재생 가드도 제공합니다. 기본 제공 서비스의 경우 내구성이 뛰어난 `--guard-path` 및 `--nonce-path`를 사용하여 `vexod keys serve-remote`을 실행하세요. 외부 프로덕션 KMS/HSM 구현은 동등한 지속성 정책 및 재생 임시 데이터베이스를 유지해야 합니다. 가드 키에는 도메인 분리가 포함됩니다.
```text
chain_id/height/round/type/domain
```
유효한 서명 유형 및 도메인 쌍은 다음과 같습니다.

- `consensus_proposal` → `vexo.consensus.proposal.v1`
- `consensus_vote` → `vexo.consensus.vote.v1`
- `consensus_timeout_vote` → `vexo.consensus.timeout_vote.v1`
- `finality_proof` → `vexo.finality.proof.v1`

## 테스트 백엔드

`deterministic`은 테스트 전용입니다. 네트워크 안전 검증을 통과해서는 안 되며 가치를 지닌 배포에 사용되어서는 안 됩니다.

<!-- vexo-docs:technical-parity -->
## 기술 동등성 부록

이 부록은 영어 정본의 실행 가능한 인터페이스와 핵심 섹션을 번역본에서도 빠뜨리지 않기 위한 검증용 요약입니다. 명령어, 설정 키, RPC 메서드, 패키지 이름은 모든 언어에서 그대로 유지합니다.

### 섹션 추적
- section: Goal — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Interfaces — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Runtime Suite — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Domain Separation — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Production BLS Requirements — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Production VRF Requirements — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Remote Signer Requirements — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.
- section: Test Backends — 이 섹션은 설정값, 검증 증거, 실패 조건, 운영자가 취해야 할 조치를 함께 확인해야 합니다.

### 그대로 유지되는 인터페이스
- `vexo-consensus` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `supranational/blst` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo.consensus.proposal.v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo.consensus.vote.v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo.consensus.timeout_vote.v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo.finality.proof.v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `crypto.adapter_name` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `BLSAdapter.Metadata().Name` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `crypto.audit_evidence_sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bls_pop` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `blst-bls12381-minpk-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `github.com/supranational/blst` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RELEASE_CGO_ENABLED=1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RELEASE_REQUIRE_BLS=1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `make release-portable RELEASE_REQUIRE_BLS=0` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `circl-bls12381-g1sigg2-basic-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `bls_proof_of_possession` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.adapter_name` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.audit_report` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.dependency_audit` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.audit_evidence_sha256` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.key_source` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `committee.backend` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `crypto.NewProductionVRF` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `production_adapter: true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `ecvrf-p256-sha256-tai-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf_public_key` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `remote-vrf-http-v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `remote-http:<base-url>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `POST /prove` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `public_key` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `issued_at_unix_nano` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `deadline_unix_nano` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo.remote_vrf.prove.v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `POST /verify` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexo.remote_vrf.verify.v1` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `{ "valid": true, "nonce": "<same nonce>" }` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `VEXO_REMOTE_VRF_TOKEN` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `Authorization: Bearer <token>` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.tls_cert_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.tls_key_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.tls_ca_path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.tls_server_name` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `keys serve-vrf` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--auth-token` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--auth-token-env` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod keys serve-vrf` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `crypto.NewRemoteVRFService` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--home` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `remote-vrf-nonces.jsonl` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `remote-vrf-audit.jsonl` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--nonce-path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--audit-log` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `crypto.RemoteVRFServiceConfig.ReplayStore` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `RequireDurableReplayStore: true` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `crypto.NewFileRemoteVRFReplayStore` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_config.json` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf_key_paths` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `VEXO_KEY_PASSPHRASE` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vrf.keys` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `vexod keys serve-remote` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `--guard-path` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_proposal` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_vote` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `consensus_timeout_vote` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
- `finality_proof` — 이 이름은 실행 예제와 설정 검증에서 그대로 사용되므로 번역하지 않습니다.
