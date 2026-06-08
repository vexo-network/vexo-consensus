# Custom Crypto Backend Guide

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah panduan terjemahan berdasarkan dokumentasi kanonik berbahasa Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Tujuan

Dokumen ini membahas integrasi custom crypto backend seperti BLS, VRF, dan signer. Perintah, field JSON, nama RPC, config key, dan identifier kode yang dipakai dalam implementasi serta operasi tetap berbahasa Inggris demi kompatibilitas.

## Ruang lingkup utama

- Periksa poin berikut saat membaca dokumen ini. Perintah, field JSON, metode RPC, kunci konfigurasi, dan identifier kode dipertahankan dalam bahasa Inggris demi kompatibilitas.
- Untuk kalimat normatif yang detail, gunakan dokumen Inggris.
- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/id/sdk/custom-crypto-backend.md`

## Identifier yang dipertahankan

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`

## Bagian sumber Inggris

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## Catatan operasional

- `MUST`, `SHOULD`, `MAY`, contoh perintah, contoh JSON, dan nama RPC mempertahankan ejaan Inggris.
- Setelah mengubah terjemahan ini, jalankan `make docs-check`.
- Jika halaman ini berbeda dari sumber Inggris, gunakan sumber Inggris dan perbarui file locale ini dalam perubahan yang sama.

## Sumber kanonik

- [English canonical document](../../en/sdk/custom-crypto-backend.md)
