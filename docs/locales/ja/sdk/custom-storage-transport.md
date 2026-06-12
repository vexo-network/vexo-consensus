# Custom Storage and Transport Guide

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は custom storage と transport adapter を実装・登録する方法を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/sdk/custom-storage-transport.md`
- Locale path: `docs/locales/ja/sdk/custom-storage-transport.md`

## この文書を読む理由

- custom storage と transport adapter を実装・登録する方法
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

- `store.Store`
- `store.HistoricalSnapshotKVStore`
- `store.SnapshotKVStore`
- `transport.Transport`

## 英語原文の構造

- Custom Storage and Transport Guide
- Custom Storage
- Storage Requirements
- Custom Transport
- Transport Requirements
- Compatibility

## 正規原文

- [英語の正規文書](../../en/sdk/custom-storage-transport.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Custom Storage — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Storage Requirements — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Custom Transport — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Transport Requirements — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Compatibility — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `store.Store` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `store.HistoricalSnapshotKVStore` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `store.SnapshotKVStore` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `store.AppBlockCommitStore` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod start` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `runtime.NewNetworkSafeWithStore` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `runtime.NewNetworkSafeWithStoreContext` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `runtime.NewNetworkSafeWithStoreAndCryptoRegistryContext` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `config.ValidateNetworkSafety` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app.AtomicBlockApplication` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `transport.Transport` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `transport.GRPCConfig.RequireTLS` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
