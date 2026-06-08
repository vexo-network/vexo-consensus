# Transaction Format

> Locale: ru · Русский
> Этот документ является переводным руководством на основе канонической английской документации. Протокол, безопасность и решения о релизе остаются нормативными на английском языке.

## Назначение

Этот документ описывает transaction format, signing, fee и правила gas. Команды, JSON-поля, имена RPC, config key и идентификаторы кода, используемые в реализации и эксплуатации, сохраняются на английском для совместимости.

## Основной охват

- При чтении проверьте следующие пункты. Команды, JSON-поля, RPC-методы, ключи конфигурации и идентификаторы кода сохраняются на английском для совместимости.
- Подробные нормативные формулировки смотрите в английском оригинале.
- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/ru/specs/tx-format.md`

## Сохраняемые идентификаторы

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`

## Разделы английского оригинала

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## Операционные заметки

- `MUST`, `SHOULD`, `MAY`, примеры команд, JSON-примеры и имена RPC сохраняют английское написание.
- После изменения этого перевода выполните `make docs-check`.
- Если эта страница противоречит английскому источнику, используйте английский источник и обновите этот locale-файл в том же изменении.

## Канонический источник

- [English canonical document](../../en/specs/tx-format.md)
