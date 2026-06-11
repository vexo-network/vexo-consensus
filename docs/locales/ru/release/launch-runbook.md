# План запуска

> Locale: ru · Русский
> Этот документ — русский сопроводительный документ к английскому источнику. Протокол, безопасность и решения о release остаются нормативными на английском языке.

## Обзор

Этот документ помогает понять операторский checklist и процедуру перед запуском сети и связать это с решениями по реализации и эксплуатации.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/ru/release/launch-runbook.md`

## Зачем читать этот документ

- операторский checklist и процедуру перед запуском сети
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
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## Структура английского источника

- План запуска
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Доказательства совместимости EVM/Web3

Перед публичным релизом храните `--evm-web3-conformance-evidence` отдельно от `--sdk-conformance-evidence`. Файл должен содержать `evm_fixtures`, `evm_execution`, `web3_rpc` и `evm_corpus`, чтобы `release gate` мог отклонять непроверяемые сводки.

## VRF audit evidence SHA-256

При проверке release candidate передайте в `release gate` оба digest: BLS и VRF audit evidence. Минимально используйте `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256` и `--evidence-manifest`, затем проверьте совпадение всех evidence файлов с SHA-256 в manifest.

## Канонический источник

- [Английский канонический документ](../../en/release/launch-runbook.md)
