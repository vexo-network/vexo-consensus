# EVM and Native Accounting

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 将 native coin 与 EVM gas/accounting 保持一致的方法，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/zh/specs/evm-native-accounting.md`

## 为什么阅读本文档

- 将 native coin 与 EVM gas/accounting 保持一致的方法
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

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`
- `bank query balance`

## 英文原文结构

- EVM and Native Accounting
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM Execution
- Compatibility Boundary
- Failure Modes

## 规范来源

- [English canonical document](../../en/specs/evm-native-accounting.md)
