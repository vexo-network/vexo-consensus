# 面向模块化权益证明网络的自适应恢复门控 HotStuff

> Locale: zh · 中文  
> 文档类型：研究论文草稿与可复现实验协议  
> 状态：以当前实现为依据；任何性能结论都必须由实测工件支持。

## 摘要

本文研究一种用于模块化 Proof-of-Stake 网络的 HotStuff 风格 BFT 状态机复制系统。当前实现把 three-chain finality 和按高度版本化的 validator set 与三项运行机制组合起来。第一项是有界的自适应 round timeout 控制器，它根据 proposal、vote、commit 的 p95 处理延迟以及 active peer 健康度增大或减小 timeout。第二项是 recovery finality gate：当持久化 block history 与 application state history 不一致时，它阻止 finalized application commit 超过双方一致的安全恢复高度。第三项是保留 nonce 依赖的 deterministic transaction ordering：同一 transaction 集合不受本地 mempool 到达顺序影响，同时同一 signer 的 transaction 仍按 nonce 递增。

本文不声称 PoS、BFT、HotStuff、adaptive view synchronization 或 order fairness 本身是新发明。研究问题是：这一具体的有界控制、恢复门控和确定性排序组合，能否在不改变 HotStuff 基本安全规则的前提下减少不必要的 timeout 和恢复期间的不一致。因此，文档明确区分已实现事实、待检验假设和必须通过实验才可提出的结论。在固定 binary、config、topology 和 workload 的重复实验完成之前，不报告 throughput 或 latency 改进数字。

## 研究问题与假设

RQ1 检验可变网络延迟下 adaptive policy 相比 fixed timeout 是否减少 round timeout 次数和 p95 commit latency。RQ2 检验注入 storage/restart fault 时 recovery gate 是否阻止 application state 超过 durable block/state 的共同安全高度。RQ3 检验同一 transaction 集合的不同输入排列是否得到相同 proposal order，并保持每个 signer 的 nonce 单调递增。RQ4 衡量无故障稳定网络中的 CPU、memory、network 和 latency 开销。

H1 至 H4 都是可证伪的方向性假设，而不是结果。代码路径已经存在并不能证明它优于 baseline。如果实验没有发现显著收益，也应作为负结果或适用边界如实发表。

## 先行工作与新颖性边界

HotStuff 已提出 partial synchrony 下的 leader-based BFT、quorum certificate、chained commit、正常路径线性通信和 responsiveness。LibraBFT/DiemBFT 与 AptosBFT 已证明 HotStuff 派生 BFT 可与 stake-weighted validator governance 结合。Jolteon 和 Ditto 研究低延迟、network adaptation 和 asynchronous fallback，Fever 研究 responsive view synchronization。Tendermint 属于另一种 round-based PoS BFT 路线。Narwhal/Tusk 将 transaction dissemination 与 ordering 分离。Aequitas、Wendy、Themis 给出了比本实现 hash ordering 更强的 order-fairness 定义。

因此不得声称“第一个结合 PoS 与 BFT 的 blockchain”“第一个使用 HotStuff 的 PoS network”“与 AptosBFT 完全相同”“未经证明的 asynchronous liveness 或最优通信复杂度”“仅凭 hash ordering 完全阻止 MEV”或“single-host Docker test 已证明 production-ready”。候选系统贡献应严格表述为：在一个 Go 编写的 modular PoS node 中集成 bounded feedback controller、local durable-history commit gate 和 nonce-aware deterministic ordering，并用 fixed 和 gate-disabled baseline 做可复现实证评估。

## 系统模型

令高度 h 的 active validator set 为 Vh，总 voting power 为 Ph。只有当不重复、已知 signer 的 power 至少达到 Ph 的三分之二时 QC 才有效。validator set 和对应 hash 按高度版本化。admission 可以是 minimum stake 约束下的 permissionless 模式、数量上限模式或受限模式。该层解决 Sybil resistance 与 governance，不改变 BFT fault threshold。

network 假设为 partially synchronous。当 Byzantine voting power 小于三分之一，并且 signature、validator-set binding 和 durable store 假设成立时，系统追求 safety。liveness 还要求最终存在 bounded delay、honest quorum、可用 signer 和足够 peer connectivity。本文不声称在永久 asynchronous network 中仍有进展保证。

EVM 是 Vexo consensus 下的 application workload。它提供 Ethereum bytecode 执行与 `/web3` tooling compatibility，但不等同于 Ethereum fork choice 或 devp2p consensus。

## 协议与控制策略

基础安全状态跟踪 `locked_qc` 和 `high_qc`。proposal 只有在扩展 lock，或携带不旧于 lock 的 justify QC 时才安全。validator 不能在同一 height/round 为不同 block 投票。只有高度和 hash 都连续绑定的三个 certified link 才能 finalize grandparent。adaptive controller 不修改安全 predicate、quorum threshold、QC verification 或 three-chain commit rule。

