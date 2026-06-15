> Locale: id · Bahasa Indonesia

# Panduan Observabilitas

Panduan ini menjelaskan cara mengetahui apakah node Vexo sehat dari RPC, metrik, log, dan bukti rilis.

Ini ditulis untuk operator yang membutuhkan sinyal praktis: apa yang harus diperhatikan, arti setiap angka, dan kapan suatu nilai harus dianggap berbahaya.

## Sekilas

Jika ada node yang terlihat salah, periksa secara berurutan:

1. `running` dan `latest_height` di `/v1/status`
2. `latest_finalized_height` dan jumlah rekan
3. `round_timeout`, latensi proposal/pemungutan suara, ukuran mempool, dan metrik latensi penerapan
4. kegagalan penandatangan, kesehatan snapshot, dan kesehatan pemutaran ulang
5. larangan rekan dan kegagalan panggilan rekan

Urutan tersebut penting karena memisahkan “proses yang berjalan” dari “rantai yang benar-benar membuat kemajuan yang aman.”

## Titik Akhir Inti

| Titik akhir | Gunakan |
|---|---|
| `/v1/status` | Proses cepat, tinggi, hash aplikasi, finalitas, dan ringkasan rekan |
| `/v1/metrics` | Metrik JSON untuk dasbor dan otomatisasi |
| `/metrics/text` | Metrik teks yang kompatibel dengan Prometheus |
| `/v1/diagnostics` | Gabungan pemeriksaan kesiapan, kemampuan, status, rekan, penyimpanan, dan metrik |
| `/v1/finality/latest` | Bukti finalitas terbaru untuk pemeriksaan klien ringan dan keselamatan |
| `/v1/state/latest` | Root status terbaru dan pengikatan set validator |
| `/v1/recovery/report` | Diagnostik konsistensi crash/restart |
| `/v1/snapshot` | Kesehatan snapshot dan ekspor metadata |

Titik akhir admin seperti pemangkasan, pemutaran ulang, dan kontrol konsensus biasanya hanya dapat dijangkau melalui loopback, jaringan operator, mTLS, atau gateway yang diautentikasi. Token admin yang tercakup tetap bersifat opsional dan diterapkan saat dikonfigurasi.

## Membaca `/v1/status`

Bidang penting:

| Bidang | Arti | Catatan Operator |
|---|---|---|
| `running` | Proses node telah dimulai dan memiliki status runtime | `true` tidak membuktikan keaktifan konsensus dengan sendirinya |
| `latest_height` | Ketinggian aplikasi lokal terbaru | Harus meningkat seiring waktu di jaringan validator langsung |
| `latest_finalized_height` | Tinggi akhir tiga rantai HotStuff terbaru | Tidak boleh ketinggalan tanpa batas waktu di belakang ketinggian yang dieksekusi/dilakukan |
| `latest_app_hash` | Hash penerapan aplikasi | Harus mencocokkan rekan-rekan pada ketinggian yang sama |
| `peer_count` | Ringkasan rekan yang terhubung/mencetak skor | Pilih bidang rekan yang lebih spesifik di bawah |
| `active_peer_count` | Sesi transportasi aktif, saat transportasi dapat melaporkannya | Sinyal cepat terbaik untuk konektivitas P2P langsung |
| `configured_peer_count` | Alamat rekan yang dikonfigurasi atau dipelajari | Jangkauan tidak dijamin |
| `scored_peer_count` | Rekan yang diketahui tabel skor | Berguna untuk riwayat pelarangan/batas tarif, bukan bukti sesi langsung |
| `banned_peers` | Rekan saat ini dilarang oleh kebijakan skor | Lonjakan menunjukkan serangan, konfigurasi rekan yang buruk, atau batasan yang terlalu ketat |

Contoh bagus untuk jaringan host tunggal 4 validator: `running=true`, `latest_height` meningkat, `latest_finalized_height` ada, `active_peer_count` mendekati `3`, dan `banned_peers=0`.

## Metrik Prometheus

Titik akhir teks memperlihatkan pengukur seperti:

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
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

