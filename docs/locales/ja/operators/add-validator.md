# Adding a Validator

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は validator 追加手順、設定検証、staking 確認を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/ja/operators/add-validator.md`

## この文書を読む理由

- validator 追加手順、設定検証、staking 確認
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

- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `bls_pop`
- `.vexo-validator-new/network_config.json`
- `network_config.json`
- `p2p.listen_address`
- `rpc.address`
- `p2p.peers`
- `p2p_address`
- `rpc_address`
- `active_from`
- `active_until`
- `config audit --strict`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## 英語原文の構造

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## 正規原文

- [英語の正規文書](../../en/operators/add-validator.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: 1. Initialize Validator Home — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 2. Configure Network Addresses and Peers — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 3. Submit Validator Admission — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 4. Verify Validator Set Update — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 5. Plan Validator Key Rotation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 6. Start Validator — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 7. Monitor — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Safety Notes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `VEXO_KEY_PASSPHRASE` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--passphrase` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bls_pop` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blst-bls12381-minpk-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `.vexo-validator-new/network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.listen_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `active_from` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `active_until` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `config audit --strict` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
