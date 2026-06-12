# 发布运行手册

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 网络上线前的运维检查清单和执行流程，并把它连接到实际实现和运维判断。

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/zh/release/launch-runbook.md`

## 为什么阅读本文档

- 网络上线前的运维检查清单和执行流程
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

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## 英文原文结构

- 发布运行手册
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## EVM/Web3 合规证据

公开发布前，请将 `--evm-web3-conformance-evidence` 与 `--sdk-conformance-evidence` 分开归档。该文件必须包含 `evm_fixtures`、`evm_execution`、`web3_rpc` 和 `evm_corpus`，这样 `release gate` 才能拒绝无法验证的摘要。

## VRF audit evidence SHA-256

验证 release candidate 时，`release gate` 命令要同时提供 BLS 和 VRF audit evidence digest。至少一起使用 `--bls-audit`、`--bls-audit-sha256`、`--vrf-audit`、`--vrf-audit-sha256`、`--evidence-manifest`，并确认所有 evidence 文件与 manifest 的 SHA-256 一致。

## 规范来源

- [英文规范文档](../../en/release/launch-runbook.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Prelaunch Gate — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Release Candidate Gate — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Genesis Gate — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Launch Window — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Postlaunch Archive — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `release docs-quality` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `checksums.txt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `sbom-go-modules.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `sbom-go-version.txt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release-manifest.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release-audit-pack.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release collect-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network analyze-longrun` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `longrun-evidence.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-default-fixtures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-tx-fixtures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-tx-fixtures-dir` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-execution-fixtures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-execution-fixtures-dir` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-tx-fixtures-sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-execution-fixtures-sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-web3-conformance-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_fixtures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_execution` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `web3_rpc` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_corpus` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod ops conformance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `relayer soak-plan` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `chain_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evidence-manifest.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
