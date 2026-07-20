> Locale: ja · 日本語

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

-ビザンチンの投票権の3分の1未満
-ドメイン区切りの提案、投票、タイムアウト投票、および最終署名
-関連する証明高さでのバリデータセットハッシュバインド
- QCおよび最終性証明における固有の既知の署名者
-検証者の曖昧さに対する説明責任のある証拠
-同じ確定した高さで競合するコミット決定を拒否する

##暗号境界

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

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
