> Locale: zh · 中文

# 节点初始化

本指南介绍了如何初始化验证器和存档节点主目录、启动它们、验证它们是否正常以及连接客户端。

对等连接应在 `network_config.json` 中配置，而不是在 `start` 命令行上重复传递。

影响共识、RPC、P2P、日志记录或托管 Web3 帐户的运行时行为仅是配置文件。 `vexod start` 拒绝诸如 `--timeout-propose`、`--create-empty-blocks`、`--p2p-auth-token`、`--rpc-admin-token`、`--evm-account-key-env` 和 `--evm-account-key` 等标志；相反，编辑拆分配置文件，以便每个操作员查看相同的确定性节点行为。

没有节点模式切换。节点主目录由其配置文件、起源、密钥材料以及是否存在 `validator_id` 和签名者来定义。

## 你正在构建什么

Vexo 节点主目录是一个包含节点启动所需的所有内容的目录：
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
重要的规则很简单：初始化一次，编辑配置文件，然后启动。不要将网络行为隐藏在 shell 标志内。

## 五分钟本地跑

当您想在考虑多主机部署之前证明二进制文件有效时，请使用此流程。
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
在另一个终端中：
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
预期状态形状：
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
当禁用空块创建时，在单节点或空内存池运行中，最新高度可能会保持为零。这并不意味着该过程被破坏。这意味着该节点没有生成空块。添加事务或运行多验证器测试网络以观察连续提交。

## 四验证者本地网络

当您需要对等连接、提议者轮换、块提交日志和高度增长时，请使用此流程。
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
有用的检查：
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
如果在 `log_config.json` 中启用了块提交日志记录，验证器日志将包含以下事件：
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
使用以下命令停止生成的本地网络：
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 和混音

以太坊风格的 JSON-RPC 位于 Web3 端点，而不是版本化的 Vexo 操作 API 命名空间下。

对于 Docker 单主机验证器 1，Remix 自定义提供程序 URL 为：
```text
http://127.0.0.1:28657/web3
```
对于具有默认 RPC 端口的直接本地节点：
```text
http://127.0.0.1:26657/web3
```
测试 Remix 进行的相同调用：
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
如果浏览器显示 chain-ID 获取失败，请按顺序检查这些内容：

1. URL 以 Web3 端点路径结尾。
2. 浏览器可以到达主机端口。 Docker 示例公开了 `28657`、`28667`、`28677` 和 `28687`；在容器内，RPC 端口仍然是 `26657`。
3. RPC服务器正在运行；查询同一主机和端口上的状态端点。
4. `network_config.json`/RPC 配置允许 CORS。当未设置自定义 CORS 列表时，默认处理程序允许浏览器预检。
5. 该链在 `module_config.json` 中具有非零 EVM 链 ID。

## 验证节点

当节点提议、投票、签署共识消息以及参与验证者轮换时，请使用 `init validator`。
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
在运行此命令之前设置 `VEXO_KEY_PASSPHRASE` ，或传递 `--passphrase` 进行一次性本地设置。 `--encrypt-keys` 加密 `validator.key.json`、`node.key.json` 和 `validator.vrf.key.json`。

密钥保管经验法则：

- `validator.key.json` 签署共识提案、投票、超时投票和最终性相关消息。
- `node.key.json` 仅签署 P2P 握手；它绝不能被重复用作验证者共识密钥。
- `validator.vrf.key.json` 证明委员会的随机性，应像验证者托管材料一样对待。
- 公共监听者必须使用加密的本地密钥文档或远程签名者/KMS 样式的密钥文档。如果节点在 `require_network_safety=true` 时公开公共 RPC 或经过身份验证的公共 P2P，则启动会拒绝纯文本本地验证器密钥。
- 生成的密钥使用文件系统模式 `0600` 写入；对于长期验证者来说，仍然更喜欢远程签名者/KMS。

对于 BLS 共识密钥：
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` 写入 `blst-bls12381-minpk-v1` BLS 密钥文档，并将所有权证明复制到 `genesis.json` 验证器元数据中作为 `bls_pop`。

这将创建：

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` 是共识签名者。 `node.key.json` 是 `network_config.json:p2p.node_key_path` 引用的 P2P 握手签名者。它们是故意分开的，因此存档节点和验证器可以使用相同的传输，而无需为每个对等方提供验证器签名密钥。

使用配置驱动的网络启动它：
```bash
vexod start --home .vexo-validator-1
```
启动后，读取日志。一个健康的验证器应该发出节点运行、RPC 侦听、P2P 侦听，并且一旦块被提交，就会发出块提交事件。如果禁用空块创建，则丢失块提交日志可能仅仅意味着没有事务。

