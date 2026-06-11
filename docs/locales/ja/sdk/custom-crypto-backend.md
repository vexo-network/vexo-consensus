# Custom Crypto Backend Guide

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は BLS、VRF、signer など custom crypto backend の接続方法を理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/ja/sdk/custom-crypto-backend.md`

## この文書を読む理由

- BLS、VRF、signer など custom crypto backend の接続方法
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

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `ecvrf-p256-sha256-tai-v1`
- `remote-vrf-http-v1`
## 英語原文の構造

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## VRF audit evidence SHA-256

VRF backend も BLS と同じ水準で監査境界を示す必要があります。`vrf.adapter_name`、`vrf.audit_report`、`vrf.dependency_audit`、`vrf.audit_evidence_sha256`、`vrf.key_source` をすべて設定し、adapter metadata と config が一致しない場合は runtime が fail closed になるべきです。built-in ECVRF adapter は go.mod dependency pin と audit evidence digest を検証し、remote VRF adapter は外部 KMS/HSM audit reference を使います。

## 正規原文

- [英語の正規文書](../../en/sdk/custom-crypto-backend.md)

## Remote VRF service

`vexod keys serve-vrf` は ECVRF key で `POST /prove` と `POST /verify` を提供し、`vexod keys verify-vrf` は remote prover を end-to-end で確認します。`VEXO_REMOTE_VRF_TOKEN`、`remote-vrf-http-v1`、`vexo.remote_vrf.prove.v1`、`vexo.remote_vrf.verify.v1` は翻訳しません。

Keep these interface names unchanged: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.
