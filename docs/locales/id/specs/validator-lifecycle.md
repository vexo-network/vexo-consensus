# Validator Lifecycle

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah dokumen pendamping Bahasa Indonesia untuk dibaca bersama sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.


## Urutan baca

Dokumen ini menjelaskan spesifikasi normatif dari Validator Lifecycle. Jika ini bacaan pertama, ikuti urutan ini.

1. Scope
2. Admission
3. Validator Set
4. Rotation
5. Evidence Lifecycle
6. Slashing
7. Jail and Unbonding

Urutan ini sesuai dengan cara membaca yang benar: mulai dari scope dan state, lalu aturan message, safety, dan liveness, lalu evidence di bagian akhir.

## Gambaran umum

Dokumen ini membantu memahami lifecycle validator join, rotation, jail, slashing, dan leave dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/id/specs/validator-lifecycle.md`

## Mengapa membaca dokumen ini

- lifecycle validator join, rotation, jail, slashing, dan leave
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

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Struktur sumber Inggris

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Sumber kanonik

- [Dokumen kanonik bahasa Inggris](../../en/specs/validator-lifecycle.md)

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: Scope — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Admission — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Validator Set — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Rotation — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Evidence Lifecycle — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Slashing — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Jail and Unbonding — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `vexovaloper...` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexovalcons...` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo...` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `staking tx withdraw-unbonded` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
