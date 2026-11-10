# Vexo Lanes: Certified Batch Dissemination for a Stake-Weighted Two-Chain HotStuff

> Document type: research manuscript and reproducibility protocol  
> Status: implementation-grounded draft; the results below were measured on one host with an emulated link and must be repeated on a real network before publication  
> Canonical implementation: the repository revision recorded with each experiment

## Abstract

A leader-based BFT protocol in the HotStuff family is bounded by its leader's uplink: every block's transactions leave one machine once per validator, so throughput falls as `1/(n−1)` while the other validators' links idle. Vexo Lanes moves dissemination off the leader without replacing the ordering protocol. Every validator seals the transactions it owns into signed batches on its own lane; validators holding two thirds of the stake acknowledge storing a batch, which makes it certified available; a leader then proposes a block by naming certified batches. The block format, its hash, its transaction root, the signed bytes, the voting rule and the two-chain commit rule are unchanged: a validator rebuilds exactly the block the header commits to, or fetches it whole. The design adds sender-to-lane ownership with a censorship fallback, authentication of a proposal before any batch is fetched, per-lane storage shares, and a stake-proportional view-only leader schedule.

On one host with an emulated 100 Mbit/s, 25 ms link per validator, Lanes commits 60,080, 45,323 and 37,099 tx/s with 4, 7 and 10 validators, against 21,073, 10,909 and 8,297 tx/s for the same node broadcasting whole blocks, a speedup of 2.9×, 4.2× and 4.5× that grows with `n`. The busiest validator's upload falls from 335–726 to 82–218 bytes per transaction. Below capacity (4,000 tx/s offered) Lanes adds 35 ms of median latency with 4 validators and removes 25 ms and 130 ms with 7 and 10, where the baseline's p99 reaches 0.8 s and 1.7 s. When the link is not the bottleneck (unshaped, 3 validators) Lanes costs 4% throughput. Building the evaluation also exposed nine defects, among them a leader schedule under which only even-indexed validators of an even-sized set ever proposed a block, and a proposal-judging race that collapsed throughput to 2,280 tx/s in one of four 10-validator runs.

## 1. Research Question

The primary question is:

> Can certified batch dissemination be added beneath an existing stake-weighted HotStuff-style protocol, without changing what validators vote on or how blocks commit, so that throughput scales with the aggregate uplink of the validators instead of the uplink of the leader?

- **RQ1, leader bottleneck:** How much of the baseline's throughput limit is the leader's upload, and how much of it does Lanes remove?
- **RQ2, scaling:** How do both designs behave as the validator count grows at a fixed per-validator link?
- **RQ3, preservation:** Are safety, the block a validator judges, nonce order and censorship resistance preserved?
- **RQ4, cost:** What do certification and indirection cost in latency and CPU when bandwidth is plentiful?

Hypotheses, each falsifiable by the measurements of Section 8:

- H1: the baseline's busiest upload per transaction grows linearly with `n` and Lanes' stays near the transaction size.
- H2: the Lanes speedup over the baseline grows with `n` on a bandwidth-limited link.
- H3: on an unconstrained link Lanes is not faster, and costs at most one extra round trip.

## 2. Prior Art and Novelty Boundary

Separating dissemination from ordering is established. Narwhal and Tusk build a certified DAG of batches and order it with a DAG or HotStuff consensus; Bullshark and Shoal order the DAG itself. Stratus adds shared-mempool batches with availability certificates beneath HotStuff. Mir-BFT and ISS run multiple leaders over partitioned request buckets. Autobahn decouples data lanes from a partially synchronous agreement core. Aptos's Quorum Store deploys certified batches beneath its HotStuff-derived consensus.

This manuscript must not claim:

- that decoupling dissemination from ordering, availability certificates, or per-validator lanes are new;
- a DAG protocol, asynchronous liveness, or optimal communication complexity;
- results on a real wide-area network: every number here is from one host with an emulated link;
- that the leader schedule is a new idea in isolation: deterministic stake-weighted rotation is standard.

