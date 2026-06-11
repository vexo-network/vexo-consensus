# Release Pipeline

> Locale: ru · Русский
> Этот документ — русский сопроводительный документ к английскому источнику. Протокол, безопасность и решения о release остаются нормативными на английском языке.

## Обзор

Этот документ помогает понять release pipeline с подписанными бинарями, checksums и SBOM и связать это с решениями по реализации и эксплуатации.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/ru/release/release-pipeline.md`

## Зачем читать этот документ

- release pipeline с подписанными бинарями, checksums и SBOM
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

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
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
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## Структура английского источника

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- План запуска

## Доказательства совместимости EVM/Web3

`--sdk-conformance-evidence` и `--evm-web3-conformance-evidence` должны оставаться отдельными доказательствами. Одной строки вроде “EVM passed” недостаточно: EVM/Web3 evidence должно содержать машиночитаемые разделы `evm_fixtures`, `evm_execution`, `web3_rpc` и `evm_corpus`, а перед публичными заявлениями о совместимости оно должно быть привязано к `evidence-manifest.json` через SHA-256.

## VRF audit evidence SHA-256

`release gate` закрепляет SHA-256 не только для BLS audit evidence, но и для VRF audit evidence. Файл `--vrf-audit` должен быть включён в `evidence-manifest.json`, а `--vrf-audit-sha256` должен точно совпадать с содержимым файла. При использовании config значение `vrf.audit_evidence_sha256` служит digest pin по умолчанию. Это связывает VRF service, KMS/HSM custody, TLS/mTLS или pinned CA, auth token и защиту от nonce replay с release evidence.

## Канонический источник

- [Английский канонический документ](../../en/release/release-pipeline.md)

## Термины attestation для release evidence

Для публичного релиза каждая запись в `evidence-manifest.json` должна проверяться подписью Ed25519. Следующие CLI-флаги и JSON-поля нужно оставлять без перевода.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
<!-- vexo-docs-ops-update-2026-06 -->

## Как читать network E2E

`make network-e2e` — не только build test: он запускает 4 validators реальным binary и проверяет signed-shape smoke transaction, peer connection, рост height и clean stop. `NETWORK_E2E_GO_TIMEOUT` — внешний лимит Go test и должен быть больше внутреннего network timeout.
