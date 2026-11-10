# Vexo Lanes：面向权益加权 two-chain HotStuff 的认证批次分发

> Locale: zh · 中文  
> 文档类型：研究稿件与可复现性协议  
> 状态：基于实现的草稿；结果在单台主机和模拟链路上测得，发表前必须在真实网络上重复

## 摘要

HotStuff 系列的基于领导者的 BFT 协议受领导者上行链路限制：区块中的每笔交易都要从一台机器向每个验证者发送一次，因此吞吐量按 `1/(n−1)` 下降，而其他验证者的链路处于空闲。Vexo Lanes 在不替换排序协议的前提下，把分发从领导者身上移走。每个验证者把自己拥有的交易封装成自己通道上的签名批次；持有三分之二权益的验证者确认已存储后，该批次即获得可用性认证；领导者只需列出已认证批次即可提出区块。区块格式、哈希、交易根、签名字节、投票规则和 two-chain 提交规则都保持不变。

在每个验证者模拟 100 Mbit/s、25 ms 链路的单台主机上，Lanes 在 4、7、10 个验证者时提交 60,080、45,323、37,099 tx/s，而同一节点广播整块的基线为 21,073、10,909、8,297 tx/s，即 2.9×、4.2×、4.5×，差距随 `n` 增大。低于容量（4,000 tx/s）时，4 个验证者下 Lanes 的中位延迟增加 35 ms，7 和 10 个验证者下分别减少 25 ms 和 130 ms。链路不受限时 Lanes 损失 4% 吞吐量。评估过程暴露了九个缺陷，其中之一是在验证者数量为偶数时只有偶数索引的验证者才会提出区块的领导者调度。

## 1. 研究问题

能否在不改变验证者投票对象和区块提交方式的前提下，在现有权益加权 HotStuff 协议之下加入认证批次分发，使吞吐量与全体验证者上行链路之和而不是领导者的上行链路成比例？子问题涉及领导者瓶颈（RQ1）、扩展性（RQ2）、安全性、nonce 顺序和抗审查性的保持（RQ3），以及带宽充足时的成本（RQ4）。假设为：H1，基线每笔交易的最大上传量随 `n` 线性增长，而 Lanes 接近交易大小；H2，加速比随 `n` 增大；H3，链路不受限时 Lanes 不会更快，额外成本最多一次往返。

## 2. 先前工作与新颖性边界

分离分发与排序是成熟的思路：Narwhal 与 Tusk、Bullshark 与 Shoal、Stratus、Mir-BFT 与 ISS、Autobahn 以及 Aptos 的 Quorum Store。本文不声称这种分离、可用性证书、按验证者划分的通道或真实广域网结果是新的。贡献偏工程：逐位保留区块、区块头、签名和提交规则的改造；按发送者划分通道所有权并提供防审查回退；在获取任何数据之前认证提案；以及可复现的测量框架及其暴露的缺陷。

## 3. 系统模型

`n` 个具有权益加权投票权的验证者，拜占庭验证者持有的投票权少于三分之一，法定人数为 `>= 2/3`。假设部分同步、认证信道和按域分离的签名。验证者可能崩溃、双重签名、扣留批次、淹没自己的通道或发送伪造提案。

## 4. 协议

**通道与证书。** 每个验证者在达到 512 KiB 或首笔交易后 5 ms 时封装一个 `lanes.Batch`，受 `lanes.MaxBatchTxs` 与 `lanes.MaxBatchBytes` 限制。上一批次小于 16 KiB 的低负载通道还会等待该批次的证书，最多 50 ms。验证者在 `DomainLaneBatchAck` 域中确认存储；`>= 2/3` 的确认组成 `BatchCert`，因此至少一个诚实验证者持有该批次。重传采用指数退避。

**所有权与回退。** 交易属于通道 `lanes.Owner(members, sender)`。同一发送者的交易都按接收顺序走同一条通道，因此通道不会打乱 nonce 顺序。若别的通道拥有的交易在 `lanes.FallbackDelay`（3 秒）后仍不在任何已存批次中，持有者会把它放进自己的通道。故障的所有者只能延迟，不能审查。

**通道提案。** 领导者只选取已认证批次，按通道顺序直到第一个未认证批次，各通道轮流，直到区块上限；照常签名区块，并广播包含批次 ID、被拒绝位置（`Excluded`）和不在批次中的交易（`Extra`）的 `lane_proposal`。

**获取前认证。** 接收方在获取任何数据之前先验证领导者和签名。若验证者集合依赖尚未执行的区块，提案会被保留（最多 16 个），并在下一次执行后重新判定。区块由已存批次重建，只有哈希和交易根与签名的区块头一致时才判定；否则验证者通过 `proposal_request` 请求完整的 `proposal`。

**按权益比例的领导者调度。** 视图 `v` 的领导者是 `validator.SelectProposer(validators, v)`：验证者在 `[0, W)` 中拥有 `VotingPower` 个连续槽位，视图 `v` 对应槽位 `v·S mod W`，步长是从 `ceil(W·(φ−1))` 开始找到的第一个与 `W` 互素的整数。任意 `W` 个视图的窗口都恰好按权益分配给每个验证者。调度只依赖视图。

