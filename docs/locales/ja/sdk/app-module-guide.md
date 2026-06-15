# App Module Guide

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 最初に読む順序

この文書は Vexo に application module を追加する方法を説明します。初めて module を追加するなら、次の順で読むと分かりやすいです。

1. Module interface
2. Transaction routing
3. Module configuration
4. State and events
5. Genesis and ante handling
6. CLI commands and tests

この順番は、実際の実装順ともほぼ一致します。module の形を決め、transaction をどう受け取るかを決め、どの state を持つかを決めたあと、CLI と test をつなげます。

## 文書概要

この文書は 新しい app module を作り CLI/RPC/状態保存へ接続する方法を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/sdk/app-module-guide.md`
- Locale path: `docs/locales/ja/sdk/app-module-guide.md`

## この文書を読む理由

- 新しい app module を作り CLI/RPC/状態保存へ接続する方法
- 英語原文の MUST/SHOULD/MAY 文を先に確認します。
- このローカライズ文書は理解補助です。監査、リリース、セキュリティ判断は英語原文で確定します。

## 読後にできるべきこと

- この文書がどの実装・運用判断を支えるか説明できるようにします。
- 英語原文の規範要件を現在のネットワーク設定と対応づけます。
- 例をコピーする前に chain ID、validator ID、fee/gas、peer アドレスを確認します。

## 安全利用チェックリスト

- 英語原文の MUST/SHOULD/MAY 文を先に確認します。
- コマンド、config key、RPC 名、JSON フィールド、コード識別子は翻訳しません。
- 例の値をコピーする前に chain ID、validator ID、fee/gas、peer アドレスが自分のネットワークに合うか確認します。
- 文書を変更したら `make docs-check` で locale tree と翻訳 guard を確認します。

## 注意点

- このローカライズ文書は理解補助です。監査、リリース、セキュリティ判断は英語原文で確定します。
- 実装が変わった場合は英語文書と全ローカライズ文書を同じ変更で更新してください。

## 原文のまま保持するインターフェース

- `app.Module`
- `app.QueryHandler`
- `app.ValidatorUpdateProvider`
- `app.TxEventEmitter`
- `app.PruneHook`
- `bank`
- `bank:`
- `module_config.json`
- `config.json`
- `module_config_path`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `app.Context.Store`
- `ctx.GoContext()`
- `CheckTx`
- `PrepareProposal`
- `ProcessProposal`
- `FinalizeBlock`
- `Query`
- `params`

## 英語原文の構造

- App Module Guide
- 目的
- Module Interface
- Transaction Routing
- Module Configuration
- State
- Events and Query Proofs
- IBC and Contract Extension Points
- Genesis
- Ante Handling
- CLI Commands
- Tests

## 正規原文

- [英語の正規文書](../../en/sdk/app-module-guide.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Goal — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Module Interface — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Transaction Routing — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Module Configuration — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: State — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Events and Query Proofs — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: IBC and Contract Extension Points — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Genesis — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Ante Handling — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: CLI Commands — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Tests — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `app.Module` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app.QueryHandler` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app.ValidatorUpdateProvider` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app.TxEventEmitter` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app.PruneHook` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bank:` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `mempool_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `log_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app.Context.Store` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `ctx.GoContext()` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `params:set:<authority>:<module>:<key>:<base64-value>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `params/param/<module>/<key>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `events.Indexer` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `queryproof.Build` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `queryproof.Verify` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `contract.Result` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `modules/evm/backend/geth` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `modules/evm/ethcompat` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm state-backend` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `github.com/ethereum/go-ethereum` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-tx-fixtures-sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-execution-fixtures-sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sendRawTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution.allow_unprotected_legacy_tx` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getProof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm/storage/{address}/{slot}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_ethstate/{height}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `state_diff` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vm_trace` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBalance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getTransactionCount` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getCode` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getStorageAt` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_call` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_estimateGas` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `params.ChainConfig` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_createAccessList` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getTransactionReceipt` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBlockReceipts` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getTransactionByHash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getLogs` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `relayer_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `ibc/capabilities` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo-queryproof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `client-create` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--authority` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--signer` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `client-update` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `proof_json_base64` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/state/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `relayer client-update --source-rpc` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `failure_backoff` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_modules` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_web3Capabilities` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_clientVersion` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_sha3` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `net_version` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `net_listening` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `net_peerCount` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_chainId` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_protocolVersion` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_syncing` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_mining` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_hashrate` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_accounts` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_coinbase` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_blockNumber` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBlockByNumber` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBlockByHash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBlockTransactionCountByNumber` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBlockTransactionCountByHash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getTransactionByBlockNumberAndIndex` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getTransactionByBlockHashAndIndex` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getUncleCountByBlockNumber` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getUncleCountByBlockHash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
