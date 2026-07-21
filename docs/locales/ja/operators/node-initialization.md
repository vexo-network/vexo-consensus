> Locale: ja · 日本語

# ノードの初期化

このガイドでは、バリデーターとアーカイブ ノード ホームの初期化、起動、正常であることの確認、クライアントの接続方法について説明します。

ピア接続は `network_config.json` で設定する必要があり、`start` コマンド ラインで繰り返し渡さないでください。

コンセンサス、RPC、P2P、ロギング、または管理対象の Web3 アカウントに影響を与える実行時の動作は、構成ファイルのみです。 `vexod start` は、`--timeout-propose`、`--create-empty-blocks`、`--p2p-auth-token`、`--rpc-admin-token`、`--evm-account-key-env`、`--evm-account-key` などのフラグを拒否します。代わりに分割設定ファイルを編集して、すべてのオペレーターが同じ決定的なノードの動作を確認できるようにします。

ノードモードスイッチはありません。ノード ホームは、その構成ファイル、ジェネシス、キー マテリアル、および `validator_id` と署名者が存在するかどうかによって定義されます。

## あなたが構築しているもの

Vexo ノード ホームは、ノードの起動に必要なものがすべて含まれるディレクトリです。
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
重要なルールは単純です。一度初期化し、構成ファイルを編集してから開始します。ネットワークの動作をシェル フラグ内に隠さないでください。

## 5 分間のローカル実行

マルチホスト展開を検討する前に、バイナリが動作することを証明したい場合は、このフローを使用します。
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
別の端末で:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
予想されるステータスの形状:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
空ブロックの作成が無効になっている場合、単一ノードまたは空のメモリプールの実行では、最新の高さがゼロのままになることがあります。それはプロセスが壊れているという意味ではありません。これは、ノードが空のブロックを生成していないことを意味します。トランザクションを追加するか、マルチバリデータ テスト ネットワークを実行して、継続的なコミットを観察します。

## 4 つのバリデーター ローカル ネットワーク

このフローは、ピア接続、プロポーザー ローテーション、ブロック コミット ログ、および高さの増加が必要な場合に使用します。
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
便利なチェック:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
ブロック コミット ログが `log_config.json` で有効になっている場合、バリデータ ログには次のようなイベントが含まれます。
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
次のコマンドを使用して、生成されたローカル ネットワークを停止します。
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 とリミックス

Ethereum スタイルの JSON-RPC は、バージョン管理された Vexo 運用 API 名前空間ではなく、Web3 エンドポイントに存在します。

Docker 単一ホスト検証ツール 1 の場合、Remix カスタム プロバイダー URL は次のとおりです。
```text
http://127.0.0.1:28657/web3
```
デフォルトの RPC ポートを持つ直接ローカル ノードの場合:
```text
http://127.0.0.1:26657/web3
```
Remix が行うのと同じ呼び出しをテストします。
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
ブラウザがチェーン ID の取得に失敗したと表示する場合は、次のことを順番に確認してください。

1. URL は Web3 エンドポイント パスで終わります。
2. ブラウザはホスト ポートにアクセスできます。 Docker サンプルは、`28657`、`28667`、`28677`、および `28687` を公開します。コンテナー内では、RPC ポートは依然として `26657` です。
3. RPC サーバーが実行中です。同じホストとポート上のステータス エンドポイントをクエリします。
4. CORS は `network_config.json`/RPC 構成によって許可されます。デフォルトのハンドラーでは、カスタム CORS リストが設定されていない場合、ブラウザーのプリフライトが許可されます。
5. チェーンの `module_config.json` にはゼロ以外の EVM チェーン ID があります。

## バリデーターノード

ノードが提案、投票、コンセンサスメッセージに署名し、バリデータローテーションに参加する場合は、`init validator` を使用します。
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
このコマンドを実行する前に `VEXO_KEY_PASSPHRASE` を設定するか、1 回限りのローカル セットアップの場合は `--passphrase` を渡します。 `--encrypt-keys` は、`validator.key.json`、`node.key.json`、および `validator.vrf.key.json` を暗号化します。

鍵の保管に関する経験則:

