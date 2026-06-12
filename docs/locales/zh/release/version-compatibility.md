# Version Compatibility Matrix

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 版本兼容矩阵和升级判断标准，并把它连接到实际实现和运维判断。

- Canonical path: `docs/release/version-compatibility.md`
- Locale path: `docs/locales/zh/release/version-compatibility.md`

## 为什么阅读本文档

- 版本兼容矩阵和升级判断标准
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

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `/v1/*`
- `vexod upgrade plan --json`
- `vexod upgrade apply`
- `rollback_required`
- `make release-candidate`

## 英文原文结构

- Version Compatibility Matrix
- Current Matrix
- Upgrade Compatibility Checklist
- Rollback Drill

## 规范来源

- [英文规范文档](../../en/release/version-compatibility.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Current Matrix — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Upgrade Compatibility Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Rollback Drill — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `mempool_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `log_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod upgrade plan --json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod upgrade apply` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rollback_required` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make release-candidate` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
