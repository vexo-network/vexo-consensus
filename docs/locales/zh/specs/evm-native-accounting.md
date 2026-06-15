# EVM 与原生记账

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Evm Native Accounting 的规范定义。第一次阅读时，建议按下面顺序看。

1. Core Rule
2. Amount Encoding
3. Fee Accounting
4. EVM Execution
5. State Root Policy
6. Compatibility Boundary
7. Failure Modes

这个顺序对应你的阅读方式：先看范围和状态，再看消息、正确性与活性规则，最后看证据。

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

- EVM 与原生记账
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM 执行
- State Root Policy
- Compatibility Boundary
- Failure Modes

## 规范来源

- [英文规范文档](../../en/specs/evm-native-accounting.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Core Rule — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Amount Encoding — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Fee Accounting — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: EVM Execution — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: State Root Policy — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Compatibility Boundary — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Failure Modes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `base_fee * gas` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `contract.Invocation` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `value_hex` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `gas_price_hex` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_fee_per_gas_hex` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_priority_fee_per_gas_hex` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBalance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_sendRawBlobTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_sendRawBlobTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_sendRawTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution.strict_evm_state_root` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
