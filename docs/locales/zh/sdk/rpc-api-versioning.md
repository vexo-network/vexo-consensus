# RPC API Versioning

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 RPC API 版本管理、兼容别名和稳定策略，并把它连接到实际实现和运维判断。

- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/zh/sdk/rpc-api-versioning.md`

## 为什么阅读本文档

- RPC API 版本管理、兼容别名和稳定策略
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

- `/v1`
- `/v1/healthz`
- `/v1/readyz`
- `/v1/status`
- `/v1/diagnostics`
- `/v1/metrics`
- `/v1/metrics/text`
- `/v1/peers`
- `/v1/tx`
- `/v1/evidence`
- `/v1/recovery`
- `/v1/snapshot/latest`
- `/v1/snapshot/export`
- `/v1/snapshot/chunk?index=0&size=10000`
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`

## 英文原文结构

- RPC API Versioning
- Stability Goal
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## 规范来源

- [英文规范文档](../../en/sdk/rpc-api-versioning.md)

## RPC capability discovery

新的 RPC capability discovery 接口用于检查节点实际挂载的 provider 功能。运行方可以调用 `/v1/capabilities`，SDK 集成方可以使用 `rpc.Config.RequiredCapabilities` 或 `rpc.Config.RequireAllCapabilities` 在启动时 fail closed。

Keep these interface names unchanged: `/v1/capabilities`, `CapabilityResponse`, `CapabilitySnapshot`, `RequiredCapabilities`, `RequireAllCapabilities`, `metrics`, `blocks`, `finality`, `strict_replay`, `consensus_control`.
