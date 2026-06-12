# Production Readiness Guide

> Locale: zh · 中文
> 安全和发布判断必须以英文规范文档和 release gate 结果为准。

## 概览

本文说明在把 Vexo 网络称为生产可用之前，必须验证哪些协议、运行时、运维和证据条件。

本地化文档会保留命令、JSON 字段、RPC 方法、配置键和包名不变，确保不同语言中的示例都可以直接复制使用。

## 为什么重要

Vexo combines BFT consensus, application modules, native accounting, optional EVM execution, validator economics, peer networking, and release evidence. A reader should be able to explain not just that a feature exists, but how to operate it safely and how to prove that it works on the target network.

## 必须验证

- **Protocol correctness**: `consensus`, `finality`, validator-set hash, vote sign bytes, timeout certificate, and three-chain finality must agree for the target validator set.
- **Runtime correctness**: `app`, `runtime`, `store`, and module writes must commit atomically, replay deterministically, and recover from crash boundaries.
- **Crypto custody**: BLS, VRF, remote signer, KMS/HSM, proof-of-possession, replay nonce, and double-sign guard evidence must match the release binary.
- **Networking safety**: `network_config.json` must bind chain ID, genesis hash, node ID, TLS/auth policy, durable replay path, peer scoring, ban, and backoff settings.
- **EVM/native accounting**: The EVM module uses the native Vexo coin as the balance asset; gas, base fee, blob base fee, receipts, proofs, and traces must pass external corpora.
- **Release evidence**: Release claims need signed artifacts, SBOM, evidence manifest, longrun, chaos, E2E, state sync, economics, governance, MEV, SDK, and EVM/Web3 evidence.

## 运维动作

- **System view**: A Vexo network is safe only when protocol, runtime, operations, and evidence are ready together. Do not treat enabled code as a production claim.
- **Configuration review**: Review `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before `vexod start`.
- **Release decision**: Attach evidence from the exact binary, genesis, config schema, module set, and validator topology that will be released.

## 保持不变的接口名称

- `vexod validate --home <home>`
- `vexod config audit --home <home> --strict`
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `peer_count`
- `active_peer_count`
- `configured_peer_count`
- `scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `network_config.json`
- `consensus_config.json`
- `module_config.json`
- `mempool_config.json`
- `release gate`

## 常见错误

- Do not assume configured peers are connected peers; active sessions must be checked separately.
- Do not call BLS, VRF, EVM, state sync, or governance production-ready without release evidence.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## 规范参考

- [规范原文](../en/production-readiness.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: The Short Version — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: How To Use This Guide — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Readiness Levels — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: System Map — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Configuration Review Order — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Consensus and Finality Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Runtime and Storage Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: EVM and Native Coin Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Crypto and Key Custody Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Networking Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Observability Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Release Evidence Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Common Failure Modes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: What This Guide Does Not Claim — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `docs/specs/consensus-spec.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/specs/finality-proof-format.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `modules/staking` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/specs/validator-lifecycle.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `modules/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/sdk/app-module-guide.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/specs/storage-schema.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `modules/bank` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/specs/tx-format.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/specs/evm-native-accounting.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `modules/evm` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/sdk/rpc-api-versioning.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `cmd/vexod keys` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/sdk/custom-crypto-backend.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/security/audit-readiness.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/specs/networking-spec.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/operators/node-initialization.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/release/launch-runbook.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `cmd/vexod` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/operators/observability.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/release/release-pipeline.md` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.tls_cert_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.tls_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.tls_ca_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `mempool_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `log_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod validate --home <home>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod config audit --home <home> --strict` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution_commit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `allow_unsafe_qc_commit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_propose` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_prevote` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_precommit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_commit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `create_empty_blocks` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getProof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `go.mod` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_score` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make check` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `v1/status` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `active_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_web3Capabilities` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