- `validator.key.json` は、コンセンサス提案、投票、タイムアウト投票、ファイナリティ関連のメッセージに署名します。
- `node.key.json` は P2P ハンドシェイクのみに署名します。バリデーターコンセンサスキーとして再利用してはなりません。
- `validator.vrf.key.json` は委員会のランダム性を証明しており、バリデーターの保管資料と同様に扱う必要があります。
- パブリック リスナーは、暗号化されたローカル鍵ドキュメントまたはリモート署名者/KMS スタイルの鍵ドキュメントを使用する必要があります。 `require_network_safety=true` の間にノードがパブリック RPC または認証されたパブリック P2P を公開すると、起動時にプレーンテキストのローカル検証キーが拒否されます。
- 生成されたキーはファイルシステム モード `0600` で書き込まれます。有効期間の長いバリデーターには、リモート署名者/KMS が依然として好まれます。

BLS コンセンサス キーの場合:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` は、`blst-bls12381-minpk-v1` BLS キー ドキュメントを書き込み、所有証明を `genesis.json` バリデーター メタデータに `bls_pop` としてコピーします。

これにより、以下が作成されます。

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` はコンセンサス署名者です。 `node.key.json` は、`network_config.json:p2p.node_key_path` によって参照される P2P ハンドシェイク署名者です。これらは意図的に分離されているため、アーカイブ ノードとバリデーターは、すべてのピアにバリデーター署名キーを与えることなく、同じトランスポートを使用できます。

構成主導型ネットワーキングから開始します。
```bash
vexod start --home .vexo-validator-1
```
After startup, read the logs.正常なバリデータは、ノード実行イベント、RPC リスニング イベント、P2P リスニング イベント、およびブロックがコミットされるとブロック コミット イベントを発行する必要があります。空のブロックの作成が無効になっている場合、ブロックコミットされたログが見つからないということは、単にトランザクションが存在しないことを意味している可能性があります。

## アーカイブ ノード

ノードがチェーン データを保持し、RPC を公開し、ピアから同期し、バリデーター署名を回避する必要がある場合は、`init archive` を使用します。
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
これにより、以下が作成されます。

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

`validator.key.json` は**作成されません**。

以下から始めてください:
```bash
vexod start --home .vexo-archive-1
```
アーカイブ ノードはコンセンサス投票に署名しません。これらは、RPC、インデックス作成、状態同期、履歴証明の提供、およびプルーニング バリデータよりも広範なクエリ履歴の保持に役立ちます。

## 構成ファイルの分割

ノード ホームは別個の構成ファイルを使用するため、オペレーターは無関係な設定を混合することなく 1 つのサブシステムを編集できます。

- `config.json` には、ノード ID、チェーン ID、データ パス、および分割構成ファイルへのポインターが含まれます。
- `module_config.json` には、アプリケーション モジュールの選択、実行/事前ポリシー、およびモジュール レベルのガバナンス ポリシーが含まれます。
- `network_config.json` には、RPC、P2P ノード ID、リッスン/ピア/シード設定、TLS/認証設定、ピア スコアリング ポリシーが含まれます。
- `consensus_config.json` には、コンセンサス ループ タイミング、空ブロック ポリシー、暗号バックエンド、VRF、バリデーター アドミッション、および委員会ポ​​リシーが含まれます。
- `mempool_config.json` には、メモリプールのサイズ、料金、優先度、WAL、重複、および TTL ポリシーが含まれます。
- `log_config.json` には、ログ形式、レベル、ブロックコミットイベントログ、およびピアイベントログが含まれます。
- `genesis.json` には、不変の Genesis バリデーター、バリデーターのメタデータ、および Genesis モジュールの状態が含まれます。

`network_config.json` RPC 設定には、`shutdown_timeout`、`web3_max_subscriptions_per_connection`、`web3_idle_timeout` も含まれます。 `shutdown_timeout` は、コンセンサス ループ、RPC サーバー、およびノー​​ド トランスポートの正常なシャットダウンを制限するため、オペレーターはスタックした停止パスで永遠に待機する必要がありません。生成されるデフォルトは `10s` です。 Web3 サブスクリプションは、`2m` アイドル タイムアウトで接続ごとにデフォルトで 256 に設定されているため、パブリック RPC エンドポイントは無制限のアイドル サブスクリプションを蓄積できません。

