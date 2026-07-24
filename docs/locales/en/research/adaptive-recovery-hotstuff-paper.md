# Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks

> Document type: research manuscript and reproducibility protocol  
> Status: implementation-grounded draft; performance claims require measured artifacts  
> Canonical implementation: the repository revision recorded with each experiment

## Abstract

This paper studies a systems variant of HotStuff-style Byzantine fault-tolerant state-machine replication for modular Proof-of-Stake networks. The implementation combines three-chain finality and height-versioned validator sets with three operational mechanisms: a bounded adaptive round-timeout controller, a recovery-aware finality gate, and deterministic transaction ordering that preserves per-signer nonce dependencies. The adaptive controller uses observed proposal, vote, and commit latency together with active-peer health to increase timeouts during stalls and decrease them after progress. The recovery gate prevents a finalized application commit from advancing beyond the height at which durable block and state histories are mutually consistent. The ordering rule removes local mempool arrival order from proposal construction while retaining each account's nonce order.

The contribution is not a claim that Proof-of-Stake, BFT, HotStuff, adaptive view synchronization, or fair ordering is new in isolation. The research question is whether this specific bounded control and recovery composition improves liveness and operational consistency without changing the underlying HotStuff safety rule. The paper therefore separates implemented facts, hypotheses, and claims that require experiments. It defines fixed-timeout and gate-disabled baselines, fault and latency workloads, EVM contract-deployment workloads, metrics, statistical reporting rules, and artifact requirements. No throughput or latency improvement is claimed until the prescribed experiments are run against a pinned binary, configuration, topology, and dataset.

## 1. Research Question

The primary question is:

> Can a bounded latency- and peer-aware pacemaker, combined with a durable-history finality gate and nonce-preserving deterministic ordering, reduce avoidable round timeouts and recovery-induced stalls in a HotStuff-style PoS network without weakening its voting or three-chain finality rules?

The paper evaluates four subquestions:

- **RQ1, adaptive pacing:** Does the adaptive controller reduce timeout frequency and tail commit latency under changing network delay compared with the same implementation using a fixed timeout?
- **RQ2, recovery consistency:** Does the recovery gate prevent application state from advancing past a locally inconsistent durable block/state boundary during injected storage and restart faults?
- **RQ3, deterministic execution input:** Does nonce-preserving deterministic ordering make proposals independent of mempool arrival permutations while preventing invalid same-signer nonce order?
- **RQ4, cost:** What CPU, memory, network, and latency overhead is introduced under stable fault-free conditions?

The hypotheses are directional and falsifiable:

- H1: adaptive pacing lowers round-timeout count under variable delay without materially increasing median latency in a stable network.
- H2: the recovery gate produces zero commits above the computed safe recovery height while a block/state mismatch exists.
- H3: every permutation of an identical transaction set yields the same proposal order, and each signer chain remains nonce-monotonic.
- H4: the control-plane overhead remains small relative to proposal verification, EVM execution, and storage commit cost.

These are hypotheses, not results.

## 2. Prior Art and Novelty Boundary

HotStuff introduced a leader-based partially synchronous BFT protocol with linear communication on the happy path, optimistic responsiveness, quorum certificates, and a chained commit rule. LibraBFT/DiemBFT and AptosBFT demonstrate that HotStuff-derived BFT can be combined with stake-weighted validator governance in deployed blockchain systems. Jolteon and Ditto study lower-latency and network-adaptive BFT, including asynchronous fallback. Fever studies responsive view synchronization. Tendermint provides a different round-based PoS BFT lineage. Narwhal and Tusk separate reliable transaction dissemination from ordering. Aequitas, Wendy, and Themis study stronger definitions of transaction order fairness.

Therefore, this manuscript must not claim any of the following:

- the first blockchain to combine PoS and BFT;
- the first PoS network to use a HotStuff-derived protocol;
- a protocol identical to or replacing AptosBFT, DiemBFT, Jolteon, Ditto, or Tendermint;
- asynchronous liveness, optimistic responsiveness, or optimal communication complexity without a proof;
- strong order fairness or MEV resistance from hash-based deterministic ordering alone;
- production safety from unit tests or a single-host Docker experiment.

