# Validator Lifecycle

> Locale: ru · Русский
> Этот документ — русский сопроводительный документ к английскому источнику. Протокол, безопасность и решения о release остаются нормативными на английском языке.

## Обзор

Этот документ помогает понять validator join, rotation, jail, slashing и leave lifecycle и связать это с решениями по реализации и эксплуатации.

- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/ru/specs/validator-lifecycle.md`

## Зачем читать этот документ

- validator join, rotation, jail, slashing и leave lifecycle
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

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Структура английского источника

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Канонический источник

- [Английский канонический документ](../../en/specs/validator-lifecycle.md)
