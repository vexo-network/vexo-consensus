# モジュール型 Proof-of-Stake ネットワーク向け適応型リカバリーゲート HotStuff

> Locale: ja · 日本語  
> 文書種別: 研究論文ドラフトおよび再現プロトコル  
> 状態: 現在の実装に基づくドラフトであり、性能上の主張には測定済み成果物が必要です。

## 要旨

本稿は、モジュール型 Proof-of-Stake ネットワークを対象とした HotStuff 系 BFT 状態機械複製を検討します。実装は three-chain finality と高さごとに版管理された validator set に、三つの運用機構を組み合わせます。第一は、proposal、vote、commit の p95 処理時間と active peer の状態に基づき、round timeout を上限付きで増減する適応制御です。第二は、永続化された block 履歴と application state 履歴が一致しない場合、安全な回復高さを超える finalized commit を保留する recovery finality gate です。第三は、同一の transaction 集合に対して mempool 到着順序に依存しない結果を作り、同一 signer の nonce 依存関係を維持する決定的順序付けです。

PoS、BFT、HotStuff、適応型 view synchronization、または fair ordering そのものを新規発明として主張するものではありません。研究対象は、この限定された制御・回復・順序付けの合成が、基礎となる HotStuff の安全規則を変更せずに不要な timeout と回復時の不整合を減らせるかどうかです。実装済みの事実、検証対象の仮説、実験を必要とする主張を明確に分離し、固定した binary、config、topology、workload による反復測定が完了するまで throughput や latency の改善値を結果として提示しません。

## 研究課題と仮説

RQ1 は可変遅延下で adaptive policy が fixed timeout より timeout 回数と p95 commit latency を減らすかを問います。RQ2 は storage/restart fault の注入時に recovery gate が整合可能な durable height より先へ application state を進めないかを検証します。RQ3 は同一 transaction 集合の全ての入力順列から同じ proposal order が得られ、signer ごとの nonce が単調増加するかを調べます。RQ4 は安定した正常系で追加される CPU、memory、network、latency cost を測定します。

H1 から H4 は反証可能な方向仮説です。実装が存在することだけでは仮説の成立を意味しません。利点が統計的または運用上有意でなければ、その否定的結果や適用限界も報告対象です。

## 先行研究と新規性の境界

HotStuff は partial synchrony の下で leader-based BFT、quorum certificate、chained commit、正常経路の線形通信と responsiveness を示しました。LibraBFT/DiemBFT と AptosBFT は、HotStuff 派生 BFT と stake-weighted validator governance の実用的な組合せを示しています。Jolteon と Ditto は低遅延および network-adaptive BFT と asynchronous fallback を扱い、Fever は responsive view synchronization を扱います。Tendermint は別系統の round-based PoS BFT です。Narwhal/Tusk は transaction dissemination と ordering を分離し、Aequitas、Wendy、Themis は本実装の hash ordering より強い order-fairness を定義します。

したがって、「PoS+BFT の最初の blockchain」「HotStuff を使う最初の PoS network」「AptosBFT と同一」「証明なしの asynchronous liveness または最適通信量」「完全な MEV 防止」「single-host test による production-ready」といった表現は禁止します。候補となる systems contribution は、bounded feedback controller、local durable-history commit gate、nonce-aware deterministic ordering を Go の modular PoS node に統合し、固定 timeout と gate-disabled ablation に対して再現可能に評価することです。

## システムモデル

高さ h の active validator set を Vh、総 voting power を Ph とします。既知かつ重複しない signer の power が Ph の三分の二以上であるとき QC が有効です。validator set と hash は高さごとに版管理されます。admission は minimum stake による permissionless 方式、validator 数上限、または制限方式を選べます。これは Sybil resistance と governance の層であり、BFT の fault threshold を変更しません。

network は partially synchronous です。Byzantine voting power が三分の一未満で、signature、validator-set binding、durable store の前提が保たれるとき safety を期待します。liveness には最終的に bounded delay、honest quorum、利用可能な signer、十分な peer connectivity が必要です。永続的な asynchronous network で進行を保証しません。

EVM は Vexo consensus 配下の application workload です。Ethereum bytecode と `/web3` tooling の互換性を提供しますが、Ethereum fork choice や devp2p consensus を提供するものではありません。

## プロトコルと制御

基礎安全規則は `locked_qc` と `high_qc` を追跡します。proposal は lock を拡張するか、lock 以上に新しい justify QC を持つ場合のみ安全です。同一 height/round で異なる block に投票してはなりません。高さと hash が連続して束縛された三つの certified link が grandparent を finalize します。adaptive controller はこの predicate、quorum threshold、QC verification、commit rule を変更しません。