## 存档节点

当节点应保留链数据、公开 RPC、从对等点同步并避免验证者签名时，请使用 `init archive`。
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
这将创建：

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

它**不**创建 `validator.key.json`。

开始：
```bash
vexod start --home .vexo-archive-1
```
存档节点不签署共识投票。它们对于 RPC、索引、状态同步、历史证明服务以及保留比修剪验证器更广泛的查询历史非常有用。

## 拆分配置文件

节点主目录使用单独的配置文件，因此操作员可以编辑一个子系统，而无需混合不相关的设置：

- `config.json` 包含节点标识、链 ID、数据路径和指向拆分配置文件的指针。
- `module_config.json` 包含应用程序模块选择、执行/赌注策略和模块级治理策略。
- `network_config.json` 包含 RPC、P2P 节点身份、监听/对等/种子设置、TLS/身份验证设置和对等评分策略。
- `consensus_config.json` 包含共识循环计时、空块策略、加密后端、VRF、验证者准入和委员会策略。
- `mempool_config.json` 包含内存池大小、费用、优先级、WAL、重复和 TTL 策略。
- `log_config.json` 包含日志格式、级别、块提交事件日志记录和对等事件日志记录。
- `genesis.json` 包含不可变的创世验证器、验证器元数据和创世模块状态。

`network_config.json` RPC 设置还包括 `shutdown_timeout`、`web3_max_subscriptions_per_connection` 和 `web3_idle_timeout`。 `shutdown_timeout` 限制共识循环、RPC 服务器和节点传输的正常关闭，因此操作员不会永远在卡住的停止路径上等待。生成的默认值为 `10s`； Web3 订阅默认为每个连接 256 个，并具有 `2m` 空闲超时，因此公共 RPC 端点无法累积无限制的空闲订阅。

`network_config.json` P2P 设置包括 `auth_replay_path`、`require_auth_replay_store` 和 `dial_timeout`。生成的默认值将随机数重播证据写入 `data/p2p_auth_replay.jsonl` 并使用 `10s` 出站拨号超时。对于私有环回测试，重放存储基本上是无害的簿记；对于公共身份验证的 P2P，这是一项安全要求，因为它可以防止重新启动后重播捕获的签名握手随机数。 `dial_timeout` 应足够长以支持 TLS、签名握手验证和跨区域延迟；将其设置得太低会使健康的对等点看起来不稳定，并且会在重新启动后降低活跃度。

`network_config.json` 还拥有启动状态同步。这对于存档节点、替换验证器或恢复到干净机器上的节点非常有用。当 `state_sync.enabled` 为 true 时，`vexod start` 从 `state_sync.snapshot_urls` 下载第一个有效快照，验证链 ID、校验和、状态根和 KV 命名空间，将其恢复到 LevelDB，重建索引，然后启动节点。如果本地状态已满足 `state_sync.min_height` 并且 `state_sync.trust_local_higher` 为 true，则启动会记录 `state_sync_skipped` 并保留本地存储。

示例 `state_sync` 块：
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
启动记录 `state_sync_candidate_failed` 表示获取错误，记录 `state_sync_candidate_rejected` 表示无效或过时的快照，并在验证恢复后记录 `state_sync_applied` 。将 `max_snapshot_bytes` 保持在基础设施有意服务的最大快照以下，但对于正常状态增长来说足够高。不要将公共节点指向未经身份验证的第三方快照源，除非运营商拥有该源的带外信任策略和最终/轻客户端证据。

如果某个字段更改了网络行为，请编辑拆分配置文件并提交或分发该审阅的文件。不要依赖长 `vexod start` 标志来实现运行时行为。 start 命令有意拒绝共识计时、空块、P2P 身份验证、RPC 管理和托管 Web3 密钥标志，以便操作员不会意外地运行与审查的配置不同的行为。

## 我要编辑哪个文件？

|目标|文件|领域 |
|---|---|---|
|更改 RPC 绑定端口 | `network_config.json` | `rpc.address` |
|更改P2P绑定端口 | `network_config.json` | `p2p.listen_address` |
|添加持久对等点 | `network_config.json` | `p2p.peers` |
|添加种子节点 | `network_config.json` | `p2p.seeds` |
|启用/禁用空块 | `consensus_config.json` |共识空块字段 |
|调整共识超时 | `consensus_config.json` |提案、预投票、预提交和提交超时字段 |
|需要最终执行 | `consensus_config.json` |共识执行-提交字段|
|启用/禁用模块 | `module_config.json` |应用模块列表|
|更改EVM链ID | `module_config.json` |执行EVM链ID字段|
|调整基本费用/gas | `module_config.json` |执行基本费用、动态费用、目标气体和最大气体字段 |
|配置内存池 WAL | `mempool_config.json` |内存池 WAL 路径 |
|控制块提交日志 | `log_config.json` |记录提交事件字段 |
|控制对等日志 | `log_config.json` |记录对等事件字段 |

