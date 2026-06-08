# Node Initialization

> Locale: ru · Русский
> Этот документ является переводным руководством на основе канонической английской документации. Протокол, безопасность и решения о релизе остаются нормативными на английском языке.

## Назначение

Этот документ описывает инициализацию archive и validator узлов и работу с разделёнными config-файлами. Команды, JSON-поля, имена RPC, config key и идентификаторы кода, используемые в реализации и эксплуатации, сохраняются на английском для совместимости.

## Основной охват

- При чтении проверьте следующие пункты. Команды, JSON-поля, RPC-методы, ключи конфигурации и идентификаторы кода сохраняются на английском для совместимости.
- Подробные нормативные формулировки смотрите в английском оригинале.
- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/ru/operators/node-initialization.md`

## Сохраняемые идентификаторы

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
- `--evm-account-key`
- `validator_id`
- `init validator`
- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `--encrypt-keys`
- `validator.key.json`
- `validator.vrf.key.json`
- `--key-type bls`
- `genesis.json`
- `bls_pop`

## Разделы английского оригинала

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## Операционные заметки

- `MUST`, `SHOULD`, `MAY`, примеры команд, JSON-примеры и имена RPC сохраняют английское написание.
- После изменения этого перевода выполните `make docs-check`.
- Если эта страница противоречит английскому источнику, используйте английский источник и обновите этот locale-файл в том же изменении.

## Канонический источник

- [English canonical document](../../en/operators/node-initialization.md)
