# コンセンサス仕様

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Consensus Spec の規範仕様を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

この順番は、まず範囲と状態を理解し、次にメッセージ、safety、liveness の規則を確認し、最後に evidence を読む流れです。

## 文書概要

この文書は 合意 state machine の規範仕様を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/ja/specs/consensus-spec.md`

## この文書を読む理由

- 合意 state machine の規範仕様
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

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## 英語原文の構造

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## 正規原文

- [英語の正規文書](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## 空ブロックと Round 回復

`create_empty_blocks=false` で mempool が空なら、height が止まって見えるのは正常な idle 状態です。取引が入っても、ノードは現在の `(height, round)` の決定的な proposer の場合だけ提案し、proposer でないノードがローカルで別の round に進むことはありません。round は有効な timeout certificate または certified finality 遷移によってのみ進み、実行・保存エラーは timeout として扱いません。

## 適応型ラウンド timeout

`adaptive_round_timeout_enabled = true` の場合、base timeout、proposal/vote/commit 処理時間の rolling p95、進捗結果、active peer 不足から timeout を算出します。timeout 後は 1.5 倍、成功後は 0.8 倍とし、観測処理時間の 3 倍を安全予算に加え、base から base の 8 倍に clamp します。safe-vote、proposer、quorum power、QC、three-chain finality は変更しません。

## 復旧 finality gate

`recovery_finality_gate_enabled = true` で永続 application state height と block index height が異なる場合、その最小値を safe recovery height とします。境界より上の finalized application commit は整合性が戻るまで延期されます。現在の timeout と recovery deferral は `/v1/metrics` と `/metrics/text` で確認できます。

## 決定的 transaction ordering

Proposal は `chain_id` と height から salt を作り、signer ごとの transaction chain を nonce 昇順に並べ、salted transaction hash で chain head を merge します。同じ candidate set で mempool arrival order を除きますが、first-seen fairness、censorship resistance、confidentiality、形式的 order fairness は保証しません。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Roles — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: State — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Message Types — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Safety Rules — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Finality Rule — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Execution Commit Policy — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Liveness Assumptions — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Empty Blocks and Round Recovery — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Evidence — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `chain_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator_set_hash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `locked_qc` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `high_qc` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `last_timeout_cert` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `last_finalized` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `>= 2/3` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `B3.height = B2.height + 1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `B2.height = B1.height + 1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit = "qc"` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit = "finalized"` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `block_committed` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `create_empty_blocks = false` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `latest_height = 0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `latest_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `actual_hash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `actual_time_unix_nano` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `parity_shards` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
