# EVM and Native Accounting

> Locale: ru · Русский
> Этот документ является переводным руководством на основе канонической английской документации. Протокол, безопасность и решения о релизе остаются нормативными на английском языке.

## Назначение

Этот документ описывает согласование native coin и EVM gas/accounting. Команды, JSON-поля, имена RPC, config key и идентификаторы кода, используемые в реализации и эксплуатации, сохраняются на английском для совместимости.

## Основной охват

- При чтении проверьте следующие пункты. Команды, JSON-поля, RPC-методы, ключи конфигурации и идентификаторы кода сохраняются на английском для совместимости.
- Подробные нормативные формулировки смотрите в английском оригинале.
- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/ru/specs/evm-native-accounting.md`

## Сохраняемые идентификаторы

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`

## Разделы английского оригинала

- EVM and Native Accounting
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM Execution
- Compatibility Boundary
- Failure Modes

## Операционные заметки

- `MUST`, `SHOULD`, `MAY`, примеры команд, JSON-примеры и имена RPC сохраняют английское написание.
- После изменения этого перевода выполните `make docs-check`.
- Если эта страница противоречит английскому источнику, используйте английский источник и обновите этот locale-файл в том же изменении.

## Канонический источник

- [English canonical document](../../en/specs/evm-native-accounting.md)
