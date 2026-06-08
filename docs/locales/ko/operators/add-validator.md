# Adding a Validator

> Locale: ko · 한국어
> 이 문서는 영어 원문을 기준으로 작성된 한국어 번역 가이드입니다. 프로토콜, 보안, 릴리즈 판단은 영어 원문이 규범입니다.

## 목적

이 문서는 다음 내용을 다룹니다: validator 추가 절차, 설정 검증, staking 확인. 구현과 운영에서 쓰는 명령어, JSON 필드, RPC 이름, config key, 코드 식별자는 호환성을 위해 영어 원문 표기를 유지합니다.

## 핵심 범위

- 아래 항목은 이 문서를 읽을 때 반드시 확인해야 하는 내용입니다. 명령어, JSON 필드, RPC 메서드, config key, 코드 식별자는 호환성을 위해 원문 그대로 유지합니다.
- 상세한 규범 문장은 영어 원문을 기준으로 검토하세요.
- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/ko/operators/add-validator.md`

## 보존해야 할 식별자

- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `bls_pop`
- `.vexo-validator-new/network_config.json`
- `network_config.json`
- `p2p.listen_address`
- `rpc.address`
- `p2p.peers`
- `p2p_address`
- `rpc_address`
- `active_from`
- `active_until`
- `config audit --strict`

## 영어 원문 섹션

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## 운영 참고

- `MUST`, `SHOULD`, `MAY`, 명령어 예시, JSON 예시, RPC 이름은 영어 표기를 유지합니다.
- 이 번역을 변경한 뒤에는 `make docs-check`를 실행하세요.
- 이 문서와 영어 원문이 충돌하면 영어 원문을 기준으로 하고 같은 변경에서 이 locale 파일도 갱신하세요.

## 규범 원문

- [English canonical document](../../en/operators/add-validator.md)
