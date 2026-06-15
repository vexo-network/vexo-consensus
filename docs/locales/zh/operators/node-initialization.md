# Node Initialization

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档面向第一次创建 node home 的人，也面向已经在运维节点的人。第一次阅读时，建议按下面顺序看。

1. 你在构建什么
2. 五分钟本地运行
3. 四验证者本地网络
4. Web3 和 Remix
5. Validator Node
6. Archive Node
7. Split Configuration Files
8. Which File Do I Edit?
9. Key Types
10. Config-Based Peers
11. Consensus Timing
12. Multi-Validator Network
13. Troubleshooting
14. Minimal Operator Checklist

这个顺序就是运维时最先要确认的顺序：先理解 node home 是什么，再在本地启动确认二进制可用，然后区分 validator 和 archive，最后检查 peer、时序和故障处理。

## 文档概览

本文档帮助你理解 archive 节点与 validator 节点初始化，以及拆分配置文件的运维，并把它连接到实际实现和运维判断。

- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/zh/operators/node-initialization.md`

## 为什么阅读本文档

- archive 节点与 validator 节点初始化，以及拆分配置文件的运维
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

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
- `--evm-account-key-env`
- `--evm-account-key`
- `validator_id`
- `init validator`
- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `--encrypt-keys`
- `validator.key.json`
- `validator.vrf.key.json`
- `--key-type bls`
- `genesis.json`
- `bls_pop`
- `config.json`
- `module_config.json`
- `consensus_config.json`
- `mempool_config.json`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## 英文原文结构

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## 规范来源

- [英文规范文档](../../en/operators/node-initialization.md)
<!-- vexo-docs-ops-update-2026-06 -->

## 最新运维说明

新的节点目录需要一起审查 `network_config.json` 中的 `p2p.dial_timeout`, `p2p.auth_replay_path`, `p2p.require_auth_replay_store`。默认 `10s` dial timeout 覆盖 TCP 连接、TLS、signed handshake 和 replay-store 检查。公网部署时不要把这些行为藏在 shell flag 中，应放入配置审查流程。

## 启动时 State Sync

`network_config.json` 中的 `state_sync` 块用于新 archive 节点、替换 validator 或在干净机器上恢复的节点。`state_sync.enabled` 为 true 时，`vexod start` 会按顺序尝试 `state_sync.snapshot_urls`，校验 chain ID、checksum、state root 和 KV namespace，然后写入 LevelDB、重建索引，最后才启动节点。如果本地状态已经达到 `state_sync.min_height`，并且 `state_sync.trust_local_higher` 为 true，节点会保留本地 store 并记录 `state_sync_skipped`。

```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```

运维人员应检查 `state_sync_candidate_failed`、`state_sync_candidate_rejected` 和 `state_sync_applied` 日志。公开网络不要使用没有信任策略和 finality/light-client evidence 的第三方 snapshot 源。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Validator Node — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Archive Node — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Split Configuration Files — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Key Types — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Config-Based Peers — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Consensus Timing — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Multi-Validator Network — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod start` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--timeout-propose` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--create-empty-blocks` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--p2p-auth-token` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--rpc-admin-token` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-account-key-env` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--evm-account-key` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `validator_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `VEXO_KEY_PASSPHRASE` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--passphrase` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--encrypt-keys` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `validator.key.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node.key.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `validator.vrf.key.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `require_network_safety=true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--key-type bls` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blst-bls12381-minpk-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `genesis.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bls_pop` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `module_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `mempool_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `log_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `data/` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.node_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `shutdown_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `web3_max_subscriptions_per_connection` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `web3_idle_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `auth_replay_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `require_auth_replay_store` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `dial_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `data/p2p_auth_replay.jsonl` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--key-type ed25519` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf_key_paths` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vrf_public_key` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `<home>/<name>_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.evm_account_key_envs` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.evm_account_private_keys` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_accounts` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_sign` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_signTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_sendTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_account_key_envs` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod config paths --home <home>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `"require_network_safety": true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution_commit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `require_network_safety` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `host:port` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.listen_address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.seeds` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.node_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.node_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_cert_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_ca_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.tls_server_name` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.dial_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_propose` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_prevote` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_precommit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_commit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `create_empty_blocks: false` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution_commit: "finalized"` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `execution_commit: "qc"` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `round_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `create_empty_blocks` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod network up` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `make network-e2e` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p_host_template` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_host_template` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `validator-%d` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p_advertise_host_template` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_advertise_host_template` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p_listen_host` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_listen_host` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
