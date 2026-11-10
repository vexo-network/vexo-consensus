# Vexo Lanes: penyebaran batch tersertifikasi untuk HotStuff two-chain berbobot stake

> Locale: id · Bahasa Indonesia  
> Jenis dokumen: naskah penelitian dan protokol reproduktibilitas  
> Status: draf berbasis implementasi; hasil diukur pada satu host dengan tautan teremulasi dan harus diulang di jaringan nyata sebelum publikasi

## Abstrak

Protokol BFT berbasis pemimpin dari keluarga HotStuff dibatasi oleh uplink pemimpinnya: setiap transaksi dalam blok keluar dari satu mesin sekali untuk setiap validator, sehingga throughput turun sebesar `1/(n−1)` sementara tautan validator lain menganggur. Vexo Lanes memindahkan penyebaran dari pemimpin tanpa mengganti protokol pengurutan. Setiap validator menyegel transaksi miliknya menjadi batch bertanda tangan pada jalurnya sendiri; validator yang memegang dua pertiga stake mengakui telah menyimpannya sehingga batch tersertifikasi tersedia; pemimpin lalu mengusulkan blok dengan menyebut batch tersertifikasi. Format blok, hash, akar transaksi, byte yang ditandatangani, aturan voting, dan aturan commit two-chain tidak berubah.

Pada satu host dengan tautan teremulasi 100 Mbit/s dan 25 ms per validator, Lanes meng-commit 60.080, 45.323, dan 37.099 tx/s dengan 4, 7, dan 10 validator, dibandingkan 21.073, 10.909, dan 8.297 tx/s ketika node yang sama menyiarkan blok utuh: 2,9×, 4,2×, dan 4,5×, dan selisihnya tumbuh dengan `n`. Di bawah kapasitas (4.000 tx/s), Lanes menambah 35 ms latensi median dengan 4 validator dan memangkas 25 ms dan 130 ms dengan 7 dan 10. Tanpa batas bandwidth, Lanes kehilangan 4% throughput. Evaluasi menemukan sembilan cacat, di antaranya jadwal pemimpin yang membuat hanya validator berindeks genap yang mengusulkan ketika jumlah validator genap.

## 1. Pertanyaan penelitian

Dapatkah penyebaran batch tersertifikasi ditambahkan di bawah protokol HotStuff berbobot yang ada, tanpa mengubah apa yang dipilih validator dan cara blok di-commit, sehingga throughput mengikuti jumlah uplink semua validator, bukan uplink pemimpin? Sub-pertanyaan membahas hambatan pemimpin (RQ1), penskalaan (RQ2), terjaganya keamanan, urutan nonce, dan ketahanan sensor (RQ3), serta biaya saat bandwidth melimpah (RQ4). Hipotesis: H1, unggahan terbesar baseline per transaksi tumbuh linear dengan `n` sedangkan Lanes tetap dekat ukuran transaksi; H2, percepatan tumbuh dengan `n`; H3, tanpa batas tautan Lanes tidak lebih cepat dan paling banyak menambah satu perjalanan pulang-pergi.

## 2. Karya terdahulu dan batas kebaruan

Memisahkan penyebaran dari pengurutan sudah mapan: Narwhal dan Tusk, Bullshark dan Shoal, Stratus, Mir-BFT dan ISS, Autobahn, serta Quorum Store milik Aptos. Naskah ini tidak mengklaim pemisahan itu, sertifikat ketersediaan, jalur per validator, atau hasil WAN nyata sebagai hal baru. Kontribusinya bersifat rekayasa: retrofit yang mempertahankan blok, header, tanda tangan, dan aturan commit bit demi bit; kepemilikan jalur menurut pengirim dengan fallback anti-sensor; autentikasi usulan sebelum pengambilan apa pun; serta harness pengukuran yang dapat direproduksi beserta cacat yang ditemukannya.

## 3. Model sistem

`n` validator dengan kekuatan suara berbobot stake; validator Bizantium memegang kurang dari sepertiga. Kuorum adalah `>= 2/3` kekuatan. Sinkroni parsial, kanal terautentikasi, dan tanda tangan berpemisah domain. Validator dapat crash, bertanda tangan ganda, menahan batch, membanjiri jalurnya, atau mengirim usulan palsu.

## 4. Protokol

