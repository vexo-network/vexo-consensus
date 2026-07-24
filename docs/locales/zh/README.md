> Locale: zh · 中文

# 文档

这个目录是 `vexo-consensus` 的实务手册。

它面向那些需要在不靠猜测源码的前提下去理解、构建、运维、审查或发布网络的人。好的文档应当立刻回答下面四个问题：

1. 这个功能在系统中负责什么？
2. 哪些文件、命令、config key、RPC、JSON 字段实现了它？
3. 安全使用它需要满足什么前提？
4. 在上生产网络之前，需要哪些测试和运维证据？

英文是协议、安全、发布、SDK、命令、config 和 RPC 行为的规范来源。各语言的本地化文档是同一套树的直接翻译；发布和审计判断始终要以英文原文为准。

## 快速开始

如果时间不多，可以按下面顺序阅读：

1. [`Node Initialization`](./operators/node-initialization.md) — 了解如何创建 node home、编辑分离的 config 文件，以及启动 validator 或 archive node。
2. [`Docker Deployment`](../../../deployments/docker/README.md) — 运行单主机 4 节点部署，或准备多主机网络。
3. [`Observability Guide`](./operators/observability.md) — 查看节点活着但不正常时最先要看的信号。
4. [`RPC API Versioning`](./sdk/rpc-api-versioning.md) — 了解如何把钱包、Remix 和 Web3 工具连接到 Vexo 的 RPC/Web3 端点。

如果你在看 release candidate，建议先读 [`Production Readiness`](./production-readiness.md) 和 [`Release Pipeline`](./release/release-pipeline.md)，再去看更细的规格。

### 可直接复制的命令

| 작업 | 명령 경로 |
|---|---|
| 로컬 바이너리 빌드 | `make build` |
| 하나의 검증인 홈 만들기 | `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` |
| 홈 검증 | `vexod validate --home .vexo-validator-1` 및 `vexod config audit --home .vexo-validator-1 --strict` |
| 하나의 노드 실행 | `vexod start --home .vexo-validator-1` |
| 하나의 노드 쿼리 | `curl -s http://127.0.0.1:26657/v1/status` |
| Docker 4 validator 네트워크 실행 | `docker compose -f deployments/docker/compose.single-host-init.yml up` 다음에 `docker compose -f deployments/docker/compose.single-host.yml up` |
| Remix 연결 | Docker validator 1 Web3 URL `http://127.0.0.1:28657/web3` |
| Web3 chain ID 확인 | `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` |

## 这套文档的目标

`vexo-consensus` 是一个用来构建独立 PoS 网络的共识与运行时框架。这套文档帮助第一次接触代码的开发者、运维节点的 validator/operator、准备发布的 maintainer，以及准备审计的 security reviewer，用同一套标准理解项目。

命令、JSON 字段、RPC 名称、config key、package path 和代码标识符为了兼容性都保持英文原样；解释、阅读顺序和运维注意事项则用中文展开。

好的文档不是只说“有这个功能”。每篇文档都应该回答下面的问题：

1. 这个功能在系统里承担什么职责？
2. 哪些文件、命令、config key、RPC、JSON 字段实现了它？
3. 要安全使用它，必须满足什么条件？
4. 在上生产网络之前，需要哪些测试和运维证据？

## 首先阅读的顺序

1. [Consensus Protocol Overview](./consensus-protocol.md)
2. [Consensus Spec](./specs/consensus-spec.md)
3. [Transaction Format](./specs/tx-format.md)
4. [Validator Lifecycle](./specs/validator-lifecycle.md)
5. [Node Initialization](./operators/node-initialization.md)
6. [Security Audit Readiness](./security/audit-readiness.md)

## 按角色阅读

