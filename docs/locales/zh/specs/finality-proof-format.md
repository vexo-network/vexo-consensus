# 最终性证明格式

> Locale: zh · 中文
> 本文档是配合英文原文阅读的中文 辅助文档。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Finality Proof Format 的规范定义。第一次阅读时，建议按下面顺序看。

1. Scope
2. Proof Fields
3. Header Fields
4. Quorum Certificate Fields
5. Commit Chain Fields
6. Verification Algorithm
7. Accountable Safety Detection
8. Ed25519 Model
9. BLS Model

这个顺序对应你的阅读方式：先看范围和状态，再看消息、正确性与活性规则，最后看证据。

## 文档概览

本文档帮助你理解 finality proof 字段、验证顺序和 validator set binding，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/zh/specs/finality-proof-format.md`

## 为什么阅读本文档

- finality proof 字段、验证顺序和 validator set binding
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

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## 英文原文结构

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## 规范来源

- [英文规范文档](../../en/specs/finality-proof-format.md)

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Proof Fields — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Header Fields — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Quorum Certificate Fields — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Commit Chain Fields — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Verification Algorithm — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Accountable Safety Detection — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Ed25519 Model — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: BLS Model — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `finality.Proof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/finality/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/finality/{height}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `strict: true` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/status.latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/finality/*` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `Proof.ValidatorSetHeight <= Header.Height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `Proof.ValidatorSetHash == loaded_set.Hash()` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `Header.ValidatorSetHash == loaded_set.Hash()` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `QuorumCert.Height == Header.Height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `Header.TxRoot` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `HeaderHash(link.Header)` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `finality.AttackDetector` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `--validator-set` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blst-bls12381-minpk-v1` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `supranational/blst` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
