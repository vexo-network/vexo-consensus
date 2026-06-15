# ファイナリティ証明フォーマット

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Finality Proof Format の規範仕様を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Scope
2. Proof Fields
3. Header Fields
4. Quorum Certificate Fields
5. Commit Chain Fields
6. Verification Algorithm
7. Accountable Safety Detection
8. Ed25519 Model
9. BLS Model

この順番は、まず範囲と状態を理解し、次にメッセージ、safety、liveness の規則を確認し、最後に evidence を読む流れです。

## 文書概要

この文書は finality proof のフィールド、検証順序、validator set bindingを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/ja/specs/finality-proof-format.md`

## この文書を読む理由

- finality proof のフィールド、検証順序、validator set binding
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

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## 英語原文の構造

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## 正規原文

- [英語の正規文書](../../en/specs/finality-proof-format.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Proof Fields — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Header Fields — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Quorum Certificate Fields — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Commit Chain Fields — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Verification Algorithm — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Accountable Safety Detection — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Ed25519 Model — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: BLS Model — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `finality.Proof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/finality/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/finality/{height}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `strict: true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/status.latest_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/finality/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `Proof.ValidatorSetHeight <= Header.Height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `Proof.ValidatorSetHash == loaded_set.Hash()` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `Header.ValidatorSetHash == loaded_set.Hash()` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `QuorumCert.Height == Header.Height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `Header.TxRoot` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `HeaderHash(link.Header)` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `finality.AttackDetector` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--validator-set` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blst-bls12381-minpk-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `supranational/blst` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
