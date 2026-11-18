# 交易格式

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Tx Format 的规范定义。第一次阅读时，建议按下面顺序看。

1. Scope
2. Canonical Payload
3. Address Format
4. Signed Envelope
5. Required Ante Metadata
6. CheckTx Requirements
7. Fee and Gas
8. Load Test Payloads
9. CLI Examples

这个顺序对应你的阅读方式：先看范围和状态，再看消息、正确性与活性规则，最后看证据。

## 文档概览

本文档帮助你理解 transaction format、signing、fee 和 gas 规则，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/zh/specs/tx-format.md`

## 为什么阅读本文档

- transaction format、signing、fee 和 gas 规则
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

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## 英文原文结构

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## 规范来源

- [英文规范文档](../../en/specs/tx-format.md)

## 二进制签名 envelope

签名交易是二进制的(`txcodec.EncodeEnvelope`):两字节前缀 `0x89 'V'`(`txcodec.SignedPrefix`)、版本字节 `1`,随后是四个带 uvarint 长度前缀的字段:chain ID(最多 `txcodec.MaxChainIDLength`,即 128 字节)、公钥(最多 `txcodec.MaxPublicKeyLength`)、签名(最多 `txcodec.MaxSignatureLength`)和 payload。解码器拒绝非最小长度、超限字段、未知版本和多余的尾部字节,因此签名交易只有一种编码。`0x89` 不是合法的 UTF-8,文本 payload 不会被误认为 envelope。节点在启动时拒绝长于 `txcodec.MaxChainIDLength` 的 chain ID(`config.ErrChainIDTooLong`)。

文本接口以 base64 传递签名交易。`vexod keys sign-tx` 输出 `tx: <base64>` 和 `encoding: base64`;`POST /v1/tx` 除非请求设置 `"encoding": "plain"`,否则按 base64 读取 `tx`;`vexod tx parse --encoding base64` 读取签名交易,`--encoding plain`(默认)读取文本 payload。三者都通过 `txcodec.DecodeTxEncoding` 解码。

ante handler 只记住成功的认证(`app.VerifyCache`,最多 `app.VerifyCacheCapacity` 条),键是交易字节以及 chain ID 等判定所依赖的一切内容的摘要;失败从不被记住。区块中的签名在按顺序处理之前并行验证。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Canonical Payload — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Address Format — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Signed Envelope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Required Ante Metadata — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: CheckTx Requirements — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Fee and Gas — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Load Test Payloads — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: CLI Examples — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `gas_limit` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_chain_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `chain_id` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `base_fee` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max(min_fee, base_fee * gas)` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blob_base_fee` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blob_gas` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blob_gas_fee_cap` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_sendRawBlobTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blob_hashes` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_getBlobSidecarByTxHash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_getBlobSidecarByBlobHash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_chainId` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `net_version` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_sendRawTransaction` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `dynamic_base_fee` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `target_gas` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `dynamic_blob_base_fee` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `target_blob_gas` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bank:send` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bank:send:<from>:<to>:<amount>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bank:mint:<to>:<amount>` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_raw` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_hash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_txs` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_total_bytes` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_txs_per_sender` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
