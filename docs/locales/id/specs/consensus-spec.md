# Consensus Spec

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah dokumen pendamping Bahasa Indonesia untuk dibaca bersama sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.


## Urutan baca

Dokumen ini menjelaskan spesifikasi normatif dari Consensus Spec. Jika ini bacaan pertama, ikuti urutan ini.

1. Scope
2. Roles
3. State
4. Message Types
5. Safety Rules
6. Finality Rule
7. Execution Commit Policy
8. Liveness Assumptions
9. Empty Blocks and Round Recovery
10. Evidence

Urutan ini sesuai dengan cara membaca yang benar: mulai dari scope dan state, lalu aturan message, safety, dan liveness, lalu evidence di bagian akhir.

## Gambaran umum

Dokumen ini membantu memahami spesifikasi normatif state machine konsensus dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/id/specs/consensus-spec.md`

## Mengapa membaca dokumen ini

- spesifikasi normatif state machine konsensus
- Periksa dulu kalimat MUST/SHOULD/MAY di sumber Inggris.
- Dokumen lokal ini membantu pemahaman; keputusan audit, rilis, dan keamanan ditentukan dari sumber Inggris.

## Yang seharusnya bisa dilakukan

- Menjelaskan keputusan implementasi atau operasi yang didukung dokumen ini.
- Menghubungkan persyaratan normatif dari sumber Inggris dengan konfigurasi jaringan saat ini.
- Memeriksa chain ID, validator ID, fee/gas, dan alamat peer sebelum menyalin contoh.

## Checklist penggunaan aman

- Periksa dulu kalimat MUST/SHOULD/MAY di sumber Inggris.
- Jangan menerjemahkan perintah, config key, nama RPC, field JSON, atau identifier kode.
- Sebelum menyalin contoh, sesuaikan chain ID, validator ID, fee/gas, dan alamat peer dengan jaringan Anda.
- Setelah mengubah dokumen, jalankan `make docs-check` untuk memeriksa locale tree dan translation guards.

## Hal yang perlu diperhatikan

- Dokumen lokal ini membantu pemahaman; keputusan audit, rilis, dan keamanan ditentukan dari sumber Inggris.
- Jika implementasi berubah, perbarui sumber Inggris dan semua dokumen lokal dalam perubahan yang sama.

## Interface yang harus dipertahankan

- `(height, round)`
- `chain_id`
- `height`
- `round`
- `phase`
- `validator_set_hash`
- `locked_qc`
- `high_qc`
- `last_timeout_cert`
- `last_finalized`
- `Proposal`
- `Vote`
- `TimeoutVote`
- `QuorumCert`
- `TimeoutCert`
- `>= 2/3`
- `B3`
- `B2`
- `B1`
- `B3.height = B2.height + 1`
- `B2.height = B1.height + 1`
- `execution_commit = "qc"`

## Struktur sumber Inggris

- Consensus Spec
- Scope
- Roles
- State
- Message Types
- Safety Rules
- Finality Rule
- Execution Commit Policy
- Liveness Assumptions
- Evidence

## Sumber kanonik

- [Dokumen kanonik bahasa Inggris](../../en/specs/consensus-spec.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Empty block dan round recovery

Dengan `create_empty_blocks=false`, height yang stabil saat mempool kosong adalah kondisi idle normal. Saat transaksi masuk, node dapat maju ke local proposer round berikutnya untuk membuat block transaksi, tetapi aturan QC/finality tetap berlaku.

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: Scope — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Roles — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: State — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Message Types — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Safety Rules — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Finality Rule — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Execution Commit Policy — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Liveness Assumptions — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Empty Blocks and Round Recovery — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Evidence — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `chain_id` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `validator_set_hash` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `locked_qc` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `high_qc` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `last_timeout_cert` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `last_finalized` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `>= 2/3` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `B3.height = B2.height + 1` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `B2.height = B1.height + 1` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `execution_commit = "qc"` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `execution_commit = "finalized"` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `block_committed` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `create_empty_blocks = false` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `latest_height = 0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `latest_height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `actual_hash` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `actual_time_unix_nano` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `parity_shards` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