The contribution is narrower and engineering-centred:

1. A retrofit that keeps the existing block, header, signature and commit rule bit-for-bit, so the light client, evidence, finality proofs and state sync are untouched. A lane proposal is only a transport encoding of the signed block.
2. Sender ownership of lanes, which keeps one sender's nonce sequence on one lane, with a holder fallback that prevents a faulty owner from censoring.
3. Authentication of the proposal's leader and signature before any batch is fetched, so a forged proposal cannot make a validator pull data from the network.
4. A reproducible measurement harness with an explicit link model, the measured matrix, and the base-node defects the evaluation exposed (Section 9).

## 3. System Model

- `n` validators with stake-weighted voting power; Byzantine validators hold less than one third of the total power. Quorums are `>= 2/3` of the power.
- Partial synchrony: after an unknown global stabilisation time, messages between honest validators arrive within a known bound.
- Authenticated point-to-point channels and a validator set known per height; signatures are domain separated.
- Validators may crash, equivocate, withhold batches, flood their own lane, or send forged proposals and batches.
- The application judges transactions deterministically; a leader may exclude a transaction its application check refuses.

## 4. Protocol

### 4.1 Lanes and Batches

Every validator authors one lane. It seals the transactions it owns into a `lanes.Batch` — chain, author, sequence number, transactions — once they reach 512 KiB or 5 ms after the first of them, bounded by `lanes.MaxBatchTxs` transactions and `lanes.MaxBatchBytes` bytes. A lightly loaded lane, whose previous batch was below 16 KiB, also waits for that batch's certificate, at most 50 ms after sealing it, so a batch gathers what arrives in one certification round trip: every batch costs a signature, an acknowledgement per validator and a certificate verification at every validator, and without this pacing a lane at 400 tx/s seals two-transaction batches. A loaded lane does not wait, so its batches pipeline. The batch id hashes all of these fields, the author signs it, and sends it to every validator. Batches stand alone: there is no chain of batches and no DAG.

### 4.2 Availability Certificates

A validator that stores a well-formed batch signs an acknowledgement of its id in domain `DomainLaneBatchAck`. The author aggregates acknowledgements of `>= 2/3` of the power into a `BatchCert` and announces it. Because Byzantine validators hold less than one third, the certifiers that are honest hold more than one third of the power, so a certified batch is stored by at least one honest validator that will serve it. The author retransmits a batch to validators that did not acknowledge it with exponential backoff (1 s doubled, at most four times), so a congested link is not flooded with copies of what it is still carrying.

### 4.3 Ownership and Fallback

A transaction belongs to the lane `lanes.Owner(members, sender)`, a hash of its sender over the validators sorted by id. All transactions of one sender travel on one lane in the order its author admits them, and a leader proposes the batches of a lane in sequence order, so lanes never reorder a nonce sequence. A validator that receives a transaction another lane owns holds it; if no stored batch carries it after `lanes.FallbackDelay` (3 s), it batches the transaction on its own lane. A faulty owner can therefore delay a sender by the fallback delay but not censor it.

### 4.4 Lane Proposals

A leader takes certified batches only, per lane in sequence order up to the first uncertified batch, the lanes taking turns, up to the block byte limit. It runs the usual application check and deterministic ordering, signs the block as before, and broadcasts a `lane_proposal`: the signed proposal without its transactions, the batch ids, the increasing positions of batch transactions the application check refused (`Excluded`), and the transactions no batch carries (`Extra`). The leader's upload per validator is a few bytes per batch, independent of block size.

### 4.5 Rebuild and Authentication Before Fetch