適応 timeout は base budget T0、current budget Tt、proposal/vote/commit p95 latency の合計、および peer deficit を使います。timeout 後は current を 1.5 倍方向へ、progress 後は 0.8 倍方向へ動かし、観測 latency の三倍を候補 floor とします。最終値は T0 から 8×T0 に clamp され、active peer がゼロなら peer floor は 2×T0 です。pending work のない idle 時間や local execution/storage error は round timeout として扱いません。この方式は運用 controller であり、理論的に最適な pacemaker とは主張しません。

recovery gate は durable state height Hs と block-index height Hb が存在するとき Hsafe=min(Hs,Hb) を計算します。二つが異なる間、Hsafe を超える finalized application commit を保留します。これは local persistence restriction であり、追加の network vote phase ではありません。

deterministic ordering は chain ID と height から salt を作り、signer/nonce metadata を持つ transaction を signer chain にまとめます。各 chain 内は nonce 昇順で、chain head は salted transaction hash により決定的に merge されます。これは arrival-order dependence を除きますが、first-seen fairness、censorship resistance、confidentiality、strong order fairness を証明しません。proposer は候補集合への inclusion に影響できます。

現在の consensus vote path は full height-versioned validator set と deterministic proposer を使用します。ECVRF committee selector は component と query にありますが、quorum formation または proposal eligibility には接続されていません。したがって VRF committee consensus は今後の課題です。

## 実装対応と実験方法

state-machine safety と locked QC、three-chain height/hash binding、adaptive controller、recovery gate、fair ordering、validator registry、staking、vote WAL、EVM workload の各実装位置は英語原文の mapping table に固定されています。ファイル移動や意味変更があれば paper revision と source revision を同時に更新します。

比較 treatment は同一 binary と application config を使い、fixed adaptive-off/gate-on、adaptive-on/gate-on、隔離環境だけで使う adaptive-on/gate-off の三条件です。4、7、16、31 validator を可能な範囲で使用し、single-host は smoke test に限定します。10/50/100/250 ms latency、step delay、jitter、0/1/5/10% loss、proposer restart、minority partition/heal、signer delay、durable history mismatch を含めます。

workload は native transfer、EVM transfer、contract creation、event log、proxy deployment、UUPS upgrade を含みます。全 validator の app hash と finalized block hash が一致し、receipt と block location が整合し、deployed code と upgrade 後 state が確認できた run だけを性能分析に使います。

収集する metric は committed/finalized height、proposal/vote/commit p50/p95/p99、end-to-end finality latency、round timeout、round distribution、adaptive timeout、peer count、recovery deferral、throughput、gas、CPU、RSS、disk/network bytes、proposal rejection、double-sign、invalid nonce です。各条件は warm-up 後に原則 30 回以上反復し、順序と seed を記録します。median、IQR、p95、confidence interval、effect size を報告し、最良 run のみを選びません。

## 正しさ、再現性、倫理

adaptive policy は timeout vote を試みる時刻だけを変え、安全な vote や QC の定義を変えません。recovery gate は commit を制限するだけで、base rule が拒否した commit を許可できません。deterministic ordering は execution input の一致に寄与しますが、conflicting finality を防ぐ proof の代替ではありません。

publication-quality proof では stake-weighted quorum intersection、lock monotonicity、一つの height における finalized block の一意性、validator-set transition、vote WAL crash recovery、controller/gate の safety-neutral 性質を形式化する必要があります。test と adversarial simulation は evidence ですが formal proof や external audit そのものではありません。

各 experiment では commit、dirty-tree、Go/OS/CPU/memory/container、topology、genesis、split config、binary SHA-256、workload seed、raw JSON/JSONL/CSV、validator log、final app hash、analysis script、failed-run ledger を保存します。known mechanism の改名を発明として主張せず、失敗 run と limitation を残し、hypothesis、observation、interpretation を分けます。AI assistance は venue policy に従って開示し、著者が全ての claim、citation、experiment、proof に責任を負います。fault injection は所有または許可された isolated system だけで行い、key、token、participant data、production endpoint を公開しません。

提出には source と原稿の一致、記録された prior-art search、再現可能な baseline、multi-host fault measurement、全 figure を再生成できる raw data と script、negative result、適切な proof、external methodology review が必要です。それ以前は「実装に基づく研究ドラフト」であり、「新しく証明された consensus」ではありません。

<!-- vexo-docs:technical-parity -->

## 技術的同等性の付録

次の実装名と検証名は翻訳しません。

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
