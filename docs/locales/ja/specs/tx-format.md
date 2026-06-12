# Transaction Format

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は transaction format、signing、fee、gas ルールを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/ja/specs/tx-format.md`

## この文書を読む理由

- transaction format、signing、fee、gas ルール
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

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## 英語原文の構造

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## 正規原文

- [英語の正規文書](../../en/specs/tx-format.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Canonical Payload — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Address Format — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Signed Envelope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Required Ante Metadata — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: CheckTx Requirements — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Fee and Gas — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Load Test Payloads — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: CLI Examples — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `gas_limit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_chain_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `chain_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `base_fee` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `max(min_fee, base_fee * gas)` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blob_base_fee` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blob_gas` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blob_gas_fee_cap` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_sendRawBlobTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blob_hashes` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_getBlobSidecarByTxHash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_getBlobSidecarByBlobHash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_chainId` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `net_version` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sendRawTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `dynamic_base_fee` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `target_gas` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `dynamic_blob_base_fee` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `target_blob_gas` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bank:send` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
