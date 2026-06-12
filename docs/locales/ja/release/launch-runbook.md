# ローンチランブック

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は ネットワーク開始前の運用チェックリストと実行手順を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/ja/release/launch-runbook.md`

## この文書を読む理由

- ネットワーク開始前の運用チェックリストと実行手順
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

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## 英語原文の構造

- ローンチランブック
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## EVM/Web3 適合性証跡

公開リリース前に、`--evm-web3-conformance-evidence` を `--sdk-conformance-evidence` とは別に保管してください。このファイルには `evm_fixtures`、`evm_execution`、`web3_rpc`、`evm_corpus` が必要で、`release gate` が検証不能な要約を拒否できるようにします。

## VRF audit evidence SHA-256

release candidate を検証するときは、`release gate` に BLS と VRF の audit evidence digest を両方渡します。少なくとも `--bls-audit`、`--bls-audit-sha256`、`--vrf-audit`、`--vrf-audit-sha256`、`--evidence-manifest` を併用し、すべての evidence ファイルが manifest の SHA-256 と一致することを確認します。

## 正規原文

- [英語の正規文書](../../en/release/launch-runbook.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Prelaunch Gate — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Release Candidate Gate — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Genesis Gate — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Launch Window — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Postlaunch Archive — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `release docs-quality` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `checksums.txt` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `sbom-go-modules.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `sbom-go-version.txt` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release-manifest.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release-audit-pack.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release collect-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network analyze-longrun` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `longrun-evidence.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-default-fixtures` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-tx-fixtures` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-tx-fixtures-dir` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-execution-fixtures` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-execution-fixtures-dir` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-tx-fixtures-sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-execution-fixtures-sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-web3-conformance-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_fixtures` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_execution` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_rpc` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_corpus` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod ops conformance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `relayer soak-plan` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `chain_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evidence-manifest.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
