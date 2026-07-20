> Locale: id · Bahasa Indonesia

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| `vexo_peer_count` | Rekan saat ini dilarang oleh kebijakan skor | Paku menunjukkan serangan, konfigurasi rekan yang buruk, atau batas yang terlalu ketat |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

`vexo_peer_count` disimpan untuk dasbor lama. Dasbor baru harus memetakan `vexo_active_peer_count`, `vexo_configured_peer_count`, dan `vexo_scored_peer_count` secara terpisah.

## Aturan Peringatan yang Disarankan

Nomor tune untuk jumlah validator aktual, interval blok, latensi, dan perangkat keras. Ini adalah titik awal, bukan konstanta universal.

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

## Ambang Batas Awal yang Disarankan

Gunakan ini sebagai nilai peringatan awal, lalu tune setelah baseline jangka panjang yang sebenarnya:

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

Aturan paling penting: waspada pada **perubahan dari waktu ke waktu**. Satu angka bisa menyesatkan; tingkat tinggi, keterlambatan finalitas, churn teman sebaya, pertumbuhan mempool, dan kegagalan penandatangan bersama - sama menceritakan kisah nyata.

## Matriks Triase Insiden

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

| Lonjakan peer yang dilarang | P2P/keamanan | cuplikan skor peer dan alasan larangan | periksa gosip yang salah bentuk atau berbagi konfigurasi yang salah |

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS
<!-- vexo-docs:technical-parity -->
## Lampiran kesetaraan teknis

Lampiran ini mempertahankan nama teknis yang harus tetap sama dengan versi kanonis:

- `rpc_listening` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `p2p_listening` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `peer_configured` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `peer_connected` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `peer_disconnected` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `peer_dial_failed` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `peer_banned` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `consensus_loop_running` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `block_committed` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `round_timeout` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `validator_signing_failure` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `evidence_received` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `evidence_applied` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `snapshot_exported` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `replay_checked` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `upgrade_halt` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `upgrade_applied` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `dist/` — Nama ini dipakai tanpa perubahan pada contoh runtime dan validasi konfigurasi.
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `/v1/snapshot`
- `configured_peer_count`
- `scored_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `latest_app_hash`
- `banned_peers=0`
- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`
- `vexo_adaptive_round_timeout_enabled`
- `vexo_adaptive_round_timeout_nanos`
- `vexo_quorum_health_ratio`
- `vexo_recovery_finality_gate_enabled`
- `vexo_recovery_finality_deferrals`
- `vexo_node_running == 0`
- `vexo_active_peer_count == 0`
- `vexo_adaptive_round_timeout_enabled == 0`
- `vexo_quorum_health_ratio < 0.75`
- `vexo_recovery_finality_gate_enabled == 0`
- `vexo_snapshot_healthy == 0`
- `vexo_replay_healthy == 0`
- `vexo_validator_signing_failures > 0`
- `vexo_post_commit_reconciliation_failures > 0`
- `timeout_propose`
- `max_txs`
