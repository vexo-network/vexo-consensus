> Locale: zh · 中文

# 共识协议概览

本文是理解 Vexo 共识的高层入口。规范细节以 [Consensus Spec](./specs/consensus-spec.md)、[Finality Proof Format](./specs/finality-proof-format.md)、[Validator Lifecycle](./specs/validator-lifecycle.md)、[Storage Schema](./specs/storage-schema.md)、[Networking Spec](./specs/networking-spec.md) 和 [Transaction Format](./specs/tx-format.md) 为准。

## 模型

Vexo 使用 HotStuff 风格的 BFT 核心，包含 proposal、vote、quorum certificate(QC)、timeout certificate、locked-QC 安全规则和 three-chain finality。只有扩展 locked QC，或携带不早于当前锁的 justify QC 时，区块才可安全投票。未明确绑定区块、父区块和祖父区块高度及哈希的合成 QC 或跳高 QC 链会在终局性决定前被拒绝。

## 协议身份与研究边界

Vexo 不是原始 HotStuff 的新名称，也不等同于 AptosBFT、DiemBFT、Jolteon、Ditto、Tendermint 或 CometBFT。它在独立 Go 运行时中复用 HotStuff 系列安全概念，并组合自适应轮次计时、持久恢复、确定性交易排序、模块化执行和按高度版本化的 validator set。

当前投票路径使用完整的按高度版本化 validator set 和确定性 proposer。仓库中的 VRF committee selector 可由组件和查询接口访问，但尚未连接到 proposal 资格或 quorum 形成。因此，VRF committee 共识只能作为后续研究，而不能写成已启用特性。贡献边界和实验协议见 [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md)。

## 执行与恢复边界

QC 认证、HotStuff 终局化、应用执行和状态提交是不同事件。默认 `execution_commit=finalized` 只执行 three-chain 规则选出的祖先。自适应 pacemaker 和 `recovery_finality_gate_enabled` 仅控制延迟与重启恢复，不改变 proposer 选择、quorum power、safe-vote 规则或 three-chain finality。

## 安全边界

-不到三分之一的拜占庭投票权
-域分隔的提案、投票、超时投票和最终签名
-在相关证明高度的验证器集哈希绑定
-质量控制和最终证明中唯一的已知签名者
-验证者模棱两可的可问责证据
-在相同的最终高度拒绝冲突的提交决策

## 密码学边界

- `deterministic` backend 仅用于测试，不能通过 network safety 校验。
- `ed25519` 可用于公开网络测试和上线准备。
- `bls` 默认使用 `blst-bls12381-minpk-v1`，并要求 proof-of-possession、subgroup 检查、公钥验证、依赖审计及 release-gate 证据。
- network safety 校验需要 VRF adapter metadata，但这并不表示 VRF committee 已进入活动共识路径。

-对每个验证程序主页进行严格的配置审核
-释放门证据
-外部安全审查
-多房东长期和混乱的证据
-签名人/KMS政策证据
-特定于链的经济和治理政策审查

在将版本视为生产就绪之前，请参阅[安全审核就绪](./security/audit-readiness.md)和[发布管道](./release/release-pipeline.md)。

<!-- vexo-docs:technical-parity -->
## 技术等价性附录

这个附录的作用，是确认译文和英文正本保留了同样的可执行接口与运维边界。命令、配置键、RPC 路径和代码标识符都不要翻译。下面的中文只是解释含义，但软件和运维必须看到的那些名字仍然保持原样。
`require_network_safety` 和 `block_committed` 是必须保持原样的关键术语。
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### 章节跟踪
- section: Model - HotStuff、three-chain finality、QC、timeout certificate、locked-QC safety 需要一起阅读。
- section: Execution Terms - QC certified、finalized、executed、state committed 的运维含义不同。
- section: Safety Boundary - 少于三分之一的 Byzantine 投票权、domain separation、validator-set hash binding、accountable evidence 都是安全要求。
- section: Crypto Boundary - `deterministic`、`ed25519`、`bls`、`blst-bls12381-minpk-v1`、`ecvrf-p256-sha256-tai-v1` 需要统一对待。
- section: Operational Boundary - `vexo_quorum_health_ratio`、`adaptive_round_timeout_enabled`、`recovery_finality_gate_enabled` 和 snapshot/replay health 是运维信号。

### 保持不变的接口
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## 运维备注

创建验证人 home 时，除了 `config.json` 之外，还要一起检查 `module_config.json`、`network_config.json`、`consensus_config.json`、`mempool_config.json` 和 `log_config.json`。在真实运维里，最好把 `vexo_quorum_health_ratio` 和 `adaptive_round_timeout_enabled` 放在一起看，不要只盯着对等节点数量。

- 优先使用 `execution_commit=finalized`。
- `qc` 路径只应保留给受控测试网。
- 同时检查 `recovery_finality_gate_enabled` 与 snapshot/replay health。
