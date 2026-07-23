# 共识规范

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Consensus Spec 的规范定义。第一次阅读时，建议按下面顺序看。

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

这个顺序对应你的阅读方式：先看范围和状态，再看消息、正确性与活性规则，最后看证据。

## 文档概览

本文档帮助你理解 共识 state machine 的规范规格，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/zh/specs/consensus-spec.md`

## 为什么阅读本文档

- 共识 state machine 的规范规格
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
- 修改文档后运行 `make docs-check` 检查本地文档树和翻译检查。

## 注意事项

- 此本地化文档用于帮助理解；审计、发布和安全判断以英文原文为准。
- 实现变更时，英文文档和所有本地化文档应在同一变更中更新。

## 必须保持原样的接口

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## 英文原文结构

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## 规范来源

- [英文规范文档](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## 空块与 Round 恢复

当 `create_empty_blocks=false` 且 mempool 为空时，height 看起来不增长是正常 idle 状态。交易进入后，节点只有在当前 `(height, round)` 的确定性 proposer 身份属于自己时才提议；非 proposer 不会在本地跳到其他 round。round 只能通过有效的 timeout certificate 或已认证的 finality 转换推进，执行或存储错误不会被当作 timeout。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Roles — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: State — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Message Types — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Safety Rules — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Finality Rule — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Execution Commit Policy — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Liveness Assumptions — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Empty Blocks and Round Recovery — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Evidence — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `chain_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `validator_set_hash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `locked_qc` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `high_qc` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `last_timeout_cert` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `last_finalized` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `>= 2/3` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `B3.height = B2.height + 1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `B2.height = B1.height + 1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution_commit = "qc"` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution_commit = "finalized"` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `block_committed` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `create_empty_blocks = false` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_height = 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `actual_hash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `actual_time_unix_nano` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `parity_shards` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
