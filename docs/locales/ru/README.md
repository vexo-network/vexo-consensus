# Documentation

> Locale: ru · Русский
> Этот документ — русский сопроводительный документ к английскому источнику. Протокол, безопасность и решения о release остаются нормативными на английском языке.

## Быстрый старт

- Соберите бинарник командой `make build`.
- Создайте validator home через `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`, затем проверьте его `vexod validate --home .vexo-validator-1` и `vexod config audit --home .vexo-validator-1 --strict`, после чего запускайте `vexod start --home .vexo-validator-1`.
- Docker-сеть запускайте сначала через `docker compose -f deployments/docker/compose.single-host-init.yml up`, затем через `docker compose -f deployments/docker/compose.single-host.yml up`.
- Remix должен указывать на `http://127.0.0.1:28657/web3`; chain ID проверьте командой `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`.
- После правок запускайте `make docs-check`, чтобы проверить locale tree и translation guards.
## Обзор

Этот документ помогает понять индекс документации и рекомендуемый порядок чтения и связать это с решениями по реализации и эксплуатации.

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/ru/README.md`

## Зачем читать этот документ

- индекс документации и рекомендуемый порядок чтения
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

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## Структура английского источника

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## Канонический источник

- [Английский канонический документ](../en/README.md)

## Список неизменяемых терминов

Следующие термины не переводятся.

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

<!-- vexo-docs:technical-parity -->
## Приложение о техническом соответствии

Это приложение помогает убедиться, что перевод сохраняет исполняемые интерфейсы и ключевые разделы английского канонического документа. Команды, ключи конфигурации, методы RPC и имена пакетов остаются неизменными на всех языках.

### Отслеживание разделов
- section: How to Read This Set — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Start Here — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Protocol Specs — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: SDK and Extension Guides — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Operations and Release — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Security — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Localized Documentation — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Writing New Docs — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Production Claim Rule — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Documentation Review Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.

### Интерфейсы, сохраняемые без изменений
- `vexo-consensus` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `/v1/*` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `make docs-check` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `vexod status --json` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `feature_assurance` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `network_config.json:p2p.auth_replay_path` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `network_config.json:p2p.node_key_path` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `module_config.json:governance.RequireDeposit` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `module_config.json:governance.MinDeposit` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `consensus_config.json:consensus.execution_commit` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `mempool_config.json:mempool.WALPath` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
