# Security Audit Readiness

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah panduan terjemahan berdasarkan dokumentasi kanonik berbahasa Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Tujuan

Dokumen ini membahas threat model, asumsi keamanan, dan bukti audit. Perintah, field JSON, nama RPC, config key, dan identifier kode yang dipakai dalam implementasi serta operasi tetap berbahasa Inggris demi kompatibilitas.

## Ruang lingkup utama

- Periksa poin berikut saat membaca dokumen ini. Perintah, field JSON, metode RPC, kunci konfigurasi, dan identifier kode dipertahankan dalam bahasa Inggris demi kompatibilitas.
- Untuk kalimat normatif yang detail, gunakan dokumen Inggris.
- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/id/security/audit-readiness.md`

## Identifier yang dipertahankan

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

## Bagian sumber Inggris

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- Security Goals
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## Catatan operasional

- `MUST`, `SHOULD`, `MAY`, contoh perintah, contoh JSON, dan nama RPC mempertahankan ejaan Inggris.
- Setelah mengubah terjemahan ini, jalankan `make docs-check`.
- Jika halaman ini berbeda dari sumber Inggris, gunakan sumber Inggris dan perbarui file locale ini dalam perubahan yang sama.

## Sumber kanonik

- [English canonical document](../../en/security/audit-readiness.md)
