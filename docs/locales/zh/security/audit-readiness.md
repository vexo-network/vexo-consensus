# 安全审计准备

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 threat model、安全假设和审计提交证据，并把它连接到实际实现和运维判断。

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/zh/security/audit-readiness.md`

## 为什么阅读本文档

- threat model、安全假设和审计提交证据
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

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## 英文原文结构

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- 安全目标
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

审计材料除了 BLS 之外，也必须包含 VRF adapter audit evidence。将 `docs/security/ecvrf-audit-evidence.json` 等 evidence 文件的 SHA-256 固定到 `vrf.audit_evidence_sha256` 或 `--vrf-audit-sha256`，并把 dependency audit、key custody、TLS/mTLS 或 pinned CA、auth、replay 防护、service availability 放在同一安全边界中审查。

## 规范来源

- [英文规范文档](../../en/security/audit-readiness.md)
## 审计准备补充说明

审计人员首先关注证据是否可复现，而不只是代码是否存在。因此，威胁模型、已知限制、BLS/VRF 审计材料、EVM conformance 结果、longrun/chaos 结果、KMS 签名证据以及 snapshot/replay 证据，都必须来自同一个候选二进制文件和同一组配置。运营者不应隐藏例外情况，而应记录 release gate 拒绝的项目、负责人以及重新验证条件。
审计材料还应说明每一项证据的采集时间、命令、输入文件、输出文件、哈希值和签名者。若某项证据来自外部系统，例如 KMS、HSM、云监控或第三方审计报告，文档必须写清楚信任边界、访问控制、轮换策略以及失败时的回滚步骤。这样读者才能判断该网络是否只是“能运行”，还是已经具备可追责、可复现、可恢复的发布条件。
在提交审计包之前，团队应重新运行 release gate、docs-quality、EVM/Web3 conformance、state sync 验证和网络 E2E，并确认所有输出都写入 evidence manifest。任何手工步骤都需要负责人、时间戳和回滚条件，否则不能作为公开发布的安全依据。
此外，审计说明必须区分“代码已经实现”“测试已经通过”“外部证据已经归档”这三种状态。只有当三者都指向同一版本、同一 commit、同一 genesis 和同一配置文件时，运营者才可以把该功能写入发布声明。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Threat Model — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Security Assumptions — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Known Limitations — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Formal-ish Safety Argument — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Required Evidence for Audit — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Auditor Focus Areas — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Practical Audit Walkthrough — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Remote Signer Audit Notes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: EVM/Web3 Audit Notes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Snapshot and WAL Audit Notes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `docs/security/blst-audit-evidence.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `remote-vrf-http-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod keys serve-vrf` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release collect-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `chain_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `go.mod` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/recovery/report` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