`vexo_peer_count` disimpan untuk dasbor lama. Dasbor baru harus memetakan `vexo_active_peer_count`, `vexo_configured_peer_count`, dan `vexo_scored_peer_count` secara terpisah.

## Aturan Peringatan yang Disarankan

Sesuaikan angka untuk jumlah validator aktual, interval blok, latensi, dan perangkat keras. Ini adalah titik awal, bukan konstanta universal.

| Peringatan | Kondisi Awal | Mengapa |
|---|---|---|
| Node ke bawah | `vexo_node_running == 0` selama 1 menit | Proses/runtime terhenti |
| Tinggi terhenti | `latest_height` tidak berubah selama 2-3 interval blok yang diharapkan | Konsensus atau eksekusi terhenti |
| Finalitas terhenti | `latest_finalized_height` tidak berubah selama blok terus dijalankan | Masalah jalur finalitas atau kuorum |
| Tidak ada rekan yang aktif | `vexo_active_peer_count == 0` selama 1 menit pada node yang tidak terisolasi | Gangguan P2P, ketidakcocokan autentikasi, atau masalah alamat |
| Jumlah rekan terlalu sedikit | rekan aktif di bawah target konektivitas kuorum | Masalah partisi atau bootstrap |
| Lonjakan batas waktu putaran | penghitung batas waktu tumbuh lebih cepat dari garis dasar normal | Latensi, kegagalan pengusul, atau partisi jaringan |
| Melakukan latensi tinggi | p95/p99 mendekati anggaran batas waktu konsensus | Kelebihan penyimpanan/waktu proses |
| Tekanan mempool | ukuran mempool bertambah selama beberapa menit | Masalah kebijakan biaya, spam, atau kapasitas blokir |
| Cuplikan tidak sehat | `vexo_snapshot_healthy == 0` | Risiko sinkronisasi/pemulihan status |
| Putar ulang tidak sehat | `vexo_replay_healthy == 0` | Risiko determinisme atau konsistensi negara |
| Kegagalan penandatanganan | `vexo_validator_signing_failures > 0` | KMS/penandatangan jarak jauh/kegagalan kebijakan |
| Kegagalan rekonsiliasi | `vexo_post_commit_reconciliation_failures > 0` | Bukti kuat atau diperlukan perbaikan |
| Lonjakan rekan yang dilarang | rekan-rekan yang dilarang naik tiba-tiba | Serangan, kesalahan konfigurasi rekan, atau masalah ambang batas penilaian |

## Ambang Batas Awal yang Disarankan

Gunakan ini sebagai nilai peringatan awal, lalu sesuaikan dengan baseline jangka panjang yang sebenarnya:

| Sinyal | Peringatan | Kritis | Tindakan Pertama |
|---|---:|---:|---|
| Tingkat tinggi badan | di bawah 50% dari yang diharapkan untuk 2 jendela | pertumbuhan nol untuk interval 2-3 blok | bandingkan semua validator, periksa log pengusul/penandatangan/rekan |
| Keterlambatan ketinggian yang diselesaikan | tumbuh selama 5 menit | bertambah sementara ketinggian yang dieksekusi terus bertambah selama 10 menit | periksa log bukti QC/finalitas dan hash set validator |
| Rekan aktif | di bawah target konektivitas kuorum | nol rekan aktif | periksa alamat yang diiklankan, TLS/auth, asal/ID rantai tidak cocok |
| Batas waktu putaran | 3x garis dasar normal | putaran batas waktu terus menerus | naikkan anggaran batas waktu atau selidiki latensi/partisi |
| Latensi proposal p95 | di atas 50% dari `timeout_propose` | di atas 80% dari `timeout_propose` | pengusul profil, mempool, komitmen DA, disk |
| Latensi suara p95 | di atas 50% dari anggaran prevote/precommit | di atas 80% anggaran | periksa CPU, penandatangan, transportasi, tekanan balik gosip |
| Komit latensi p95 | di atas 50% interval blok | di atas 80% interval blok | periksa LevelDB, akar status, eksekusi EVM, snapshot |
| Ukuran mempool | meningkat selama 5 menit | dekat `max_txs` atau churn penggantian berkelanjutan | periksa biaya dasar, biaya minimum, validitas tx, spam |
| Kegagalan penandatanganan | nilai apa pun yang bukan nol | kegagalan berulang dalam satu jendela ketinggian | hentikan validator jika muncul tanda ganda penjaga atau ketidakcocokan kunci |
| Kesehatan cuplikan | satu pemeriksaan gagal | ekspor/verifikasi/pemulihan yang gagal berulang kali | jeda penayangan sinkronisasi status dan jalankan laporan pemulihan |
| Putar ulang kesehatan | satu kegagalan pemutaran ulang yang ketat | memutar ulang ketidakcocokan pada ketinggian aman terbaru | pertahankan direktori data dan hentikan pemutakhiran/rilis yang tidak aman |
| Rekan yang dilarang | lonjakan mendadak | banyak rekan yang diblokir setelah peluncuran konfigurasi | periksa batas skor, TLS CA, identitas rekan, bukti autentikasi opsional, dan kemiringan jam |

