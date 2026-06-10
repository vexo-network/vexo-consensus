# Adding a Validator

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。

## 文档概览

本文档帮助你理解 添加 validator 的流程、配置校验和 staking 检查，并把它连接到实际实现和运维判断。

- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/zh/operators/add-validator.md`

## 为什么阅读本文档

- 添加 validator 的流程、配置校验和 staking 检查
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

- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `bls_pop`
- `.vexo-validator-new/network_config.json`
- `network_config.json`
- `p2p.listen_address`
- `rpc.address`
- `p2p.peers`
- `p2p_address`
- `rpc_address`
- `active_from`
- `active_until`
- `config audit --strict`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## 英文原文结构

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## 规范来源

- [English canonical document](../../en/operators/add-validator.md)