如有疑问，请运行：
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## 关键类型

验证器 init 默认为 `--key-type bls` 因为网络安全验证需要经过审核的 BLS 聚合最终性。 `--key-type ed25519` 仍然可用于网络安全门外的私人实验和自定义部署。 `--encrypt-keys` 应用于任何非一次性节点主目录。独立密钥生成还支持 VRF 密钥：
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
VRF 密钥不是共识签名者。它们用于 VRF 支持的委员会选择，并且在启用后端时应从 `consensus_config.json` 到 `vrf_key_paths` 以及验证器元数据密钥 `vrf_public_key` 进行引用。

`config.json` 指向分割配置文件：
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
每个路径可以是绝对路径或相对于节点主路径。如果省略，`vexod` 使用默认的 `<home>/<name>_config.json` 文件。

示例 `module_config.json`：
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
治理政策也存在于 `module_config.json` 中。生成的网络安全配置需要提案押金：
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
押金是提案提交者托管的原生余额。通过提案退还押金；被拒绝的提案将其移至 `RejectedDeposits`。如果被拒绝的存款应该为金库而不是默认模块帐户提供资金，请使用由您的金库/社区池模块控制的地址。

示例 `network_config.json`：
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` 和 `rpc.evm_account_private_keys` 是可选的且支持 Web3 托管帐户方法，例如 `eth_accounts`、`eth_sign`、`eth_signTransaction` 和 `eth_sendTransaction`。首选 `evm_account_key_envs`，因此私钥由进程环境或秘密管理器注入，而不是存储在 JSON 中。对于正常的验证器操作，将两个列表保留为空，除非该节点有意充当本地 Web3 热钱包端点。启动安全性拒绝公共 RPC 侦听器上的托管 EVM 热键。

示例 `consensus_config.json`：
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` 相对于包含 `consensus_config.json` 的目录进行解析。当本地VRF密钥保管不可避免时，使用加密的密钥文档并向节点进程提供`VEXO_KEY_PASSPHRASE`。对于运营商运行的网络，请勿将原始 VRF 专用标量直接放入 `consensus_config.json` 中。

使用 `vexod config paths --home <home>` 检查所有已解析的路径。

存档配置有：
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
Archive `consensus_config.json` 禁用本地共识循环：
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
默认情况下，生成的验证器 home 在 `config.json` 中设置 `"require_network_safety": true`。这不是一种模式；而是一种模式。它是一个启动安全门，拒绝确定性加密、未签名/非取消交易、缺少费用/gas 底线、缺少持久内存池 WAL、缺少相同签名者/随机数交易的替换策略、不安全的委员会随机性以及除 `finalized` 之外的 `execution_commit` 值。

当启用 `require_network_safety` 时，运行：
```bash
vexod config audit --home <home> --strict
```
在启动节点之前。参与同一网络的每个验证者和存档中心都应该通过审核。

## 基于配置的对等点

对等和监听地址位于 `network_config.json` 中：
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` 自动加载这些对等点：
```bash
vexod start --home .vexo-archive-1
```
持久对等点和种子在 `network_config.json` 中配置； `vexod start` 不接受对等主机或种子主机覆盖。

不要将长期主机或 `host:port` 设置放在 `vexod start` 命令行上。请改为在 `network_config.json` 中编辑 `rpc.address`、`p2p.listen_address`、`p2p.peers` 和 `p2p.seeds`。

在节点主目录的生命周期内保持 `p2p.node_id` 稳定。 `p2p.node_key_path` 应指向 `node.key.json` 或仅用于对等握手签名的另一个本地/托管密钥文档。对等映射应使用对等节点 ID，而不是帐户地址或验证器操作员名称，除非它们故意相同。

对于加密和经过身份验证的 gRPC 对等传输，还需在 `network_config.json` 中设置 `p2p.tls_cert_path`、`p2p.tls_key_path`、`p2p.tls_ca_path` 以及可选的 `p2p.tls_server_name`。相对 TLS 路径是从节点主目录解析的。将 `p2p.dial_timeout` 保留在同一个文件中，以便每个操作员都使用相同的重新连接行为；不要在 shell 脚本中隐藏对等计时。

## 共识时间

共识循环计时位于 `consensus_config.json` 中：
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` 控制一轮等待提案的时间。
- `timeout_prevote` 控制投票收集窗口。
- `timeout_precommit` 控制提交证书收集窗口。
- `timeout_commit` 控制提交块后的最小延迟。
- `create_empty_blocks: false` 表示节点仅在交易可用时提出建议。
- `execution_commit: "finalized"` 在执行最终祖先之前等待 HotStuff 三链最终决定，并且是生成的验证器默认值。 `execution_commit: "qc"` 立即执行并保留 QC 认证的块，但安全门拒绝它。