`network_config.json` P2P 設定には、`auth_replay_path`、`require_auth_replay_store`、`dial_timeout` が含まれます。生成されたデフォルトは、ノンス リプレイ証拠を `data/p2p_auth_replay.jsonl` に書き込み、`10s` アウトバウンド ダイヤル タイムアウトを使用します。プライベート ループバック テストの場合、リプレイ ストアはほとんど無害な記録です。公開認証された P2P の場合、キャプチャされた署名付きハンドシェイク nonce が再起動後に再生されるのを防ぐため、これは安全要件です。 `dial_timeout` は、TLS、署名付きハンドシェイク検証、およびクロスリージョン遅延のために十分な長さである必要があります。設定が低すぎると、正常なピアが不安定に見え、再起動後の活性が低下する可能性があります。

`network_config.json` は起動状態の同期も所有します。これは、アーカイブ ノード、交換用バリデータ、またはクリーンなマシンに復元されたノードに役立ちます。 `state_sync.enabled` が true の場合、`vexod start` は `state_sync.snapshot_urls` から最初の有効なスナップショットをダウンロードし、チェーン ID、チェックサム、ステート ルート、および KV 名前空間を検証し、それを LevelDB に復元し、インデックスを再構築してからノードを起動します。ローカル状態がすでに `state_sync.min_height` を満たし、`state_sync.trust_local_higher` が true の場合、起動ログは `state_sync_skipped` を記録し、ローカル ストアを保持します。

`state_sync` ブロックの例:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
起動ログには、フェッチ エラーの場合は `state_sync_candidate_failed` が記録され、無効または古いスナップショットの場合は `state_sync_candidate_rejected` が記録され、検証済みの復元後には `state_sync_applied` が記録されます。 `max_snapshot_bytes` は、インフラストラクチャが意図的に提供する最大のスナップショットよりも低く保ちますが、通常の状態の成長には十分な大きさに保ちます。オペレータが帯域外信頼ポリシーとそのソースに対するファイナリティ/ライトクライアントの証拠を持っていない限り、パブリック ノードを認証されていないサードパーティのスナップショット ソースに向けないでください。

フィールドによってネットワークの動作が変更される場合は、分割された構成ファイルを編集し、レビューされたファイルをコミットまたは配布します。実行時の動作については長い `vexod start` フラグに依存しないでください。 start コマンドは、コンセンサス タイミング、空ブロック、P2P 認証、RPC 管理、およびマネージド Web3 キー フラグを意図的に拒否するため、オペレーターがレビューされた構成とは異なる動作を誤って実行しないようにします。

## どのファイルを編集すればよいですか?

|目標 |ファイル |フィールド |
|---|---|---|
| RPC バインド ポートを変更する | `network_config.json` | `rpc.address` |
| P2P バインド ポートを変更する | `network_config.json` | `p2p.listen_address` |
|永続的なピアを追加する | `network_config.json` | `p2p.peers` |
|シードピアを追加する | `network_config.json` | `p2p.seeds` |
|空のブロックを有効/無効にする | `consensus_config.json` |コンセンサス空ブロックフィールド |
|コンセンサス タイムアウトを調整する | `consensus_config.json` |プロポーザル、事前投票、事前コミット、およびコミットタイムアウトフィールド |
|完了した実行が必要 | `consensus_config.json` |コンセンサス実行コミットフィールド |
|モジュールを有効/無効にする | `module_config.json` |アプリケーションモジュールリスト |
| EVM チェーン ID を変更する | `module_config.json` |実行EVMチェーンIDフィールド |
|基本料金/ガスを調整する | `module_config.json` |実行基本料金、動的料金、ターゲットガス、および最大ガスフィールド |
| mempool WAL を構成する | `mempool_config.json` | mempool WAL パス |
|制御ブロックのコミットログ | `log_config.json` |ログコミットイベントフィールド |
|ピアログの制御 | `log_config.json` |ログピアイベントフィールド |

疑わしい場合は、次を実行してください。
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## キーの種類

