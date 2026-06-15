> Locale: ja · 日本語

# ドキュメント

このディレクトリは `vexo-consensus` の実務マニュアルです。

ソースコードを推測で読むのではなく、ネットワークを理解・構築・運用・監査・リリースしたい人のための文書です。各ページは次の4つにすぐ答えられる必要があります。

1. この機能はシステムの中で何を担当するのか。
2. どのファイル、コマンド、config key、RPC、JSON field がそれを実装しているのか。
3. 安全に使うために何が満たされていなければいけないのか。
4. 本番ネットワークに出す前に、どんなテストや運用証拠が必要なのか。

英語は protocol、安全性、release、SDK、command、config、RPC 動作の正本です。各ローカライズ文書は同じツリーの直訳であり、release と audit の判断は必ず英語原文を基準に確認してください。

## クイックスタート

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## まず読むもの

時間があまりないなら、次の順で読むと把握しやすいです。

1. [`Node Initialization`](./operators/node-initialization.md) — node home の作成、分割された config ファイルの編集、validator node / archive node の起動方法。
2. [`Docker Deployment`](../deployments/docker/README.md) — 単一ホストの 4 ノード構成を動かす方法、または multi-host ネットワークの準備方法。
3. [`Observability Guide`](./operators/observability.md) — ノードは生きているが正常ではないときに、まず見るべき signal。
4. [`RPC API Versioning`](./sdk/rpc-api-versioning.md) — wallet、Remix、Web3 ツールを Vexo の RPC/Web3 endpoint に接続するための規則。

release candidate を見ているなら、細かい仕様より先に [`Production Readiness`](./production-readiness.md) と [`Release Pipeline`](./release/release-pipeline.md) を読んでください。

## この文書セットの目的

`vexo-consensus` は独立した PoS network を作るための consensus と runtime の framework です。この文書セットは、初めてコードを見る developer、node を運用する validator/operator、release を準備する maintainer、audit を準備する security reviewer が、同じ基準で project を理解するためのものです。

command、JSON field、RPC 名、config key、package path、code identifier は互換性のために英語のまま維持します。説明や読む順番、運用上の注意点は日本語で補います。

良い文書は単に「機能がある」とは言いません。各文書は次の問いに答えるべきです。

1. この機能は system の中でどんな責務を持つのか。
2. どの file、command、config key、RPC、JSON field がそれを実装しているのか。
3. 安全に使うにはどの条件が必須なのか。
4. 本番ネットワークに載せる前に、どんな test と operation evidence が必要なのか。

## 読む順序

1. [Consensus Protocol Overview](./consensus-protocol.md)
2. [Consensus Spec](./specs/consensus-spec.md)
3. [Transaction Format](./specs/tx-format.md)
4. [Validator Lifecycle](./specs/validator-lifecycle.md)
5. [Node Initialization](./operators/node-initialization.md)
6. [Security Audit Readiness](./security/audit-readiness.md)

## 役割別の読み方

