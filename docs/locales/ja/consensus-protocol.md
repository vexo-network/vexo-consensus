> Locale: ja · 日本語

# コンセンサスプロトコル概要

このページは Vexo コンセンサスを理解するための上位入口です。規範的な詳細は [Consensus Spec](./specs/consensus-spec.md)、[Finality Proof Format](./specs/finality-proof-format.md)、[Validator Lifecycle](./specs/validator-lifecycle.md)、[Storage Schema](./specs/storage-schema.md)、[Networking Spec](./specs/networking-spec.md)、[Transaction Format](./specs/tx-format.md) に従います。

## モデル

Vexo は proposal、vote、quorum certificate(QC)、timeout certificate、locked-QC safety、three-chain finality を備えた HotStuff スタイルの BFT コアを使用します。ブロックが安全に投票対象となるのは locked QC を拡張するか、lock 以上に新しい justify QC を持つ場合だけです。ブロック、親、祖父の高さとハッシュを明示的に結ばない合成 QC や高さを飛ばす QC chain は、finality 決定前に拒否されます。

## プロトコルの同一性と研究境界

Vexo は未変更の HotStuff の別名ではなく、AptosBFT、DiemBFT、Jolteon、Ditto、Tendermint、CometBFT と同一のプロトコルや実装でもありません。独立した Go runtime で HotStuff 系の安全概念を再利用し、適応型ラウンド時間、永続的復旧、決定的トランザクション順序、モジュール実行、高さごとの validator set を組み合わせます。

現在の投票経路は高さ別の全 validator set と決定的 proposer を使用します。VRF committee selector は component と query surface にはありますが、proposal eligibility や quorum formation には未接続です。したがって VRF committee consensus は有効な特性ではなく将来研究として記述します。貢献範囲と実験計画は [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) を参照してください。

## 実行と復旧の境界

QC certified、HotStuff finalized、application executed、state committed は別の事象です。既定の `execution_commit=finalized` は three-chain rule が選んだ祖先だけを実行します。適応型 pacemaker と `recovery_finality_gate_enabled` は遅延および再起動復旧の方針であり、proposer、quorum power、safe-vote、three-chain finality は変更しません。

## 安全境界

-ビザンチンの投票権の3分の1未満
-ドメイン区切りの提案、投票、タイムアウト投票、および最終署名
-関連する証明高さでのバリデータセットハッシュバインド
- QCおよび最終性証明における固有の既知の署名者
-検証者の曖昧さに対する説明責任のある証拠
-同じ確定した高さで競合するコミット決定を拒否する

## 暗号境界

- `deterministic` backend はテスト専用で network safety 検証を通りません。
- `ed25519` は公開ネットワーク試験とローンチ準備で利用できます。
- `bls` は既定で `blst-bls12381-minpk-v1` を使い、proof-of-possession、subgroup check、public-key validation、依存関係監査、release-gate evidence を必要とします。
- network safety 検証に VRF adapter metadata は必要ですが、VRF committee が有効なコンセンサス経路であることを意味しません。

-すべてのバリデータホームの厳格な構成監査
-リリースゲートの証拠
-外部セキュリティレビュー
-複数ホストによる長期滞在と混沌の証拠
-署名者/KMSポリシーの証拠
-チェーン固有の経済とガバナンス政策の見直し

リリースを本番環境に対応したものとして扱う前に、[セキュリティ監査の準備状況](./security/audit-readiness.md)と[リリースパイプライン](./release/release-pipeline.md)を参照してください。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、翻訳版が英語正本と同じ実行可能インターフェイスと運用境界を保っているかを確認するための要約です。コマンド、設定キー、RPC パス、コード識別子は翻訳しません。意味だけを日本語で補足し、実運用で変えてはいけない値はそのまま残します。
`require_network_safety` と `block_committed` は、特にそのまま残すべき重要な用語です。
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### セクション追跡
- section: Model - HotStuff 風 BFT、three-chain finality、QC、timeout certificate、locked-QC safety をまとめて確認します。
- section: Execution Terms - QC certified、finalized、executed、state committed の違いをはっきり区別します。
- section: Safety Boundary - 3分の1未満の Byzantine、domain separation、validator-set hash binding、accountable evidence を扱います。
- section: Crypto Boundary - `deterministic`、`ed25519`、`bls`、`blst-bls12381-minpk-v1`、`ecvrf-p256-sha256-tai-v1` を同じ基準で確認します。
- section: Operational Boundary - `vexo_quorum_health_ratio`、`adaptive_round_timeout_enabled`、`recovery_finality_gate_enabled`、snapshot/replay health を一緒に見ます。

### そのまま維持するインターフェイス
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## 運用メモ

バリデータホームを用意するときは、`config.json` だけでなく `module_config.json`、`network_config.json`、`consensus_config.json`、`mempool_config.json`、`log_config.json` をまとめて確認します。実運用では `vexo_quorum_health_ratio` と `adaptive_round_timeout_enabled` を並べて見て、ピア数だけで健全性を判断しないことが大切です。

- `execution_commit=finalized` を優先します。
- `qc` 経路は管理された試験網だけで使います。
- `recovery_finality_gate_enabled` と snapshot/replay health を同時に確認します。
