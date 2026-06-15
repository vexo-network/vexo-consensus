# 自定义存储与传输指南

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明如何实现并注册 custom storage 和 transport adapter。第一次实现时，建议按下面顺序阅读。

1. Custom Storage
2. Storage Requirements
3. Custom Transport
4. Transport Requirements
5. Compatibility

这个顺序基本对应你真正需要先确认的风险：先看 storage 能否扛住崩溃、pruning、snapshot 和 replay，再看 transport 是否正确处理认证、版本协商、重连和封禁。

## 文档概览

本文档帮助你理解 实现并注册 custom storage 和 transport adapter 的方法，并把它连接到实际实现和运维判断。

- Canonical path: `docs/sdk/custom-storage-transport.md`
- Locale path: `docs/locales/zh/sdk/custom-storage-transport.md`

## 为什么阅读本文档

- 实现并注册 custom storage 和 transport adapter 的方法
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

- `store.Store`
- `store.HistoricalSnapshotKVStore`
- `store.SnapshotKVStore`
- `transport.Transport`

## 英文原文结构

- Custom Storage and Transport Guide
- Custom Storage
- Storage Requirements
- Custom Transport
- Transport Requirements
- Compatibility

## 规范来源

- [英文规范文档](../../en/sdk/custom-storage-transport.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Custom Storage — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Storage Requirements — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Custom Transport — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Transport Requirements — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Compatibility — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `store.Store` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `store.HistoricalSnapshotKVStore` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `store.SnapshotKVStore` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `store.AppBlockCommitStore` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod start` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `runtime.NewNetworkSafeWithStore` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `runtime.NewNetworkSafeWithStoreContext` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `runtime.NewNetworkSafeWithStoreAndCryptoRegistryContext` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `config.ValidateNetworkSafety` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `app.AtomicBlockApplication` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `transport.Transport` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `transport.GRPCConfig.RequireTLS` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