Aturan paling penting: waspada terhadap **perubahan seiring waktu**. Satu nomor saja bisa menyesatkan; tingkat tinggi, kelambatan finalitas, churn rekan, pertumbuhan mempool, dan kegagalan penandatanganan bersama-sama menceritakan kisah sebenarnya.

## Matriks Triase Insiden

| Situasi | Kemungkinan Lapisan | Apa yang Harus Dilestarikan | Langkah Aman Berikutnya |
|---|---|---|---|
| Tinggi Badan Berhenti, Teman Sebaya Sehat | konsensus/penandatangan/runtime | log konsensus, log penandatangan, sampel mempool | verifikasi kunci pengusul dan bulatkan log batas waktu |
| Rekan turun setelah penerapan | jaringan/konfigurasi | konfigurasi jaringan, sertifikat TLS, buku tambahan, log rekan | memutar kembali alamat yang diiklankan/TLS/perubahan autentikasi |
| Hash aplikasi berbeda pada ketinggian yang sama | eksekusi/penyimpanan | direktori data, catatan blok, log aplikasi, keluaran pemutaran ulang | hentikan node yang terkena dampak dan jalankan pemutaran ulang yang ketat |
| Bukti finalitas ditolak | kumpulan finalitas/validator | bukti JSON, validator disetel pada ketinggian bukti | verifikasi hash set validator dan tandatangani domain byte |
| Pemulihan cuplikan gagal | sinkronisasi/penyimpanan status | file snapshot, checksum, akar status, pulihkan log | jangan mencoba lagi terhadap data langsung; pulihkan ke direktori bersih |
| Penanda tangan jarak jauh menolak permintaan | penjagaan kunci | log audit penandatangan, file penjaga, file nonce, log simpul | membedakan penolakan kebijakan dari penghentian transportasi |
| Lonjakan rekan-rekan yang dilarang | P2P/keamanan | cuplikan skor rekan dan alasan pelarangan | periksa gosip yang salah format atau bagikan konfigurasi yang salah |

Saat terjadi insiden, lebih baik menyimpan data daripada “membersihkan”. Menghapus WAL, buku tambahan, penjaga penandatangan, atau direktori LevelDB dapat menghancurkan bukti yang diperlukan untuk membedakan bug dari kesalahan operator.

## Catat Acara yang Perlu Disimpan

Log terstruktur harus disimpan dengan ID node, ID validator, ID rantai, tinggi, putaran, hash blok, dan ID rekan jika relevan.

Peristiwa penting:

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

Untuk kandidat rilis, arsipkan log bersama dengan sampel metrik, sampel pprof, file konfigurasi, genesis, checksum biner, dan manifes bukti.

## Buku Pedoman Respon Pertama

Saat operator melihat masalah:

