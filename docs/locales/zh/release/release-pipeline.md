# Release Pipeline

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 包含签名二进制、checksums 和 SBOM 的发布流水线，并把它连接到实际实现和运维判断。

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/zh/release/release-pipeline.md`

## 为什么阅读本文档

- 包含签名二进制、checksums 和 SBOM 的发布流水线
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
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
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
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## 英文原文结构

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- 发布运行手册

## EVM/Web3 合规证据

`--sdk-conformance-evidence` 和 `--evm-web3-conformance-evidence` 是两类独立证据。仅写一句 “EVM passed” 不够；EVM/Web3 证据必须包含可机器检查的 `evm_fixtures`、`evm_execution`、`web3_rpc` 和 `evm_corpus`，并且在公开声明兼容性前通过 SHA-256 绑定到 `evidence-manifest.json`。

## VRF audit evidence SHA-256

`release gate` 不只固定 BLS audit evidence，也必须用 SHA-256 固定 VRF audit evidence。`--vrf-audit` 文件必须进入 `evidence-manifest.json`，`--vrf-audit-sha256` 必须与文件内容完全一致。使用 config 时，`vrf.audit_evidence_sha256` 作为默认 digest pin。这个规则用于确认 VRF service、KMS/HSM custody、TLS/mTLS 或 pinned CA、auth token 以及 nonce replay 防护都绑定到发布证据。

## 规范来源

- [英文规范文档](../../en/release/release-pipeline.md)

## 发布证据 attestation 术语

公开发布时，`evidence-manifest.json` 中的每个条目都必须通过 Ed25519 签名验证。下面的 CLI 标志和 JSON 字段应保持原样，不要翻译。

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
