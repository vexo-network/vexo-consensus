# 存储架构

> Locale: zh · 中文
> 本文档是英文原文的中文直译。协议、安全和发布判断以英文原文为准。


## 先读什么

本文档说明 Storage Schema 的规范定义。第一次阅读时，建议按下面顺序看。

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

这个顺序对应你的阅读方式：先看范围和状态，再看消息、正确性与活性规则，最后看证据。

## 文档概览

本文档帮助你理解 durable storage namespace、key schema 和 recovery marker，并把它连接到实际实现和运维判断。

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/zh/specs/storage-schema.md`

## 为什么阅读本文档

- durable storage namespace、key schema 和 recovery marker
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

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## 英文原文结构

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## 规范来源

- [英文规范文档](../../en/specs/storage-schema.md)

## Pebble 后端与二进制记录

默认后端是 Pebble(`github.com/cockroachdb/pebble/v2`),以 256 MiB block cache 和 64 MiB memtable 打开。记录以 `types` 包的规范二进制编码存储:最小 uvarint、带长度前缀的字节串和列表、原样 32 字节的哈希、`0` 或 `1` presence byte 之后的可选值,以及按声明结构体顺序排列的字段。解码器拒绝其他任何内容,包括多余的尾部字节,因此损坏的记录会被报告为错误而不会被读成另一个值,每条记录只有一种编码。早期构建(LevelDB、JSON 记录)写入的数据目录无法读取,也不会被转换:Pebble 会以 `format major version 1` 错误拒绝打开。此类节点应从 snapshot 恢复,或同步到空的数据目录。

相关标识符:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## 状态历史(undo log)与 tree cache

状态历史是按 key 的版本组成的 undo log。key 在某个高度变化时(区块的写入在其高度,第一个 committed state 之后的直接写入(`Set`、`Delete`、`SetBatch`)在下一个 committed 高度),store 把 key 在此之前的值(包括不存在)记录为该高度的版本,每个 key 和高度只记录一次。在提交该高度的区块之前进行的直接写入保留其中第一个记录的版本(`hdirect` 指出该高度)。key 在某个 committed 高度的值是该高度之上最低版本所记录的值;没有这样的版本时就是当前值。记录变化只读取 key 的当前值,不读取历史;历史读取是对 key 版本的一次 seek。`PruneBelow` 删除保留起始高度及以下的所有版本。tree node 以同样方式记录版本。两个内存结构减少区块的 tree 工作,都不持久化,也不改变结果:tree node cache 最多保存当前 tree 的 2^18 个已编码 node(store 提交的每个 batch 都会更新它,batch 提交期间为奇数的 generation 使 reader 只在其 snapshot 的状态下使用 cache);以及按 namespace 保存最近一次 root 计算的 root 和 node 变化,以该 generation 和 namespace 写入的 SHA-256 digest 为键,使 app hash、state root 和 commit 对相同写入只计算一次。

## Mempool WAL 压缩与 genesis batch

只有当 mempool WAL 至少 4 MiB 且达到上次压缩(或打开)后大小的两倍时,commit 才会压缩日志。因此每个追加的字节平均只被重写约一次,重启时 replay 的日志保持在 live 内容的两倍或 4 MiB 以内。显式压缩(`Node.Compact`,或启动时丢弃 application 拒绝的 replay 交易)总是重写日志。`InitChain` 把所有 module 的 genesis state 作为一个 batch 写入,因此账户很多的 genesis 也只需一次持久写入,失败的 genesis 不会在 store 中留下任何内容。

## Commit batch 中的 finality proof 与延迟的 mempool 移除

节点 commit 路径把 block 的 QC 所完成的 finality proof 与 block/state/app-write batch 一起写入(`CommitBlockStateWithWrites` 的最后一个参数)。因此一次 commit 只需一次 durable write,而不是每个 proof 再多一次。store 会拒绝的 proof 不进入 batch,由 commit 之后的步骤保存并报告失败。commit 从 pool 中移除自己的交易,并在不等待 sync 的情况下把移除 record 放入队列。下一次 commit 在执行自己的 block 之前等待该 record 变为 durable,因为重启只会重做最后一个已 commit block 的移除。移除 record 通常与随后 admission 的 sync 共用一次。

## State tree 的延迟哈希

state tree 的计算会推迟哈希:write 只标记它改变的那一半,在请求 root、proof 或变更时,把改变过的 node 自下而上各哈希一次。因此区块中每个 write 都会重写的 root 附近的 node,每个区块只哈希一次,而不是每个 write 一次。得到的哈希、node 和 root 与每次 write 都哈希时相同。

## Mempool WAL 的定期 sync

mempool WAL 在每次变更时写入 log,每 100 ms sync 一次(`walSyncInterval`)。admission 只等待写入,不等待 sync。进程崩溃时记录仍在,机器崩溃时最多丢失最后一个间隔内的 admission,由客户端重新提交。已 commit block 的移除 record 在下一次 commit 之前完成 sync(`WaitDurable`)。

## History 记录复用读取

commit 把被改 key 的旧值记录为 version,但通常不再读取:store 记住自上次 batch commit 以来 `Get` 读到的值(最多 2^16 个),而 block 执行大多会先读它要改的 key。只有读取之后没有 batch commit 时才使用该值,因此它就是当前值。不存在的 key 不会被记住,仍然读取。

## State tree node 的版本存储

tree node 按版本存储,之后的高度不会重写。batch 把改变的 node 写到 `tree: | namespace | height | node key`,internal node 同时存储两个 half 的版本,因此可以从 parent 找到 child。root record(`troot:`)标记 tree 改变的高度,任何高度的 tree 都从该高度及以下最后一个 root record 开始读取。被替换的 node 为更低的高度保留,并列入替换高度的 orphan list(`torph:`,每个 batch 一个),由 `PruneBelow` 删除。版本只增不减,区块的写入不会分散到已有的 key 之间,使 Pebble 的 flush 和 compaction 更小。key 的 version index(`hidx:`)也是每个 batch 一个条目。

相关标识符:

- `troot: | namespace | height`
- `torph: | height | first entry`
- `hidx: | height | first group`

## Tree node cache 保存解码后的 node

tree node cache 以解码后的形式保存当前 tree 的 node 及其 half 的版本,最多 2^18 个,因此读取既不解码也不分配内存。

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Scope — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Backend — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Records — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Indexes — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: EVM Records — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Recovery Rules — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Snapshot Validation — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Schema Migration — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `store.Store` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getBalance` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `eth_getProof` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `bank/{0x_address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `auth/nonce/{0x_address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm/code/{0x_address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm/storage/{0x_address}/{slot}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `seen_ttl` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `code/{address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `storage/{address}/{slot}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `receipts/{tx_hash}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `logs/by_height/{height}/{tx_hash}/{log_index}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `logs/{address}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evm_receipts` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `receipt_index/{tx_hash}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blob_sidecars/by_tx/{tx_hash}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `blob_sidecars/by_blob/{blob_hash}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `tree:` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `receipt_height/{height}/{tx_hash}` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `receipt_pruned_below` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
