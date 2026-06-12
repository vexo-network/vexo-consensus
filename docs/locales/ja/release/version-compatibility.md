# Version Compatibility Matrix

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は バージョン互換性マトリクスとアップグレード判断基準を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/release/version-compatibility.md`
- Locale path: `docs/locales/ja/release/version-compatibility.md`

## この文書を読む理由

- バージョン互換性マトリクスとアップグレード判断基準
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

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `/v1/*`
- `vexod upgrade plan --json`
- `vexod upgrade apply`
- `rollback_required`
- `make release-candidate`

## 英語原文の構造

- Version Compatibility Matrix
- Current Matrix
- Upgrade Compatibility Checklist
- Rollback Drill

## 正規原文

- [英語の正規文書](../../en/release/version-compatibility.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Current Matrix — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Upgrade Compatibility Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Rollback Drill — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `mempool_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `log_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod upgrade plan --json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod upgrade apply` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rollback_required` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make release-candidate` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