A receiving validator first judges the proposal's leader and signature against the header hash. A forged proposal is rejected and counts against its sender before anything is fetched. A proposal whose validator set depends on a block the validator has not executed yet — under the two-chain rule the proposal that finalizes a block often arrives before its receiver executed that block — is kept (at most 16) and judged again as soon as the validator executed a block or accepted a proposal, without fetching anything for it. An authentic proposal is rebuilt from stored batches; a missing batch is requested from its author and then from other certifiers. The rebuilt block is judged only when its hash and transaction root equal the signed header; otherwise, or when the validator cannot yet judge the proposal (its validator set is pending), it sends the sender a `proposal_request` and waits for the whole `proposal`, which a node serves at most twice per peer. The bytes a validator judges are always the bytes the leader signed.

### 4.6 Stake-Proportional Leader Schedule

The leader of view `v` is `validator.SelectProposer(validators, v)`. Validators sorted by id own `VotingPower` consecutive slots of `[0, W)`; view `v` maps to slot `v·S mod W`, where the stride `S` is the first integer from `ceil(W·(φ−1))` coprime to `W`. The views `0..W−1` visit every slot exactly once, so every window of `W` views gives each validator exactly its power in views, and the golden-ratio stride spreads one validator's views apart. The schedule depends on the view only.

### 4.7 Resource Bounds

- Each author's unconsumed batches are capped at 64 MiB of a 256 MiB store, so flooding a lane fills the flooder's share only.
- Unconsumed batches expire after 30 s; consumed ones are served 5 s longer to validators that catch up.
- A node serves one batch to one peer at most twice; lane requests name at most 256 ids; a lane proposal names at most `consensus.MaxLaneRefs` batches; a node keeps at most 16 proposals it cannot judge yet.
- The wire format is a versioned binary encoding that refuses truncated or trailing bytes.

## 5. Correctness Argument

- **Safety.** The vote rule, quorum, lock and commit rule are unchanged, and a validator votes only for a block whose rebuilt hash and transaction root equal the header the leader signed. Lanes can delay a vote but cannot change its subject, so safety reduces to that of the base two-chain protocol.
- **Availability.** A leader names certified batches only; each is held by at least one honest validator, which serves it. If a fetch fails anyway, the whole proposal is fetched from its sender, which holds it.
- **Liveness.** After stabilisation honest authors certify their batches within a bounded time and honest leaders propose them; the base pacemaker handles faulty leaders. A validator that receives a proposal before executing the block its validator set depends on judges it right after that execution, so falling behind by one block does not cost the views it leads. A crashed validator costs its views and its lane: other validators batch its owned transactions after the fallback delay (tested with one of four validators down).
- **Order and censorship.** One sender's transactions share a lane and are proposed in lane order; the fallback bounds an owner's ability to censor; the leader schedule gives each validator exactly its stake share of views.
- **Resource exhaustion.** Forged proposals trigger no fetch; lane shares, expiry and serve limits bound what a Byzantine validator can make others store or upload.

This is an argument, not a proof. A mechanised proof of the composition is future work.

## 6. Implementation Mapping

- `lanes/batch.go`: batches, acknowledgements, certificates, `VerifyBatch`, `VerifyCert`.
- `lanes/lanes.go`: sealing, dissemination, retransmission, storage shares, candidates, fallback.
- `lanes/wire.go`: binary lane messages.
- `consensus/lane_proposal.go`: `consensus.LaneProposal`, rebuild, `consensus.MaxLaneRefs`.
- `consensus/transport_reactor.go`: authentication before fetch, batch fetch, deferred judging, whole-proposal fallback.
- `validator/proposer.go`: the leader schedule.
- `node/lanes.go`: node wiring, ownership routing and lane candidates for proposals.
- `node/throughput_test.go`, `node/shaped_transport_test.go`: the measurement harness and link model.

## 7. Experimental Method

### 7.1 Treatments

- **Lanes:** the default node.
- **Baseline:** the same binary with `DisseminateByProposal`: a validator gossips the transactions submitted to it and the leader broadcasts each block whole. Everything else — consensus, application, mempool, ordering, execution — is shared.

### 7.2 Link Model

