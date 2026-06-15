# Storage Schema

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Storage Schema の規範仕様を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

この順番は、まず範囲と状態を理解し、次にメッセージ、safety、liveness の規則を確認し、最後に evidence を読む流れです。

## 文書概要

この文書は durable storage namespace、key schema、recovery markerを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/ja/specs/storage-schema.md`

## この文書を読む理由

- durable storage namespace、key schema、recovery marker
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
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## 英語原文の構造

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## 正規原文

- [英語の正規文書](../../en/specs/storage-schema.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Backend — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Records — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Indexes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: EVM Records — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Recovery Rules — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Snapshot Validation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Schema Migration — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `store.Store` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_ethstate` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBalance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getProof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bank/{0x_address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `auth/nonce/{0x_address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm/code/{0x_address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm/storage/{0x_address}/{slot}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_ethstate/{height}/meta` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_ethstate/{height}/accounts/{0x_address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_ethstate/{height}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `seen_ttl` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `code/{address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `storage/{address}/{slot}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `receipts/{tx_hash}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `logs/by_height/{height}/{tx_hash}/{log_index}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `logs/{address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
