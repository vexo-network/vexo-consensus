# EVM 更新指南

> Locale: zh · 中文
> 本文是英文原文的中文翻译。协议、安全和发布判断以英文原文为准。

本文说明如何更新内置 EVM 栈，同时不破坏 chain ID 处理、Web3 兼容性和发布证据。它面向需要升级 go-ethereum、调整 fork preset，或在受控发布中修改 EVM 行为的运维人员和维护者。

## 什么算作 EVM 更新

只要下面任一项发生变化，就应把它看作发布敏感的功能更新，而不是普通重构：

- `modules/evm/backend/geth` 中的 `go-ethereum` 版本升级
- `modules/evm/ethcompat` 的变更
- `modules/evm` 的变更
- `execution.evm_fork_preset` 的变更
- `execution.evm_chain_config_json` 的变更
- raw transaction 进入、gas accounting、receipts、traces、proofs 或 block response fields 的变更
- `eth_accounts`、`eth_coinbase`、`eth_sign`、`eth_signTransaction`、`eth_sendTransaction` 等 managed Web3 account 处理逻辑的变更

## 安全的更新顺序

请按以下顺序执行，保证代码、配置和文档始终一致：

1. 先更新独立的 geth-backed adapter。
2. 再更新 fixture corpus 和 conformance tests。
3. 语义发生变化时，更新 `docs/specs/evm-native-accounting.md`、`docs/specs/tx-format.md` 和 `docs/sdk/rpc-api-versioning.md`。
4. 发布证据格式变化时，更新 `docs/release/release-pipeline.md`。
5. 运维可见的配置开关变化时，更新节点配置文档。
6. 合并之前重新运行 validation matrix。

不要在同一次提交里既升级 EVM runtime version 又直接发布，除非 conformance suites、RPC smoke checks 和 Docker deployment checks 都已经通过。

## 更新流程

### 1. 锁定变更范围

先把更新意图写清楚：

- 仅修改 fork behavior
- 仅修改 transaction admission
- 仅修改 execution semantics
- 仅修改 RPC compatibility
- 仅修改 blob / receipt / trace 处理
- 仅修改 managed account 或 wallet behavior

这样可以让 review 保持聚焦，避免无关代码一起移动。

### 2. 在最窄层面修改代码

优先使用这些边界：

- `modules/evm/backend/geth`：上游 go-ethereum 集成变化
- `modules/evm/ethcompat`：raw transaction decoding、hash 保持、fixture 处理
- `modules/evm`：state transition、receipts、logs、storage、snapshot 行为
- `rpc`：Web3 request/response 表面变化
- `cmd/vexod`：只有当 CLI 或 release workflow 必须暴露新行为时才改

如果变更已经碰到 application modules，就要明确 module boundary，并保持 deterministic state writes。

### 3. 刷新默认配置

当语义变化时，请在同一补丁里更新默认配置：

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- 必要时更新 `network_config.json` 中 managed account 的 RPC 字段
- `module_config.json` 里的 EVM chain ID

不要依赖隐藏的 CLI flag 来解释运行时行为。配置文件本身就应该能看出节点行为。

### 4. 运行 conformance 栈

至少执行：

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

然后检查用户最先会遇到的路径：

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

如果是 Docker single-host 部署，也要验证：

```text
http://127.0.0.1:28657/web3
```

至少确认这些行为：

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

然后用生产环境同样的 RPC endpoint 依次测试简单 contract deploy、proxy contract deploy 和 UUPS upgrade 路径。

### 5. 确认 proxy 和 upgrade 行为

只有以下条件全部成立，EVM 更新才算完成：

- 普通 contract deploy 成功
- proxy deploy 成功
- UUPS upgrade 调用成功
- upgrade 后读到的 storage 和 code 符合预期
- nonce tracking 仍然单调递增
- block producer 能接受这些交易且不会报 unsafe proposal

如果 proxy deploy 能过但 upgrade 失败，就还不能发布。这不是警告，而是 release blocker。

### 6. 刷新证据

当 EVM surface 发生变化时，也要更新 release evidence bundle：

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- 已固定的 SHA-256 fixture reference

release evidence 应该写清楚改了什么、测了什么、验证了哪个 commit 或 version。只要证据和实际执行过的代码不一致，就不要把 EVM 更新说成已经完成。

## 验证矩阵

把这张表当作 merge gate。

| Check | 为什么重要 |
| --- | --- |
| `make evm-conformance` | 捕获 fork rule 和 execution regression |
| `go test ./modules/evm -count=1` | 验证 receipts、logs、storage、balances、snapshots |
| `go test ./rpc -count=1` | 验证 Web3 request/response 兼容性 |
| `make network-e2e` | 确认节点仍能启动、建立 peer、完成 commit |
| Docker single-host smoke | 确认 Remix 和浏览器工具实际使用的路径 |
| Contract deploy | 确认 transaction admission 和 receipt generation |
| Proxy deploy | 确认 ABI 和 storage layout 假设 |
| UUPS upgrade | 确认 upgrade semantics 和 upgrade 后读取 |

只要有一项不通过，就不能说更新完成。

## 回滚条件

出现以下任一情况，就要回滚 EVM 更新：

- `eth_chainId` 意外变化
- `eth_sendRawTransaction` 开始拒绝有效交易
- `eth_call` 或 `eth_estimateGas` 偏离预期 fork rules
- receipts、logs、proofs 不再匹配 committed state
- proxy 或 upgrade 交易开始失败
- release evidence 与当前代码路径不一致

回滚时要同时恢复最后一次确认无误的 adapter version、config default 和 fixture set。

## 技术一致性附录

这个附录用于让更新指南与文档树中的其他内容保持一致。

- `modules/evm/backend/geth`、`modules/evm/ethcompat`、`modules/evm`、`rpc`、`cmd/vexod` 继续作为稳定实现边界。
- `execution.evm_fork_preset`、`execution.evm_chain_config_json`、`execution.allow_unprotected_legacy_tx`、`eth_chainId`、`eth_call`、`eth_estimateGas`、`eth_sendRawTransaction`、`eth_getTransactionReceipt`、`eth_getProof`、`eth_getStorageAt`、`eth_accounts`、`eth_coinbase`、`eth_signTransaction`、`eth_sendTransaction` 的拼写保持不变。
- `make evm-conformance`、`make network-e2e`、`--evm-default-fixtures`、`--evm-tx-fixtures`、`--evm-execution-fixtures`、`--evm-web3-conformance-evidence` 的拼写也保持不变。
- 运维上只需要回答一个简单问题：这次更新是否在保持 Ethereum-style execution 的同时，仍然符合 Vexo consensus 和 release safety？

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
