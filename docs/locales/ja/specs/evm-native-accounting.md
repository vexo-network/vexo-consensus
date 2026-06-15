# EVM とネイティブ会計

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Evm Native Accounting の規範仕様を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Core Rule
2. Amount Encoding
3. Fee Accounting
4. EVM Execution
5. State Root Policy
6. Compatibility Boundary
7. Failure Modes

この順番は、まず範囲と状態を理解し、次にメッセージ、safety、liveness の規則を確認し、最後に evidence を読む流れです。

## 文書概要

この文書は native coin と EVM gas/accounting を一貫して接続する方法を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/ja/specs/evm-native-accounting.md`

## この文書を読む理由

- native coin と EVM gas/accounting を一貫して接続する方法
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
- `bank query balance`

## 英語原文の構造

- EVM とネイティブ会計
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM 実行
- State Root Policy
- Compatibility Boundary
- Failure Modes

## 正規原文

- [英語の正規文書](../../en/specs/evm-native-accounting.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Core Rule — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Amount Encoding — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Fee Accounting — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: EVM Execution — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: State Root Policy — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Compatibility Boundary — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Failure Modes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `base_fee * gas` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `contract.Invocation` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `value_hex` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `gas_price_hex` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `max_fee_per_gas_hex` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `max_priority_fee_per_gas_hex` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBalance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sendRawBlobTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_sendRawBlobTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sendRawTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution.strict_evm_state_root` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
