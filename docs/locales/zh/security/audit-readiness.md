# Security Audit Readiness

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

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
- 修改文档后运行 `make docs-check` 检查 locale tree 和翻译 guard。

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
