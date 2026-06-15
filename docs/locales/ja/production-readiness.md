# 本番運用準備ガイド

> Locale: ja · 日本語
> セキュリティとリリース判断は英語の正本と release gate の結果で確定します。

## 概要

この文書は、Vexo ベースのネットワークを本番利用可能と呼ぶ前に確認すべき条件を説明します。

このローカライズ文書では、コマンド、JSON フィールド、RPC メソッド、設定キー、パッケージ名をそのまま残し、どの言語でも例をコピーして使えるようにします。

## なぜ重要か

Vexo には、BFT コンセンサス、アプリケーションモジュール、ネイティブ会計、任意の EVM 実行、バリデータ経済、ピアネットワーク、リリース証跡が含まれます。読者は、機能があるかどうかだけでなく、それを安全に運用する方法と、対象ネットワークで正しく動くことをどう証明するかまで説明できる必要があります。

## 必ず確認すること

- **Protocol correctness**: `consensus`, `finality`, validator-set hash, vote sign bytes, timeout certificate, and three-chain finality must agree for the target validator set.
- **Runtime correctness**: `app`, `runtime`, `store`, and module writes must commit atomically, replay deterministically, and recover from crash boundaries.
- **Crypto custody**: BLS, VRF, remote signer, KMS/HSM, proof-of-possession, replay nonce, and double-sign guard evidence must match the release binary.
- **Networking safety**: `network_config.json` must bind chain ID, genesis hash, node ID, TLS/auth policy, durable replay path, peer scoring, ban, and backoff settings.
- **EVM/native accounting**: The EVM module uses the native Vexo coin as the balance asset; gas, base fee, blob base fee, receipts, proofs, and traces must pass external corpora.
- **Release evidence**: Release claims need signed artifacts, SBOM, evidence manifest, longrun, chaos, E2E, state sync, economics, governance, MEV, SDK, and EVM/Web3 evidence.

## 運用者の作業

- **System view**: A Vexo network is safe only when protocol, runtime, operations, and evidence are ready together. Do not treat enabled code as a production claim.
- **Configuration review**: Review `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before `vexod start`.
- **Release decision**: Attach evidence from the exact binary, genesis, config schema, module set, and validator topology that will be released.

## 変更しないインターフェイス名

- `vexod validate --home <home>`
- `vexod config audit --home <home> --strict`
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `peer_count`
- `active_peer_count`
- `configured_peer_count`
- `scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `network_config.json`
- `consensus_config.json`
- `module_config.json`
- `mempool_config.json`
- `release gate`

## よくある間違い

- 設定済みの peer が実際に接続されているとは限りません。アクティブなセッションを別途確認してください。
- リリース証跡なしに BLS、VRF、EVM、state sync、または governance を本番対応とみなさないでください。
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## 規範参照

- [規範となる原文](../en/production-readiness.md)

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: The Short Version — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: How To Use This Guide — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Readiness Levels — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: System Map — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Configuration Review Order — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Consensus and Finality Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Runtime and Storage Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: EVM and Native Coin Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Crypto and Key Custody Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Networking Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Observability Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Release Evidence Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Common Failure Modes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: What This Guide Does Not Claim — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `docs/specs/consensus-spec.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/specs/finality-proof-format.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `modules/staking` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/specs/validator-lifecycle.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `modules/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/sdk/app-module-guide.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/specs/storage-schema.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `modules/bank` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/specs/tx-format.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/specs/evm-native-accounting.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `modules/evm` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/sdk/rpc-api-versioning.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `cmd/vexod keys` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/sdk/custom-crypto-backend.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/security/audit-readiness.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/specs/networking-spec.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/operators/node-initialization.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/release/launch-runbook.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `cmd/vexod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/operators/observability.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/release/release-pipeline.md` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.tls_cert_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.tls_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.tls_ca_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `mempool_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `log_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod validate --home <home>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod config audit --home <home> --strict` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `allow_unsafe_qc_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_propose` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_prevote` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_precommit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `create_empty_blocks` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getProof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `go.mod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `max_score` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `latest_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make check` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `v1/status` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `active_peer_count` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_web3Capabilities` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