ネットワーク安全性の検証には監査済みの BLS 集約ファイナリティが必要であるため、Validator init のデフォルトは `--key-type bls` です。 `--key-type ed25519` は、ネットワーク セーフティ ゲートの外側でプライベートな実験やカスタム展開に引き続き利用できます。 `--encrypt-keys` は、使い捨てではないノード ホームに使用する必要があります。スタンドアロン キー生成も VRF キーをサポートします。
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
VRF キーはコンセンサス署名者ではありません。これらは VRF 支援委員会の選択に使用され、バックエンドが有効な場合は `consensus_config.json` から `vrf_key_paths` とバリデーター メタデータ キー `vrf_public_key` から参照される必要があります。

`config.json` は分割された構成ファイルを指します。
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
各パスは絶対パスでも、ノード ホームに対して相対パスでもかまいません。省略した場合、`vexod` はデフォルトの `<home>/<name>_config.json` ファイルを使用します。

例 `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
ガバナンス ポリシーも `module_config.json` にあります。生成されたネットワークセーフ構成には、プロポーザルのデポジットが必要です。
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
デポジットは、提案提出者からエスクローされたネイティブ残高です。提案が可決された場合、保証金は返金されます。拒否された提案は `RejectedDeposits` に移動します。拒否された入金がデフォルトのモジュールアカウントではなく財務省に資金を提供する必要がある場合は、財務省/コミュニティプールモジュールによって制御されるアドレスを使用してください。

例 `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` および `rpc.evm_account_private_keys` はオプションであり、`eth_accounts`、`eth_sign`、`eth_signTransaction`、`eth_sendTransaction` などのバック Web3 管理アカウント メソッドです。 `evm_account_key_envs` を推奨します。これにより、秘密キーは JSON に保存されるのではなく、プロセス環境またはシークレット マネージャーによって挿入されます。このノードが意図的にローカル Web3 ホットウォレット エンドポイントとして機能する場合を除き、通常のバリデーター操作では両方のリストを空にしておきます。スタートアップ セーフティは、パブリック RPC リスナー上のマネージド EVM ホット キーを拒否します。

例 `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` は、`consensus_config.json` を含むディレクトリに対して相対的に解決されます。ローカル VRF キーの保管が避けられない場合は、暗号化されたキー ドキュメントを使用し、`VEXO_KEY_PASSPHRASE` をノード プロセスに提供します。オペレーターが実行するネットワークの場合は、生の VRF プライベート スカラーを `consensus_config.json` に直接配置しないでください。

`vexod config paths --home <home>` を使用して、解決されたすべてのパスを検査します。

アーカイブ構成には次のものがあります。
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
アーカイブ `consensus_config.json` はローカル コンセンサス ループを無効にします。
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
生成されたバリデーター ホームは、デフォルトで `config.json` に `"require_network_safety": true` を設定します。これはモードではありません。これは、決定論的な暗号、署名なし/非署名オフのトランザクション、手数料/ガスフロアの欠落、耐久性のあるメモリプール WAL の欠落、同じ署名者/ノンストランザクションの置換ポリシーの欠落、安全でない委員会のランダム性、および `finalized` 以外の `execution_commit` 値を拒否するスタートアップ セーフティ ゲートです。

`require_network_safety` が有効になっている場合は、次を実行します。
```bash
vexod config audit --home <home> --strict
```
ノードを起動する前に。監査は、同じネットワークに参加しているすべてのバリデータとアーカイブ ホームに合格する必要があります。

## 構成ベースのピア

ピア アドレスとリッスン アドレスは `network_config.json` にあります。
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` はこれらのピアを自動的にロードします。
```bash
vexod start --home .vexo-archive-1
```
永続的なピアとシードは `network_config.json` で構成されます。 `vexod start` はピアまたはシード ホストのオーバーライドを受け入れません。

`vexod start` コマンド ラインに長期間存続するホストまたは `host:port` 設定を入力しないでください。代わりに、`network_config.json` の `rpc.address`、`p2p.listen_address`、`p2p.peers`、および `p2p.seeds` を編集します。

`p2p.node_id` は、ノード ホームの存続期間中安定した状態に保ちます。 `p2p.node_key_path` は、`node.key.json`、またはピア ハンドシェイク署名のみに使用される別のローカル/管理キー ドキュメントを指す必要があります。ピア マップでは、意図的に同じものでない限り、アカウント アドレスやバリデーター オペレーター名ではなく、ピア ノード ID を使用する必要があります。

