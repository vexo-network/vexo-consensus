> Locale: ru · Русский

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| Задача | Путь к команде |
|---|---|
| Построить локальный двоичный файл | `make build` |
| Создайте один валидатор | `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` |
| Подтвердить одно жилье | `vexod validate --home .vexo-validator-1` и `vexod config audit --home .vexo-validator-1 --strict` |
| Запустите один узел | `vexod start --home .vexo-validator-1` |
| Запрос одного узла | `curl -s http://127.0.0.1:26657/v1/status` |
| Запустите сеть Docker с четырьмя валидаторами | __ VEXO_CODE_5__, а затем __ VEXO_CODE_6__ |
| Подключить ремикс | Используйте Docker validator 1 Web3 URL `http://127.0.0.1:28657/web3` |
| Проверьте идентификатор сети Web3 | `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` |

## Быстрый старт

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## Начните здесь

| Документ | Цель |
|---|---|
| [Руководство по готовности производства](./production-readiness.md) | Единая карта протокола, времени выполнения, операций, доказательств и готовности к выпуску |

## Спецификации протокола

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| [Матрица совместимости версий](./release/version-compatibility.md) | Ожидания совместимости для двоичных файлов, конфигураций, хранилищ, приложений, RPC и контрольных форматов |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

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
