# EVM 更新ガイド

> Locale: ja · 日本語
> この文書は英語原文の日本語訳です。プロトコル、セキュリティ、リリース判断は英語原文が規範です。

このガイドは、chain ID の扱い、Web3 互換性、リリース証跡を壊さずに内蔵 EVM スタックを更新する方法を説明します。go-ethereum の更新、fork preset の調整、EVM 挙動の制御された変更を行う運用者と保守担当者向けです。

## EVM 更新に当たる変更

次のいずれかが変わるなら、単なるリファクタではなくリリースに敏感な機能更新として扱ってください。

- `modules/evm/backend/geth` における `go-ethereum` のバージョン更新
- `modules/evm/ethcompat` の変更
- `modules/evm` の変更
- `execution.evm_fork_preset` の変更
- `execution.evm_chain_config_json` の変更
- raw transaction の受付、gas accounting、receipts、traces、proofs、block response fields の変更
- `eth_accounts`、`eth_coinbase`、`eth_sign`、`eth_signTransaction`、`eth_sendTransaction` のような managed Web3 account の扱いの変更

## 安全な更新順序

コード、設定、文書がずれないよう、次の順序で進めます。

1. まず geth-backed adapter を独立して更新します。
2. 次に fixture corpus と conformance tests を更新します。
3. 意味が変わるなら `docs/specs/evm-native-accounting.md`、`docs/specs/tx-format.md`、`docs/sdk/rpc-api-versioning.md` を更新します。
4. release evidence の形が変わるなら `docs/release/release-pipeline.md` を更新します。
5. オペレータ向けの設定つまみが変わるなら node configuration docs を更新します。
6. マージ前に validation matrix を再実行します。

EVM runtime version を上げた直後にそのまま出荷してはいけません。conformance suites、RPC smoke checks、Docker deployment checks がすべて通る必要があります。

## 更新フロー

### 1. 変更範囲を固定する

更新の目的を正確に記録します。

- fork behavior のみ
- transaction admission のみ
- execution semantics のみ
- RPC compatibility のみ
- blob / receipt / trace handling のみ
- managed account または wallet behavior のみ

この分け方により review が集中し、無関係なコードが一緒に動くのを防げます。

### 2. 最も狭い層で修正する

次の境界を優先します。

- `modules/evm/backend/geth` は upstream go-ethereum integration changes
- `modules/evm/ethcompat` は raw transaction decoding、hash preservation、fixture handling
- `modules/evm` は state transition、receipts、logs、storage、snapshot behavior
- `rpc` は Web3 request/response surface changes
- `cmd/vexod` は CLI か release workflow が新しい挙動を公開する必要がある場合のみ

変更が application modules に届くなら、module boundary を明示し、deterministic state writes を維持してください。

### 3. デフォルト設定を更新する

意味が変わるなら、同じパッチでデフォルト config も更新します。

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- 必要に応じて `network_config.json` の managed account RPC fields
- `module_config.json` の EVM chain ID

隠れた CLI flag で runtime behavior を説明しようとしないでください。config ファイルだけで node の動作が分かるべきです。

### 4. conformance stack を走らせる

最低限、次を実行します。

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

その後、利用者が最初に壊しやすい経路を確認します。

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Docker single-host なら、次の URL も確認します。

```text
http://127.0.0.1:28657/web3
```

少なくとも次の挙動を確認してください。

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

その後、simple contract deploy、proxy contract deploy、UUPS upgrade path を本番で使うのと同じ RPC endpoint で試します。

### 5. proxy と upgrade の挙動を確認する

次の条件がすべて満たされて初めて更新完了です。

- 通常の contract deploy に成功する
- proxy deploy に成功する
- UUPS upgrade 呼び出しに成功する
- upgrade 後の storage と code が期待どおり読める
- nonce tracking が単調増加のままである
- block producer が unsafe proposal エラーなしでトランザクションを受け入れる

proxy deploy は成功するのに upgrade が失敗するなら、まだ出荷できません。これは warning ではなく release blocker です。

### 6. 証跡を更新する

EVM surface が変わったら release evidence bundle も更新します。

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- 固定済みの SHA-256 fixture reference

release evidence には、何が変わったか、何をテストしたか、どの commit または version を検証したかを必ず書いてください。実際に実行したコードと証跡が一致しないなら、EVM 更新が完了したとは言えません。

## 検証マトリクス

これを merge gate として使います。

| Check | 重要な理由 |
| --- | --- |
| `make evm-conformance` | fork rule と execution regression を検出する |
| `go test ./modules/evm -count=1` | receipts、logs、storage、balances、snapshots を検証する |
| `go test ./rpc -count=1` | Web3 request/response 互換性を検証する |
| `make network-e2e` | node が起動し、peer を張り、commit できるか確認する |
| Docker single-host smoke | Remix と browser tools が使う経路を確認する |
| Contract deploy | transaction admission と receipt generation を確認する |
| Proxy deploy | ABI と storage layout の前提を確認する |
| UUPS upgrade | upgrade semantics と upgrade 後の read を確認する |

どれか 1 つでも赤なら、更新完了とは言わないでください。

## ロールバック条件

次のいずれかが起きたら EVM 更新を戻します。

- `eth_chainId` が予期せず変わる
- `eth_sendRawTransaction` が有効な transaction を拒否し始める
- `eth_call` または `eth_estimateGas` が想定 fork rules から外れる
- receipts、logs、proofs が committed state と一致しなくなる
- proxy または upgrade transaction が失敗し始める
- release evidence が現在の code path と合わなくなる

rollback では、最後に正常確認できた adapter version、config default、fixture set をまとめて戻します。

## 技術的整合性付録

この付録は、更新ガイドを他の文書と同じ基準にそろえるためのものです。

- `modules/evm/backend/geth`、`modules/evm/ethcompat`、`modules/evm`、`rpc`、`cmd/vexod` は安定した実装境界として維持します。
- `execution.evm_fork_preset`、`execution.evm_chain_config_json`、`execution.allow_unprotected_legacy_tx`、`eth_chainId`、`eth_call`、`eth_estimateGas`、`eth_sendRawTransaction`、`eth_getTransactionReceipt`、`eth_getProof`、`eth_getStorageAt`、`eth_accounts`、`eth_coinbase`、`eth_signTransaction`、`eth_sendTransaction` の綴りは変更しません。
- `make evm-conformance`、`make network-e2e`、`--evm-default-fixtures`、`--evm-tx-fixtures`、`--evm-execution-fixtures`、`--evm-web3-conformance-evidence` の綴りもそのまま維持します。
- 運用上の問いは単純です。この更新は Ethereum-style execution を保ちながら、Vexo consensus と release safety に適合しているか?

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
