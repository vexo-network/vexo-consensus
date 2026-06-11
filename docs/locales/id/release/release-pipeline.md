# Release Pipeline

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah dokumen pendamping Bahasa Indonesia untuk dibaca bersama sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Gambaran umum

Dokumen ini membantu memahami pipeline rilis dengan binary bertanda tangan, checksums, dan SBOM dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/id/release/release-pipeline.md`

## Mengapa membaca dokumen ini

- pipeline rilis dengan binary bertanda tangan, checksums, dan SBOM
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

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
## Struktur sumber Inggris

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Runbook peluncuran

## VRF audit evidence SHA-256

`release gate` tidak hanya mengunci BLS audit evidence; VRF audit evidence juga harus dipin dengan SHA-256. File `--vrf-audit` wajib masuk ke `evidence-manifest.json`, dan `--vrf-audit-sha256` harus cocok persis dengan isi file. Jika memakai config, `vrf.audit_evidence_sha256` menjadi digest pin default. Aturan ini memastikan VRF service, KMS/HSM custody, TLS/mTLS atau pinned CA, auth token, dan nonce replay defense terikat ke release evidence.

## Sumber kanonik

- [Dokumen kanonik bahasa Inggris](../../en/release/release-pipeline.md)

## Istilah attestation bukti rilis

Untuk rilis publik, setiap entri di `evidence-manifest.json` harus diverifikasi dengan tanda tangan Ed25519. Biarkan flag CLI dan field JSON berikut tanpa diterjemahkan.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
