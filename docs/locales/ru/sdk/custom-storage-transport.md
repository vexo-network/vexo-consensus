# Custom Storage and Transport Guide

> Locale: ru · Русский
> Этот документ является переводным руководством на основе канонической английской документации. Протокол, безопасность и решения о релизе остаются нормативными на английском языке.

## Назначение

Этот документ описывает реализацию и регистрацию custom storage и transport adapter. Команды, JSON-поля, имена RPC, config key и идентификаторы кода, используемые в реализации и эксплуатации, сохраняются на английском для совместимости.

## Основной охват

- При чтении проверьте следующие пункты. Команды, JSON-поля, RPC-методы, ключи конфигурации и идентификаторы кода сохраняются на английском для совместимости.
- Подробные нормативные формулировки смотрите в английском оригинале.
- Canonical path: `docs/sdk/custom-storage-transport.md`
- Locale path: `docs/locales/ru/sdk/custom-storage-transport.md`

## Сохраняемые идентификаторы

- `store.Store`
- `store.HistoricalSnapshotKVStore`
- `store.SnapshotKVStore`
- `transport.Transport`

## Разделы английского оригинала

- Custom Storage and Transport Guide
- Custom Storage
- Storage Requirements
- Custom Transport
- Transport Requirements
- Compatibility

## Операционные заметки

- `MUST`, `SHOULD`, `MAY`, примеры команд, JSON-примеры и имена RPC сохраняют английское написание.
- После изменения этого перевода выполните `make docs-check`.
- Если эта страница противоречит английскому источнику, используйте английский источник и обновите этот locale-файл в том же изменении.

## Канонический источник

- [English canonical document](../../en/sdk/custom-storage-transport.md)
