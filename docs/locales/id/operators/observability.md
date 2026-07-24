> Locale: id · Bahasa Indonesia

# Panduan observability

Panduan ini menjelaskan cara menilai kesehatan node Vexo dari RPC, metrics, log, dan bukti release. Periksa `running` dan `latest_height` lebih dahulu, lalu `latest_finalized_height` serta peer aktif, latency dan `round_timeout`, kemudian signer, snapshot, replay, dan ban. Proses yang hidup belum membuktikan consensus maju dengan aman.

## Endpoint dan status

Gunakan `/v1/status` untuk height, app hash, finalitas, dan peer; `/v1/metrics` untuk JSON; `/metrics/text` untuk Prometheus; `/v1/diagnostics` untuk readiness; serta `/v1/finality/latest`, `/v1/state/latest`, `/v1/recovery/report`, `/v1/snapshot` untuk bukti dan recovery. Endpoint admin harus berada di balik loopback, jaringan operator, mTLS, atau gateway terautentikasi.

Pada `/v1/status`, `running=true` hanya berarti runtime telah mulai. `latest_height` dan `latest_finalized_height` harus maju, `latest_app_hash` harus sama pada height yang sama, dan `active_peer_count` lebih mewakili sesi nyata daripada peer yang hanya dikonfigurasi atau diberi score.

| `vexo_peer_count` | Rekan saat ini dilarang oleh kebijakan skor | Paku menunjukkan serangan, konfigurasi rekan yang buruk, atau batas yang terlalu ketat |

## Metrics Prometheus

Pantau `vexo_node_running`, `vexo_latest_height`, `vexo_active_peer_count`, `vexo_configured_peer_count`, `vexo_quorum_health_ratio`, `vexo_height_rate_per_minute`, `vexo_round_timeouts`, `vexo_adaptive_round_timeout_nanos`, p95 proposal/vote/commit, `vexo_mempool_size`, `vexo_snapshot_healthy`, `vexo_replay_healthy`, `vexo_validator_signing_failures`, dan `vexo_recovery_finality_deferrals`.

`vexo_peer_count` disimpan untuk dasbor lama. Dasbor baru harus memetakan `vexo_active_peer_count`, `vexo_configured_peer_count`, dan `vexo_scored_peer_count` secara terpisah.

## Aturan Peringatan yang Disarankan

Nomor tune untuk jumlah validator aktual, interval blok, latensi, dan perangkat keras. Ini adalah titik awal, bukan konstanta universal.

| Alert | Kondisi awal | Tindakan |
|---|---|---|
| Height berhenti | tidak berubah selama 2 atau 3 interval | bandingkan validator, proposer, signer, peer |
| Finalitas berhenti | execution maju tetapi finalized height tidak | periksa QC, proof, validator-set hash |
| Tanpa peer aktif | `vexo_active_peer_count == 0` satu menit | periksa address, identity, auth, chain ID |
| Quorum rendah | `vexo_quorum_health_ratio < 0.75` beberapa window | cari partition, latency, peer loss |
| Timeout tinggi | counter atau adaptive timeout di atas baseline | periksa network, proposer, CPU, disk, signer |
| Recovery tertunda | `vexo_recovery_finality_deferrals` naik | ekspor recovery report sebelum mengubah data |

## Ambang Batas Awal yang Disarankan

Gunakan ini sebagai nilai peringatan awal, lalu tune setelah baseline jangka panjang yang sebenarnya:

| Sinyal | Peringatan | Kritis |
|---|---|---|
| Height rate | di bawah 50 % baseline | tidak tumbuh |
| Peer aktif | di bawah target quorum | nol peer |
| Latency p95 | di atas 50 % budget | di atas 80 % |
| Signer | setiap error | berulang pada satu height |
| Snapshot atau replay | satu check gagal | gagal berulang atau divergence |

Aturan paling penting: waspada pada **perubahan dari waktu ke waktu**. Satu angka bisa menyesatkan; tingkat tinggi, keterlambatan finalitas, churn teman sebaya, pertumbuhan mempool, dan kegagalan penandatangan bersama - sama menceritakan kisah nyata.

## Matriks Triase Insiden

| Situasi | Lapisan mungkin | Langkah aman |
|---|---|---|
| Height berhenti, peer sehat | consensus, signer, runtime | simpan log dan periksa proposer/timeout |
| Peer hilang setelah deploy | network atau config | simpan config dan rollback address/auth |
| App hash berbeda | execution atau storage | hentikan node terdampak dan jalankan strict replay |
| Finality proof ditolak | finality atau validator set | verifikasi height, set hash, signature domain |
| Snapshot gagal restore | state sync atau storage | restore ke directory bersih |
| Remote signer menolak | custody atau policy | bedakan policy rejection dan transport outage |

| Lonjakan peer yang dilarang | P2P/keamanan | cuplikan skor peer dan alasan larangan | periksa gosip yang salah bentuk atau berbagi konfigurasi yang salah |

Saat incident, pertahankan WAL, addrbook, signer guard, data directory, config, dan log. Menghapusnya menghancurkan bukti yang membedakan bug dari operator error.

## Log dan respons pertama

Event terstruktur harus membawa node ID, validator ID, chain ID, height, round, block hash, dan peer ID. Simpan `peer_connected`, `peer_dial_failed`, `block_committed`, `round_timeout`, `validator_signing_failure`, `snapshot_exported`, `replay_checked`, `upgrade_halt`, dan `upgrade_applied`.

Bandingkan `/v1/status` pada setidaknya dua validator, lalu `/v1/diagnostics`, peer log, mempool dan fee metrics, signer, kemudian `/v1/recovery/report`. Arsipkan metrics, pprof, config, genesis, binary checksum, dan evidence manifest bersama log release candidate.
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
