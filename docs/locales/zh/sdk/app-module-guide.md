# 应用模块指南

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。

## 先读什么

本文档说明如何向 Vexo 添加 application module。第一次接 module 时，建议按下面顺序阅读。

1. Module interface
2. Transaction routing
3. Module configuration
4. State and events
5. Genesis and ante handling
6. CLI commands and tests

这个顺序基本对应实际开发流程：先定义 module 形状，再决定它如何接收 transaction，接着明确它拥有哪些 state，最后把 CLI 和 test 接上去。

## 文档概览

本文档帮助你理解 创建新的 app module 并接入 CLI/RPC/状态存储的方法，并把它连接到实际实现和运维判断。

- Canonical path: `docs/sdk/app-module-guide.md`
- Locale path: `docs/locales/zh/sdk/app-module-guide.md`

## 为什么阅读本文档

- 创建新的 app module 并接入 CLI/RPC/状态存储的方法
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

- `app.Module`
- `app.QueryHandler`
- `app.ValidatorUpdateProvider`
- `app.TxEventEmitter`
- `app.PruneHook`
- `bank`
- `bank:`
- `module_config.json`
- `config.json`
- `module_config_path`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `app.Context.Store`
- `ctx.GoContext()`
- `CheckTx`
- `PrepareProposal`
- `ProcessProposal`
- `FinalizeBlock`
- `Query`
- `params`

## 英文原文结构

- App Module Guide
- 目标
- Module Interface
- Transaction Routing
- Module Configuration
- State
- Events and Query Proofs
- IBC and Contract Extension Points
- Genesis
- Ante Handling
- CLI Commands
- Tests

## 规范来源

- [英文规范文档](../../en/sdk/app-module-guide.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Goal — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Module Interface — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Transaction Routing — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Module Configuration — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: State — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Events and Query Proofs — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: IBC and Contract Extension Points — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Genesis — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Ante Handling — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: CLI Commands — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Tests — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `app.Module` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `app.QueryHandler` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `app.ValidatorUpdateProvider` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `app.TxEventEmitter` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `app.PruneHook` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bank:` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `mempool_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `log_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `app.Context.Store` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `ctx.GoContext()` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `params:set:<authority>:<module>:<key>:<base64-value>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `params/param/<module>/<key>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `events.Indexer` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `queryproof.Build` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `queryproof.Verify` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `contract.Result` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `modules/evm/backend/geth` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `modules/evm/ethcompat` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm state-backend` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `github.com/ethereum/go-ethereum` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-tx-fixtures-sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-execution-fixtures-sha256` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_sendRawTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution.allow_unprotected_legacy_tx` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getProof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm/storage/{address}/{slot}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_ethstate/{height}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `state_diff` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vm_trace` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBalance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getTransactionCount` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getCode` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getStorageAt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_call` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_estimateGas` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `params.ChainConfig` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_createAccessList` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getTransactionReceipt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBlockReceipts` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getTransactionByHash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getLogs` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `relayer_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `ibc/capabilities` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo-queryproof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `client-create` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--authority` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--signer` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `client-update` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `proof_json_base64` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/state/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `relayer client-update --source-rpc` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `failure_backoff` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_modules` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_web3Capabilities` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `web3_clientVersion` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `web3_sha3` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `net_version` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `net_listening` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `net_peerCount` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_chainId` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_protocolVersion` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_syncing` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_mining` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_hashrate` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_accounts` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_coinbase` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_blockNumber` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBlockByNumber` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBlockByHash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBlockTransactionCountByNumber` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBlockTransactionCountByHash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getTransactionByBlockNumberAndIndex` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getTransactionByBlockHashAndIndex` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getUncleCountByBlockNumber` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getUncleCountByBlockHash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。

## Stable Terms

- `execution.evm_fork_preset = "latest"`
- `execution.evm_chain_config_json`