1. Periksa `/v1/status` pada setidaknya dua validator.
2. Bandingkan `latest_height`, `latest_finalized_height`, `latest_app_hash`, dan jumlah rekan.
3. Periksa `/v1/diagnostics` untuk mengetahui kemampuan yang hilang atau pemeriksaan penyimpanan/pemutaran ulang/snapshot yang tidak sehat.
4. Periksa log peristiwa rekan untuk kesalahan autentikasi, TLS, genesis, ID rantai, atau backoff.
5. Periksa metrik mempool dan biaya dasar jika txs tidak disertakan.
6. Verifikasi log penandatangan dan penanda tangan jarak jauh jika tanda tangan validator gagal.
7. Ekspor laporan pemulihan sebelum menghapus atau mengubah data.
8. Jika diduga ada konflik finalitas, hentikan otomatisasi, simpan log/bukti, dan jalankan deteksi konflik finalitas.

## Tata Letak Dasbor

Dasbor yang berguna biasanya memiliki lima baris:

1. **Kehidupan**: node berjalan, tinggi terbaru, tinggi akhir, laju tinggi.
2. **Latensi konsensus**: batas waktu putaran, proposal/pemungutan suara/komit p95 dan p99.
3. **Jaringan**: rekan yang aktif/dikonfigurasi/diberi skor, rekan yang diblokir, pesan jendela rekan.
4. **Eksekusi**: ukuran mempool, biaya gas/basis, jumlah tx, latensi penerapan.
5. **Pemulihan dan keamanan**: kondisi snapshot, kondisi pemutaran ulang, kegagalan penanda tangan, kegagalan rekonsiliasi.

Buat dasbor tetap membosankan. Tujuannya bukan untuk menampilkan setiap penghitung internal; ini untuk memperjelas keadaan berbahaya sebelum validator menyimpang atau pengguna menyadari transaksi terhenti.

## Melepaskan Bukti Dari Observabilitas

Bagi calon pelepasliaran, observabilitas bukan sekadar pemantauan langsung. Ini menjadi bukti:

1. Kumpulkan baseline `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest`, dan `/v1/recovery/report` dari setiap validator.
2. Jalankan beban untuk durasi dan kecepatan yang dipilih.
3. Suntikkan setidaknya satu kali restart, satu gangguan rekan, dan satu latihan ekspor/verifikasi/pemulihan snapshot.
4. Kumpulkan metrik akhir dari setiap validator.
5. Simpan sampel sebelum/sesudah, log, sampel pprof, log audit penandatangan, dan manifes bukti di `dist/`.

Kumpulan bukti yang baik memungkinkan pengulas menjawab: apakah tinggi badan bertambah, apakah finalitas mengalami kemajuan, apakah rekan-rekan pulih, apakah txs melakukan, apakah snapshot terverifikasi, apakah pemutaran ulang tetap sehat, apakah penandatangan menghindari penandatanganan ganda, dan apakah biner rilis yang tepat memberikan hasil?

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: Core Endpoints — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Reading `/v1/status` — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Prometheus Metrics — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Suggested Alert Rules — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Suggested Starting Thresholds — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Incident Triage Matrix — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Log Events to Keep — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: First Response Playbook — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Dashboard Layout — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Release Evidence From Observability — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `/v1/status` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/metrics` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/metrics/text` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/diagnostics` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/finality/latest` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/state/latest` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/recovery/report` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `/v1/snapshot` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `latest_height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `latest_finalized_height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `latest_app_hash` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `active_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `configured_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `scored_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `banned_peers` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `banned_peers=0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_node_running` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_latest_height` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_active_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_configured_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_scored_peer_count` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_banned_peers` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_height_rate_per_minute` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_round_timeouts` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_proposal_latency_p95_nanos` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_vote_latency_p95_nanos` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_commit_latency_p95_nanos` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_mempool_size` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_snapshot_healthy` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_replay_healthy` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_validator_signing_failures` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_post_commit_reconciliation_failures` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_node_running == 0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_active_peer_count == 0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_snapshot_healthy == 0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_replay_healthy == 0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_validator_signing_failures > 0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexo_post_commit_reconciliation_failures > 0` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_propose` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `max_txs` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `node_running` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc_listening` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p_listening` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `peer_configured` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `peer_connected` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `peer_disconnected` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `peer_dial_failed` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `peer_banned` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `consensus_loop_running` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `block_committed` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `round_timeout` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `validator_signing_failure` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evidence_received` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evidence_applied` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `snapshot_exported` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `replay_checked` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `upgrade_halt` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `upgrade_applied` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `dist/` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