**Jalur dan sertifikat.** Setiap validator menyegel transaksinya menjadi `lanes.Batch` saat mencapai 512 KiB atau 5 ms setelah transaksi pertama, dibatasi `lanes.MaxBatchTxs` dan `lanes.MaxBatchBytes`. Jalur bermuatan ringan yang batch sebelumnya di bawah 16 KiB juga menunggu sertifikat batch itu, paling lama 50 ms. Validator mengakui penyimpanan di domain `DomainLaneBatchAck`; pengakuan dari `>= 2/3` kekuatan membentuk `BatchCert`, sehingga setidaknya satu validator jujur menyimpan batch. Pengiriman ulang memakai backoff eksponensial.

**Kepemilikan dan fallback.** Transaksi milik jalur `lanes.Owner(members, sender)`. Semua transaksi satu pengirim melewati satu jalur menurut urutan penerimaan, sehingga jalur tidak pernah mengacak urutan nonce. Jika setelah `lanes.FallbackDelay` (3 detik) tidak ada batch tersimpan yang memuat transaksi milik jalur lain, pemegangnya membuat batch sendiri; pemilik yang rusak dapat menunda tetapi tidak menyensor.

**Usulan berbasis jalur.** Pemimpin hanya mengambil batch tersertifikasi, per jalur berurutan sampai batch pertama yang belum tersertifikasi, bergantian antarjalur sampai batas blok, menandatangani blok seperti biasa, dan menyiarkan `lane_proposal` berisi ID batch, posisi yang ditolak (`Excluded`), dan transaksi di luar batch (`Extra`).

**Autentikasi sebelum pengambilan.** Penerima memeriksa pemimpin dan tanda tangan sebelum mengambil apa pun. Jika set validator bergantung pada blok yang belum dieksekusi, usulan disimpan (paling banyak 16) dan dinilai ulang setelah eksekusi berikutnya. Blok dibangun ulang dari batch tersimpan dan hanya dinilai bila hash dan akarnya sama dengan header bertanda tangan; jika tidak, validator meminta `proposal` utuh lewat `proposal_request`.

**Jadwal pemimpin proporsional stake.** Pemimpin view `v` adalah `validator.SelectProposer(validators, v)`: validator memiliki `VotingPower` slot berurutan dari `[0, W)`, view `v` jatuh pada slot `v·S mod W`, dengan langkah berupa bilangan pertama dari `ceil(W·(φ−1))` yang koprima dengan `W`. Setiap jendela `W` view memberi setiap validator tepat bagiannya. Jadwal hanya bergantung pada view.

**Batas sumber daya.** 64 MiB per penulis dalam penyimpanan 256 MiB; kedaluwarsa setelah 30 detik; paling banyak dua kali melayani satu batch ke satu peer; paling banyak `consensus.MaxLaneRefs` batch per usulan; enkode biner berversi.

## 5. Argumen kebenaran

Keamanan: aturan voting dan commit tidak berubah, dan validator hanya memilih blok yang hash hasil bangun ulangnya sama dengan header bertanda tangan; jalur dapat menunda suara tetapi tidak mengubah objeknya. Ketersediaan: setiap batch yang disebut disimpan validator jujur, dan sebagai upaya terakhir usulan utuh diambil. Liveness: setelah stabil, batch jujur tersertifikasi lalu diusulkan; validator yang tertinggal menilai usulan segera setelah eksekusi. Ini argumen, bukan bukti yang diverifikasi mesin.

## 6. Metode eksperimen

Perlakuan: Lanes (node bawaan) dibandingkan baseline dengan `DisseminateByProposal`, di mana pemimpin menyiarkan blok utuh; selebihnya sama. Semua validator berjalan dalam satu proses pada Apple M4; setiap unggahan melewati tautan virtual berlaju dan latensi tetap, pesan konsensus mendahului lalu lintas massal, tanpa kehilangan paket atau jitter. Klien mengirim transaksi 89 byte; uji throughput mengirim 150.000 sekaligus, uji latensi 20.000 dalam loop terbuka pada 4.000 tx/s (`VEXO_RATE`). Sel throughput 10 validator memakai timeout 5 detik untuk kedua perlakuan, karena dengan bawaan baseline berhenti setelah tinggi 6. Setiap sel diukur tiga kali.

## 7. Hasil

