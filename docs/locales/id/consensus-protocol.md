> Locale: id · Bahasa Indonesia

# Ikhtisar protokol konsensus

Halaman ini adalah pintu masuk tingkat tinggi untuk dokumentasi konsensus Vexo. Rincian normatif berada di [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md), dan [Transaction Format](./specs/tx-format.md).

## Model

Vexo memakai inti BFT bergaya HotStuff dengan proposal, vote, quorum certificate(QC), timeout certificate, keamanan locked-QC, dan finalitas tiga rantai. Sebuah blok aman untuk dipilih hanya jika memperpanjang locked QC atau membawa justify QC yang setidaknya sama baru. Rantai QC sintetis atau yang melompati height tanpa mengikat height dan hash blok, parent, serta grandparent secara eksplisit ditolak sebelum keputusan finalitas.

## Identitas protokol dan batas riset

Vexo bukan nama baru untuk HotStuff tanpa modifikasi dan bukan protokol atau implementasi yang sama dengan AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint, atau CometBFT. Runtime Go yang terpisah menggabungkan konsep keselamatan HotStuff dengan waktu ronde adaptif, pemulihan durable, urutan transaksi deterministik, eksekusi modular, dan validator set berversi menurut height.

Jalur vote aktif memakai seluruh validator set pada height dan proposer deterministik. VRF committee selector tersedia sebagai komponen dan query, tetapi belum menentukan proposal eligibility atau quorum formation. Karena itu fitur tersebut harus ditulis sebagai pekerjaan lanjutan. Lihat [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) untuk batas kontribusi dan protokol eksperimen.

## Batas eksekusi dan pemulihan

Sertifikasi QC, finalisasi HotStuff, eksekusi aplikasi, dan commit state adalah peristiwa berbeda. Nilai default `execution_commit=finalized` hanya mengeksekusi ancestor yang dipilih aturan tiga rantai. Pacemaker adaptif dan `recovery_finality_gate_enabled` mengendalikan latensi serta pemulihan tanpa mengubah proposer, quorum power, safe-vote, atau finality.

## Batas keselamatan

- kurang dari sepertiga suara Bizantium
- proposal yang dipisahkan domain, pemungutan suara, timeout - vote, dan tanda tangan finalitas
- pengikatan hash validator - set pada ketinggian bukti yang relevan
- penandatangan unik yang dikenal di QC dan bukti finalitas
- bukti yang dapat dipertanggungjawabkan untuk validator equivocation
- penolakan keputusan komitmen yang bertentangan pada ketinggian akhir yang sama

## Batas Kripto

- Backend `deterministic` hanya untuk pengujian dan gagal dalam validasi network safety.
- `ed25519` didukung untuk pengujian jaringan publik dan persiapan peluncuran.
- `bls` secara default memakai `blst-bls12381-minpk-v1` dan membutuhkan proof-of-possession, pemeriksaan subgroup, validasi public key, audit dependensi, serta bukti release-gate.
- Validasi membutuhkan metadata VRF adapter, tetapi hal itu tidak berarti VRF committee aktif pada jalur konsensus.

- audit konfigurasi yang ketat untuk setiap rumah validator
- bukti release - gate
- tinjauan keamanan eksternal
- bukti jangka panjang dan kekacauan multi - tuan rumah
- bukti kebijakan penandatangan/KMS
- tinjauan kebijakan ekonomi dan tata kelola khusus rantai

Lihat [Kesiapan Audit Keamanan](./security/audit-readiness.md) dan [Release Pipeline](./release/release-pipeline.md) sebelum memperlakukan rilis sebagai siap produksi.
<!-- vexo-docs:technical-parity -->
## Lampiran kesetaraan teknis

Lampiran ini mempertahankan istilah teknis dan antarmuka yang tidak boleh berubah antara versi kanonis dan terjemahan.

### Pelacakan bagian
- section: Model - HotStuff, finalitas tiga rantai, QC, timeout certificate, dan locked-QC safety harus dibaca bersama.
- section: Execution Terms - perbedaan antara qc certified, finalized, executed, dan state committed harus tetap jelas.
- section: Safety Boundary - verifikasi ambang byzantine di bawah sepertiga, domain separation, hash validator set, dan accountable evidence.
- section: Crypto Boundary - pertahankan pengenal `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1`, dan `ecvrf-p256-sha256-tai-v1`.
- section: Operational Boundary - baca `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled`, dan sinyal snapshot/replay bersama.
- `require_network_safety` dan `block_committed` harus tetap terlihat apa adanya dalam terjemahan.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### Antarmuka yang dipertahankan
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## Catatan operasional

Saat membuat validator home baru, periksa `config.json` bersama `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, dan `log_config.json`.
Di produksi, `vexo_quorum_health_ratio` dan `adaptive_round_timeout_enabled` harus dipantau bersama.

- `execution_commit=finalized` tetap menjadi prioritas.
- `qc` hanya boleh diaktifkan pada testnet yang dikontrol.
- `recovery_finality_gate_enabled` harus diverifikasi dengan bukti snapshot dan replay.
