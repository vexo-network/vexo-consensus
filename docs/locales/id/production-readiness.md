# Panduan kesiapan produksi

> Locale: id · Bahasa Indonesia
> Keputusan keamanan dan rilis harus mengikuti sumber Inggris dan hasil release gate.

## Ringkasan

Dokumen ini menjelaskan apa yang harus diverifikasi sebelum jaringan berbasis Vexo disebut siap produksi.

Panduan lokal ini mempertahankan command, field JSON, metode RPC, key konfigurasi, dan nama package agar contoh tetap bisa disalin di semua bahasa.

## Mengapa penting

Vexo menggabungkan konsensus BFT, modul aplikasi, akuntansi native, eksekusi EVM opsional, ekonomi validator, jaringan peer, dan bukti rilis. Pembaca harus bisa menjelaskan bukan hanya bahwa fitur ada, tetapi juga bagaimana mengoperasikannya dengan aman dan bagaimana membuktikan bahwa fitur itu bekerja di jaringan target.

## Yang wajib diverifikasi

- **Protocol correctness**: `consensus`, `finality`, validator-set hash, vote sign bytes, timeout certificate, and three-chain finality must agree for the target validator set.
- **Runtime correctness**: `app`, `runtime`, `store`, and module writes must commit atomically, replay deterministically, and recover from crash boundaries.
- **Crypto custody**: BLS, VRF, remote signer, KMS/HSM, proof-of-possession, replay nonce, and double-sign guard evidence must match the release binary.
- **Networking safety**: `network_config.json` must bind chain ID, genesis hash, node ID, TLS/auth policy, durable replay path, peer scoring, ban, and backoff settings.
- **EVM/native accounting**: The EVM module uses the native Vexo coin as the balance asset; gas, base fee, blob base fee, receipts, proofs, and traces must pass external corpora.
- **Release evidence**: Release claims need signed artifacts, SBOM, evidence manifest, longrun, chaos, E2E, state sync, economics, governance, MEV, SDK, and EVM/Web3 evidence.

## Tindakan operator

- **System view**: A Vexo network is safe only when protocol, runtime, operations, and evidence are ready together. Do not treat enabled code as a production claim.
- **Configuration review**: Review `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before `vexod start`.
- **Release decision**: Attach evidence from the exact binary, genesis, config schema, module set, and validator topology that will be released.

## Nama interface yang dipertahankan

- `vexod validate --home <home>`
- `vexod config audit --home <home> --strict`
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `peer_count`
- `active_peer_count`
- `configured_peer_count`
- `scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `network_config.json`
- `consensus_config.json`
- `module_config.json`
- `mempool_config.json`
- `release gate`

## Kesalahan umum

- Jangan menganggap peer yang dikonfigurasi sudah tersambung; sesi aktif harus dicek terpisah.
- Jangan menyebut BLS, VRF, EVM, state sync, atau governance siap produksi tanpa bukti rilis.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## Referensi normatif

- [Sumber normatif](../en/production-readiness.md)

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: The Short Version — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: How To Use This Guide — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Readiness Levels — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: System Map — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Configuration Review Order — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Consensus and Finality Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Runtime and Storage Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: EVM and Native Coin Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Crypto and Key Custody Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Networking Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Observability Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Release Evidence Checklist — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Common Failure Modes — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: What This Guide Does Not Claim — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `docs/specs/consensus-spec.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/specs/finality-proof-format.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `modules/staking` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/specs/validator-lifecycle.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `modules/*` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/sdk/app-module-guide.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/specs/storage-schema.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `modules/bank` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/specs/tx-format.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/specs/evm-native-accounting.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `modules/evm` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/sdk/rpc-api-versioning.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `cmd/vexod keys` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/sdk/custom-crypto-backend.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/security/audit-readiness.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/specs/networking-spec.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/operators/node-initialization.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/release/launch-runbook.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `cmd/vexod` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/operators/observability.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `docs/release/release-pipeline.md` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `network_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc.tls_cert_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc.tls_key_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc.tls_ca_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `consensus_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `module_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `mempool_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `log_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod validate --home <home>` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod config audit --home <home> --strict` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `execution_commit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `allow_unsafe_qc_commit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_propose` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_prevote` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_precommit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_commit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `create_empty_blocks` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `eth_getProof` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `go.mod` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `max_score` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `latest_height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `make check` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `v1/status` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `active_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_web3Capabilities` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