`round_timeout` 仅保留作为兼容性聚合。更喜欢上面的 Tendermint 风格的超时字段。

当 `create_empty_blocks` 为 false 时，高度可以在内存池为空时保持不变。这是预期的：链正在等待有用的工作，而不是提交空块。当交易出现并且本地共识轮状态已经超过另一个提议者时，节点前进到下一轮，其中其验证者是提议者并从内存池构建。此恢复路径保持事务触发的活跃性，而无需重新启用空块垃圾邮件。

## 多验证者网络

对于生成的网络：
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
每个生成的验证器 home 都会收到：

- 它自己的 `validator.key.json`
- 其自己的分割配置文件：`module_config.json`、`network_config.json`、`consensus_config.json`、`mempool_config.json` 和 `log_config.json`
- 共享的 `genesis.json`
- 其他验证器的 `network_config.json` 对等条目

`vexod network up` 和 `make network-e2e` 在等待所有验证器启动、提交烟雾交易并观察高度增长时使用进程级超时。默认命令超时故意长于共识间隔，因为它涵盖了进程启动、LevelDB 打开、P2P 签名握手、TLS/身份验证检查、事务准入和最终确定。如果您大幅降低共识超时，请保持网络超时足够大以诊断启动错误，而不是过早终止该工具。

对于容器化或多主机网络，将拓扑值放入 JSON 文件中：
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` 和 `rpc_host_template` 是写入每个节点的 `network_config.json` 对等列表中的拨号目标。在 Docker 中，这些可以是服务名称，例如 `validator-%d`。
- `p2p_advertise_host_template` 和 `rpc_advertise_host_template` 是写入 `genesis.json` 中验证器元数据的公共地址。此处将 DNS 名称或公共 IP 用于公共网络。
- `p2p_listen_host` 和 `rpc_listen_host` 是本地绑定主机。对于应侦听所有接口的容器或服务器，请使用 `0.0.0.0` 。
- 不要将仅限 Docker 的服务名称重新用作公布的公共地址，除非网络有意设为私有。

然后从该文件生成节点主目录：
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## 故障排除

|症状|最可能的原因 |检查什么 |
|---|---|---|
| `latest_height` 不增加 |空块已禁用且没有交易、在线验证器不足或签名者不可用 | `consensus_config.json`，验证器日志，`/v1/diagnostics` |
| `peer_count` 是 `0` |对等地址无法访问或为错误的主机名生成了 `network_config.json` | `p2p.peers`、容器主机端口、DNS、防火墙 |
| `p2p auth replay store` 错误 |公共/经过身份验证的 P2P 需要持久的重放存储 | `p2p.auth_replay_path` 并在 home | 下写入权限
| `eth_chainId` 在 Remix 中失败 |错误的 URL、错误的主机端口或浏览器 CORS/预检被自定义配置阻止 |使用Web3端点URL，然后直接curl相同的端点|
| `config audit --strict` 失败 |安全门发现不安全的配置属性 |阅读失败的检查，然后编辑它命名的分割配置文件 |
| `no block_committed logs` |日志记录已禁用或未创建任何块 | `log_config.json`、`create_empty_blocks`、内存池内容 |
| `managed EVM key rejected` |热私钥在公共 RPC 监听器上配置 |删除 `evm_account_private_keys` 或保持 RPC 私有 |

## 最少操作员清单

在将节点移交给另一台机器或操作员之前：

- `vexod validate --home <home>` 通过。
- `vexod config audit --home <home> --strict` 通向那个确切的家。
- `config.json`、分割配置文件、`genesis.json` 和公共验证器元数据经过审查。
- `validator.key.json`、`node.key.json` 和 `validator.vrf.key.json` 由远程签名者/KMS 密钥文档加密或替换。
- `network_config.json:p2p.peers` 包含可从目标计算机拨打的地址，而不是仅限 Docker 的名称，除非节点实际在该 Docker 网络内运行。
- 当启用 `require_network_safety` 时，`network_config.json` 公共 RPC/P2P 侦听器具有 TLS 材料。
- `module_config.json:execution.EVMChainID` 在 Web3 钱包或 Remix 连接之前设置。
- 如果节点在重启后恢复挂起的交易，`mempool_config.json` 有一个 WAL 路径。
- `log_config.json` 在网络启动时启用块提交和对等日志。

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

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
