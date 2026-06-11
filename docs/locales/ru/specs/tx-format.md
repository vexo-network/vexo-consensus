# Transaction Format

> Locale: ru · Русский
> Этот документ — русский сопроводительный документ к английскому источнику. Протокол, безопасность и решения о release остаются нормативными на английском языке.

## Обзор

Этот документ помогает понять transaction format, signing, fee и правила gas и связать это с решениями по реализации и эксплуатации.

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/ru/specs/tx-format.md`

## Зачем читать этот документ

- transaction format, signing, fee и правила gas
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
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## Структура английского источника

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

## Канонический источник

- [Английский канонический документ](../../en/specs/tx-format.md)
