# Documentation

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 文档索引和推荐阅读顺序，并把它连接到实际实现和运维判断。

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/zh/README.md`

## 为什么阅读本文档

- 文档索引和推荐阅读顺序
- 先在英文原文中确认 MUST/SHOULD/MAY 语句。
- 此本地化文档用于帮助理解；审计、发布和安全判断以英文原文为准。

## 读完后应能做到

- 说明本文档支持哪些实现或运维决策。
- 把英文原文中的规范要求与当前网络配置对应起来。
- 复制示例前检查 chain ID、validator ID、fee/gas 和 peer 地址。

## 安全使用检查清单

- 先在英文原文中确认 MUST/SHOULD/MAY 语句。
- 不要翻译命令、config key、RPC 名称、JSON 字段和代码标识符。
- 复制示例值前，请确认 chain ID、validator ID、fee/gas 和 peer 地址适合你的网络。
- 修改文档后运行 `make docs-check` 检查 locale tree 和翻译 guard。

## 注意事项

- 此本地化文档用于帮助理解；审计、发布和安全判断以英文原文为准。
- 实现变更时，英文文档和所有本地化文档应在同一变更中更新。

## 必须保持原样的接口

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## 英文原文结构

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## 规范来源

- [英文规范文档](../en/README.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: How to Read This Set — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Start Here — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Protocol Specs — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: SDK and Extension Guides — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Operations and Release — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Security — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Localized Documentation — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Writing New Docs — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Production Claim Rule — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Documentation Review Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `vexo-consensus` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make docs-check` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod status --json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `feature_assurance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.auth_replay_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.node_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json:governance.RequireDeposit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json:governance.MinDeposit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_config.json:consensus.execution_commit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `mempool_config.json:mempool.WALPath` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
