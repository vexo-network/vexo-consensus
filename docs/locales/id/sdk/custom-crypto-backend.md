# Custom Crypto Backend Guide

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah dokumen pendamping Bahasa Indonesia untuk dibaca bersama sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Gambaran umum

Dokumen ini membantu memahami integrasi custom crypto backend seperti BLS, VRF, dan signer dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/id/sdk/custom-crypto-backend.md`

## Mengapa membaca dokumen ini

- integrasi custom crypto backend seperti BLS, VRF, dan signer
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
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `ecvrf-p256-sha256-tai-v1`
- `remote-vrf-http-v1`
## Struktur sumber Inggris

- Custom Crypto Backend Guide
- Goal
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## VRF audit evidence SHA-256

VRF backend harus membuka audit boundary sejelas BLS. Isi `vrf.adapter_name`, `vrf.audit_report`, `vrf.dependency_audit`, `vrf.audit_evidence_sha256`, dan `vrf.key_source`; jika adapter metadata tidak sama dengan config, runtime harus fail closed. Built-in ECVRF adapter memverifikasi go.mod dependency pin dan audit evidence digest, sedangkan remote VRF adapter memakai referensi audit KMS/HSM eksternal.

## Sumber kanonik

- [English canonical document](../../en/sdk/custom-crypto-backend.md)

## Remote VRF service

`vexod keys serve-vrf` menyediakan `POST /prove` dan `POST /verify` dengan ECVRF key, dan `vexod keys verify-vrf` memvalidasi remote prover end-to-end. Jangan terjemahkan `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, dan `vexo.remote_vrf.verify.v1`.

Keep these interface names unchanged: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.
