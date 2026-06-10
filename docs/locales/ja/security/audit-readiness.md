# Security Audit Readiness

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

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
- 文書を変更したら `make docs-check` で locale tree と翻訳 guard を確認します。

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
- Security Goals
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

監査提出物には BLS だけでなく VRF adapter audit evidence も含めます。`docs/security/ecvrf-audit-evidence.json` のような evidence ファイルの SHA-256 を `vrf.audit_evidence_sha256` または `--vrf-audit-sha256` に固定し、dependency audit、key custody、TLS/mTLS または pinned CA、auth、replay 防御、service availability を同じ境界で確認します。

## 正規原文

- [English canonical document](../../en/security/audit-readiness.md)
