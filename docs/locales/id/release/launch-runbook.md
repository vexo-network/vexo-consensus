# Runbook peluncuran

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah dokumen pendamping Bahasa Indonesia untuk dibaca bersama sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.


## Urutan baca

Dokumen ini menjelaskan alur release dan operasi dari Launch Runbook. Jika ini bacaan pertama, ikuti urutan ini.

1. At a Glance
2. Prelaunch Gate
3. Release Candidate Gate
4. Genesis Gate
5. Launch Window
6. Postlaunch Archive

Urutan ini sesuai dengan penggunaan nyata: mulai dari tujuan dan gate, lalu artefak dan kebutuhan evidence, lalu langkah eksekusi.

## Gambaran umum

Dokumen ini membantu memahami checklist operator dan prosedur sebelum peluncuran jaringan dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/id/release/launch-runbook.md`

## Mengapa membaca dokumen ini

- checklist operator dan prosedur sebelum peluncuran jaringan
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

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## Struktur sumber Inggris

- Runbook peluncuran
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Bukti kesesuaian EVM/Web3

Sebelum rilis publik, arsipkan `--evm-web3-conformance-evidence` terpisah dari `--sdk-conformance-evidence`. File tersebut harus memuat `evm_fixtures`, `evm_execution`, `web3_rpc`, dan `evm_corpus` agar `release gate` dapat menolak ringkasan yang tidak bisa diverifikasi.

## VRF audit evidence SHA-256

Saat memvalidasi release candidate, berikan digest BLS dan VRF audit evidence ke `release gate`. Minimal gunakan `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256`, dan `--evidence-manifest`, lalu cocokkan semua evidence file dengan SHA-256 di manifest.

## Sumber kanonik

- [Dokumen kanonik bahasa Inggris](../../en/release/launch-runbook.md)

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: Prelaunch Gate — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Release Candidate Gate — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Genesis Gate — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Launch Window — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Postlaunch Archive — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `release docs-quality` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `checksums.txt` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `sbom-go-modules.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `sbom-go-version.txt` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `release-manifest.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `release-audit-pack.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `release collect-evidence` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `network analyze-longrun` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `longrun-evidence.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-default-fixtures` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-tx-fixtures` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-tx-fixtures-dir` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-execution-fixtures` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-execution-fixtures-dir` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-tx-fixtures-sha256` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-execution-fixtures-sha256` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-web3-conformance-evidence` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evm_fixtures` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evm_execution` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `web3_rpc` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evm_corpus` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod ops conformance` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `relayer soak-plan` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `chain_id` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evidence-manifest.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
