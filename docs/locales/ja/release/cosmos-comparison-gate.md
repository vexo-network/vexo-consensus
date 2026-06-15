# Cosmos/Tendermint 比較ゲート

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Cosmos Comparison Gate の release と運用手順を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Required Evidence Properties
2. Release Rule

この順番は、まず目的と gate を理解し、次に artifact と evidence 要件を確認し、最後に実行手順へ進む流れです。

## 文書概要

この文書は Cosmos/Tendermint 風の期待値に対するリリースゲートを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/ja/release/cosmos-comparison-gate.md`

## この文書を読む理由

- Cosmos/Tendermint 風の期待値に対するリリースゲート
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

- `release gate`
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## 英語原文の構造

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## 正規原文

- [英語の正規文書](../../en/release/cosmos-comparison-gate.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Required Evidence Properties — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Release Rule — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `--longrun-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--chaos-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--ops-runbook-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--external-audit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--formal-safety-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--fuzz-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--sdk-conformance-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-web3-conformance-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--p2p-scale-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--state-sync-light-client-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--snapshot-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--validator-economics-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--upgrade-governance-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--mev-fee-market-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--kms-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--bls-audit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
