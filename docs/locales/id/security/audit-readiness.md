# Security Audit Readiness

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah dokumen pendamping Bahasa Indonesia untuk dibaca bersama sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Gambaran umum

Dokumen ini membantu memahami threat model, asumsi keamanan, dan bukti audit dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/id/security/audit-readiness.md`

## Mengapa membaca dokumen ini

- threat model, asumsi keamanan, dan bukti audit
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
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## Struktur sumber Inggris

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- Tujuan keamanan
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

Materi audit harus mencakup VRF adapter audit evidence selain BLS. Pin SHA-256 file seperti `docs/security/ecvrf-audit-evidence.json` ke `vrf.audit_evidence_sha256` atau `--vrf-audit-sha256`, lalu tinjau dependency audit, key custody, TLS/mTLS atau pinned CA, auth, replay defense, dan service availability dalam satu boundary.

## Sumber kanonik

- [Dokumen kanonik bahasa Inggris](../../en/security/audit-readiness.md)
