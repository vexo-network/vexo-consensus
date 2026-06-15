# Format bukti finalitas

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah dokumen pendamping Bahasa Indonesia untuk dibaca bersama sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.


## Urutan baca

Dokumen ini menjelaskan spesifikasi normatif dari Finality Proof Format. Jika ini bacaan pertama, ikuti urutan ini.

1. Scope
2. Proof Fields
3. Header Fields
4. Quorum Certificate Fields
5. Commit Chain Fields
6. Verification Algorithm
7. Accountable Safety Detection
8. Ed25519 Model
9. BLS Model

Urutan ini sesuai dengan cara membaca yang benar: mulai dari scope dan state, lalu aturan message, safety, dan liveness, lalu evidence di bagian akhir.

## Gambaran umum

Dokumen ini membantu memahami field finality proof, urutan verifikasi, dan validator set binding dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/id/specs/finality-proof-format.md`

## Mengapa membaca dokumen ini

- field finality proof, urutan verifikasi, dan validator set binding
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

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## Struktur sumber Inggris

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## Sumber kanonik

- [Dokumen kanonik bahasa Inggris](../../en/specs/finality-proof-format.md)

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: Scope — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Proof Fields — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Header Fields — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Quorum Certificate Fields — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Commit Chain Fields — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Verification Algorithm — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Accountable Safety Detection — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Ed25519 Model — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: BLS Model — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `finality.Proof` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/finality/latest` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/finality/{height}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `strict: true` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/status.latest_height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/finality/*` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `Proof.ValidatorSetHeight <= Header.Height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `Proof.ValidatorSetHash == loaded_set.Hash()` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `Header.ValidatorSetHash == loaded_set.Hash()` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `QuorumCert.Height == Header.Height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `Header.TxRoot` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `HeaderHash(link.Header)` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `finality.AttackDetector` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--validator-set` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `blst-bls12381-minpk-v1` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `supranational/blst` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
