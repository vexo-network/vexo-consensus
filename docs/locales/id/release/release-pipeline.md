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
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `RELEASE_CGO_ENABLED=1`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `release-candidate-plan`
- `make release-portable RELEASE_REQUIRE_BLS=0`
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
- Tujuans
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Runbook peluncuran

## Bukti kesesuaian EVM/Web3

`--sdk-conformance-evidence` dan `--evm-web3-conformance-evidence` adalah bukti yang berbeda. Ringkasan teks seperti “EVM passed” tidak cukup; bukti EVM/Web3 harus memuat bagian yang dapat dibaca mesin: `evm_fixtures`, `evm_execution`, `web3_rpc`, dan `evm_corpus`, lalu diikat ke `evidence-manifest.json` dengan SHA-256 sebelum ada klaim kompatibilitas publik.

## Kebijakan release candidate

Release candidate publik memakai `make release-candidate` secara default. Target ini adalah gate nyata, masuk ke `release-candidate-real`, dan mewajibkan `RELEASE_CGO_ENABLED=1` agar artifact benar-benar memuat adapter BLS `supranational/blst` berbasis cgo. `make release-candidate-plan` hanya untuk PR smoke dan perencanaan operator; target itu memakai fixture bawaan dan dry-run plan, jadi tidak boleh dipakai sebagai evidence final. Jika perlu artifact no-cgo, gunakan `make release-portable RELEASE_REQUIRE_BLS=0`, tetapi jangan mengumumkannya sebagai release BLS-capable. Jika `RELEASE_CGO_ENABLED=1` dan `RELEASE_TARGETS` tidak diatur, Makefile hanya membangun target host saat ini. Untuk artifact multi OS/architecture, set `RELEASE_TARGETS` secara eksplisit pada runner yang memiliki cgo cross-compiler yang sesuai.

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
<!-- vexo-docs-ops-update-2026-06 -->

## Membaca network E2E

`make network-e2e` bukan hanya build test: target ini menjalankan 4 validators dengan binary nyata dan memverifikasi signed-shape smoke transaction, peer connection, height growth, dan clean stop. `NETWORK_E2E_GO_TIMEOUT` adalah batas luar Go test dan harus lebih besar dari network timeout internal.
