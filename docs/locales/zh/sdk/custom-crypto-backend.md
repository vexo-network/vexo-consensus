# 自定义加密后端指南

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明如何添加 custom crypto backend。第一次阅读时，建议按下面顺序看。

1. Interfaces
2. Runtime Suite
3. Domain Separation
4. Production BLS Requirements
5. VRF Backend Requirements
6. Remote Signer Requirements
7. Test Backends

这个顺序对应你真正要先做的决定：先选择需要哪一种 backend，再固定 sign bytes 和 domain，最后确认它能否用于生产环境。

## 文档概览

本文档帮助你理解 BLS、VRF、signer 等 custom crypto backend 的接入方式，并把它连接到实际实现和运维判断。

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/zh/sdk/custom-crypto-backend.md`

## 为什么阅读本文档

- BLS、VRF、signer 等 custom crypto backend 的接入方式
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

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `ecvrf-p256-sha256-tai-v1`
- `remote-vrf-http-v1`
## 英文原文结构

- Custom Crypto Backend Guide
- 目标
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## VRF audit evidence SHA-256

VRF backend 也要像 BLS 一样暴露清晰的审计边界。请填写 `vrf.adapter_name`、`vrf.audit_report`、`vrf.dependency_audit`、`vrf.audit_evidence_sha256`、`vrf.key_source`；如果 adapter metadata 与 config 不一致，runtime 应失败关闭。内置 ECVRF adapter 会验证 go.mod dependency pin 和 audit evidence digest；remote VRF adapter 使用外部 KMS/HSM audit reference。

## 规范来源

- [英文规范文档](../../en/sdk/custom-crypto-backend.md)

## Remote VRF service

`vexod keys serve-vrf` 使用 ECVRF key 提供 `POST /prove` 和 `POST /verify`，`vexod keys verify-vrf` 用于端到端检查 remote prover。`VEXO_REMOTE_VRF_TOKEN`、`remote-vrf-http-v1`、`vexo.remote_vrf.prove.v1`、`vexo.remote_vrf.verify.v1` 保持不翻译。

以下接口名称必须保持不变： `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Goal — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Interfaces — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Runtime Suite — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Domain Separation — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Production BLS Requirements — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Production VRF Requirements — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Remote Signer Requirements — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Test Backends — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `vexo-consensus` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `supranational/blst` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo.consensus.proposal.v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo.consensus.vote.v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo.consensus.timeout_vote.v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo.finality.proof.v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `crypto.adapter_name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `BLSAdapter.Metadata().Name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `crypto.audit_evidence_sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bls_pop` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blst-bls12381-minpk-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `github.com/supranational/blst` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RELEASE_CGO_ENABLED=1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RELEASE_REQUIRE_BLS=1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make release-portable RELEASE_REQUIRE_BLS=0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `circl-bls12381-g1sigg2-basic-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bls_proof_of_possession` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.adapter_name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.audit_report` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.dependency_audit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.audit_evidence_sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.key_source` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `committee.backend` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `crypto.NewProductionVRF` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `production_adapter: true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `ecvrf-p256-sha256-tai-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf_public_key` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `remote-vrf-http-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `remote-http:<base-url>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `POST /prove` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `public_key` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `issued_at_unix_nano` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `deadline_unix_nano` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo.remote_vrf.prove.v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `POST /verify` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo.remote_vrf.verify.v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `{ "valid": true, "nonce": "<same nonce>" }` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `VEXO_REMOTE_VRF_TOKEN` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `Authorization: Bearer <token>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.tls_cert_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.tls_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.tls_ca_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.tls_server_name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `keys serve-vrf` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--auth-token` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--auth-token-env` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod keys serve-vrf` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `crypto.NewRemoteVRFService` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--home` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `remote-vrf-nonces.jsonl` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `remote-vrf-audit.jsonl` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--nonce-path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--audit-log` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `crypto.RemoteVRFServiceConfig.ReplayStore` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `RequireDurableReplayStore: true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `crypto.NewFileRemoteVRFReplayStore` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf_key_paths` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `VEXO_KEY_PASSPHRASE` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf.keys` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod keys serve-remote` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--guard-path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_proposal` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_vote` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_timeout_vote` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `finality_proof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
