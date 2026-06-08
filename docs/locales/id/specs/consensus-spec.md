# Consensus Spec

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah panduan terjemahan berdasarkan dokumentasi kanonik berbahasa Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Tujuan

Dokumen ini membahas spesifikasi normatif state machine konsensus. Perintah, field JSON, nama RPC, config key, dan identifier kode yang dipakai dalam implementasi serta operasi tetap berbahasa Inggris demi kompatibilitas.

## Ruang lingkup utama

- Periksa poin berikut saat membaca dokumen ini. Perintah, field JSON, metode RPC, kunci konfigurasi, dan identifier kode dipertahankan dalam bahasa Inggris demi kompatibilitas.
- Untuk kalimat normatif yang detail, gunakan dokumen Inggris.
- Canonical path: `docs/specs/consensus-spec.md`
- Locale path: `docs/locales/id/specs/consensus-spec.md`

## Identifier yang dipertahankan

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

## Bagian sumber Inggris

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

## Catatan operasional

- `MUST`, `SHOULD`, `MAY`, contoh perintah, contoh JSON, dan nama RPC mempertahankan ejaan Inggris.
- Setelah mengubah terjemahan ini, jalankan `make docs-check`.
- Jika halaman ini berbeda dari sumber Inggris, gunakan sumber Inggris dan perbarui file locale ini dalam perubahan yang sama.

## Sumber kanonik

- [English canonical document](../../en/specs/consensus-spec.md)
