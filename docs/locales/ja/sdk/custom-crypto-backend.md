# カスタム暗号バックエンドガイド

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は custom crypto backend を追加する方法を説明します。初めて読むなら、次の順で進めるのが最短です。

1. Interfaces
2. Runtime Suite
3. Domain Separation
4. Production BLS Requirements
5. VRF Backend Requirements
6. Remote Signer Requirements
7. Test Backends

この順番は、実際に先に決めるべき内容と一致します。どの backend が必要かを選び、次に sign bytes と domain を固定し、最後に本番利用できるかを確認します。

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
- 文書を変更したら `make docs-check` でローカル文書ツリーと翻訳ガードを確認します。

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
- 目的
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

次のインターフェイス名は変更しないでください: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Goal — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Interfaces — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Runtime Suite — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Domain Separation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Production BLS Requirements — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Production VRF Requirements — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Remote Signer Requirements — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Test Backends — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `vexo-consensus` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `supranational/blst` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo.consensus.proposal.v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo.consensus.vote.v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo.consensus.timeout_vote.v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo.finality.proof.v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `crypto.adapter_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `BLSAdapter.Metadata().Name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `crypto.audit_evidence_sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bls_pop` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blst-bls12381-minpk-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `github.com/supranational/blst` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RELEASE_CGO_ENABLED=1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RELEASE_REQUIRE_BLS=1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make release-portable RELEASE_REQUIRE_BLS=0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `circl-bls12381-g1sigg2-basic-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bls_proof_of_possession` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.adapter_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.audit_report` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.dependency_audit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.audit_evidence_sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.key_source` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `committee.backend` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `crypto.NewProductionVRF` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `production_adapter: true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `ecvrf-p256-sha256-tai-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf_public_key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `remote-vrf-http-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `remote-http:<base-url>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `POST /prove` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `public_key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `issued_at_unix_nano` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `deadline_unix_nano` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo.remote_vrf.prove.v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `POST /verify` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo.remote_vrf.verify.v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `{ "valid": true, "nonce": "<same nonce>" }` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `VEXO_REMOTE_VRF_TOKEN` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `Authorization: Bearer <token>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.tls_cert_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.tls_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.tls_ca_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.tls_server_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `keys serve-vrf` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--auth-token` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--auth-token-env` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod keys serve-vrf` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `crypto.NewRemoteVRFService` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--home` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `remote-vrf-nonces.jsonl` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `remote-vrf-audit.jsonl` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--nonce-path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--audit-log` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `crypto.RemoteVRFServiceConfig.ReplayStore` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RequireDurableReplayStore: true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `crypto.NewFileRemoteVRFReplayStore` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf_key_paths` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `VEXO_KEY_PASSPHRASE` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.keys` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod keys serve-remote` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--guard-path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_proposal` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_vote` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_timeout_vote` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `finality_proof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
