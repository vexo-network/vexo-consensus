# Release Pipeline

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah panduan terjemahan berdasarkan dokumentasi kanonik berbahasa Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Tujuan

Dokumen ini membahas pipeline rilis dengan binary bertanda tangan, checksums, dan SBOM. Perintah, field JSON, nama RPC, config key, dan identifier kode yang dipakai dalam implementasi serta operasi tetap berbahasa Inggris demi kompatibilitas.

## Ruang lingkup utama

- Periksa poin berikut saat membaca dokumen ini. Perintah, field JSON, metode RPC, kunci konfigurasi, dan identifier kode dipertahankan dalam bahasa Inggris demi kompatibilitas.
- Untuk kalimat normatif yang detail, gunakan dokumen Inggris.
- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/id/release/release-pipeline.md`

## Identifier yang dipertahankan

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `make network-e2e`
- `RC_DRY_RUN=1`

## Bagian sumber Inggris

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Launch Runbook

## Catatan operasional

- `MUST`, `SHOULD`, `MAY`, contoh perintah, contoh JSON, dan nama RPC mempertahankan ejaan Inggris.
- Setelah mengubah terjemahan ini, jalankan `make docs-check`.
- Jika halaman ini berbeda dari sumber Inggris, gunakan sumber Inggris dan perbarui file locale ini dalam perubahan yang sama.

## Sumber kanonik

- [English canonical document](../../en/release/release-pipeline.md)
