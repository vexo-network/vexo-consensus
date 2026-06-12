# Cosmos/Tendermint Comparison Gate

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 相对于 Cosmos/Tendermint 风格预期的发布门禁，并把它连接到实际实现和运维判断。

- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/zh/release/cosmos-comparison-gate.md`

## 为什么阅读本文档

- 相对于 Cosmos/Tendermint 风格预期的发布门禁
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

- `release gate`
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## 英文原文结构

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## 规范来源

- [英文规范文档](../../en/release/cosmos-comparison-gate.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Required Evidence Properties — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Release Rule — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `--longrun-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--chaos-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--ops-runbook-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--external-audit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--formal-safety-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--fuzz-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--sdk-conformance-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-web3-conformance-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--p2p-scale-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--state-sync-light-client-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--snapshot-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--validator-economics-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--upgrade-governance-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--mev-fee-market-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--kms-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--bls-audit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
