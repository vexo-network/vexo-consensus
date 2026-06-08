# Vexo ドキュメント

このディレクトリは Vexo ドキュメントの日本語入口です。英語 (`en`) が正規の技術文書であり、日本語ツリーは同じ構造で参照しやすく整理されています。

## 最初に読むもの

1. [コンセンサスプロトコル概要](./consensus-protocol.md)
2. [コンセンサス仕様](./specs/consensus-spec.md)
3. [トランザクション形式](./specs/tx-format.md)
4. [バリデータライフサイクル](./specs/validator-lifecycle.md)
5. [セキュリティ監査準備](./security/audit-readiness.md)

## 文書セット

| 種別 | パス | 内容 |
|---|---|---|
| オペレーター | `operators/` | ノード初期化、バリデータ追加、設定管理 |
| リリース | `release/` | リリースパイプライン、ランブック、互換性、ゲート |
| SDK | `sdk/` | アプリモジュール、カスタム crypto/storage/transport、RPC バージョン管理 |
| セキュリティ | `security/` | 脅威モデル、前提、監査準備 |
| 仕様 | `specs/` | コンセンサス、ネットワーク、ストレージ、トランザクション、finality proof |

コマンド、JSON フィールド、RPC メソッド、コード識別子は翻訳せずそのまま保持します。
