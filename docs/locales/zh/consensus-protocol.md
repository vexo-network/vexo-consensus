> Locale: zh · 中文

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

-不到三分之一的拜占庭投票权
-域分隔的提案、投票、超时投票和最终签名
-在相关证明高度的验证器集哈希绑定
-质量控制和最终证明中唯一的已知签名者
-验证者模棱两可的可问责证据
-在相同的最终高度拒绝冲突的提交决策

# #加密边界

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

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
