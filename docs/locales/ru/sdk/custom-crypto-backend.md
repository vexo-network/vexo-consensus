# Custom Crypto Backend Guide

> Locale: ru · Русский
> Этот документ — русский сопроводительный документ к английскому источнику. Протокол, безопасность и решения о release остаются нормативными на английском языке.

## Обзор

Этот документ помогает понять подключение custom crypto backend, включая BLS, VRF и signer и связать это с решениями по реализации и эксплуатации.

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/ru/sdk/custom-crypto-backend.md`

## Зачем читать этот документ

- подключение custom crypto backend, включая BLS, VRF и signer
- Сначала проверьте MUST/SHOULD/MAY в английском источнике.
- Этот локализованный документ помогает пониманию; audit, release и security решения принимаются по английскому источнику.

## Что нужно уметь после чтения

- Объяснить, какое решение по реализации или эксплуатации поддерживает этот документ.
- Связать нормативные требования английского источника с текущей конфигурацией сети.
- Перед копированием примеров проверить chain ID, validator ID, fee/gas и peer-адреса.

## Чеклист безопасного использования

- Сначала проверьте MUST/SHOULD/MAY в английском источнике.
- Не переводите команды, config key, имена RPC, JSON-поля и идентификаторы кода.
- Перед копированием примеров адаптируйте chain ID, validator ID, fee/gas и peer-адреса к своей сети.
- После изменений выполните `make docs-check`, чтобы проверить locale tree и translation guards.

## На что обратить внимание

- Этот локализованный документ помогает пониманию; audit, release и security решения принимаются по английскому источнику.
- При изменении реализации обновляйте английский источник и все локализованные документы в одном изменении.

## Интерфейсы, которые нужно сохранить

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `ecvrf-p256-sha256-tai-v1`
- `remote-vrf-http-v1`
## Структура английского источника

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## VRF audit evidence SHA-256

VRF backend должен показывать такую же чёткую audit boundary, как BLS. Заполните `vrf.adapter_name`, `vrf.audit_report`, `vrf.dependency_audit`, `vrf.audit_evidence_sha256` и `vrf.key_source`; если metadata adapter не совпадают с config, runtime должен fail closed. Встроенный ECVRF adapter проверяет go.mod dependency pin и audit evidence digest, а remote VRF adapter использует внешнюю KMS/HSM audit reference.

## Канонический источник

- [Английский канонический документ](../../en/sdk/custom-crypto-backend.md)

## Remote VRF service

`vexod keys serve-vrf` публикует `POST /prove` и `POST /verify` на базе ECVRF key, а `vexod keys verify-vrf` проверяет remote prover end-to-end. Имена `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1` и `vexo.remote_vrf.verify.v1` не переводятся.

Keep these interface names unchanged: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.