暗号化および認証された gRPC ピア トランスポートの場合は、`p2p.tls_cert_path`、`p2p.tls_key_path`、`p2p.tls_ca_path`、およびオプションで `network_config.json` の `p2p.tls_server_name` も設定します。相対 TLS パスは、ノードのホーム ディレクトリから解決されます。すべてのオペレーターが同じ再接続動作を使用できるように、`p2p.dial_timeout` を同じファイル内に保持します。シェルスクリプトでピアタイミングを隠さないでください。

## コンセンサスのタイミング

コンセンサス ループのタイミングは `consensus_config.json` にあります。
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` は、ラウンドが提案を待つ時間を制御します。
- `timeout_prevote` は投票収集ウィンドウを制御します。
- `timeout_precommit` は、コミット証明書収集ウィンドウを制御します。
- `timeout_commit` は、コミットされたブロック後の最小遅延を制御します。
- `create_empty_blocks: false` は、トランザクションが利用可能な場合にのみノードが提案することを意味します。
- `execution_commit: "finalized"` は、ファイナライズされた祖先を実行する前に HotStuff の 3 チェーンのファイナリティ決定を待機し、生成されたバリデータのデフォルトです。 `execution_commit: "qc"` は QC 認定ブロックをすぐに実行して永続化しますが、セーフティ ゲートはそれを拒否します。

`round_timeout` は互換性集約としてのみ保持されます。上記の Tendermint スタイルのタイムアウト フィールドを優先します。

`create_empty_blocks` が false の場合、メモリプールが空の間、高さは変更されないままにすることができます。これは予想通りです。チェーンは空のブロックをコミットするのではなく、有用な作業を待っています。トランザクションが発生し、ローカル コンセンサス ラウンドの状態が別のプロポーザーを超えてドリフトすると、ノードはバリデーターがプロポーザーとなりメモリプールから構築される次のラウンドに進みます。この回復パスにより、空ブロック スパムを再度有効にすることなく、トランザクションによってトリガーされる稼働状態が維持されます。

## マルチバリデータネットワーク

生成されたネットワークの場合:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
生成された各バリデーター ホームは以下を受け取ります。

- 独自の `validator.key.json`
- 独自の分割構成ファイル: `module_config.json`、`network_config.json`、`consensus_config.json`、`mempool_config.json`、および `log_config.json`
- 共有`genesis.json`
- 他のバリデーターの `network_config.json` ピア エントリ

`vexod network up` および `make network-e2e` は、すべてのバリデーターが開始し、スモーク トランザクションを送信し、高さの増加を観察するのを待機している間、プロセス レベルのタイムアウトを使用します。デフォルトのコマンド タイムアウトは、プロセスの起動、LevelDB のオープン、P2P 署名付きハンドシェイク、TLS/認証チェック、トランザクション許可、およびファイナリティをカバーするため、コンセンサス間隔よりも意図的に長く設定されています。コンセンサス タイムアウトを積極的に下げる場合は、ハーネスを早期に強制終了するのではなく、起動エラーを診断するのに十分なネットワークアップ タイムアウトを維持してください。

コンテナ化されたネットワークまたはマルチホスト ネットワークの場合は、トポロジ値を JSON ファイルに記述します。
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` および `rpc_host_template` は、各ノードの `network_config.json` ピア リストに書き込まれるダイヤル ターゲットです。 Docker では、これらは `validator-%d` などのサービス名になります。
- `p2p_advertise_host_template` および `rpc_advertise_host_template` は、`genesis.json` のバリデーター メタデータに書き込まれるパブリック アドレスです。ここでは、パブリック ネットワークの DNS 名またはパブリック IP を使用します。
- `p2p_listen_host` および `rpc_listen_host` はローカル バインド ホストです。すべてのインターフェイスでリッスンする必要があるコンテナーまたはサーバーには `0.0.0.0` を使用します。
- ネットワークが意図的にプライベートでない限り、Docker 専用サービス名をアドバタイズされたパブリック アドレスとして再利用しないでください。

次に、そのファイルからノード ホームを生成します。
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## トラブルシューティング

