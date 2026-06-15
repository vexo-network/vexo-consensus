# 验证人生命周期

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Validator Lifecycle 的规范定义。第一次阅读时，建议按下面顺序看。

1. Scope
2. Admission
3. Validator Set
4. Rotation
5. Evidence Lifecycle
6. Slashing
7. Jail and Unbonding

这个顺序对应你的阅读方式：先看范围和状态，再看消息、正确性与活性规则，最后看证据。

## 文档概览

本文档帮助你理解 validator join、rotation、jail、slashing、leave 生命周期，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/zh/specs/validator-lifecycle.md`

## 为什么阅读本文档

- validator join、rotation、jail、slashing、leave 生命周期
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

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## 英文原文结构

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## 规范来源

- [英文规范文档](../../en/specs/validator-lifecycle.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Admission — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Validator Set — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Rotation — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Evidence Lifecycle — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Slashing — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Jail and Unbonding — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `vexovaloper...` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexovalcons...` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo...` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `staking tx withdraw-unbonded` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
