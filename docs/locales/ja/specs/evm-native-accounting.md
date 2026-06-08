# EVM and Native Accounting

> Locale: ja · 日本語
> この文書は英語の正規文書を基準にした日本語翻訳ガイドです。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 目的

この文書は native coin と EVM gas/accounting を一貫して接続する方法を扱います。 実装と運用で使うコマンド、JSON フィールド、RPC 名、config key、コード識別子は互換性のため英語表記を保持します。

## 主な範囲

- この文書を読むときは次の項目を必ず確認してください。コマンド、JSON フィールド、RPC メソッド、設定キー、コード識別子は互換性のため原文のまま保持します。
- 詳細な規範文は英語原文で確認してください。
- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/ja/specs/evm-native-accounting.md`

## 保持する識別子

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`

## 英語原文のセクション

- EVM and Native Accounting
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM Execution
- Compatibility Boundary
- Failure Modes

## 運用メモ

- `MUST`、`SHOULD`、`MAY`、コマンド例、JSON 例、RPC 名は英語表記を保持します。
- この翻訳を変更した後は `make docs-check` を実行してください。
- このページと英語原文が矛盾する場合は英語原文を採用し、同じ変更でこの locale ファイルも更新してください。

## 正規原文

- [English canonical document](../../en/specs/evm-native-accounting.md)
