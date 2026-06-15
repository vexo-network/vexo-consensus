# 网络规范

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Networking Spec 的规范定义。第一次阅读时，建议按下面顺序看。

1. Scope
2. Transport
3. Topics
4. Handshake
5. Wire Compatibility
6. Address Roles
7. Peer Scoring
8. Reconnect and Backoff
9. DoS/DDOS Defenses
10. Operational Signals

这个顺序对应你的阅读方式：先看范围和状态，再看消息、正确性与活性规则，最后看证据。

## 文档概览

本文档帮助你理解 P2P handshake、gossip、peer scoring 和 ban 策略，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/networking-spec.md`
- Locale path: `docs/locales/zh/specs/networking-spec.md`

## 为什么阅读本文档

- P2P handshake、gossip、peer scoring 和 ban 策略
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

- `consensus`
- `tx`
- `commit`
- `evidence`
- `network_config.json`
- `rpc.address`
- `p2p.listen_address`
- `p2p.peers`
- `p2p.seeds`
- `p2p_address`
- `rpc_address`
- `host:port`
- `0.0.0.0:26656`
- `[::]:26656`
- `0`
- `p2p.tls_cert_path`
- `p2p.tls_key_path`
- `p2p.tls_ca_path`
- `p2p.tls_server_name`
- `start`
- `BanThreshold`
- `MaxScore`

- `validator_id`
- `p2p.node_id`
- `node.key.json`
- `p2p.node_key_path`
- `signature_nonce`
- `node_public_key`
- `signature`
- `Wire Compatibility`
## 英文原文结构

- Networking Spec
- Scope
- Transport
- Topics
- Handshake
- Address Roles
- Transport TLS
- Peer Scoring
- Reconnect and Backoff
- DoS/DDOS Defenses
- Operational Signals

## 规范来源

- [英文规范文档](../../en/specs/networking-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Peer 时序与固定 Peer

仅因为临时 dial 失败，不会 ban configured peer 或 seed。失败会进入 backoff 和诊断信息；ban 应来自恶意 gossip、认证失败或 rate-limit abuse 等行为证据。`p2p.dial_timeout` 应根据跨区域延迟以及 TLS/auth 成本来设置。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Transport — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Topics — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Handshake — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Wire Compatibility — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Address Roles — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Peer Scoring — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Reconnect and Backoff — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: DoS/DDOS Defenses — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Operational Signals — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `validator_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.node_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node.key.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.auth_replay_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.node_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.dial_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `signature_nonce` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node_public_key` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.listen_address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.seeds` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p_address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `host:port` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `0.0.0.0:26656` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `[::]:26656` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_cert_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_ca_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_server_name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.tls_cert_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.tls_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.tls_ca_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.tls_server_name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.admin_token` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.admin_tokens` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
