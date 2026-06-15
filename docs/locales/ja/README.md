# ドキュメント

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## まず始める手順

- `make build` でバイナリをビルドします。
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` で validator home を作成し、`vexod validate --home .vexo-validator-1` と `vexod config audit --home .vexo-validator-1 --strict` で確認してから `vexod start --home .vexo-validator-1` で起動します。
- Docker ネットワークは `docker compose -f deployments/docker/compose.single-host-init.yml up` の後に `docker compose -f deployments/docker/compose.single-host.yml up` を実行します。
- Remix には `http://127.0.0.1:28657/web3` を使い、chain ID は `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` で確認します。
- 文書を変更したら `make docs-check` を実行して locale tree と翻訳ガードを確認します。
## 文書概要

この文書は ドキュメント索引と推奨される読み順を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/ja/README.md`

## この文書を読む理由

- ドキュメント索引と推奨される読み順
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

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## 英語原文の構造

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## 正規原文

- [英語の正規文書](../en/README.md)

## 用語をそのまま残す一覧

以下の用語は翻訳せず、そのまま使います。

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: How to Read This Set — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Start Here — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Protocol Specs — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: SDK and Extension Guides — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Operations and Release — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Security — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Localized Documentation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Writing New Docs — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Production Claim Rule — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Documentation Review Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `vexo-consensus` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make docs-check` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod status --json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `feature_assurance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.auth_replay_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json:governance.RequireDeposit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json:governance.MinDeposit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json:consensus.execution_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `mempool_config.json:mempool.WALPath` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
