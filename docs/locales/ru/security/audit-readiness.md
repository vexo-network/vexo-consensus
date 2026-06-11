# Security Audit Readiness

> Locale: ru · Русский
> Этот документ — русский сопроводительный документ к английскому источнику. Протокол, безопасность и решения о release остаются нормативными на английском языке.

## Обзор

Этот документ помогает понять threat model, предположения безопасности и доказательства для аудита и связать это с решениями по реализации и эксплуатации.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/ru/security/audit-readiness.md`

## Зачем читать этот документ

- threat model, предположения безопасности и доказательства для аудита
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

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## Структура английского источника

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

## VRF audit evidence SHA-256

Материалы аудита должны включать VRF adapter audit evidence наряду с BLS. Закрепите SHA-256 файла вроде `docs/security/ecvrf-audit-evidence.json` в `vrf.audit_evidence_sha256` или `--vrf-audit-sha256` и проверяйте dependency audit, key custody, TLS/mTLS или pinned CA, auth, replay defense и service availability как единую границу.

## Канонический источник

- [Английский канонический документ](../../en/security/audit-readiness.md)