**资源上限。** 256 MiB 存储中每个作者 64 MiB；30 秒后过期；每个批次对每个对等节点最多提供两次；每个提案最多 `consensus.MaxLaneRefs` 个批次；带版本的二进制编码。

## 5. 正确性论证

安全性：投票和提交规则不变，验证者只为重建哈希与签名区块头一致的区块投票；通道可以延迟投票，但不能改变投票对象。可用性：每个被列出的批次都由诚实验证者持有，最后手段是获取完整提案。活性：稳定之后，诚实批次会被认证并提出；落后的验证者在执行后立即判定提案。这是论证，不是机器验证的证明。

## 6. 实验方法

处理组为 Lanes（默认节点）与使用 `DisseminateByProposal` 的基线（领导者广播整块），其余部分完全共享。所有验证者在一台 Apple M4 的一个进程中运行；每个上传经过固定速率和延迟的虚拟链路，共识消息优先于批量流量，无丢包和抖动。客户端提交 89 字节交易；吞吐量测试一次提交 150,000 笔，延迟测试以 4,000 tx/s 开环提交 20,000 笔（`VEXO_RATE`）。10 个验证者的吞吐量单元两组都使用 5 秒超时，因为默认值下基线在高度 6 之后停滞。每个单元测量三次。

## 7. 结果

| 验证者 | Lanes tx/s | 基线 tx/s | 倍数 | Lanes B/tx | 基线 B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

所有运行都提交了全部 150,000 笔交易。仅提案一项，基线每笔交易就要花费 `s·(n−1)` 字节，Lanes 为 0.2–0.4 B/tx。这支持 H1 和 H2。

| 验证者 | Lanes p50 / p99 | 基线 p50 / p99 | Lanes 提案 B/tx | 基线提案 B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

4 个验证者时认证约花费一次往返；从 7 个验证者起，在此负载下基线的领导者链路已经拖慢提案，10 个验证者时只有 Lanes 跟上了提供的负载。H3 只对小集合成立。

| 处理组 | tx/s | 上传 B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| 基线 | 114,522 (113,095–116,408) | 147.4–148.3 |

链路不受限时基线快 4%，但 Lanes 上传的字节不到一半。基线受限于领导者链路，而在此矩阵中 Lanes 受限于共享主机。

## 8. 评估暴露的缺陷

1. **按奇偶的领导者调度。** `(height−1+round) mod n` 在 `n` 为偶数时只让偶数索引提出区块；已替换为按权益比例的调度（`node/leader_schedule_test.go`）。
2. **已提交交易的重放。** 迟到的 gossip 被重新接收（150,000 笔交易提交了 152,629 次）。
3. **seen 缓存的二次方过期。** 每次接收都扫描整个缓存；现在按最旧优先过期且有上限。
4. **重传风暴。** 拥塞链路上固定 200 ms 重传；现在采用指数退避。
5. **无界重复的证据。** 现在每种违规只在 1,024 条的窗口中记录一次。
6. **提交正文请求。** 现在只向一处请求，一秒后重试。
7. **只在重传时判定的提案。** 每四次 10 个验证者的运行中有一次跌到 2,280 tx/s；现在会保留并在执行后判定（`consensus/transport_reactor_test.go`）。
8. **低负载下的单批次开销。** 只含两笔交易的批次使主机饱和；通过通道节奏控制解决。
9. **标签错误的延迟指标。** 现在测量每笔交易从提交到确认的延迟。

## 9. 有效性威胁

- 单台主机上的模拟网络，无丢包和抖动。
- 共享 CPU 限制了大 `n` 下的 Lanes。
- 基线在 10 个验证者时需要 5 秒超时；更紧凑的编码会缩小差距。
- 请求完整提案前的 300 ms 等待在拥塞链路上过短。
- 只有小额转账；只重复三次（10 个验证者时 34.9k–39.4k tx/s）。
- Ed25519 证书串联签名；BLS 可以缩小它们。

## 10. 复现步骤

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` 用于修改超时；省略 `VEXO_UPLINK_MBIT` 和 `VEXO_LATENCY_MS` 即为不受限链路。每个公布的数字都必须记录版本、主机、Go 版本和完整日志行。

<!-- vexo-docs:technical-parity -->

## 技术一致性附录

以下实现名称和实验名称不做翻译：

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`, `VerifyBatch`, `VerifyCert`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`, `consensus/transport_reactor_test.go`
- `validator/proposer.go`, `node/lanes.go`, `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `DomainLaneBatchAck`, `BatchCert`, `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `Excluded`, `Extra`
- `lane_proposal`, `proposal_request`, `proposal`, `DisseminateByProposal`
- `validator.SelectProposer(validators, v)`, `VotingPower`, `[0, W)`, `v·S mod W`, `ceil(W·(φ−1))`, `0..W−1`, `W`, `S`, `v`
- `n`, `s`, `>= 2/3`, `1/(n−1)`, `s·(n−1)`, `(height−1+round) mod n`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
- `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