自适应 timeout 使用 base budget T0、current budget Tt、proposal/vote/commit p95 latency 之和以及 peer deficit。发生 timeout 时向 1.5×current 增长，取得 progress 时向 0.8×current 衰减，观测 latency 之和乘 3 作为候选下限。最终值限制在 T0 与 8×T0 之间；active peer 为零时 peer floor 为 2×T0。没有 pending work 的 idle 时间以及 local execution/storage error 不会消耗 round。该机制是有界运行控制器，不宣称是理论最优 pacemaker。

recovery gate 在 durable state height Hs 和 block-index height Hb 均存在时计算 Hsafe=min(Hs,Hb)。若二者不相等，系统延迟所有 h>Hsafe 的 finalized application commit，直到恢复一致。它只是 local persistence restriction，不是额外 vote phase 或 network certificate。

deterministic ordering 根据 chain ID 和 height 生成 salt，将含 signer/nonce metadata 的 transaction 组成 signer chain。chain 内按 nonce 升序，多个 chain head 再按 salted transaction hash 确定性合并。该方法消除同一候选集合的 arrival-order dependence，但不证明 first-seen fairness、censorship resistance、confidentiality 或 strong order fairness，proposer 仍可能影响候选集合的 inclusion。

当前 consensus vote path 使用完整的 height-versioned validator set 和 deterministic proposer。仓库中的 ECVRF committee selector 只连接到 component 与 query surface，尚未进入 quorum formation 或 proposal eligibility。因此 VRF committee consensus 只能列为 future work。

## 实现映射与实验方法

state-machine safety、three-chain height/hash binding、adaptive controller、recovery gate、deterministic ordering、validator registry、staking、vote WAL 和 EVM workload 的权威文件位置列在英文原文的 implementation mapping 中。任何文件或语义变更都必须同时更新 source revision 和 paper revision。

实验保持相同 binary 与 application config，比较 fixed/adaptive-off gate-on、adaptive-on gate-on、以及仅限隔离研究网络的 adaptive-on gate-off 三种 treatment。资源允许时使用 4、7、16、31 validator，single-host 只作 smoke。网络条件至少包括 10/50/100/250 ms latency、step delay、jitter、0/1/5/10% loss、proposer restart、minority partition/heal、signer delay 和 durable history mismatch。

workload 包括 native transfer、EVM transfer、contract creation、event log、proxy deployment 和 UUPS upgrade。只有所有 validator 在比较高度具有一致 app hash 与 finalized block hash、receipt 和 block location 一致、deployed code 存在且 upgrade 后 storage 正确的 run 才进入性能统计。

收集 committed/finalized height、proposal/vote/commit p50/p95/p99、end-to-end finality latency、round timeout、round distribution、current adaptive timeout、peer count、recovery deferral、throughput、gas、CPU、RSS、disk/network bytes，以及 rejection、double-sign、invalid nonce。每个条件 warm-up 后原则上独立运行至少 30 次，随机化 treatment 顺序并保存 seed。报告 median、IQR、p95、confidence interval 和 effect size，不能只选择最好的一次。

## 正确性、复现性与研究伦理

adaptive policy 只改变何时尝试 timeout vote，不改变什么 vote 或 QC 是安全的。recovery gate 只收紧 commit 条件，不能授权 base rule 拒绝的 commit。deterministic ordering 帮助 deterministic execution input，但不能替代 conflicting finality safety proof。

可发表的 formal argument 需要覆盖 stake-weighted quorum intersection、lock monotonicity、同一 height finalized block 的唯一性、validator-set transition、vote WAL crash recovery，以及 controller/gate 的 safety-neutral 性。unit test 和 adversarial simulation 是 evidence，不能代替 formal proof 或 independent audit。

每次 experiment 必须保存 commit、dirty-tree status、Go/OS/CPU/memory/container 信息、topology、genesis、split config、binary SHA-256、workload seed、raw JSON/JSONL/CSV、validator log、final app hash、analysis script 与 failed-run ledger。不得把已知机制改名后宣称为新发明，不得制造 throughput、latency 或 validator count，不得删除失败 run 或事后选择 outlier 规则。hypothesis、observation 和 interpretation 必须分开。

AI assistance 按目标 venue 政策披露，作者对所有 claim、citation、experiment 和 proof 负责。fault injection 仅能在研究者拥有或获授权的 isolated system 中执行。artifact 中不得暴露 private key、operator token、participant data 或 production endpoint。发现漏洞时遵守 coordinated vulnerability disclosure。

投稿前必须确认 paper 与 pinned source 一致、prior-art search 有记录、baseline 可复现、multi-host fault measurement 完成、raw data 可重建所有 table/figure、negative result 保留、proof wording 适当并经 external methodology review。在此之前，准确称谓是“以实现为依据的研究草稿”，而不是“全新且已证明的 consensus”。

<!-- vexo-docs:technical-parity -->

## 技术一致性附录

以下实现、配置和验证名称保持不翻译。

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
