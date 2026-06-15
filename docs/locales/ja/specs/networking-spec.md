# ネットワーク仕様

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Networking Spec の規範仕様を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Scope
2. Transport
3. Topics
4. Handshake
5. Wire Compatibility
6. Address Roles
7. Peer Scoring
8. Reconnect and Backoff
9. DoS/DDOS Defenses
10. Operational Signals

この順番は、まず範囲と状態を理解し、次にメッセージ、safety、liveness の規則を確認し、最後に evidence を読む流れです。

## 文書概要

この文書は P2P handshake、gossip、peer scoring、ban ポリシーを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/networking-spec.md`
- Locale path: `docs/locales/ja/specs/networking-spec.md`

## この文書を読む理由

- P2P handshake、gossip、peer scoring、ban ポリシー
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
- 文書を変更したら `make docs-check` でローカル文書ツリーと翻訳ガードを確認します。

## 注意点

- このローカライズ文書は理解補助です。監査、リリース、セキュリティ判断は英語原文で確定します。
- 実装が変わった場合は英語文書と全ローカライズ文書を同じ変更で更新してください。

## 原文のまま保持するインターフェース

- `consensus`
- `tx`
- `commit`
- `evidence`
- `network_config.json`
- `rpc.address`
- `p2p.listen_address`
- `p2p.peers`
- `p2p.seeds`
- `p2p_address`
- `rpc_address`
- `host:port`
- `0.0.0.0:26656`
- `[::]:26656`
- `0`
- `p2p.tls_cert_path`
- `p2p.tls_key_path`
- `p2p.tls_ca_path`
- `p2p.tls_server_name`
- `start`
- `BanThreshold`
- `MaxScore`

- `validator_id`
- `p2p.node_id`
- `node.key.json`
- `p2p.node_key_path`
- `signature_nonce`
- `node_public_key`
- `signature`
- `Wire Compatibility`
## 英語原文の構造

- Networking Spec
- Scope
- Transport
- Topics
- Handshake
- Address Roles
- Transport TLS
- Peer Scoring
- Reconnect and Backoff
- DoS/DDOS Defenses
- Operational Signals

## 正規原文

- [英語の正規文書](../../en/specs/networking-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Peer タイミングと永続 Peer

一時的な dial 失敗だけでは configured peer や seed を ban しません。失敗は backoff と診断に残りますが、ban は悪意ある gossip、認証失敗、rate-limit abuse などの行動証拠で判断します。`p2p.dial_timeout` はリージョン間遅延と TLS/auth コストを見て設定します。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Transport — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Topics — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Handshake — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Wire Compatibility — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Address Roles — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Peer Scoring — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Reconnect and Backoff — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: DoS/DDOS Defenses — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Operational Signals — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `validator_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.auth_replay_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.dial_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `signature_nonce` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node_public_key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.listen_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.seeds` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `host:port` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `0.0.0.0:26656` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `[::]:26656` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_cert_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_ca_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_server_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.tls_cert_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.tls_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.tls_ca_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.tls_server_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.admin_token` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.admin_tokens` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
