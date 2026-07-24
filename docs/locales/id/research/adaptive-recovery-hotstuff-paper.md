# HotStuff adaptif dengan gerbang pemulihan untuk jaringan Proof-of-Stake modular

> Locale: id · Bahasa Indonesia  
> Jenis dokumen: naskah penelitian dan protokol reproduktibilitas  
> Status: draf berbasis implementasi; klaim kinerja memerlukan artefak pengukuran.

## Ringkasan

Naskah ini meneliti replikasi state machine BFT bergaya HotStuff untuk jaringan Proof-of-Stake modular. Implementasi menggabungkan finality tiga rantai dan validator set yang diberi versi berdasarkan height dengan tiga mekanisme operasional. Pengendali adaptif terbatas menyesuaikan round timeout dari latensi pemrosesan p95 proposal, vote, dan commit serta kesehatan peer aktif. Recovery finality gate menunda finalized application commit ketika riwayat block yang durable dan riwayat application state berbeda di atas tinggi aman yang sama. Deterministic transaction ordering menghapus pengaruh urutan kedatangan lokal di mempool untuk kumpulan transaction yang sama, sambil mempertahankan ketergantungan nonce setiap signer.

Kontribusi penelitian ini bukan klaim bahwa PoS, BFT, HotStuff, adaptive view synchronization, atau order fairness baru ditemukan. Pertanyaannya lebih sempit: apakah komposisi pengendali, recovery gate, dan ordering yang terbatas ini mengurangi timeout yang tidak perlu dan inkonsistensi pemulihan tanpa mengubah aturan keselamatan HotStuff dasar? Fakta yang sudah diimplementasikan, hipotesis yang dapat dibantah, dan kesimpulan yang masih memerlukan eksperimen dipisahkan. Angka peningkatan throughput atau latency tidak boleh dilaporkan sebelum pengulangan dengan binary, config, topology, dan workload yang dipatok.

## Pertanyaan penelitian

RQ1 membandingkan kebijakan adaptif dengan sistem yang sama dalam mode fixed timeout ketika network delay berubah, menggunakan jumlah timeout dan p95 commit latency. RQ2 menyuntikkan storage/restart fault untuk memeriksa bahwa gate mencegah application state melewati tinggi bersama dari riwayat durable block dan state. RQ3 mengubah seluruh permutasi input dari kumpulan transaction yang sama dan mengharuskan proposal order identik serta nonce meningkat per signer. RQ4 mengukur overhead CPU, memory, network, dan latency pada jaringan stabil tanpa fault.

H1 sampai H4 adalah hipotesis terarah dan falsifiable, bukan hasil. Adanya code path tidak membuktikan manfaat. Bila perbedaannya tidak signifikan, hasil negatif atau batas penerapan harus dilaporkan secara jujur.

## Penelitian terdahulu dan batas kebaruan

HotStuff telah memperkenalkan leader-based BFT di bawah partial synchrony, quorum certificate, chained commit, komunikasi linear pada happy path, dan responsiveness. LibraBFT/DiemBFT serta AptosBFT telah menggabungkan keturunan HotStuff dengan stake-weighted validator governance. Jolteon dan Ditto meneliti latency yang lebih rendah, network adaptation, dan asynchronous fallback; Fever meneliti responsive view synchronization. Tendermint merupakan keluarga round-based PoS BFT yang berbeda. Narwhal/Tusk memisahkan reliable transaction dissemination dari ordering. Aequitas, Wendy, dan Themis mendefinisikan order fairness yang lebih kuat daripada hash-based determinism yang dipakai di sini.

Karena itu, dokumen tidak boleh memakai klaim “blockchain PoS+BFT pertama”, “jaringan PoS pertama yang memakai HotStuff”, “identik dengan AptosBFT”, “asynchronous liveness” atau “optimal communication” tanpa proof, “perlindungan MEV lengkap”, maupun “production-ready” hanya berdasarkan single-host test. Kandidat systems contribution lebih terbatas: mengintegrasikan bounded feedback controller, local durable-history commit gate, dan nonce-aware deterministic ordering ke node PoS modular dalam Go, kemudian mengevaluasinya secara reproducible terhadap baseline fixed dan gate-disabled.

## Model sistem dan mekanisme

Pada height h, Vh adalah active validator set dan Ph total voting power. QC sah bila signer unik yang dikenal menyumbang setidaknya dua pertiga Ph. Set dan hash-nya diberi versi berdasarkan height. Admission dapat permissionless dengan minimum stake, dibatasi jumlah, atau restricted melalui config. Lapisan ini menangani Sybil resistance dan governance; ia tidak mengubah BFT fault threshold.

Network diasumsikan partially synchronous. Safety mengandalkan Byzantine voting power kurang dari sepertiga, signature yang sah, validator-set binding yang benar, dan durable store yang bekerja. Liveness juga memerlukan delay yang akhirnya bounded, honest quorum yang dapat dicapai, signer tersedia, dan peer connectivity cukup. Tidak ada klaim kemajuan untuk network yang selamanya asynchronous.

EVM adalah application workload di bawah Vexo consensus. Ethereum bytecode execution dan kompatibilitas tooling `/web3` tidak berarti Ethereum fork choice atau devp2p consensus diimplementasikan.

Aturan keselamatan dasar melacak `locked_qc` dan `high_qc`. Proposal hanya aman bila memperpanjang lock atau membawa justify QC setidaknya sama baru dengan lock. Validator tidak boleh vote untuk block berbeda pada height/round yang sama. Tiga certified link berturut-turut yang terikat pada height dan hash memfinalkan grandparent. Adaptive controller tidak mengubah predicate tersebut, quorum threshold, QC verification, atau three-chain rule.

