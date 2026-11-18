# Skema penyimpanan

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah terjemahan langsung ke Bahasa Indonesia dari sumber Inggris. Keputusan protokol, keamanan, dan rilis tetap normatif dalam bahasa Inggris.


## Urutan baca

Dokumen ini menjelaskan spesifikasi normatif dari Storage Schema. Jika ini bacaan pertama, ikuti urutan ini.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Urutan ini sesuai dengan cara membaca yang benar: mulai dari scope dan state, lalu aturan message, safety, dan liveness, lalu evidence di bagian akhir.

## Gambaran umum

Dokumen ini membantu memahami namespace durable storage, key schema, dan recovery marker dan menghubungkannya dengan keputusan implementasi serta operasi.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/id/specs/storage-schema.md`

## Mengapa membaca dokumen ini

- namespace durable storage, key schema, dan recovery marker
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
- Setelah mengubah dokumen, jalankan `make docs-check` untuk memeriksa pohon lokal dan pemeriksaan terjemahan.

## Hal yang perlu diperhatikan

- Dokumen lokal ini membantu pemahaman; keputusan audit, rilis, dan keamanan ditentukan dari sumber Inggris.
- Jika implementasi berubah, perbarui sumber Inggris dan semua dokumen lokal dalam perubahan yang sama.

## Interface yang harus dipertahankan

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Struktur sumber Inggris

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## Sumber kanonik

- [Dokumen kanonik bahasa Inggris](../../en/specs/storage-schema.md)

## Backend Pebble dan record biner

Backend bawaan adalah Pebble (`github.com/cockroachdb/pebble/v2`), dibuka dengan block cache 256 MiB dan memtable 64 MiB. Record disimpan dalam encoding biner kanonis paket `types`: uvarint minimal, string byte dan daftar berawalan panjang, hash sebagai 32 byte-nya, nilai opsional di belakang byte keberadaan `0` atau `1`, dan field dalam urutan struct yang mendeklarasikannya. Decoder menolak hal lain apa pun, termasuk byte sisa, sehingga record yang rusak dilaporkan alih-alih dibaca sebagai nilai lain; setiap record hanya punya satu encoding. Direktori data yang ditulis build sebelumnya (LevelDB, record JSON) tidak dapat dibaca dan tidak dikonversi: Pebble menolak membukanya dengan galat `format major version 1`. Pulihkan node seperti itu dari snapshot atau sinkronkan ke direktori data kosong.

Pengenal terkait:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## Riwayat state (undo log) dan cache pohon

Riwayat state adalah undo log versi per key. Ketika sebuah key berubah pada suatu tinggi (tulisan blok pada tingginya, tulisan langsung (`Set`, `Delete`, `SetBatch`) setelah state committed pertama pada tinggi committed berikutnya), store mencatat nilai yang dimiliki key sebelumnya, termasuk ketiadaan, sebagai versi key pada tinggi itu, sekali per key dan tinggi; tulisan langsung sebelum blok yang meng-commit tingginya mempertahankan versi yang dicatat yang pertama (`hdirect` menunjuk tinggi itu). Nilai sebuah key pada tinggi committed adalah nilai yang dicatat versi terendahnya di atas tinggi itu, atau nilai saat ini bila tidak ada. Mencatat perubahan hanya membaca nilai key saat ini, tidak pernah riwayatnya, dan pembacaan historis adalah satu seek ke versi key. `PruneBelow` menghapus setiap versi pada atau di bawah tinggi awal yang dipertahankan. Node pohon diberi versi dengan cara yang sama. Dua struktur di memori mengurangi pekerjaan pohon per blok; keduanya tidak disimpan permanen dan tidak mengubah hasil: cache node pohon yang memuat hingga 2^18 node terenkode dari pohon saat ini (setiap batch store memperbaruinya, dan generation yang ganjil selama batch seperti itu di-commit membuat pembaca memakai cache hanya pada state snapshot-nya), dan per namespace, root dan perubahan node dari perhitungan root terakhir, dikunci oleh generation itu dan digest SHA-256 tulisan namespace, sehingga app hash, state roots, dan commit menghitung tulisan yang sama sekali saja.

## Kompaksi WAL mempool dan batch genesis

Commit hanya memadatkan WAL mempool bila isinya paling sedikit 4 MiB dan dua kali isinya setelah kompaksi atau pembukaan terakhir. Jadi setiap byte yang ditambahkan rata-rata ditulis ulang sekitar sekali, dan log yang diputar ulang saat restart tetap dalam dua kali isi hidup atau 4 MiB. Kompaksi eksplisit (`Node.Compact`, atau start yang membuang transaksi replay yang ditolak aplikasi) selalu menulis ulang log. `InitChain` menulis state genesis semua modul sebagai satu batch, sehingga genesis dengan banyak akun cukup satu tulisan durable dan genesis yang gagal tidak meninggalkan apa pun di store.

## Finality proof dalam batch commit dan penghapusan mempool yang ditunda

Jalur commit node menulis finality proof yang dilengkapi QC blok ke batch block/state/app-write yang sama (`CommitBlockStateWithWrites` menerimanya sebagai argumen terakhir). Dengan begitu satu commit hanya butuh satu durable write, bukan satu tambahan per proof. Proof yang akan ditolak store dikeluarkan dari batch dan disimpan oleh langkah setelah commit, yang melaporkan kegagalannya. Commit menghapus transaksinya dari pool dan mengantrekan record penghapusan tanpa menunggu sync-nya. Commit berikutnya menunggu record itu durable sebelum mengeksekusi bloknya, karena restart hanya mengulang penghapusan untuk blok terakhir yang di-commit. Record itu biasanya ikut dalam sync admission berikutnya.

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: Scope — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Backend — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Records — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Indexes — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: EVM Records — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Recovery Rules — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Snapshot Validation — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Schema Migration — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `store.Store` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `eth_getBalance` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `eth_getProof` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `bank/{0x_address}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `auth/nonce/{0x_address}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evm/code/{0x_address}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evm/storage/{0x_address}/{slot}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `seen_ttl` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `code/{address}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `storage/{address}/{slot}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `receipts/{tx_hash}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `logs/{address}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evm_receipts` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `receipt_index/{tx_hash}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `blob_sidecars/by_tx/{tx_hash}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `blob_sidecars/by_blob/{blob_hash}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `tree:` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `receipt_height/{height}/{tx_hash}` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `receipt_pruned_below` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