The candidate systems contribution is narrower: a reproducible evaluation of a bounded feedback controller, a local durable-history commit gate, and nonce-aware deterministic ordering integrated with a modular HotStuff-style PoS node. If experiments do not show a meaningful benefit, the valid research result is a negative or boundary result rather than a stronger novelty claim.

### 2.1 Comparison with Aptos

Both systems use stake-weighted validator concepts and HotStuff-derived vocabulary, but they are not the same implementation or protocol.

| Dimension | This repository | Aptos lineage |
| --- | --- | --- |
| Consensus core | HotStuff-style three-chain rule implemented in Go | AptosBFT lineage derived from DiemBFT/Jolteon, implemented in Rust |
| Execution | Modular Vexo application runtime with a geth-backed EVM adapter | Move VM and Aptos execution pipeline |
| Transaction dissemination | Vexo mempool and proposal gossip | Aptos mempool/Quorum Store architecture |
| Proposed study | Bounded adaptive timeout, recovery gate, nonce-preserving deterministic ordering | Separate production protocol and performance evolution |
| Compatibility | Vexo RPC plus Ethereum-style `/web3` bridge | Aptos REST/gRPC and Move tooling |

Similarity at the level of PoS, BFT, leaders, rounds, votes, and quorum certificates is prior art, not evidence of identity.

### 2.2 Primary References

