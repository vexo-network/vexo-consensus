> Locale: zh · 中文

# 可观察性指南

本指南解释了如何通过 RPC、指标、日志和发布证据判断 Vexo 节点是否健康。

它是为需要实用信号的操作员编写的：要观察什么、每个数字的含义以及何时应将某个值视为危险值。

## 概览

如果某个节点看起来有问题，请按顺序检查这些：

1. `/v1/status` 中的 `running` 和 `latest_height`
2. `latest_finalized_height` 和对等计数
3. `round_timeout`，提案/投票延迟、内存池大小和提交延迟指标
4. 签名者失败、快照健康状况和重放健康状况
5. 对等禁止和对等拨号失败

这个顺序很重要，因为它将“流程处于活动状态”与“链条实际上正在安全进展”分开。

## 核心端点

|端点 |使用|
|---|---|
| `/v1/status` |快速流程、高度、应用程序哈希、最终性和同行总结 |
| `/v1/metrics` |用于仪表板和自动化的 JSON 指标 |
| `/metrics/text` | Prometheus 兼容的文本指标 |
| `/v1/diagnostics` |组合准备情况、功能、状态、对等、存储和指标检查 |
| `/v1/finality/latest` |轻客户端和安全检查的最新最终性证明 |
| `/v1/state/latest` |最新状态根和验证器集绑定 |
| `/v1/recovery/report` |崩溃/重启一致性诊断|
| `/v1/snapshot` |快照运行状况和导出元数据 |

修剪、重放和共识控制等管理端点通常只能通过环回、运营商网络、mTLS 或经过身份验证的网关访问。范围管理令牌仍然是可选的，并且在配置时强制执行。

## 读取 `/v1/status`

重要字段：

|领域 |意义|操作员注意事项|
|---|---|---|
| `running` |节点进程已启动并拥有运行时状态 | `true` 本身并不能证明共识活跃度 |
| `latest_height` |最新本地承诺应用程序高度 |实时验证器网络上必须随着时间的推移而增加 |
| `latest_finalized_height` |最新HotStuff三链定型高度|不应无限期地落后于执行/承诺的高度 |
| `latest_app_hash` |应用提交哈希 |应该与同龄人匹配|
| `peer_count` |向后兼容的连接/评分对等摘要 |更喜欢下面更具体的同行字段 |
| `active_peer_count` |活动传输会话，当传输可以报告它们时 |实时 P2P 连接的最佳快速信号 |
| `configured_peer_count` |配置或学习的对等地址 |无法保证可达性 |
| `scored_peer_count` |同行已知分数表|对于禁令/速率限制历史很有用，而不是实时会话的证明 |
| `banned_peers` |目前评分政策禁止的同行 |峰值表示存在攻击、错误的对等配置或限制过于严格 |

4 验证器单主机网络的健康示例：`running=true`、`latest_height` 增加、`latest_finalized_height` 存在、`active_peer_count` 靠近 `3` 和 `banned_peers=0`。

## 普罗米修斯指标

