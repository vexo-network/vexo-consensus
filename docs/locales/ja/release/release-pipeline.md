# リリースパイプライン

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Release Pipeline の release と運用手順を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Goals
2. Release Commands
3. CI Gates
4. Evidence Quality Rules
5. Artifacts
6. Reproducibility Notes
7. Signed Binaries
8. SBOM
9. Audit Pack
10. Release Candidate Targets
11. Launch Runbook

この順番は、まず目的と gate を理解し、次に artifact と evidence 要件を確認し、最後に実行手順へ進む流れです。

## 文書概要

この文書は 署名付きバイナリ、checksums、SBOM を含むリリースパイプラインを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/ja/release/release-pipeline.md`

## この文書を読む理由

- 署名付きバイナリ、checksums、SBOM を含むリリースパイプライン
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

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `RELEASE_CGO_ENABLED=1`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `release-candidate-smoke`
- `release-candidate-plan`
- `make release-portable RELEASE_REQUIRE_BLS=0`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## 英語原文の構造

- Release Pipeline
- 目的s
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- ローンチランブック

## EVM/Web3 適合性証跡

`--sdk-conformance-evidence` と `--evm-web3-conformance-evidence` は別々の証跡です。“EVM passed” という要約だけでは不十分です。EVM/Web3 証跡には機械判定できる `evm_fixtures`、`evm_execution`、`web3_rpc`、`evm_corpus` を含め、公開互換性を主張する前に SHA-256 で `evidence-manifest.json` へ結び付けてください。

## リリース候補ポリシー

公開リリース候補では既定の `make release-candidate` を使用します。この target は実際の gate であり `release-candidate-real` に接続され、BLS を含む artifact を作るために `RELEASE_CGO_ENABLED=1` を要求します。`make release-candidate-plan` は PR smoke と運用計画レビュー用です。組み込み fixture と dry-run plan を使うため、最終リリース evidence として提出してはいけません。no-cgo artifact が必要な場合は `make release-portable RELEASE_REQUIRE_BLS=0` を使えますが、それを BLS-capable release として公開してはいけません。 `RELEASE_CGO_ENABLED=1` で `RELEASE_TARGETS` を指定しない場合、Makefile は現在の host target だけをビルドします。複数 OS/architecture の artifact が必要な場合は、各 target 用の cgo cross-compiler を備えた runner で `RELEASE_TARGETS` を明示してください。

## VRF audit evidence SHA-256

`release gate` は BLS audit evidence だけでなく、VRF audit evidence も SHA-256 で固定します。`--vrf-audit` ファイルは `evidence-manifest.json` に含め、`--vrf-audit-sha256` はファイル内容と完全に一致させます。config を使う場合は `vrf.audit_evidence_sha256` が既定の digest pin になります。この規則は VRF service、KMS/HSM custody、TLS/mTLS または pinned CA、auth token、nonce replay 防御がリリース証拠に結び付いていることを確認するためです。

## 正規原文

- [英語の正規文書](../../en/release/release-pipeline.md)

## リリース証拠 attestation 用語

公開リリースでは、`evidence-manifest.json` の各項目が Ed25519 署名で検証される必要があります。次の CLI フラグと JSON フィールドは翻訳せず、そのまま保持します。

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
<!-- vexo-docs-ops-update-2026-06 -->

## ネットワーク E2E の読み方

`make network-e2e` は単なる build test ではありません。実バイナリで 4 validator を起動し、signed-shape smoke transaction、peer 接続、height 増加、clean stop を確認します。`NETWORK_E2E_GO_TIMEOUT` は外側の Go test 制限で、内側の network timeout より十分大きくします。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Goals — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Release Commands — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: CI Gates — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Evidence Quality Rules — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Artifacts — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Reproducibility Notes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Signed Binaries — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: SBOM — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Audit Pack — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Release Candidate Targets — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Launch Runbook — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `network analyze-longrun` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release collect-evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `ops-runbook` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p-scale` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `state-sync-light-client` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `snapshot-replay` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make check` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make fuzz-smoke` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod consensus adversarial` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod ops conformance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod network longrun` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod network chaos-plan` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make network-e2e` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make race` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `NETWORK_E2E_GO_TIMEOUT` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make test` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make vet` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make docs-check` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make build` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make release-candidate-smoke VERSION=ci`
- `make release-candidate-plan VERSION=ci` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make release-candidate VERSION=<rc> RELEASE_CGO_ENABLED=1 RC_EVM_CONFORMANCE_FLAGS=...` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evidence-manifest.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--allow-external-pending` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--private-rc` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo-release-evidence-attestation-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release evidence-manifest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--signing-key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--signing-key-env` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `<evidence-file>.sig` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `<evidence-file>.sig.pub` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `<evidence-file>.pub` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `dist/` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod-<version>-<os>-<arch>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `checksums.txt` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `checksums.txt.asc` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `sbom-go-modules.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `sbom-go-version.txt` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release-manifest.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release-audit-pack.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `longrun-analysis.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs-quality.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RELEASE_CGO_ENABLED=1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `supranational/blst` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `go build -trimpath` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `BUILD_DATE` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make release-candidate` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make release-portable RELEASE_REQUIRE_BLS=0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RELEASE_TARGETS` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release-candidate` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release-candidate-real` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod ops conformance --strict` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RC_EVM_CONFORMANCE_FLAGS` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RC_LONGRUN_DURATION` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `release-candidate-plan` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `RELEASE_REQUIRE_BLS=0` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `allow_noop_migrations=true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod upgrade apply --allow-empty-migrations` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--bls-audit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--bls-audit-sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--config <path>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `crypto.audit_evidence_sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--vrf-audit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--vrf-audit-sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf.audit_evidence_sha256` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/security/blst-audit-evidence.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/security/ecvrf-audit-evidence.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
