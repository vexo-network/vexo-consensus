# Transaction Format

> Locale: zh · 中文
> 本文档是基于英文规范文档编写的中文翻译指南。协议、安全和发布判断以英文原文为准。

## 目的

本文档说明 transaction format、signing、fee 和 gas 规则。 实现和运维中使用的命令、JSON 字段、RPC 名称、config key 和代码标识符为保持兼容性保留英文原样。

## 核心范围

- 阅读本文档时必须检查以下项目。命令、JSON 字段、RPC 方法、配置键和代码标识符为保持兼容性保留英文原样。
- 详细的规范性表述请以英文原文为准。
- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/zh/specs/tx-format.md`

## 需要保留的标识符

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`

## 英文原文章节

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## 运维说明

- `MUST`、`SHOULD`、`MAY`、命令示例、JSON 示例和 RPC 名称保留英文拼写。
- 修改此翻译后请运行 `make docs-check`。
- 如果本页与英文来源不一致，请以英文来源为准，并在同一次变更中更新该 locale 文件。

## 规范来源

- [English canonical document](../../en/specs/tx-format.md)