Adaptive timeout memakai base budget T0, current budget Tt, jumlah proposal/vote/commit p95 latency, dan floor berdasarkan peer deficit. Setelah timeout nilai tumbuh menuju 1,5×Tt; setelah progress turun menuju 0,8×Tt. Tiga kali observed latency menjadi candidate floor dan hasil dibatasi antara T0 dan 8×T0. Bila tidak ada active peer, peer floor menjadi 2×T0. Idle tanpa pending work serta local execution/storage error tidak menghabiskan round. Ini bounded operational controller, bukan pacemaker yang terbukti optimal.

Recovery gate menghitung Hsafe=min(Hs,Hb) ketika durable state height Hs dan block-index height Hb tersedia. Selama keduanya berbeda, finalized application commit di atas Hsafe ditunda. Gate merupakan local persistence restriction, bukan vote phase tambahan atau network certificate.

Deterministic ordering membuat salt dari chain ID dan height. Transaction yang memiliki signer/nonce metadata dikelompokkan menjadi signer chain, diurutkan berdasarkan nonce naik, lalu kepala chain digabung berdasarkan salted transaction hash. Ketergantungan arrival order hilang untuk candidate set yang sama. Namun first-seen fairness, censorship resistance, confidentiality, dan strong order-fairness tidak dijamin karena proposer masih memengaruhi inclusion.

Consensus vote path saat ini memakai full height-versioned validator set dan deterministic proposer. ECVRF committee selector ada sebagai component dan query, tetapi belum terhubung ke quorum formation atau proposal eligibility. VRF committee consensus tetap menjadi future work.

## Metode eksperimen

Semua treatment memakai binary dan application config yang sama. Perbandingan mencakup fixed dengan adaptive off dan gate on, adaptive dengan keduanya on, serta gate-disabled ablation hanya di jaringan penelitian terisolasi dan disposable. Jika sumber daya memungkinkan, gunakan 4, 7, 16, dan 31 validator; single-host hanya untuk smoke test.

Kondisi mencakup latency 10, 50, 100, dan 250 ms, step delay, jitter, loss 0/1/5/10%, restart validator biasa dan current proposer, unavailability sedikit di bawah sepertiga voting power, minority partition lalu healing, signer delay, dan injected durable-history mismatch. Workload mencakup native transfer, EVM transfer, contract creation, event log, proxy deployment, dan UUPS upgrade.

Metric meliputi committed/finalized height, proposal/vote/commit p50/p95/p99, end-to-end finality latency, timeout count, round distribution, current adaptive timeout, peer count, recovery deferral, throughput, gas, CPU, RSS, disk/network bytes, rejection, double-sign, serta invalid nonce. Performance run hanya sah bila semua validator setuju pada app hash dan finalized block hash, transaction/receipt/block locations konsisten, deployed code tersedia, dan proxy state bertahan setelah upgrade.

Sesudah warm-up, lakukan sekurangnya tiga puluh pengulangan independen per kondisi kecuali jumlah yang lebih kecil dibenarkan sebelumnya melalui power analysis. Acak urutan treatment dan simpan seed. Laporkan median, IQR, p95, confidence interval, dan effect size. Jangan hanya memilih run terbaik; exclusion rule harus ditentukan sebelum hasil dilihat.

## Kebenaran, reproduksi, dan etika

Kebijakan adaptif hanya mengubah kapan timeout vote dicoba, bukan apa yang membuat vote atau QC aman. Gate hanya memperketat commit dan tidak dapat mengizinkan commit yang ditolak aturan dasar. Deterministic ordering membantu menghasilkan execution input yang sama, tetapi tidak menggantikan proof terhadap conflicting finality.

Proof yang layak publikasi harus memformalkan stake-weighted quorum intersection, lock monotonicity, keunikan finalized block per height, validator-set transition, vote WAL crash recovery, dan sifat safety-neutral controller/gate. Unit tests dan adversarial simulations adalah evidence, bukan pengganti formal proof atau independent audit.

Setiap experiment mengarsipkan commit, dirty-tree status, Go/OS/CPU/memory/container, topology, genesis, split configs, binary SHA-256, workload seed, raw JSON/JSONL/CSV, validator logs, final app hashes, analysis scripts, dan failed-run ledger. Mekanisme yang dikenal tidak boleh sekadar diganti nama lalu diklaim sebagai penemuan. Throughput, latency, dan validator count tidak boleh direkayasa; hypothesis, observation, dan interpretation harus terpisah.

AI assistance diungkapkan sesuai kebijakan venue dan penulis tetap bertanggung jawab atas setiap claim, citation, experiment, dan proof. Fault injection hanya dilakukan pada isolated system yang dimiliki atau diotorisasi. Private key, operator token, participant data, dan production endpoint tidak dimasukkan ke artifact. Temuan security mengikuti coordinated vulnerability disclosure.

Sebelum submission, naskah harus cocok dengan pinned source revision, prior-art search diarsipkan, baseline reproducible, multi-host fault measurements selesai, dan setiap table/figure dapat dibuat ulang dari raw data serta script. Negative result, limitation, proof wording yang sesuai, dan external methodology review tetap ada dalam versi kirim. Sebelum itu, istilah yang benar adalah “draf penelitian berbasis implementasi”, bukan “consensus baru yang telah terbukti”.

<!-- vexo-docs:technical-parity -->

## Lampiran kesetaraan teknis

Nama berikut dipertahankan tanpa terjemahan:

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
