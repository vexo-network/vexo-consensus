> Locale: id · Bahasa Indonesia

# Dokumentasi

Direktori ini adalah panduan praktis `vexo-consensus`. Panduan ditujukan bagi developer, operator, pengelola release, dan reviewer yang harus memahami jaringan tanpa menebak perilakunya hanya dari source code.

Setiap halaman harus menjelaskan tanggung jawab komponen, file, command, config key, dan API yang menerapkannya, syarat keselamatan, serta bukti sebelum dipakai pada jaringan nyata. Bahasa Inggris tetap menjadi sumber normatif untuk protocol, security, release, SDK, command, config, dan RPC; terjemahan ini membantu pembacaan tetapi tidak menggantikan sumber Inggris untuk keputusan audit.

Untuk memulai, jalankan command di bawah lalu baca `Node Initialization`, `Docker Deployment`, `Observability Guide`, dan `RPC API Versioning`.

| Tugas | Jalur Perintah |
|---|---|
| Bangun biner lokal | __ VEXO_CODE_0 __ |
| Buat satu rumah validator | __ VEXO_CODE_1 __ |
| Validasi satu rumah | __ VEXO_CODE_2__dan __ VEXO_CODE_3 __ |
| Jalankan satu simpul | __ VEXO_CODE_4 __ |
| Kueri satu simpul |' curl - s __ VEXO_URL_0 __ |
| Jalankan jaringan validator empat Docker | __ VEXO_CODE_5 __ diikuti oleh __ VEXO_CODE_6 __ |
| Hubungkan Remix | Gunakan validator Docker 1 URL Web3 `__ VEXO_URL_1 __ |
| Periksa ID rantai Web3 | __ VEXO_CODE_7 __ |

## Mulai cepat

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## Mulai Di Sini

| Dokumen | Tujuan |
|---|---|
| [Panduan Kesiapan Produksi](./production-readiness.md) | Peta tunggal protokol, runtime, operasi, bukti, dan kesiapan rilis |

## Spesifikasi Protokol

- [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), dan [Validator Lifecycle](./specs/validator-lifecycle.md) menjelaskan keselamatan, finalitas, dan perubahan validator set.
- [Networking Spec](./specs/networking-spec.md), [Storage Schema](./specs/storage-schema.md), dan [Transaction Format](./specs/tx-format.md) mencakup transport, durable recovery, dan transaction admission.
- [EVM and Native Accounting](./specs/evm-native-accounting.md) menetapkan batas accounting native dan EVM.

## SDK dan ekstensi

[App Module Guide](./sdk/app-module-guide.md), [Custom Crypto Backend](./sdk/custom-crypto-backend.md), [Custom Storage and Transport](./sdk/custom-storage-transport.md), dan `RPC API Versioning` menjelaskan cara memperluas runtime tanpa merusak kontrak consensus atau RPC.

## Operasi, release, dan keamanan

`Node Initialization`, [Adding a Validator](./operators/add-validator.md), `Observability Guide`, [Runbook peluncuran](./release/launch-runbook.md), `Release Pipeline`, dan [Version Compatibility Matrix](./release/version-compatibility.md) membentuk jalur operator. [Security Audit Readiness](./security/audit-readiness.md) mencatat threat model dan bukti wajib.

## Aturan kematangan

Keberadaan code saja tidak membuktikan kesiapan produksi. Diperlukan unit, adversarial, dan E2E test, artefak operasional, asumsi, mode kegagalan, serta hasil release gate. Command, metode RPC, dan config key tetap identik di semua terjemahan.

## Riset dan publikasi

Untuk menyiapkan makalah, mulai dari [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md). Dokumen ini memisahkan mekanisme yang benar-benar telah diterapkan, termasuk timeout ronde adaptif, gerbang finalitas saat pemulihan, dan pengurutan transaksi deterministik, dari penelitian terdahulu. Dokumen tersebut merangkum pertanyaan riset, hipotesis, protokol eksperimen, artefak yang dapat direproduksi, dan etika penelitian. Kinerja yang belum diukur tidak ditulis sebagai hasil, dan PoS, BFT, maupun HotStuff tidak diklaim sebagai temuan baru.

Nama normatif yang dipertahankan untuk navigasi lintas bahasa adalah `Node Initialization`, `Docker Deployment`, `Observability Guide`, `RPC API Versioning`, `Production Readiness`, `Release Pipeline`, dan `Adaptive Recovery-Gated HotStuff Research Draft`.

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: How to Read This Set — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Start Here — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Protocol Specs — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: SDK and Extension Guides — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Operations and Release — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Security — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Localized Documentation — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Writing New Docs — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Production Claim Rule — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Documentation Review Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `vexo-consensus` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/*` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `make docs-check` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod status --json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `feature_assurance` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `network_config.json:p2p.auth_replay_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `network_config.json:p2p.node_key_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `module_config.json:governance.RequireDeposit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `module_config.json:governance.MinDeposit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `consensus_config.json:consensus.execution_commit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `mempool_config.json:mempool.WALPath` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