| 役割 | 最初に読む文書 | あわせて見る文書 |
|---|---|---|
| protocol を理解したい developer | [Consensus Spec](./specs/consensus-spec.md) | [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md) |
| app module を追加する developer | [App Module Guide](./sdk/app-module-guide.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| EVM 機能を追加する developer | [EVM and Native Accounting](./specs/evm-native-accounting.md) | [Transaction Format](./specs/tx-format.md), [RPC API Versioning](./sdk/rpc-api-versioning.md) |
| node を運用する人 | [Node Initialization](./operators/node-initialization.md) | [Adding a Validator](./operators/add-validator.md), [Observability Guide](./operators/observability.md) |
| release / audit を準備する人 | [Production Readiness](./production-readiness.md) | [Security Audit Readiness](./security/audit-readiness.md), [Release Pipeline](./release/release-pipeline.md) |

## protocol specs

| 文書 | 目的 |
|---|---|
| [Consensus Spec](./specs/consensus-spec.md) | consensus state machine、安全規則、liveness の前提、evidence surface |
| [Finality Proof Format](./specs/finality-proof-format.md) | full node と light client 向けの proof field と verifier rule |
| [Networking Spec](./specs/networking-spec.md) | transport、handshake、peer scoring、backoff、DoS 防御の期待値 |
| [Storage Schema](./specs/storage-schema.md) | durable record、index、recovery rule、snapshot、schema migration の期待値 |
| [Transaction Format](./specs/tx-format.md) | 標準 transaction payload、signed envelope、nonce、fee、gas、CheckTx 要件 |
| [EVM and Native Accounting](./specs/evm-native-accounting.md) | native/EVM 共通の balance model、256-bit amount、fee 処理、compatibility boundary |
| [Validator Lifecycle](./specs/validator-lifecycle.md) | validator admission、rotation、evidence lifecycle、slashing、jailing、unbonding |

## SDK と拡張ガイド

| 文書 | 目的 |
|---|---|
| [App Module Guide](./sdk/app-module-guide.md) | custom application module と module CLI command の追加方法 |
| [Custom Crypto Backend](./sdk/custom-crypto-backend.md) | signing/finality backend と production BLS adapter metadata の追加方法 |
| [Custom Storage and Transport](./sdk/custom-storage-transport.md) | custom store あるいは peer transport の実装方法 |
| [RPC API Versioning](./sdk/rpc-api-versioning.md) | `/v1/*` の compatibility rule と endpoint stability の理解 |

## 運用と release

| 文書 | 目的 |
|---|---|
| [Node Initialization](./operators/node-initialization.md) | validator/archive node の初期化と、分割された subsystem config の管理 |
| [Adding a Validator](./operators/add-validator.md) | validator を追加し、height ごとの validator set 更新を確認する運用フロー |
| [Observability Guide](./operators/observability.md) | health、metric、log、alert threshold、初動対応 playbook |
| [起動ランブック](./release/launch-runbook.md) | release 運用フロー、halt 条件、monitoring、post-launch archive 要件 |
| [Release Pipeline](./release/release-pipeline.md) | build、sign、package、release artifact gate |
| [Cosmos/Tendermint Comparison Gate](./release/cosmos-comparison-gate.md) | Tendermint/Cosmos の成熟度を Vexo の release evidence に変換する基準 |
| [Version Compatibility Matrix](./release/version-compatibility.md) | binary、config、store、app、RPC、proof format の互換性期待 |

## security

| 文書 | 目的 |
|---|---|
| [Security Audit Readiness](./security/audit-readiness.md) | threat model、assumption、limitation、安全論証、必要な audit evidence |

## ローカライズ文書

locale file は canonical tree から逸脱してはいけません。command、JSON field、RPC 名、config key、code identifier を変えずに説明だけを翻訳するので、例はどの言語でもコピーして使えます。

| 文書 | 目的 |
|---|---|
| [Documentation Locales](./locales/README.md) | locale ディレクトリの対応表と翻訳ポリシー |
| [English Canonical Docs](./locales/en/README.md) | 規範となる英語文書ツリー |
| [Japanese Docs](./locales/ja/README.md) | 日本語 locale ツリー |
| [Korean Docs](./locales/ko/README.md) | 韓国語 locale ツリー |
| [Chinese Docs](./locales/zh/README.md) | 中国語 locale ツリー |
| [French Docs](./locales/fr/README.md) | フランス語 locale ツリー |
| [German Docs](./locales/de/README.md) | ドイツ語 locale ツリー |
| [Spanish Docs](./locales/es/README.md) | スペイン語 locale ツリー |
| [Portuguese Docs](./locales/pt/README.md) | ポルトガル語 locale ツリー |
| [Russian Docs](./locales/ru/README.md) | ロシア語 locale ツリー |
| [Arabic Docs](./locales/ar/README.md) | アラビア語 locale ツリー |
| [Hindi Docs](./locales/hi/README.md) | ヒンディー語 locale ツリー |
| [Indonesian Docs](./locales/id/README.md) | インドネシア語 locale ツリー |
| [Vietnamese Docs](./locales/vi/README.md) | ベトナム語 locale ツリー |

## 新しい文書を書くとき

文書は次の基準で書きます。

- 読者の目的と、そのページが支える決定を先に書く。
- その文書が normative spec、implementation guide、operator guide、release/audit checklist のどれかを明示する。
- 関連する command、package path、config key、RPC method、JSON field を含める。
- 安全境界、失敗モード、避けるべき shortcut を説明する。
- evidence がないまま production-ready と言わない。
- 例はできるだけそのまま使えるようにしつつ、変更が必要な値は明示する。
- すべての Markdown file を `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` に mirror する。
- `make docs-check` を通して locale tree が canonical tree からずれないようにする。

## production claim rule

機能があるだけでは production-ready とは呼びません。production claim には以下が必要です。

- implementation code
- unit/property/adversarial tests
- プロセスや machine をまたぐ機能なら operation または E2E evidence
- 仮定と failure mode の文書化
- BLS、VRF、Web3/EVM compatibility、slashing、state sync、upgrade、validator economics のような security-sensitive 領域では release-gate evidence

`vexod status --json` も同じ規則に従います。`features` map は config でその code path が有効かどうかを示し、`feature_assurance` map は、その機能が単なる実装なのか、operator artifact が必要なのか、release evidence が必要なのか、external audit が必要なのかを示します。

分離された config file には運用上の安全な初期値を置きます。node home を確認するときは、まず次を見ます。

- restart-safe P2P handshake replay protection 用の `network_config.json:p2p.auth_replay_path`
- peer-authentication key 用の `network_config.json:p2p.node_key_path`（validator consensus custody とは分離）
- proposal spam / economic-friction policy 用の `module_config.json:governance.RequireDeposit` と `module_config.json:governance.MinDeposit`
- execution/finality boundary 用の `consensus_config.json:consensus.execution_commit`
- restart-safe pending transaction recovery 用の `mempool_config.json:mempool.WALPath`

## 文書レビューのチェックリスト

文書変更を merge する前に、次を確認します。

- 英語文書が release / audit の正本として十分な精度を持っている。
- すべての locale file が正しい英語 canonical document を指している。
- command、RPC 名、config key、JSON field、package name はそのまま維持されている。
- `make docs-check` を実行した。
- command example、config schema、generated artifact が変わった場合は、より広い project check も実行した。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: How to Read This Set — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Start Here — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Protocol Specs — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: SDK and Extension Guides — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Operations and Release — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Security — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Localized Documentation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Writing New Docs — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Production Claim Rule — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Documentation Review Checklist — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `vexo-consensus` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `make docs-check` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod status --json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `feature_assurance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.auth_replay_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json:p2p.node_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json:governance.RequireDeposit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `module_config.json:governance.MinDeposit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_config.json:consensus.execution_commit` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `mempool_config.json:mempool.WALPath` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
