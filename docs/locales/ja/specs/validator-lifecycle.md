# Validator Lifecycle

> Locale: ja · 日本語
> この文書は英語の正規文書を基準にした日本語翻訳ガイドです。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 目的

この文書は validator join、rotation、jail、slashing、leave lifecycleを扱います。 実装と運用で使うコマンド、JSON フィールド、RPC 名、config key、コード識別子は互換性のため英語表記を保持します。

## 主な範囲

- この文書を読むときは次の項目を必ず確認してください。コマンド、JSON フィールド、RPC メソッド、設定キー、コード識別子は互換性のため原文のまま保持します。
- 詳細な規範文は英語原文で確認してください。
- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/ja/specs/validator-lifecycle.md`

## 保持する識別子

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## 英語原文のセクション

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## 運用メモ

- `MUST`、`SHOULD`、`MAY`、コマンド例、JSON 例、RPC 名は英語表記を保持します。
- この翻訳を変更した後は `make docs-check` を実行してください。
- このページと英語原文が矛盾する場合は英語原文を採用し、同じ変更でこの locale ファイルも更新してください。

## 正規原文

- [English canonical document](../../en/specs/validator-lifecycle.md)