All validators run in one process on an Apple M4. Each validator's upload is shaped by a virtual link with a fixed rate and one-way latency; consensus, commit and evidence messages are queued before bulk transaction and batch traffic, so that the baseline's proposals are not delayed behind gossip. The model has no loss, jitter or cross traffic.

### 7.3 Workload and Metrics

Clients submit unsequenced 89-byte transactions round-robin to all validators. The throughput runs submit 150,000 transactions at once (300,000 in the unshaped run); throughput is committed transactions divided by the time from the first submission to the last commit. The latency runs offer 20,000 transactions open loop at 4,000 tx/s (`VEXO_RATE`), below both treatments' capacity; latency runs from a transaction's acceptance to the first validator observing its block committed, and is reported as p50/p95/p99. At saturation that latency only measures the backlog, so it is not reported for the throughput runs. Upload is the bytes each validator's link carried per committed transaction, reported for the busiest validator, with the proposal topic separately. Each cell was run three times.

The round timeout defaults to 1 s plus eight one-way latencies (1.2 s). The 10-validator throughput cells use 5 s for both treatments: with the default timeout the baseline makes no progress after height 6, because uploading a 1 MiB block to nine validators over a queueing link takes about as long as the timeout, so every view times out. The production configuration enables the adaptive timeout, which this harness disables to compare fixed treatments.

## 8. Results

### 8.1 Bandwidth-Limited Throughput (100 Mbit/s, 25 ms)

| Validators | Lanes tx/s, mean (min–max) | Baseline tx/s, mean (min–max) | Speedup | Lanes busiest upload B/tx | Baseline busiest upload B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

Every run committed all 150,000 transactions. Lanes had no round timeout; one baseline run at 7 validators had two. The baseline's proposal topic alone costs the leader `s·(n−1)` bytes per transaction for transaction size `s` — 266 B/tx at 4 validators and 580 B/tx at 7 — while Lanes' costs 0.2–0.4 B/tx. In two of three 7-validator runs some validators fetched whole proposals at the start of the run (53 B/tx): the burst seals 512 KiB batches, and the last third of the validators had not received them 300 ms after a leader proposed them. This supports H1 and H2.

### 8.2 Latency Below Capacity (4,000 tx/s offered)

| Validators | Lanes p50 / p99 | Baseline p50 / p99 | Lanes proposal B/tx | Baseline proposal B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

With 4 validators certification costs Lanes about 35 ms of median latency, one round trip. From 7 validators on, the baseline's leader uplink already delays proposals at this load: Lanes has the lower median and a tail two to three times shorter, and only Lanes kept up with the offered load at 10 validators (3,655–3,677 against 3,082–3,261 tx/s committed over the run). H3 holds for small sets only.

### 8.3 Unconstrained Link (3 validators)

| Treatment | tx/s, mean (min–max) | Busiest upload B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| Baseline | 114,522 (113,095–116,408) | 147.4–148.3 |

With bandwidth plentiful the baseline is 4% faster: Lanes' extra signatures and acknowledgements are pure cost, as H3 predicts. Lanes still uploads less than half the bytes.

### 8.4 Attribution

The baseline is bounded by the leader's link: its measured proposal bytes match `s·(n−1)` and its throughput falls with `n`. Lanes is not link-bound in this matrix — its busiest validator used well under its link — but host-bound: all validators share one machine's ten cores, so adding validators adds signature verification and execution on the same cores. The Lanes numbers are therefore lower bounds for a deployment with one machine per validator; the baseline numbers are not, since its bottleneck is the leader's link. This attribution must be re-measured on separate machines before it is claimed. Two interleaved A/B runs separated code effects from host drift: the proposal-judging fix did not change the baseline at 7 validators (11,203–11,894 tx/s either way), and lane pacing with a 16 KiB threshold changed saturation throughput at 4 validators by under 2% (59,295–60,402 against 60,592–61,449 tx/s) while cutting the 10-validator light-load median from 1.43 s to 0.44 s.