| Validator | Lanes tx/s | Baseline tx/s | Faktor | Lanes B/tx | Baseline B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

Semua uji meng-commit seluruh 150.000 transaksi. Dari usulan saja, baseline membayar `s·(n−1)` byte per transaksi, Lanes 0,2–0,4 B/tx. Ini mendukung H1 dan H2.

| Validator | Lanes p50 / p99 | Baseline p50 / p99 | Usulan Lanes B/tx | Usulan baseline B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

Dengan 4 validator, sertifikasi memakan sekitar satu perjalanan pulang-pergi; mulai 7 validator, uplink pemimpin baseline sudah menunda usulan pada beban ini, dan pada 10 validator hanya Lanes yang mengikuti beban. H3 hanya berlaku untuk set kecil.

| Perlakuan | tx/s | Unggahan B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| Baseline | 114,522 (113,095–116,408) | 147.4–148.3 |

Tanpa batas tautan, baseline 4% lebih cepat; namun Lanes mengunggah kurang dari separuh byte. Baseline dibatasi tautan pemimpin, sedangkan Lanes dalam matriks ini dibatasi host bersama.

## 8. Cacat yang ditemukan evaluasi

1. **Jadwal pemimpin berdasarkan paritas.** `(height−1+round) mod n` hanya membuat indeks genap mengusulkan saat `n` genap; diganti jadwal proporsional stake (`node/leader_schedule_test.go`).
2. **Pemutaran ulang transaksi ter-commit.** Gossip terlambat diterima kembali (152.629 commit untuk 150.000 transaksi).
3. **Kedaluwarsa kuadratik seen cache.** Setiap penerimaan memindai seluruh cache; kini yang tertua lebih dulu dan terbatas.
4. **Badai pengiriman ulang.** 200 ms tetap pada tautan padat; kini backoff eksponensial.
5. **Bukti duplikat tanpa batas.** Kini dicatat sekali per pelanggaran dalam jendela 1.024.
6. **Permintaan isi commit.** Kini satu permintaan, diulang setelah satu detik.
7. **Usulan hanya dinilai saat dikirim ulang.** Satu dari empat uji 10 validator jatuh ke 2.280 tx/s; kini disimpan dan dinilai setelah eksekusi (`consensus/transport_reactor_test.go`).
8. **Biaya per batch saat beban ringan.** Batch berisi dua transaksi menjenuhkan host; diperbaiki dengan pacing jalur.
9. **Metrik latensi salah label.** Kini mengukur latensi dari pengiriman sampai commit per transaksi.

## 9. Ancaman terhadap validitas

- Jaringan teremulasi pada satu host tanpa kehilangan paket dan jitter.
- CPU bersama membatasi Lanes pada `n` besar.
- Baseline butuh timeout 5 detik pada 10 validator; enkode yang lebih ringkas akan mempersempit selisih.
- Penantian 300 ms sebelum meminta usulan utuh terlalu singkat pada tautan padat.
- Hanya transfer kecil; hanya tiga pengulangan (34,9k–39,4k tx/s pada 10 validator).
- Sertifikat Ed25519 menyambung tanda tangan; BLS akan mengecilkannya.

## 10. Reproduksi

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` mengubah timeout; tanpa `VEXO_UPLINK_MBIT` dan `VEXO_LATENCY_MS` tautan tidak dibatasi. Setiap angka yang dipublikasikan harus mencatat revisi, host, versi Go, dan baris log lengkap.

<!-- vexo-docs:technical-parity -->

## Lampiran kesetaraan teknis

Nama implementasi dan eksperimen berikut tidak diterjemahkan:

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`, `VerifyBatch`, `VerifyCert`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`, `consensus/transport_reactor_test.go`
- `validator/proposer.go`, `node/lanes.go`, `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `DomainLaneBatchAck`, `BatchCert`, `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `Excluded`, `Extra`
- `lane_proposal`, `proposal_request`, `proposal`, `DisseminateByProposal`
- `validator.SelectProposer(validators, v)`, `VotingPower`, `[0, W)`, `v·S mod W`, `ceil(W·(φ−1))`, `0..W−1`, `W`, `S`, `v`
- `n`, `s`, `>= 2/3`, `1/(n−1)`, `s·(n−1)`, `(height−1+round) mod n`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
- `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
