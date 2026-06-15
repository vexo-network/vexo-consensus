# 发布流水线

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Release Pipeline 的发布与运维流程。第一次阅读时，建议按下面顺序看。

1. Goals
2. Release Commands
3. CI Gates
4. Evidence Quality Rules
5. Artifacts
6. Reproducibility Notes
7. Signed Binaries
8. SBOM
9. Audit Pack
10. Release Candidate Targets
11. Launch Runbook

这个顺序对应实际使用方式：先看目标和 gate，再看产物与证据要求，最后看执行步骤。

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
- 修改文档后运行 `make docs-check` 检查本地文档树和翻译检查。

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
- `RELEASE_CGO_ENABLED=1`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `release-candidate-smoke`
- `release-candidate-plan`
- `make release-portable RELEASE_REQUIRE_BLS=0`
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
- 目标s
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

## 发布候选策略

公开发布候选版本应使用默认的 `make release-candidate`。该 target 是真实 gate，会进入 `release-candidate-real`，并要求 `RELEASE_CGO_ENABLED=1`，确保包含 cgo-backed `supranational/blst` BLS adapter。`make release-candidate-plan` 只用于 PR smoke 和运维计划检查；它使用内置 fixture 与 dry-run plan，不能作为最终发布 evidence。若需要 no-cgo artifact，只能使用 `make release-portable RELEASE_REQUIRE_BLS=0`，并且不得声明为 BLS-capable release。 当 `RELEASE_CGO_ENABLED=1` 且未设置 `RELEASE_TARGETS` 时，Makefile 只构建当前 host target。若需要多个 OS/architecture artifact，请在具备对应 cgo cross-compiler 的 runner 上显式设置 `RELEASE_TARGETS`。

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
<!-- vexo-docs-ops-update-2026-06 -->

## 网络 E2E 的含义

`make network-e2e` 不只是构建测试；它用真实二进制启动 4 个 validator，验证 signed-shape smoke transaction、peer 连接、height 增长和 clean stop。`NETWORK_E2E_GO_TIMEOUT` 是外层 Go test 限制，必须大于内部 network timeout 才能保留真实失败原因。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Goals — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Release Commands — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: CI Gates — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Evidence Quality Rules — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Artifacts — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Reproducibility Notes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Signed Binaries — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: SBOM — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Audit Pack — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Release Candidate Targets — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Launch Runbook — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `network analyze-longrun` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release collect-evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `ops-runbook` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p-scale` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `state-sync-light-client` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `snapshot-replay` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make check` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make fuzz-smoke` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod consensus adversarial` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod ops conformance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod network longrun` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod network chaos-plan` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make network-e2e` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make race` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `NETWORK_E2E_GO_TIMEOUT` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make test` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make vet` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make docs-check` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make build` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make release-candidate-smoke VERSION=ci`
- `make release-candidate-plan VERSION=ci` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make release-candidate VERSION=<rc> RELEASE_CGO_ENABLED=1 RC_EVM_CONFORMANCE_FLAGS=...` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evidence-manifest.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--allow-external-pending` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--private-rc` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo-release-evidence-attestation-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release evidence-manifest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--signing-key` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--signing-key-env` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `<evidence-file>.sig` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `<evidence-file>.sig.pub` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `<evidence-file>.pub` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `dist/` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod-<version>-<os>-<arch>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `checksums.txt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `checksums.txt.asc` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `sbom-go-modules.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `sbom-go-version.txt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release-manifest.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release-audit-pack.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `longrun-analysis.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs-quality.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RELEASE_CGO_ENABLED=1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `supranational/blst` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `go build -trimpath` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `BUILD_DATE` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make release-candidate` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make release-portable RELEASE_REQUIRE_BLS=0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RELEASE_TARGETS` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release-candidate` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release-candidate-real` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod ops conformance --strict` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RC_EVM_CONFORMANCE_FLAGS` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RC_LONGRUN_DURATION` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `release-candidate-plan` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RELEASE_REQUIRE_BLS=0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `allow_noop_migrations=true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod upgrade apply --allow-empty-migrations` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--bls-audit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--bls-audit-sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--config <path>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `crypto.audit_evidence_sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--vrf-audit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--vrf-audit-sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.audit_evidence_sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/security/blst-audit-evidence.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/security/ecvrf-audit-evidence.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
