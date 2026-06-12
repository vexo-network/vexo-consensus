# RPC API Versioning

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 RPC API 版本管理、兼容别名和稳定策略，并把它连接到实际实现和运维判断。

- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/zh/sdk/rpc-api-versioning.md`

## 为什么阅读本文档

- RPC API 版本管理、兼容别名和稳定策略
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

- `/v1`
- `/v1/healthz`
- `/v1/readyz`
- `/v1/status`
- `/v1/diagnostics`
- `/v1/metrics`
- `/v1/metrics/text`
- `/v1/peers`
- `/v1/tx`
- `/v1/evidence`
- `/v1/recovery`
- `/v1/snapshot/latest`
- `/v1/snapshot/export`
- `/v1/snapshot/chunk?index=0&size=10000`
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`

## 英文原文结构

- RPC API Versioning
- 稳定性目标
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## 规范来源

- [英文规范文档](../../en/sdk/rpc-api-versioning.md)

## RPC capability discovery

新的 RPC capability discovery 接口用于检查节点实际挂载的 provider 功能。运行方可以调用 `/v1/capabilities`，SDK 集成方可以使用 `rpc.Config.RequiredCapabilities` 或 `rpc.Config.RequireAllCapabilities` 在启动时 fail closed。

以下接口名称必须保持不变： `/v1/capabilities`, `CapabilityResponse`, `CapabilitySnapshot`, `RequiredCapabilities`, `RequireAllCapabilities`, `metrics`, `blocks`, `finality`, `strict_replay`, `consensus_control`.

<!-- vexo-docs:technical-parity -->
- `admin_token` and `admin_tokens` are stable configuration keys and must remain unchanged when describing optional bearer-token enforcement.

## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Stability Goal — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Current Stable API — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Versioning Rules — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Capability Discovery — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Compatibility Aliases — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Error Format — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Query Proofs — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Event Queries — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: IBC Queries — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Web3 JSON-RPC Bridge — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Web3 EVM Configuration — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Operational Compatibility — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `/v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/healthz` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/readyz` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/status` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/diagnostics` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/capabilities` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/metrics` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/metrics/text` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/tx` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/evidence` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/recovery` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/snapshot/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/snapshot/export` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/snapshot/chunk?index=0&size=10000` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/blocks` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/blocks/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/blocks/{height}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/state/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/state/{height}/{namespace}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/events?key={attribute_key}&value={attribute_value}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/proof?namespace={namespace}&key={key}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/proof?namespace={namespace}&key={key}&height=latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/finality/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/finality/{height}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/ibc/client/{client_id}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/ibc/connection/{connection_id}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/ibc/channel/{port_id}/{channel_id}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/validators/{height}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/committee/{height}/{round}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/prune` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/replay` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/consensus/start` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/consensus/stop` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `tls_cert_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `tls_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `tls_ca_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `tls_server_name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod start` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `strict: true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_gasPrice` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_web3Capabilities` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `require_network_safety` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.NewNetworkSafeServer` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.NewNetworkSafeHandlerWithConfig` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.Config.RequiredCapabilities` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.Config.RequireAllCapabilities` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `pending_txs` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `state_by_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `app_query` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `strict_replay` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_control` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/status` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/tx` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/blocks/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v2/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/proof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `commit_chain` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/status.latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/events` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `Index: true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `{ "path": [...], "value": ... }` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `packets/{source_port}/{source_channel}/{sequence}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_modules` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
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
- `eth_blockNumber` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_blobBaseFee` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_maxPriorityFeePerGas` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_feeHistory` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
