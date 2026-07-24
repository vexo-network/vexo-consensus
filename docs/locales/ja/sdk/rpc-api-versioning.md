# RPC API バージョン管理

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。

## 文書概要

この文書は RPC API バージョン管理、互換エイリアス、安定化ポリシーを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/ja/sdk/rpc-api-versioning.md`

## この文書を読む理由

- RPC API バージョン管理、互換エイリアス、安定化ポリシー
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

- `/v1`
- `/v1/healthz`
- `/v1/readyz`
- `/v1/status`
- `/v1/diagnostics`
- `/v1/metrics`
- `/v1/metrics/text`
- `/v1/peers`
- `/v1/tx`
- `/v1/evidence`
- `/v1/recovery`
- `/v1/snapshot/latest`
- `/v1/snapshot/export`
- `/v1/snapshot/chunk?index=0&size=10000`
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`

## 英語原文の構造

- RPC API Versioning
- 安定性の目標
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## 正規原文

- [英語の正規文書](../../en/sdk/rpc-api-versioning.md)

## RPC capability discovery

新しい RPC capability discovery インターフェースです。運用者は `/v1/capabilities` で実際に接続された provider 機能を確認し、SDK 側は `rpc.Config.RequiredCapabilities` または `rpc.Config.RequireAllCapabilities` で起動時に fail closed にできます。

次のインターフェイス名は変更しないでください: `/v1/capabilities`, `CapabilityResponse`, `CapabilitySnapshot`, `RequiredCapabilities`, `RequireAllCapabilities`, `metrics`, `blocks`, `finality`, `strict_replay`, `consensus_control`.

## マイニング済み EVM オブジェクト契約

マイニング済み transaction response の `gas` は送信時の gas limit を保持し、実使用量は receipt の `gasUsed` に置きます。適用可能な `v`、`r`、`s`、`yParity` と non-null block location を返し、receipt log も block、transaction、log index を含めます。ethers と Remix は receipt 成功後にこれらを再解析するため、表示用ではなく互換性契約です。`eth_getBlockByNumber` は genesis `0x0` をサポートし、parent hash に不正な `null` ではなく zero hash を使います。

<!-- vexo-docs:technical-parity -->
- `admin_token` and `admin_tokens` are stable configuration keys and must remain unchanged when describing optional bearer-token enforcement.

## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Stability Goal — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Current Stable API — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Versioning Rules — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Capability Discovery — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Compatibility Aliases — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Error Format — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Query Proofs — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Event Queries — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: IBC Queries — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Web3 JSON-RPC Bridge — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Web3 EVM Configuration — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Operational Compatibility — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `/v1` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/healthz` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/readyz` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/status` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/diagnostics` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/capabilities` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/metrics` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/metrics/text` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/peers` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/tx` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/evidence` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/recovery` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/snapshot/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/snapshot/export` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/snapshot/chunk?index=0&size=10000` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/blocks` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/blocks/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/blocks/{height}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/state/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/state/{height}/{namespace}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/events?key={attribute_key}&value={attribute_value}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/proof?namespace={namespace}&key={key}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/proof?namespace={namespace}&key={key}&height=latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/finality/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/finality/{height}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/ibc/client/{client_id}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/ibc/connection/{connection_id}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/ibc/channel/{port_id}/{channel_id}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/validators/{height}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/committee/{height}/{round}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/prune` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/replay` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/consensus/start` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/consensus/stop` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `network_config.json` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `tls_cert_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `tls_key_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `tls_ca_path` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `tls_server_name` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexod start` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `strict: true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_gasPrice` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `vexo_web3Capabilities` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `require_network_safety` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.NewNetworkSafeServer` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.NewNetworkSafeHandlerWithConfig` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.Config.RequiredCapabilities` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc.Config.RequireAllCapabilities` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `pending_txs` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `state_by_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `app_query` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `strict_replay` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `consensus_control` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/status` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/tx` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/blocks/latest` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v2/*` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/proof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `commit_chain` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/status.latest_height` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `/v1/events` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `Index: true` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `{ "path": [...], "value": ... }` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `packets/{source_port}/{source_channel}/{sequence}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `rpc_modules` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_clientVersion` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `web3_sha3` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_accounts` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_coinbase` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `net_version` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `net_listening` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `net_peerCount` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_chainId` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_protocolVersion` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_syncing` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_mining` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_hashrate` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_blockNumber` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_blobBaseFee` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_maxPriorityFeePerGas` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_feeHistory` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
