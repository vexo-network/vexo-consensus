# セキュリティ監査準備

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は threat model、セキュリティ前提、監査提出証跡を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/ja/security/audit-readiness.md`

## この文書を読む理由

- threat model、セキュリティ前提、監査提出証跡
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

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## 英語原文の構造

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- セキュリティ目標
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

監査提出物には BLS だけでなく VRF adapter audit evidence も含めます。`docs/security/ecvrf-audit-evidence.json` のような evidence ファイルの SHA-256 を `vrf.audit_evidence_sha256` または `--vrf-audit-sha256` に固定し、dependency audit、key custody、TLS/mTLS または pinned CA、auth、replay 防御、service availability を同じ境界で確認します。

## 正規原文

- [英語の正規文書](../../en/security/audit-readiness.md)
## 監査準備の補足説明

監査者は、コードが存在するかだけでなく、証拠が再現できるかを最初に確認します。そのため、脅威モデル、既知の制限、BLS/VRF 監査資料、EVM conformance 結果、longrun/chaos 結果、KMS 署名証拠、snapshot/replay 証拠は、同じ候補バイナリと同じ設定から生成されている必要があります。運用者は例外を隠さず、release gate が拒否した項目、担当者、再検証条件を runbook に残すべきです。

## mempool WAL と consensus WAL

受け入れたすべての tx または batch イベントは、送信が返る前に永続化されます。レコードは `uvarint(length) | op | payload | CRC-32C` で、同時に来た admission は一つの fsync を共有し(group commit)、ノードは admission ロックを保持せずにそれを待ちます。replay は途切れた、または破損した最後のレコードを切り詰め、終端より前で壊れたログは拒否します(`ErrWALCorrupt`)。I/O 失敗が起きると、ログを pool から書き直すまで append を止めます。consensus WAL も同じ framing(4 バイト big-endian の長さ、payload、4 バイトの CRC-32C)を使い、途切れた末尾は切り詰め、途中の破損は拒否します。

## Consensus WAL の group record と mempool 除去の耐久性

vote と、その vote が machine を移す safety state は一つの group record として fsync 一回で書かれ、復旧時には丸ごと残るか丸ごと捨てられます。proposal record は自前の fsync を持たず、proposer の vote と一緒に、proposal が node を出る前に書かれます。commit された block の mempool 除去 record は次の block が commit される前に durable になり、再起動は最後に commit された block の除去をやり直します。

## Mempool WAL の定期 sync

mempool WAL は変更ごとに log に書き、100 ms ごとに sync します(`walSyncInterval`)。admission は書き込みだけを待ち、sync は待ちません。プロセスのクラッシュでは記録が残り、マシンのクラッシュでは最後の間隔の admission だけが失われ、クライアントが再提出します。commit された block の除去 record は次の commit の前に sync されます(`WaitDurable`)。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Threat Model — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Security Assumptions — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Known Limitations — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Formal-ish Safety Argument — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Required Evidence for Audit — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Auditor Focus Areas — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Practical Audit Walkthrough — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Remote Signer Audit Notes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: EVM/Web3 Audit Notes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Snapshot and WAL Audit Notes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `docs/security/blst-audit-evidence.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `remote-vrf-http-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod keys serve-vrf` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release collect-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `chain_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `go.mod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/recovery/report` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `da_mismatch` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `missing_data` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator_set_hash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app_hash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `finality.LightClient` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod light verify` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `next_validator_set_hash` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `parent_linkage` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus.MaxBlockBytes`
