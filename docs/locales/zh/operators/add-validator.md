> Locale: zh · 中文

# 添加验证器

本指南描述了向 Vexo 网络添加验证器的操作流程。

确切的准入路径取决于链的质押和治理政策。至少，验证器必须以链状态表示，具有有效的凭证，并成为高度版本验证器集更新的一部分。

## 1. 初始化验证器主页
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
对于 BLS 验证器密钥：
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
在运行这些命令之前设置 `VEXO_KEY_PASSPHRASE` ，或传递 `--passphrase` 进行一次性本地设置。

当允许 BLS 验证器加入现有链时，请将生成的 `bls_pop` 元数据包含在验证器更新提案中。
默认 BLS 键路径使用 `blst-bls12381-minpk-v1`；仅将 `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` 用于参考/兼容性测试。

归档生成的公钥：
```bash
vexod keys show --home .vexo-validator-new --json
```
同时保留生成的 `node.key.json`。它为 `network_config.json:p2p.node_id` 签署 P2P 握手；它不是验证者共识密钥，不应重复用作帐户密钥。

## 2. 配置网络地址和对等点

编辑 `.vexo-validator-new/network_config.json` 并设置本地监听地址和持久对等点：
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
不要依赖生产验证器的长期命令行网络覆盖。将持久对等地址保留在 `network_config.json` 中。

使用单独的地址角色：

- `p2p.listen_address` 和 `rpc.address` 是该机器或容器的本地绑定地址。
- `p2p.node_id` 是该节点的对等身份。同伴学会后保持稳定。
- `p2p.node_key_path` 指向该对等身份的本地握手签名密钥。
- `p2p.peers` 包含该节点用于联系其他对等点的拨号目标；映射键应该是远程节点的 `p2p.node_id` 值。
- 验证器元数据 `p2p_address` 和 `rpc_address` 应包含公共公布的地址，而不是仅限 Docker 的服务名称，除非网络有意设为私有。

## 3. 提交验证者准入

例如质押流程，构建一个质押交易：
```bash
vexod staking --help
```
验证者准入交易应包括：

- 验证者ID
- 验证者地址
- 共识公钥
- 投票权或股权参考
- 验证者佣金基点，如果链允许自助佣金更新
- P2P `node_id` 元数据（如果链使用创世/验证器元数据来预置对等映射）
- 公共P2P地址元数据
- 公共 RPC 地址元数据（如果是公共的）
- 启用 BLS 时的 BLS 所有权证明元数据

验证器更新必须在特定高度生效并产生新的验证器集哈希。

验证人激活后，运营商可以通过质押模块公开奖励状态：
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. 验证验证器集更新

更新高度后：
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
检查：

- 验证器出现在特定高度集中
- 投票权正确
- 验证器集哈希值按预期更改
- 最终性证明参考正确的验证器设置高度

## 5. 计划验证器密钥轮换

可以通过使用不重叠的 `active_from` 和 `active_until` 元数据准备下一个密钥文档来轮换验证器密钥，然后使用额外的轮换密钥启动节点：
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
签名时，节点使用活动窗口包含共识高度的密钥。远程签名者密钥文档保留相同的策略、身份验证令牌和双重签名防护要求。

## 6. 启动验证器
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
启动时没有网络模式开关。当网络预计满足公共网络安全假设时，请在启动前使用 `config audit --strict` 。

## 7. 监控

观看：

- 提案/投票延迟
- 回合超时
- 验证器签名失败
- 同行禁令
- 内存池大小
- 提交延迟
- 快照/重播健康状况

用途：
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## 安全注意事项

- 切勿在独立链上重复使用验证器密钥。
- 为生产验证器启用远程签名者策略。
- 请勿接纳没有持有证明或同等流氓密钥防御的 BLS 验证者。
- 如果没有经过验证的证据与正确的证据高度验证器集相关联，请勿削减或监禁验证器。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: 1. Initialize Validator Home — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: 2. Configure Network Addresses and Peers — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: 3. Submit Validator Admission — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: 4. Verify Validator Set Update — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: 5. Plan Validator Key Rotation — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: 6. Start Validator — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: 7. Monitor — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Safety Notes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `VEXO_KEY_PASSPHRASE` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--passphrase` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bls_pop` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blst-bls12381-minpk-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node.key.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json:p2p.node_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `.vexo-validator-new/network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `network_config.json` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.listen_address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc.address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.node_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.node_key_path` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p.peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p_address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_address` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `active_from` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `active_until` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `config audit --strict` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
