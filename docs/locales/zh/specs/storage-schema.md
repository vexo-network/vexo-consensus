# Storage Schema

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 durable storage namespace、key schema 和 recovery marker，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/zh/specs/storage-schema.md`

## 为什么阅读本文档

- durable storage namespace、key schema 和 recovery marker
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
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## 英文原文结构

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## 规范来源

- [英文规范文档](../../en/specs/storage-schema.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Backend — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Records — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Indexes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: EVM Records — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Recovery Rules — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Snapshot Validation — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Schema Migration — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `store.Store` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_ethstate` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBalance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getProof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bank/{0x_address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `auth/nonce/{0x_address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm/code/{0x_address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm/storage/{0x_address}/{slot}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_ethstate/{height}/meta` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_ethstate/{height}/accounts/{0x_address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_ethstate/{height}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `seen_ttl` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `code/{address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `storage/{address}/{slot}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `receipts/{tx_hash}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `logs/by_height/{height}/{tx_hash}/{log_index}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `logs/{address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