| 角色 | 先读什么 | 还要一起看什么 |
|---|---|---|
| 想理解协议的开发者 | [Consensus Spec](./specs/consensus-spec.md) | [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md) |
| 要接入 app module 的开发者 | [App Module Guide](./sdk/app-module-guide.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| 要接入 EVM 功能的开发者 | [EVM and Native Accounting](./specs/evm-native-accounting.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| 运维节点的人 | [Node Initialization](./operators/node-initialization.md) | [Adding a Validator](./operators/add-validator.md), [Observability Guide](./operators/observability.md) |
| 准备发布或审计的人 | [Production Readiness](./production-readiness.md) | [Security Audit Readiness](./security/audit-readiness.md), [Release Pipeline](./release/release-pipeline.md) |

## 协议规格

| 文档 | 目的 |
|---|---|
| [Consensus Spec](./specs/consensus-spec.md) | 共识状态机、安全规则、liveness 假设和证据面 |
| [Finality Proof Format](./specs/finality-proof-format.md) | 给 full node 和 light client 用的 proof 字段与 verifier 规则 |
| [Networking Spec](./specs/networking-spec.md) | transport、handshake、peer scoring、backoff 和 DoS 防御预期 |
| [Storage Schema](./specs/storage-schema.md) | 持久化记录、索引、恢复规则、snapshot 和 schema migration 预期 |
| [Transaction Format](./specs/tx-format.md) | 标准交易载荷、signed envelope、nonce、fee、gas 和 CheckTx 要求 |
| [EVM and Native Accounting](./specs/evm-native-accounting.md) | 共享的 native/EVM 余额模型、256-bit 数值、fee 处理和兼容边界 |
| [Validator Lifecycle](./specs/validator-lifecycle.md) | validator 准入、轮换、证据生命周期、slashing、jailing、unbonding |

## SDK 和扩展指南

| 文档 | 目的 |
|---|---|
| [App Module Guide](./sdk/app-module-guide.md) | 如何添加自定义 application module 和 module CLI 命令 |
| [Custom Crypto Backend](./sdk/custom-crypto-backend.md) | 如何添加 signing/finality backend 与 production BLS adapter metadata |
| [Custom Storage and Transport](./sdk/custom-storage-transport.md) | 如何实现自定义 store 或 peer transport |
| [RPC API Versioning](./sdk/rpc-api-versioning.md) | 如何理解 `/v1/*` 的兼容规则与 endpoint 稳定性 |

## 运维与发布

| 文档 | 目的 |
|---|---|
| [Node Initialization](./operators/node-initialization.md) | validator/archive node 初始化以及分拆后的 subsystem config 管理 |
| [Adding a Validator](./operators/add-validator.md) | 添加 validator 并验证按高度更新 validator set 的运维流程 |
| [Observability Guide](./operators/observability.md) | health、metric、log、alert threshold 和 first-response playbook |
| [发布运行手册](./release/launch-runbook.md) | 运维发布流程、halt 条件、监控和发布后归档要求 |
| [Release Pipeline](./release/release-pipeline.md) | build、sign、package 和 release artifact gate |
| [Cosmos/Tendermint Comparison Gate](./release/cosmos-comparison-gate.md) | 把 Tendermint/Cosmos 的成熟度优势映射成 Vexo 的 release evidence |
| [Version Compatibility Matrix](./release/version-compatibility.md) | binary、config、store、app、RPC 和 proof format 之间的兼容性预期 |

## 安全

| 文档 | 目的 |
|---|---|
| [Security Audit Readiness](./security/audit-readiness.md) | threat model、assumption、limitation、安全论证和必需的审计证据 |

## 本地化文档

locale 文件不能偏离 canonical tree。它们是在不改变命令、JSON 字段、RPC 名称、config key 和代码标识符的前提下做的直接翻译，所以示例在不同语言里复制粘贴后，行为和含义都不应该变。

| 文档 | 目的 |
|---|---|
| [Documentation Locales](../README.md) | locale 目录映射和翻译政策 |
| [English Canonical Docs](../en/README.md) | 规范性的英文文档树 |
| [Chinese Docs](./README.md) | 中文 locale 树 |
| [Korean Docs](../ko/README.md) | 韩语 locale 树 |
| [Japanese Docs](../ja/README.md) | 日语 locale 树 |
| [French Docs](../fr/README.md) | 法语 locale 树 |
| [German Docs](../de/README.md) | 德语 locale 树 |
| [Spanish Docs](../es/README.md) | 西班牙语 locale 树 |
| [Portuguese Docs](../pt/README.md) | 葡萄牙语 locale 树 |
| [Russian Docs](../ru/README.md) | 俄语 locale 树 |
| [Arabic Docs](../ar/README.md) | 阿拉伯语 locale 树 |
| [Hindi Docs](../hi/README.md) | 印地语 locale 树 |
| [Indonesian Docs](../id/README.md) | 印度尼西亚语 locale 树 |
| [Vietnamese Docs](../vi/README.md) | 越南语 locale 树 |

## 编写新文档

文档应该：

- 先写清楚读者的目标，以及这个页面支持什么决策。
- 说明这是 normative spec、implementation guide、operator guide 还是 release/audit checklist。
- 包含相关的命令、package path、config key、RPC method 和 JSON field。
- 解释安全边界、失败模式和不该走的 shortcut。
- 在没有证据之前，不要声称它已经 production-ready。
- 示例尽量可以直接复制，但必须替换的值要明确标出。
- 所有 Markdown 文件都要在 `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` 下镜像一份。
- 运行 `make docs-check`，保证 locale tree 不会和 canonical tree 漂移。

## Production 声明规则

功能有代码不代表就能叫 production-ready。production 声明需要：

- implementation code
- unit/property/adversarial tests
- 如果功能跨进程或跨机器，还需要运维或 E2E 证据
- 对假设和失败模式的文档说明
- 对 BLS、VRF、Web3/EVM compatibility、slashing、state sync、upgrade、validator economics 这类安全敏感领域，还需要 release-gate evidence

`vexod status --json` 也遵循同样的规则。`features` map 表示某条代码路径是否通过 config 打开；`feature_assurance` map 表示这个功能只是实现了、还需要 operator artifact、还需要 release evidence，还是还需要外部审计证据。

分离后的 config 文件放的是运维安全默认值。检查 node home 时，先看下面这些：

- 用于 restart-safe P2P handshake replay protection 的 `network_config.json:p2p.auth_replay_path`
- 用于 peer-authentication key 的 `network_config.json:p2p.node_key_path`（和 validator consensus custody 分开）
- 用于 proposal spam 与 economic-friction policy 的 `module_config.json:governance.RequireDeposit` 和 `module_config.json:governance.MinDeposit`
- 用于 execution/finality 边界的 `consensus_config.json:consensus.execution_commit`
- 用于 restart-safe pending transaction recovery 的 `mempool_config.json:mempool.WALPath`

## 文档检查清单

在合并文档改动之前，请确认：

- 英文文档仍然准确到足以作为发布/审计原文。
- 每个 locale 文件都指向正确的英文 canonical document。
- 命令、RPC 名称、config key、JSON field 和 package name 都保持不变。
- 运行 `make docs-check`。
- 如果命令示例、config schema 或生成物有变化，再运行更宽的项目检查。

## 研究与论文写作

准备论文时，请先阅读 [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md)。该文档把已经实现的自适应轮次超时、恢复终局性门控和确定性交易排序与既有工作区分开，并集中说明研究问题、假设、实验流程、可复现产物及研究伦理。文档不会把尚未测量的性能写成实验结果，也不会把 PoS、BFT 或 HotStuff 本身宣称为新贡献。

跨语言导航保留以下规范文档名称：`Node Initialization`、`Docker Deployment`、`Observability Guide`、`RPC API Versioning`、`Production Readiness`、`Release Pipeline`、`Adaptive Recovery-Gated HotStuff Research Draft`。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: How to Read This Set — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Start Here — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Protocol Specs — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: SDK and Extension Guides — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Operations and Release — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Security — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Localized Documentation — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Writing New Docs — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Production Claim Rule — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Documentation Review Checklist — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `vexo-consensus` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make docs-check` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod status --json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `feature_assurance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.auth_replay_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.node_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json:governance.RequireDeposit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json:governance.MinDeposit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_config.json:consensus.execution_commit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `mempool_config.json:mempool.WALPath` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