|症状 |最も考えられる原因 |何を確認するか |
|---|---|---|
| `latest_height` が増加しない |空のブロックが無効で TX がない、オンラインのバリデーターが不足している、または署名者が利用できない | `consensus_config.json`、バリデーター ログ、`/v1/diagnostics` |
| `peer_count` は `0` |ピア アドレスに到達できないか、間違ったホスト名に対して `network_config.json` が生成されました。 `p2p.peers`、コンテナー ホスト ポート、DNS、ファイアウォール |
| `p2p auth replay store` エラー |パブリック/認証済み P2P には耐久性のあるリプレイ ストレージが必要です。 `p2p.auth_replay_path` とホーム | の下の書き込み権限
| `eth_chainId` はリミックスに失敗します |間違った URL、間違ったホスト ポート、またはブラウザの CORS/プリフライトがカスタム構成によってブロックされました。 Web3 エンドポイント URL を使用し、同じエンドポイントを直接カールします。
| `config audit --strict` は失敗します |セーフティ ゲートで安全でない構成プロパティが見つかりました。失敗したチェックを読み、そのチェックに指定された分割構成ファイルを編集します。
| `no block_committed logs` |ロギングが無効になっているか、ブロックが作成されていません。 `log_config.json`、`create_empty_blocks`、メモリプールの内容 |
| `managed EVM key rejected` |ホット秘密キーはパブリック RPC リスナーで構成されます。 `evm_account_private_keys` を削除するか、RPC をプライベートのままにしておきます。

## 最小限のオペレーターのチェックリスト

ノードを別のマシンまたはオペレーターにホームに渡す前に、次のことを行ってください。

- `vexod validate --home <home>` は合格します。
- `vexod config audit --home <home> --strict` はその正確なホームに合格します。
- `config.json`、分割構成ファイル、`genesis.json`、およびパブリック バリデーター メタデータがレビューされます。
- `validator.key.json`、`node.key.json`、および `validator.vrf.key.json` は暗号化されるか、リモート署名者/KMS 鍵ドキュメントによって置き換えられます。
- `network_config.json:p2p.peers` には、ノードが実際にその Docker ネットワーク内で実行されていない限り、Docker のみの名前ではなく、ターゲット マシンからダイヤル可能なアドレスが含まれます。
- `require_network_safety` が有効な場合、`network_config.json` パブリック RPC/P2P リスナーには TLS マテリアルが含まれます。
- `module_config.json:execution.EVMChainID` は、Web3 ウォレットまたは Remix 接続の前に設定されます。
- 再起動後にノードが保留中の TXS を回復する必要がある場合、`mempool_config.json` には WAL パスが含まれます。
- `log_config.json` は、ネットワークの起動中にブロック コミットとピア ログを有効にします。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Validator Node — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Archive Node — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Split Configuration Files — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Key Types — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Config-Based Peers — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Consensus Timing — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Multi-Validator Network — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod start` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--timeout-propose` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--create-empty-blocks` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--p2p-auth-token` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--rpc-admin-token` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-account-key-env` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--evm-account-key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `VEXO_KEY_PASSPHRASE` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--passphrase` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--encrypt-keys` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `node.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator.vrf.key.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `require_network_safety=true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--key-type bls` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blst-bls12381-minpk-v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `genesis.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bls_pop` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `mempool_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `log_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `data/` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `shutdown_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_max_subscriptions_per_connection` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_idle_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `auth_replay_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `require_auth_replay_store` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `dial_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `data/p2p_auth_replay.jsonl` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `--key-type ed25519` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf_key_paths` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vrf_public_key` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `<home>/<name>_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.evm_account_key_envs` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.evm_account_private_keys` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_accounts` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sign` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_signTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_sendTransaction` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_account_key_envs` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod config paths --home <home>` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `"require_network_safety": true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `require_network_safety` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `host:port` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.listen_address` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.seeds` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_id` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_cert_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_ca_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.tls_server_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p.dial_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_propose` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_prevote` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_precommit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `timeout_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `create_empty_blocks: false` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit: "finalized"` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `execution_commit: "qc"` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `round_timeout` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `create_empty_blocks` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod network up` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make network-e2e` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `validator-%d` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_advertise_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_advertise_host_template` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `p2p_listen_host` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_listen_host` — この名前は実行例と設定検証でそのまま使うため翻訳しません。

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
