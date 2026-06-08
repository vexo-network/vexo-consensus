# Security Audit Readiness

> Locale: ru · Русский
> Этот документ является переводным руководством на основе канонической английской документации. Протокол, безопасность и решения о релизе остаются нормативными на английском языке.

## Назначение

Этот документ описывает threat model, предположения безопасности и доказательства для аудита. Команды, JSON-поля, имена RPC, config key и идентификаторы кода, используемые в реализации и эксплуатации, сохраняются на английском для совместимости.

## Основной охват

- При чтении проверьте следующие пункты. Команды, JSON-поля, RPC-методы, ключи конфигурации и идентификаторы кода сохраняются на английском для совместимости.
- Подробные нормативные формулировки смотрите в английском оригинале.
- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/ru/security/audit-readiness.md`

## Сохраняемые идентификаторы

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

## Разделы английского оригинала

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- Security Goals
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## Операционные заметки

- `MUST`, `SHOULD`, `MAY`, примеры команд, JSON-примеры и имена RPC сохраняют английское написание.
- После изменения этого перевода выполните `make docs-check`.
- Если эта страница противоречит английскому источнику, используйте английский источник и обновите этот locale-файл в том же изменении.

## Канонический источник

- [English canonical document](../../en/security/audit-readiness.md)
