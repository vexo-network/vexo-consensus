# Node Initialization

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は、最初に node home を作る人にも、すでに運用している人にも向けた案内です。初めて読むなら、次の順で進めると分かりやすいです。

1. 何を作っているか
2. 5分で動かすローカル実行
3. 4 validator のローカルネットワーク
4. Web3 と Remix
5. Validator Node
6. Archive Node
7. Split Configuration Files
8. Which File Do I Edit?
9. Key Types
10. Config-Based Peers
11. Consensus Timing
12. Multi-Validator Network
13. Troubleshooting
14. Minimal Operator Checklist

この順番は、実際に運用者が最初に確認すべき順番でもあります。まず node home の意味を理解し、その後ローカル起動を確認し、validator と archive の違いを見て、最後に peer、タイミング、障害対応を確認します。

## 文書概要

この文書は archive ノードと validator ノードの初期化、分割設定ファイルの運用を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/ja/operators/node-initialization.md`

## この文書を読む理由

- archive ノードと validator ノードの初期化、分割設定ファイルの運用
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

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
- `--evm-account-key-env`
- `--evm-account-key`
- `validator_id`
- `init validator`
- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `--encrypt-keys`
- `validator.key.json`
- `validator.vrf.key.json`
- `--key-type bls`
- `genesis.json`
- `bls_pop`
- `config.json`
- `module_config.json`
- `consensus_config.json`
- `mempool_config.json`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## 英語原文の構造

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## 正規原文

- [英語の正規文書](../../en/operators/node-initialization.md)
<!-- vexo-docs-ops-update-2026-06 -->

## 最新の運用メモ

新しいノードホームでは `network_config.json` の `p2p.dial_timeout`, `p2p.auth_replay_path`, `p2p.require_auth_replay_store` をまとめて確認します。既定の `10s` dial timeout は TCP 接続、TLS、signed handshake、replay-store 検査を含みます。公開ネットワークでは shell flag に隠さず、設定レビューの対象にしてください。

## 起動時の State Sync

`network_config.json` の `state_sync` ブロックは、新しい archive ノード、交換 validator、またはクリーンなマシンへ復元したノードが起動前に検証済み snapshot を取り込むための設定です。`state_sync.enabled` が true の場合、`vexod start` は `state_sync.snapshot_urls` を順に試し、chain ID、checksum、state root、KV namespace を検証してから LevelDB に復元し、index を再構築してノードを起動します。ローカル状態がすでに `state_sync.min_height` 以上で `state_sync.trust_local_higher` が true なら、ローカル store を維持して `state_sync_skipped` を記録します。

```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```

運用ログでは `state_sync_candidate_failed`、`state_sync_candidate_rejected`、`state_sync_applied` を確認してください。公開ネットワークでは、finality/light-client evidence と信頼ポリシーなしに第三者 snapshot を使わないでください。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Validator Node — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Archive Node — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Split Configuration Files — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Key Types — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Config-Based Peers — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Consensus Timing — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Multi-Validator Network — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod start` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--timeout-propose` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--create-empty-blocks` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--p2p-auth-token` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--rpc-admin-token` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-account-key-env` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-account-key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `VEXO_KEY_PASSPHRASE` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--passphrase` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--encrypt-keys` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator.vrf.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `require_network_safety=true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--key-type bls` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blst-bls12381-minpk-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `genesis.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bls_pop` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `mempool_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `log_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `data/` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `shutdown_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_max_subscriptions_per_connection` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_idle_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `auth_replay_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `require_auth_replay_store` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `dial_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `data/p2p_auth_replay.jsonl` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--key-type ed25519` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf_key_paths` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf_public_key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `<home>/<name>_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.evm_account_key_envs` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.evm_account_private_keys` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_accounts` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sign` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_signTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sendTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_account_key_envs` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod config paths --home <home>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `"require_network_safety": true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `require_network_safety` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `host:port` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.listen_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.seeds` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_cert_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_ca_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_server_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.dial_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_propose` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_prevote` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_precommit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `create_empty_blocks: false` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit: "finalized"` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit: "qc"` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `round_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `create_empty_blocks` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod network up` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make network-e2e` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator-%d` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_advertise_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_advertise_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_listen_host` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_listen_host` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
