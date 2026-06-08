# Custom Crypto Backend Guide

> Locale: ru · Русский
> Этот документ является переводным руководством на основе канонической английской документации. Протокол, безопасность и решения о релизе остаются нормативными на английском языке.

## Назначение

Этот документ описывает подключение custom crypto backend, включая BLS, VRF и signer. Команды, JSON-поля, имена RPC, config key и идентификаторы кода, используемые в реализации и эксплуатации, сохраняются на английском для совместимости.

## Основной охват

- При чтении проверьте следующие пункты. Команды, JSON-поля, RPC-методы, ключи конфигурации и идентификаторы кода сохраняются на английском для совместимости.
- Подробные нормативные формулировки смотрите в английском оригинале.
- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/ru/sdk/custom-crypto-backend.md`

## Сохраняемые идентификаторы

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
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`

## Разделы английского оригинала

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## Операционные заметки

- `MUST`, `SHOULD`, `MAY`, примеры команд, JSON-примеры и имена RPC сохраняют английское написание.
- После изменения этого перевода выполните `make docs-check`.
- Если эта страница противоречит английскому источнику, используйте английский источник и обновите этот locale-файл в том же изменении.

## Канонический источник

- [English canonical document](../../en/sdk/custom-crypto-backend.md)
