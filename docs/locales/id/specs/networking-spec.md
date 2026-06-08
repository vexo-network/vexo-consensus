# Networking Spec

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah panduan terjemahan berdasarkan dokumentasi kanonik berbahasa Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.

## Tujuan

Dokumen ini membahas P2P handshake, gossip, peer scoring, dan kebijakan ban. Perintah, field JSON, nama RPC, config key, dan identifier kode yang dipakai dalam implementasi serta operasi tetap berbahasa Inggris demi kompatibilitas.

## Ruang lingkup utama

- Periksa poin berikut saat membaca dokumen ini. Perintah, field JSON, metode RPC, kunci konfigurasi, dan identifier kode dipertahankan dalam bahasa Inggris demi kompatibilitas.
- Untuk kalimat normatif yang detail, gunakan dokumen Inggris.
- Canonical path: `docs/specs/networking-spec.md`
- Locale path: `docs/locales/id/specs/networking-spec.md`

## Identifier yang dipertahankan

- `consensus`
- `tx`
- `commit`
- `evidence`
- `network_config.json`
- `rpc.address`
- `p2p.listen_address`
- `p2p.peers`
- `p2p.seeds`
- `p2p_address`
- `rpc_address`
- `host:port`
- `0.0.0.0:26656`
- `[::]:26656`
- `0`
- `p2p.tls_cert_path`
- `p2p.tls_key_path`
- `p2p.tls_ca_path`

## Bagian sumber Inggris

- Networking Spec
- Scope
- Transport
- Topics
- Handshake
- Address Roles
- Transport TLS
- Peer Scoring
- Reconnect and Backoff
- DoS/DDOS Defenses
- Operational Signals

## Catatan operasional

- `MUST`, `SHOULD`, `MAY`, contoh perintah, contoh JSON, dan nama RPC mempertahankan ejaan Inggris.
- Setelah mengubah terjemahan ini, jalankan `make docs-check`.
- Jika halaman ini berbeda dari sumber Inggris, gunakan sumber Inggris dan perbarui file locale ini dalam perubahan yang sama.

## Sumber kanonik

- [English canonical document](../../en/specs/networking-spec.md)