文本端点公开仪表，例如：

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`

`vexo_peer_count` 保留用于较旧的仪表板。新仪表板应分别绘制 `vexo_active_peer_count`、`vexo_configured_peer_count` 和 `vexo_scored_peer_count` 图表。

## 建议的警报规则

调整实际验证器数量、块间隔、延迟和硬件的数字。这些是起点，而不是普遍常数。

|警报 |起始条件 |为什么 |
|---|---|---|
|节点向下| `vexo_node_running == 0` 1 分钟 |进程/运行时停止 |
|身高停滞| `latest_height` 在 2-3 个预期区块间隔内保持不变 |共识或执行陷入停滞 |
|最终性陷入停滞| `latest_finalized_height` 不变，而块继续执行 |最终路径或法定人数问题 |
|没有活跃的同行 |在非隔离节点上 `vexo_active_peer_count == 0` 1 分钟 | P2P 中断、身份验证不匹配或地址问题 |
|同行人数太少 |活动对等点低于仲裁连接目标 |分区或引导程序问题 |
|回合超时秒杀|超时计数器的增长速度快于正常基线 |延迟、提议者失败或网络分区 |
|提交延迟高 | p95/p99 接近共识超时预算 |存储/运行时过载 |
|内存池压力 |内存池大小增长了几分钟 |费用政策、垃圾邮件或区块容量问题 |
|快照不健康 | `vexo_snapshot_healthy == 0` |状态同步/恢复风险 |
|重播不健康| `vexo_replay_healthy == 0` |决定论或状态一致性风险|
|签名者失败 | `vexo_validator_signing_failures > 0` | KMS/远程签名者/策略失败 |
|协调失败 | `vexo_post_commit_reconciliation_failures > 0` |需要持久证据或进行维修|
|禁止同行秒杀|同行封禁率骤然上升|攻击、错误配置的对等点或评分阈值问题 |

## 建议的起始阈值

使用这些作为初始警报值，然后在真正的长期运行基线后进行调整：

|信号|警告|关键|第一个行动|
|---|---:|---:|---|
|身高比率|低于 2 个窗口预期的 50% | 2-3 个区块间隔零增长 |比较所有验证器，检查提议者/签名/对等日志 |
|最终确定的高度滞后 |生长5分钟| 10分钟内执行身高持续增长|检查 QC/最终性证明日志和验证器集哈希 |
|活跃的同行|低于法定连接目标|零活跃同伴 |检查广告地址、TLS/auth、创世/链 ID 不匹配 |
|回合暂停 | 3x 正常基线 |连续超时循环|提高超时预算或调查延迟/分区 |
|提案延迟 p95 |超过 `timeout_propose` 的 50% |超过 `timeout_propose` 的 80% |配置文件提议者、mempool、DA 承诺、磁盘 |
|投票延迟 p95 |超过预投票/预提交预算的 50% |超过预算的 80% |检查 CPU、签名者、传输、八卦背压 |
|提交延迟 p95 |超过区块间隔的 50% |超过区块间隔的 80% |检查 LevelDB、状态根、EVM 执行、快照 |
|内存池大小 |增加 5 分钟 |接近 `max_txs` 或持续更换流失 |检查基本费用、最低费用、交易有效性、垃圾邮件 |
|签名者失败 |任何非零值 |一高窗屡屡失败|如果出现双符号保护或密钥不匹配，则停止验证器 |
|快照健康状况 |一项失败的检查|重复失败的导出/验证/恢复 |暂停状态同步服务并运行恢复报告 |
|重播健康 |一次严格重放失败 |在最新安全高度重放不匹配|保留数据目录并停止不安全的升级/发布 |
|被禁止的同行 |突然飙升|配置推出后许多同行被禁止 |检查分数上限、TLS CA、对等身份、可选的身份验证证明和时钟偏差 |

最重要的规则：对**随时间变化**发出警报。单一数字可能会产生误导；高度率、最终确定滞后、同行流失、内存池增长和签名者失败共同讲述了真实的故事。

## 事件分类矩阵

|情况|可能的层 |保存什么？安全下一步|
|---|---|---|---|
|身高停止，同龄人健康|共识/签名者/运行时 |共识日志、签名者日志、内存池样本 |验证提议者密钥和回合超时日志 |
|部署后对等点就掉线了 |网络/配置 |网络配置、TLS 证书、addrbook、对等日志 |回滚广告地址/TLS/身份验证更改 |
|应用程序哈希值在相同高度上不同 |执行/存储 |数据目录、块记录、应用程序日志、重放输出 |停止受影响的节点并运行严格重放 |
|最终性证明被拒绝 |最终性/验证器集 |证明 JSON，验证器设置在证明高度 |验证验证器集哈希和符号字节域 |
|快照恢复失败 |状态同步/存储|快照文件、校验和、状态根、恢复日志 |不要针对实时数据重试；恢复到干净的目录 |
|远程签名者拒绝请求 |钥匙保管|签名者审核日志、保护文件、随机数文件、节点日志 |区分政策拒绝和运输中断|
|封禁同行秒杀| P2P/安全 |同行评分快照和禁止原因 |检查格式错误的八卦或共享错误的配置|

在发生事件期间，更喜欢保留数据而不是“清理”。删除 WAL、addrbook、signerguard 或 LevelDB 目录可能会破坏区分 bug 和操作员错误所需的证据。

## 记录要保留的事件

结构化日志应保留节点 ID、验证者 ID、链 ID、高度、轮次、区块哈希和相关的对等 ID。

重要事件：

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

对于候选版本，将日志与指标样本、pprof 样本、配置文件、起源、二进制校验和和证据清单一起存档。

## 第一反应手册

当操作员发现问题时：

1. 在至少两个验证器上检查 `/v1/status`。
2. 比较 `latest_height`、`latest_finalized_height`、`latest_app_hash` 和对等计数。
3. 检查 `/v1/diagnostics` 是否缺少功能或不健康的存储/重播/快照检查。
4. 检查对等事件日志中的身份验证、TLS、创世、链 ID 或退避错误。
5. 如果不包括交易，则检查内存池和基本费用指标。
6. 如果验证者签名失败，请验证签名者和远程签名者日志。
7. 在删除或修改数据之前导出恢复报告。
8. 如果怀疑最终性冲突，请停止自动化，保留日志/证据，并运行最终性冲突检测。

## 仪表板布局

有用的仪表板通常有五行：

1. **活跃度**：节点运行情况、最新高度、最终高度、高度比率。
2. **共识延迟**：回合超时、提案/投票/提交 p95 和 p99。
3. **网络**：活动/配置/评分对等点、禁止对等点、对等点窗口消息。
4. **执行**：内存池大小、gas/基本费用、交易计数、提交延迟。
5. **恢复和安全**：快照运行状况、重放运行状况、签名者失败、协调失败。

让仪表板保持无聊。目标不是显示每个内部计数器；而是显示每个内部计数器。这是为了在验证者出现分歧或用户注意到交易停滞之前使危险状态变得明显。

## 从可观察性中释放证据

对于候选版本来说，可观察性不仅仅是实时监控。它成为证据：

1. 从每个验证器收集基线 `/v1/status`、`/v1/metrics`、`/v1/diagnostics`、`/v1/finality/latest` 和 `/v1/recovery/report`。
2. 以选定的持续时间和速率运行负载。
3. 注入至少一次重新启动、一次对等中断和一次快照导出/验证/恢复演练。
4. 从每个验证器收集最终指标。
5. 将之前/之后的样本、日志、pprof 样本、签名者审核日志和证据清单存储在 `dist/` 中。

一个好的证据包可以让审阅者回答：高度是否增长、最终性是否取得进展、同行是否恢复、交易是否提交、快照是否验证、重放是否保持健康、签名者是否避免了双重签名以及确切的发布二进制文件是否产生了结果？

<!-- vexo-docs:technical-parity -->
## 技术等价附录

本附录用于确保译文没有遗漏英文正本中的可执行接口和关键章节。命令、配置键、RPC 方法和包名在所有语言中保持不变。

### 章节追踪
- section: Core Endpoints — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Reading `/v1/status` — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Prometheus Metrics — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Suggested Alert Rules — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Suggested Starting Thresholds — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Incident Triage Matrix — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Log Events to Keep — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: First Response Playbook — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Dashboard Layout — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。
- section: Release Evidence From Observability — 本节需要同时检查配置值、验证证据、失败条件以及运营者应采取的操作。

### 保持不变的接口
- `/v1/status` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/metrics` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/metrics/text` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/diagnostics` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/finality/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/state/latest` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/recovery/report` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `/v1/snapshot` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_finalized_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `latest_app_hash` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `active_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `configured_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `scored_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `banned_peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `banned_peers=0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_node_running` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_latest_height` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_active_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_configured_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_scored_peer_count` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_banned_peers` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_height_rate_per_minute` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_round_timeouts` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_proposal_latency_p95_nanos` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_vote_latency_p95_nanos` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_commit_latency_p95_nanos` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_mempool_size` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_snapshot_healthy` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_replay_healthy` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_validator_signing_failures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_post_commit_reconciliation_failures` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_node_running == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_active_peer_count == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_snapshot_healthy == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_replay_healthy == 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_validator_signing_failures > 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `vexo_post_commit_reconciliation_failures > 0` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `timeout_propose` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `max_txs` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `node_running` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `rpc_listening` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `p2p_listening` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_configured` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_connected` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_disconnected` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_dial_failed` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `peer_banned` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `consensus_loop_running` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `block_committed` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `round_timeout` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `validator_signing_failure` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evidence_received` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `evidence_applied` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `snapshot_exported` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `replay_checked` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `upgrade_halt` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `upgrade_applied` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
- `dist/` — 此名称会直接用于执行示例和配置验证，因此不要翻译。
