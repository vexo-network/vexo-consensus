# EVM and Native Accounting

> Locale: zh · 中文
> 本文档是基于英文规范文档编写的中文翻译指南。协议、安全和发布判断以英文原文为准。

## 目的

本文档说明 将 native coin 与 EVM gas/accounting 保持一致的方法。 实现和运维中使用的命令、JSON 字段、RPC 名称、config key 和代码标识符为保持兼容性保留英文原样。

## 核心范围

- 阅读本文档时必须检查以下项目。命令、JSON 字段、RPC 方法、配置键和代码标识符为保持兼容性保留英文原样。
- 详细的规范性表述请以英文原文为准。
- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/zh/specs/evm-native-accounting.md`

## 需要保留的标识符

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

## 英文原文章节

- EVM and Native Accounting
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM Execution
- Compatibility Boundary
- Failure Modes

## 运维说明

- `MUST`、`SHOULD`、`MAY`、命令示例、JSON 示例和 RPC 名称保留英文拼写。
- 修改此翻译后请运行 `make docs-check`。
- 如果本页与英文来源不一致，请以英文来源为准，并在同一次变更中更新该 locale 文件。

## 规范来源

- [English canonical document](../../en/specs/evm-native-accounting.md)