1. M. Yin, D. Malkhi, M. K. Reiter, G. G. Gueta, and I. Abraham, [HotStuff: BFT Consensus in the Lens of Blockchain](https://arxiv.org/abs/1803.05069), 2018/2019.
2. M. Baudet et al., [State Machine Replication in the Libra Blockchain](https://sonnino.com/papers/librabft.pdf), 2019.
3. R. Gelashvili et al., [Jolteon and Ditto: Network-Adaptive Efficient Consensus with Asynchronous Fallback](https://arxiv.org/abs/2106.10362), 2021.
4. A. Lewis-Pye and I. Abraham, [Fever: Optimal Responsive View Synchronisation](https://arxiv.org/abs/2301.09881), 2023.
5. E. Buchman, J. Kwon, and Z. Milosevic, [The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938), 2018.
6. G. Danezis et al., [Narwhal and Tusk: A DAG-based Mempool and Efficient BFT Consensus](https://arxiv.org/abs/2105.11827), 2021/2022.
7. M. Kelkar et al., [Order-Fairness for Byzantine Consensus](https://eprint.iacr.org/2020/269), 2020.
8. K. Kursawe, [Wendy, the Good Little Fairness Widget](https://arxiv.org/abs/2007.08303), 2020.
9. M. Kelkar et al., [Themis: Fast, Strong Order-Fairness in Byzantine Consensus](https://eprint.iacr.org/2021/1465), 2021.
10. Aptos Labs, [aptos-core](https://github.com/aptos-labs/aptos-core) and [The Aptos Blockchain white paper](https://legacy.aptos.dev/assets/files/Aptos-Whitepaper-47099b4b907b432f81fc0effd34f3b6a.pdf).

## 3. System Model

Let the validator set active at height `h` be `V_h`, with total voting power `P_h`. A quorum certificate is valid when unique known signers contribute at least two-thirds of `P_h`. The validator set and its hash are versioned by height. Admission may be permissionless subject to minimum stake, capped, or restricted by chain configuration. This admission policy is a Sybil-resistance and governance layer; it does not alter the BFT threshold.

The network is partially synchronous. Safety is expected while Byzantine voting power is less than one-third and cryptographic, validator-set, and durable-store assumptions hold. Liveness additionally requires eventual bounded delay, an available honest quorum, usable signers, and sufficient peer connectivity. The implementation does not claim progress in a permanently asynchronous network.

The application is deterministic for a given ordered block. Durable storage contains block records, application state records, module state, consensus write-ahead logs, validator-set versions, and evidence. EVM execution is an application workload under Vexo consensus; Ethereum fork choice and Ethereum peer-to-peer consensus are outside the model.

## 4. Protocol and Control Policy

### 4.1 Base Safety Rule

The consensus state machine tracks `locked_qc`, `high_qc`, the current height and round, and known proposals. A proposal is safe only if it extends the locked certificate or carries a justify certificate at least as new as the lock. A validator must not vote for conflicting block hashes at the same height and round. Proposals and votes are domain-separated and bound to the chain and height-specific validator set.

Three consecutive, height-bound certified links are required to finalize the grandparent. The implementation verifies both hashes and heights: skipped-height or synthetic certificate chains do not satisfy the commit rule. The adaptive controller described below never changes this predicate, the quorum threshold, or the commit rule.

### 4.2 Bounded Adaptive Round Timeout

Let `T0` be the configured base round budget and `Tt` the current adaptive budget. Let `Lp`, `Lv`, and `Lc` be rolling p95 proposal, vote, and commit processing latency. Let `Fpeer` be a peer-health floor derived from active and configured peer counts.

The implementation computes:

```text
observed = 3 * (Lp + Lv + Lc)
candidate = max(T0, observed, Fpeer)

on timeout: next = max(1.5 * Tt, candidate)
on progress: next = max(0.8 * Tt, candidate)
otherwise: next = max(Tt, candidate)

T(t+1) = clamp(next, T0, 8 * T0)
```

When no peers are active, `Fpeer = 2 * T0`. With a partial peer deficit, the floor grows proportionally to the deficit. Idle time with no pending transaction or finality work resets the timeout window and does not consume rounds. Execution or storage failure also resets the local timeout window rather than being misclassified as a consensus timeout.

This is a bounded operational controller, not a formally optimal pacemaker. Its constants are implementation parameters to be evaluated through ablation and sensitivity experiments.

### 4.3 Recovery-Aware Finality Gate

Let `Hs` be the latest durable application-state height and `Hb` the latest durable block-index height. When both exist, the local safe recovery height is:

```text
Hsafe = min(Hs, Hb)
```

If `Hs != Hb`, a finalized commit at height `h > Hsafe` is deferred until recovery restores consistency. Commits at or below `Hsafe` remain eligible. This gate protects the local execution/persistence boundary; it does not create a new network-wide quorum rule and must not be described as an additional BFT voting phase.

The implementation records the current policy and deferral count through node metrics. Experiments must distinguish a successful deferral from a liveness failure.

### 4.4 Nonce-Preserving Deterministic Ordering

For each proposal height, the implementation derives a salt from the chain ID and height. Transactions with signer and nonce metadata are grouped into signer chains and sorted by ascending nonce. Transactions without that metadata form independent chains. The next head is selected by a deterministic hash of the salt and transaction bytes, with transaction bytes as a tie-breaker.

This gives two testable properties:

- identical transaction sets produce identical order regardless of local arrival order;
- transactions from one signer remain in ascending nonce order.

It does not prove first-seen fairness, censorship resistance, confidentiality, or strong order fairness. A proposer may still influence which valid transactions enter the candidate set. The paper uses the term deterministic ordering, not fair ordering, unless a later protocol and proof add a recognized fairness definition.

### 4.5 Validator and Committee Boundary

Consensus voting currently uses the height-versioned active validator set and deterministic proposer selection. The repository also contains deterministic and ECVRF-backed committee-selection components and query surfaces. Those committee results are not currently wired into the consensus vote-quorum path. Consequently, VRF committee consensus is future work and must not appear as an implemented experimental treatment in this paper.

## 5. Implementation Mapping

| Mechanism | Primary implementation | Principal verification |
| --- | --- | --- |
| Proposal/vote safety and locked QC | `consensus/state_machine.go` | `consensus/state_machine_test.go`, adversarial tests |
| Three-chain height/hash binding | `consensus/commit_rule.go` | `consensus/commit_rule_test.go` |
| Timeout certificates | `consensus/timeout.go`, `consensus/pacemaker.go` | timeout and state-machine tests |
| Adaptive controller | `node/adaptive_timeout.go`, `node/loop.go` | `node/adaptive_timeout_test.go`, loop tests |
| Recovery finality gate | `node/recovery.go`, `node/consensus_loop.go` | recovery and node tests |
| Deterministic ordering | `fairordering/fairordering.go`, `consensus/state_machine.go` | fair-ordering and consensus tests |
| Height-versioned PoS set | `validator`, `modules/staking` | registry and staking tests |
| Durable double-vote defense | `consensus/wal.go`, local vote reactor | WAL and transport tests |
| EVM deployment workload | `modules/evm`, `modules/evm/backend/geth`, `rpc` | EVM conformance and RPC integration tests |

The table is part of the reproducibility contract. If files move or semantics change, the paper revision and implementation revision must change together.

## 6. Experimental Method

### 6.1 Treatments

Use the same binary and application configuration for all treatments.

| Treatment | Adaptive timeout | Recovery gate | Purpose |
| --- | --- | --- | --- |
| Fixed | disabled | enabled | RQ1 baseline |
| Adaptive | enabled | enabled | proposed operational policy |
| Gate ablation | enabled | disabled | RQ2 fault-injection comparison in an isolated test environment |

The gate-disabled treatment is unsafe for production and exists only in a disposable research network. Ordering tests should compare the implemented algorithm with randomized input permutations and a separately instrumented arrival-order baseline; do not silently alter production consensus code to produce a favorable result.

### 6.2 Topologies and Faults

Evaluate at least 4, 7, 16, and 31 validators when resources permit. Include local single-host runs only as smoke tests. Report multi-host or isolated-container results for publication.

For each treatment, run:

- stable latency at 10 ms, 50 ms, 100 ms, and 250 ms;
- stepped delay and jitter, including recovery to the original delay;
- loss at 0%, 1%, 5%, and 10%;
- one non-proposing validator restart;
- current proposer restart;
- one-third-minus-one voting-power unavailability;
- short minority partition followed by healing;
- signer delay and transient signer failure;
- durable block/state mismatch injected before restart;
- burst and sustained native transfers;
- EVM transfer, contract creation, event logs, proxy deployment, and UUPS upgrade.

Never inject a fault that reaches or exceeds the assumed Byzantine threshold and then report ordinary safety or liveness as guaranteed. Such a run may be useful as a limit experiment but must be labeled outside the model.

### 6.3 Metrics

Collect, per validator and for the network:

- committed and finalized height over time;
- proposal, vote, and commit latency p50, p95, and p99;
- end-to-end transaction inclusion and finality latency;
- round-timeout count and round number distribution;
- current adaptive timeout and peer count;
- recovery-finality deferral count and duration;
- transaction throughput and EVM gas used;
- CPU, resident memory, disk write rate, and network bytes;
- proposal rejection, stale message, double-sign, and invalid nonce counts;
- validator app hash and finalized block hash agreement.

The implementation exposes relevant control metrics through `/v1/metrics` and `/metrics/text`. Raw samples, not screenshots, are required for analysis.

### 6.4 Workload Correctness

Performance data is valid only if the workload also passes semantic checks:

- all validators agree on committed app hash at the compared height;
- no conflicting finality proof exists;
- every accepted transaction has one canonical receipt;
- Ethereum transaction objects include signature fields and mined block location;
- receipt logs include transaction and block location;
- contract creation returns non-empty code;
- proxy state survives a UUPS implementation upgrade;
- sequential and concurrent account nonces commit without duplicates;
- failed EVM transactions are counted separately from successful transactions.

### 6.5 Repetitions and Statistics

Run a warm-up period, then at least 30 independent measured repetitions per condition or justify a smaller sample using power analysis. Randomize treatment order. Record seeds. Report median, interquartile range, p95, confidence intervals, and effect size. Do not present only the best run. Preserve failures and outliers, and explain exclusion rules before inspecting outcomes.

For H1, compare timeout rate and p95 commit latency between fixed and adaptive treatments. For H2, assert that the maximum committed height during inconsistency never exceeds `Hsafe`. For H3, use property-based permutation tests and count ordering mismatches and nonce inversions. For H4, compare resource usage under stable no-fault load.

## 7. Reproduction Procedure

Record the commit, dirty-tree status, Go version, operating system, CPU, memory, container runtime, topology, and every split configuration file.

Minimum pre-experiment checks:

```bash
make check
make fuzz-smoke
make ops-verify
make network-e2e
make evm-conformance
```

Generate adversarial safety evidence:

```bash
go run ./cmd/vexod consensus adversarial --json > adversarial-evidence.json
```

For a local four-validator smoke network, follow `deployments/docker/README.md`. Query each validator using `/v1/status`, `/v1/metrics`, and `/v1/finality/latest`. Exercise the browser-compatible EVM endpoint at `http://127.0.0.1:28657/web3`.

Every experiment bundle should contain:

- source revision and patch;
- binary SHA-256;
- genesis and split config files;
- workload source and seed;
- raw JSON/JSONL/CSV metrics;
- validator logs;
- final status and app hashes;
- analysis script and dependency lock;
- failed-run ledger;
- figure-generation commands;
- license and provenance for external fixtures.

## 8. Correctness Argument Scope

The safety argument inherited from the implemented HotStuff-style core is conditional on quorum, signatures, lock rules, height-bound validator sets, and less than one-third Byzantine voting power. The adaptive timeout affects when a timeout vote is attempted, not what proposal or certificate is safe. The recovery gate restricts local commits and therefore cannot authorize a commit rejected by the base rule. Deterministic ordering is checked as proposal validity and therefore contributes to deterministic execution, but it is not part of the proof that two conflicting blocks cannot both finalize.

A publication-quality proof must formalize the exact state machine and show:

1. quorum intersection for height-versioned stake-weighted sets;
2. lock monotonicity and safe voting;
3. uniqueness of a finalized block at one height;
4. preservation of safety across validator-set transitions;
5. crash-recovery behavior of vote WAL and durable state;
6. that the controller and recovery gate are safety-neutral restrictions.

The repository's tests and adversarial simulations are evidence, not a substitute for that proof or an independent audit.

## 9. Threats to Validity

- A single-host Docker network understates real network, clock, kernel, and failure diversity.
- Synthetic delay/loss does not capture every routing or DDoS pattern.
- p95 processing latency is a local observation and may lag abrupt network changes.
- Peer count is a coarse health signal and does not prove quorum reachability.
- The constants 3, 1.5, 0.8, and 8 may overfit selected workloads.
- A recovery mismatch injected through a test hook may differ from real disk corruption.
- Hash-based ordering removes arrival-order dependence but does not remove proposer censorship.
- EVM compatibility tests cover the selected fork rules and fixtures, not every Ethereum client behavior.
- Results from a small validator set must not be extrapolated to Internet scale without additional evidence.

## 10. Research Ethics and Publication Rules

This work is ethically publishable if the implementation ancestry, failed experiments, limitations, and evidence are reported honestly.

- Cite HotStuff, Diem/LibraBFT, AptosBFT, Jolteon/Ditto, Tendermint, Narwhal/Tusk, and order-fairness work where their ideas are used or compared.
- Do not rename a known mechanism and present it as invented here.
- Do not fabricate throughput, latency, validator count, attack resistance, or production use.
- Do not select only successful runs or silently remove outliers.
- Separate hypotheses, observed results, and interpretation.
- Archive the exact code and configuration used for every figure.
- Disclose AI assistance according to the target venue's policy; authors remain responsible for every claim, citation, experiment, and proof.
- Run fault experiments only on isolated systems owned or authorized by the researchers.
- Do not expose private keys, operator tokens, participant information, or production endpoints in artifacts.
- Follow coordinated vulnerability disclosure for newly discovered security defects.
- Respect Apache-2.0 obligations in this repository and the licenses of external code, papers, datasets, and Ethereum fixtures.

## 11. Submission Decision Gate

The manuscript is ready to submit only when all of the following are true:

- the protocol description matches a pinned source revision;
- the novelty statement survives a documented prior-art search;
- the fixed and adaptive treatments are reproducible;
- multi-host measurements and fault injections are complete;
- raw data and analysis scripts reproduce every table and figure;
- negative and failed results are included;
- safety claims are supported by a proof appropriate to their wording;
- external review has checked the consensus and statistical methodology;
- EVM workloads include plain deployment, logs, proxy deployment, and UUPS upgrade;
- the limitations and ethics sections remain in the submitted version.

Until then, the correct description is “implementation-grounded research draft,” not “new proven consensus” or “production-ready protocol.”

<!-- vexo-docs:technical-parity -->

## Technical Parity Appendix

Stable implementation and experiment names that translations must preserve include:

- `consensus/state_machine.go`
- `consensus/commit_rule.go`
- `node/adaptive_timeout.go`
- `node/loop.go`
- `node/recovery.go`
- `node/consensus_loop.go`
- `fairordering/fairordering.go`
- `consensus_config.json`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `execution_commit = "finalized"`
- `/v1/status`
- `/v1/metrics`
- `/v1/finality/latest`
- `/metrics/text`
- `http://127.0.0.1:28657/web3`
- `make check`
- `make fuzz-smoke`
- `make ops-verify`
- `make network-e2e`
- `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`