## 9. Defects the Evaluation Exposed

Each defect has a regression test that fails when its fix is reverted.

1. **Parity leader schedule.** The previous schedule `(height−1+round) mod n` advanced height and view together, so in a fault-free run with even `n` only even-indexed validators ever led: half the validators never proposed and their stake gave no proposal share. The stake-proportional view-only schedule of Section 4.6 replaces it (`node/leader_schedule_test.go`).
2. **Replay of committed transactions.** Late gossip re-admitted transactions already committed, which the baseline then committed twice (152,629 commits for 150,000 transactions). Committed transactions are now recorded in the seen cache.
3. **Quadratic seen-cache expiry.** Every admission scanned the whole seen cache; the cache now expires oldest first and is bounded.
4. **Retransmission storm.** A fixed 200 ms resend on a queueing link resent batches that were still in flight; resend now backs off exponentially.
5. **Unbounded duplicated evidence.** Each repetition of a conflicting vote appended another copy of the same evidence, without bound; evidence is now recorded once per offence in a bounded window of 1,024.
6. **Commit body requests.** A node that missed a commit asked every announcing validator for the same body; it now asks one and retries after a second.
7. **Proposals judged only on retransmission.** A proposal whose validator set depends on a block its receiver has not executed was refused and judged again only when its proposer retransmitted it, a quarter of a round timeout later. A validator that fell behind therefore missed views, and when it led the next view every validator waited a whole round timeout: one 10-validator run in four collapsed to 2,280 tx/s with 12 timeouts. Such proposals are now kept and judged once the block is executed; ten consecutive 10-validator runs then completed without a round timeout at the first validator (`consensus/transport_reactor_test.go`).
8. **Per-batch overhead at light load.** A 5 ms seal delay made two-transaction batches at 400 tx/s per lane, and the signatures, acknowledgements and certificate verifications of 2,000 batches per second saturated the host: at 10 validators Lanes committed only 2,900 of 4,000 offered tx/s at a 1.4 s median. Lane pacing (Section 4.1) fixes it.
9. **A mislabelled latency metric.** The harness reported the last block's local execution time as "commit latency p50". It now measures each transaction's submit-to-commit latency and its percentiles.

## 10. Threats to Validity

- **Emulated network.** One host, a lossless shaped link and a shared CPU. Real WAN loss, jitter and asymmetric links are not modelled.
- **Shared CPU.** Lanes results at larger `n` are bounded by the host; the speedup at 10 validators may be under- or over-stated on separate machines.
- **Timeout sensitivity.** The baseline needs a 5 s timeout at 10 validators; a tuned baseline could do better, and a baseline with a different proposal encoding (for example short-id compact blocks) would narrow the gap at small `n`.
- **Whole-proposal fallback.** The 300 ms wait before a validator asks for a whole proposal is shorter than a 512 KiB batch takes to reach every validator over a congested 100 Mbit/s link, which made the leader upload whole blocks at the start of two 7-validator runs.
- **Workload.** Small unsequenced transfers only; heavy execution such as EVM calls would shift the bottleneck to execution for both treatments.
- **Three repetitions.** Variance at 10 validators is visible (34.9k–39.4k tx/s); more repetitions are needed for confidence intervals.
- **Certificates.** Ed25519 certificates concatenate signatures; BLS aggregation would shrink acknowledgements and certificates.

## 11. Reproduction Procedure

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` overrides the round timeout, and `VEXO_RATE=4000 VEXO_THROUGHPUT=20000` runs a latency cell. Omitting `VEXO_UPLINK_MBIT` and `VEXO_LATENCY_MS` runs the unconstrained link. Every published number must record the repository revision, host, Go version and the full log line of each run.

## Technical Parity Appendix

Stable implementation and experiment names that translations must preserve include:

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`
- `validator/proposer.go`, `node/lanes.go`
- `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `validator.SelectProposer(validators, v)`
- `lane_proposal`, `proposal_request`, `0..W−1`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
