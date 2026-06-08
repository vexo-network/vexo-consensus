# Adding a Validator

> Locale: ru · Русский
> Этот документ является переводным руководством на основе канонической английской документации. Протокол, безопасность и решения о релизе остаются нормативными на английском языке.

## Назначение

Этот документ описывает добавление validator, проверку конфигурации и staking-контроль. Команды, JSON-поля, имена RPC, config key и идентификаторы кода, используемые в реализации и эксплуатации, сохраняются на английском для совместимости.

## Основной охват

- При чтении проверьте следующие пункты. Команды, JSON-поля, RPC-методы, ключи конфигурации и идентификаторы кода сохраняются на английском для совместимости.
- Подробные нормативные формулировки смотрите в английском оригинале.
- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/ru/operators/add-validator.md`

## Сохраняемые идентификаторы

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

## Разделы английского оригинала

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## Операционные заметки

- `MUST`, `SHOULD`, `MAY`, примеры команд, JSON-примеры и имена RPC сохраняют английское написание.
- После изменения этого перевода выполните `make docs-check`.
- Если эта страница противоречит английскому источнику, используйте английский источник и обновите этот locale-файл в том же изменении.

## Канонический источник

- [English canonical document](../../en/operators/add-validator.md)
