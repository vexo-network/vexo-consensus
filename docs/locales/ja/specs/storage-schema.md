# ストレージスキーマ

> Locale: ja · 日本語
> この文書は英語原文の日本語への直接翻訳です。プロトコル、セキュリティ、リリース判断は英語原文を規範とします。


## 最初に読む順序

この文書は Storage Schema の規範仕様を説明します。初めて読むなら、次の順で進めると分かりやすいです。

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

この順番は、まず範囲と状態を理解し、次にメッセージ、safety、liveness の規則を確認し、最後に evidence を読む流れです。

## 文書概要

この文書は durable storage namespace、key schema、recovery markerを理解し、実装・運用判断へつなげるためのものです。

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/ja/specs/storage-schema.md`

## この文書を読む理由

- durable storage namespace、key schema、recovery marker
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

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## 英語原文の構造

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## 正規原文

- [英語の正規文書](../../en/specs/storage-schema.md)

## Pebble backend とバイナリレコード

既定の backend は Pebble(`github.com/cockroachdb/pebble/v2`)で、256 MiB の block cache と 64 MiB の memtable で開きます。レコードは `types` パッケージの canonical バイナリエンコーディングで保存されます: 最小の uvarint、長さを前置したバイト列とリスト、32 バイトそのままのハッシュ、`0` または `1` の presence byte に続く任意の値、宣言した構造体の順のフィールド。デコーダは末尾の余分なバイトを含めそれ以外をすべて拒否するため、壊れたレコードは別の値として読まれずエラーとして報告され、レコードのエンコーディングは一つだけです。以前のビルド(LevelDB、JSON レコード)が書いたデータディレクトリは読めず、変換もされません。Pebble が `format major version 1` のエラーで開くことを拒否するので、そのノードは snapshot から復元するか空のデータディレクトリに同期してください。

関連する識別子:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## State history(undo log)と tree cache

state history は key ごとのバージョンの undo log です。key がある高さで変わると(ブロックの書き込みはその高さ、最初の committed state 以降の直接書き込み(`Set`、`Delete`、`SetBatch`)は次の committed の高さ)、store は key がそれ以前に持っていた値(不在を含む)をその高さのバージョンとして、key と高さごとに一度記録します。その高さを commit するブロックより前の直接書き込みは、最初のものが記録したバージョンを保ちます(`hdirect` がその高さを示します)。committed の高さでの key の値は、その高さより上の最も低いバージョンが記録した値で、そのようなバージョンがなければ現在の値です。変更の記録は key の現在の値だけを読み history は読まず、過去の読み出しは key のバージョンへの seek 一回です。`PruneBelow` は保持開始の高さ以下のバージョンをすべて削除します。tree node も同じ方法でバージョンを残します。二つのメモリ上の構造がブロックの tree 処理を減らし、どちらも永続化されず結果を変えません: 現在の tree のエンコード済み node を最大 2^18 個保持する tree node cache(store が commit するすべての batch が更新し、batch の commit 中は奇数になる generation により、reader は自分の snapshot の状態でのみ cache を使います)と、namespace ごとに最後の root 計算の root と node 変更をその generation と namespace の書き込みの SHA-256 digest で保持し、app hash、state root、commit が同じ書き込みを一度だけ計算するようにする構造です。

## Mempool WAL の compaction と genesis batch

commit は mempool WAL が 4 MiB 以上で、かつ最後の compaction(または open)直後の大きさの二倍以上になったときだけ log を compaction します。そのため append されたバイトは平均でおよそ一度だけ書き直され、再起動時に replay する log は live な内容の二倍か 4 MiB 以内に収まります。明示的な compaction(`Node.Compact`、または起動時に application が拒否した replay tx を捨てる場合)は常に書き直します。`InitChain` はすべての module の genesis state を一つの batch で書くため、アカウントの多い genesis でも durable write は一回で済み、失敗した genesis は store に何も残しません。

## Commit batch 内の finality proof と遅延した mempool 除去

node の commit 経路は、block の QC が完成させる finality proof を block/state/app-write batch に一緒に書きます(`CommitBlockStateWithWrites` の最後の引数)。そのため commit は proof ごとに durable write を一つ増やさず、一回で済みます。store が拒否する proof は batch から外され、commit 後の処理が保存して失敗を報告します。commit は自分の tx を pool から除去し、除去 record を sync を待たずに queue に入れます。次の commit は自分の block を実行する前に、その record が durable になるまで待ちます。再起動は最後に commit された block の除去だけをやり直すからです。除去 record は通常、後続の admission の sync に相乗りします。

## State tree の遅延ハッシュ

state tree の計算はハッシュを後回しにします。write は自分が変えた側だけを印し、root、proof、変更分を求められたときに、変わった node を下から上へ一度ずつハッシュします。そのためブロックのすべての write が書き直す root 付近の node は、write ごとではなくブロックごとに一度だけハッシュされます。ハッシュ、node、root は write ごとにハッシュする場合と同じです。

<!-- vexo-docs:technical-parity -->
## 技術的同等性付録

この付録は、英語正本にある実行可能なインターフェイスと主要セクションを翻訳版でも漏らさないための検証用要約です。コマンド、設定キー、RPC メソッド、パッケージ名は全言語でそのまま維持します。

### セクション追跡
- section: Scope — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Backend — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Records — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Indexes — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: EVM Records — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Recovery Rules — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Snapshot Validation — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。
- section: Schema Migration — このセクションでは、設定値、検証証拠、失敗条件、運用者が取るべき対応をまとめて確認します。

### そのまま維持するインターフェイス
- `store.Store` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getBalance` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `eth_getProof` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `bank/{0x_address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `auth/nonce/{0x_address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm/code/{0x_address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm/storage/{0x_address}/{slot}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `seen_ttl` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `code/{address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `storage/{address}/{slot}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `receipts/{tx_hash}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `logs/by_height/{height}/{tx_hash}/{log_index}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `logs/{address}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `evm_receipts` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `receipt_index/{tx_hash}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blob_sidecars/by_tx/{tx_hash}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `blob_sidecars/by_blob/{blob_hash}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `tree:` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `receipt_height/{height}/{tx_hash}` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
- `receipt_pruned_below` — この名前は実行例と設定検証でそのまま使うため翻訳しません。
