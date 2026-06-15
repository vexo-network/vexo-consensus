> Locale: ja · 日本語

# バリデーターの追加

このガイドでは、Vexo ネットワークにバリデーターを追加するためのオペレーター フローについて説明します。

正確な入場経路は、チェーンのステーキングとガバナンスポリシーによって異なります。少なくとも、バリデーターはチェーン状態で表され、有効な資格情報を持ち、高さバージョン管理されたバリデーター セット更新の一部になる必要があります。

## 1. バリデーターホームの初期化
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
BLS バリデーターキーの場合:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
これらのコマンドを実行する前に `VEXO_KEY_PASSPHRASE` を設定するか、1 回限りのローカル セットアップの場合は `--passphrase` を渡します。

BLS バリデーターを既存のチェーンに許可する場合は、生成された `bls_pop` メタデータをバリデーター更新提案に含めます。
デフォルトの BLS キー パスは `blst-bls12381-minpk-v1` を使用します。 `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` は参照/互換性テストのみに使用してください。

生成された公開キーをアーカイブします。
```bash
vexod keys show --home .vexo-validator-new --json
```
生成された `node.key.json` も保持します。 `network_config.json:p2p.node_id` の P2P ハンドシェイクに署名します。これはバリデーターコンセンサスキーではないため、アカウントキーとして再利用しないでください。

## 2. ネットワークアドレスとピアを構成する

`.vexo-validator-new/network_config.json` を編集し、ローカル リッスン アドレスと永続ピアを設定します。
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
実稼働バリデータの長期間有効なコマンドライン ネットワーク オーバーライドに依存しないでください。永続的なピア アドレスを `network_config.json` に保持します。

別のアドレスの役割を使用します。

- `p2p.listen_address` および `rpc.address` は、このマシンまたはコンテナーのローカル バインド アドレスです。
- `p2p.node_id` は、このノードのピア ID です。仲間が学習した後も安定した状態に保ちます。
- `p2p.node_key_path` は、そのピア ID のローカル ハンドシェイク署名キーを指します。
- `p2p.peers` には、このノードが他のピ​​アに接続するために使用するダイヤル ターゲットが含まれます。マップ キーはリモート ノードの `p2p.node_id` 値である必要があります。
- バリデーターのメタデータ `p2p_address` および `rpc_address` には、ネットワークが意図的にプライベートでない限り、Docker 専用のサービス名ではなく、パブリックにアドバタイズされたアドレスを含める必要があります。

## 3. バリデータの承認を送信する

たとえばステーキング フローでは、ステーキング トランザクションを構築します。
```bash
vexod staking --help
```
バリデーター許可トランザクションには以下を含める必要があります。

- バリデータID
- バリデータアドレス
- コンセンサス公開鍵
- 議決権または出資基準
- バリデータコミッションベーシスポイント（チェーンがセルフサービスコミッション更新を許可する場合）
- P2P `node_id` メタデータ (チェーンがジェネシス/バリデーターメタデータを使用してピアマップを事前シードする場合)
- パブリック P2P アドレス メタデータ
- パブリック RPC アドレス メタデータ (パブリックの場合)
- BLS が有効な場合の BLS 所有証明メタデータ

バリデーターの更新は特定の高さで有効になり、新しいバリデーターセットのハッシュを生成する必要があります。

バリデーターがアクティブになった後、オペレーターはステーキング モジュールを通じて報酬の状態を公開できます。
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. バリデーターセットの更新を確認する

更新後の高さ:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
確認してください:

- バリデーターは高さ固有のセットに表示されます
- 投票権は正しい
- バリデーターセットのハッシュが期待どおりに変更されました
- ファイナリティプルーフは正しいバリデータセットの高さを参照します

## 5. バリデータキーのローテーションを計画する

バリデーターキーをローテーションするには、重複しない `active_from` および `active_until` メタデータを含む次のキードキュメントを準備し、追加のローテーションキーでノードを開始します。
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
署名時に、ノードはアクティブ ウィンドウにコンセンサスの高さが含まれるキーを使用します。リモート署名者の鍵ドキュメントは、同じポリシー、認証トークン、および二重署名ガード要件を維持します。

## 6. バリデーターの開始
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
スタートアップにはネットワーク モード スイッチがありません。ネットワークがパブリック ネットワークの安全性の前提を満たすことが期待される場合は、起動前に `config audit --strict` を使用します。

## 7. モニター

見る:

- 提案/投票の待ち時間
- ラウンドタイムアウト
- バリデーター署名の失敗
- ピアバン
- メンプールのサイズ
- コミットレイテンシ
- スナップショット/リプレイの状態

使用:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## 安全上の注意事項

- 独立したチェーン間でバリデーターキーを再利用しないでください。
- 運用バリデータに対してリモート署名者ポリシーを有効にしたままにします。
- 所有証明または同等の不正キー防御を持たない BLS バリデータを許可しないでください。
- 正しい証拠の高さのバリデーターセットに関連付けられた検証済みの証拠なしに、バリデーターを切りつけたり刑務所に入れたりしないでください。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: 1. Initialize Validator Home — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 2. Configure Network Addresses and Peers — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 3. Submit Validator Admission — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 4. Verify Validator Set Update — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 5. Plan Validator Key Rotation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 6. Start Validator — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: 7. Monitor — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Safety Notes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `VEXO_KEY_PASSPHRASE` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--passphrase` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bls_pop` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blst-bls12381-minpk-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `.vexo-validator-new/network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.listen_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `active_from` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `active_until` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `config audit --strict` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
