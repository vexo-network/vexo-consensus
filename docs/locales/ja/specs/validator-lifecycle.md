# バリデータライフサイクル

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Validator Lifecycle の規範仕様を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Scope
2. Admission
3. Validator Set
4. Rotation
5. Evidence Lifecycle
6. Slashing
7. Jail and Unbonding

この順番は、まず範囲と状態を理解し、次にメッセージ、safety、liveness の規則を確認し、最後に evidence を読む流れです。

## 文書概要

この文書は validator join、rotation、jail、slashing、leave lifecycleを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/ja/specs/validator-lifecycle.md`

## この文書を読む理由

- validator join、rotation、jail、slashing、leave lifecycle
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

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## 英語原文の構造

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## 正規原文

- [英語の正規文書](../../en/specs/validator-lifecycle.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Admission — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Validator Set — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Rotation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Evidence Lifecycle — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Slashing — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Jail and Unbonding — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `vexovaloper...` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexovalcons...` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo...` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `staking tx withdraw-unbonded` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
