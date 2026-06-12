# Release Pipeline

> Locale: ja · 日本語
> この文書は英語原文と併読するための日本語 補助文書です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

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
- 文書を変更したら `make docs-check` で locale tree と翻訳 guard を確認します。

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
