> Locale: id · Bahasa Indonesia

# Menambahkan Validator

Panduan ini menjelaskan alur operator untuk menambahkan validator ke jaringan Vexo.

Jalur penerimaan yang tepat bergantung pada kebijakan pertaruhan dan tata kelola rantai. Minimal, validator harus diwakili dalam status rantai, memiliki kredensial yang valid, dan menjadi bagian dari pembaruan set validator berversi tinggi.

## 1. Inisialisasi Beranda Validator
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
Untuk kunci validator BLS:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
Setel `VEXO_KEY_PASSPHRASE` sebelum menjalankan perintah ini, atau teruskan `--passphrase` untuk pengaturan lokal satu kali.

Saat menerima validator BLS ke rantai yang ada, sertakan metadata `bls_pop` yang dihasilkan dalam proposal pembaruan validator.
Jalur kunci BLS default menggunakan `blst-bls12381-minpk-v1`; gunakan `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` hanya untuk referensi/pengujian kompatibilitas.

Arsipkan kunci publik yang dihasilkan:
```bash
vexod keys show --home .vexo-validator-new --json
```
Simpan juga `node.key.json` yang dihasilkan. Ia menandatangani jabat tangan P2P untuk `network_config.json:p2p.node_id`; ini bukan kunci konsensus validator dan tidak boleh digunakan kembali sebagai kunci akun.

## 2. Konfigurasikan Alamat Jaringan dan Rekan

Edit `.vexo-validator-new/network_config.json` dan tetapkan alamat pendengaran lokal ditambah rekan-rekan yang persisten:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
Jangan mengandalkan penggantian jaringan baris perintah yang berumur panjang untuk validator produksi. Simpan alamat rekan yang persisten di `network_config.json`.

Gunakan peran alamat terpisah:

- `p2p.listen_address` dan `rpc.address` adalah alamat pengikatan lokal untuk mesin atau kontainer ini.
- `p2p.node_id` adalah identitas rekan node ini. Jaga agar tetap stabil setelah rekan-rekan mempelajarinya.
- `p2p.node_key_path` menunjuk ke kunci penandatanganan jabat tangan lokal untuk identitas rekan tersebut.
- `p2p.peers` berisi target panggilan yang digunakan node ini untuk menjangkau rekan lainnya; kunci peta harus berupa nilai `p2p.node_id` node jarak jauh.
- metadata validator `p2p_address` dan `rpc_address` harus berisi alamat publik yang diiklankan, bukan nama layanan khusus Docker, kecuali jaringan tersebut sengaja dibuat pribadi.

## 3. Kirimkan Tiket Masuk Validator

Misalnya alur staking, buat transaksi staking:
```bash
vexod staking --help
```
Transaksi penerimaan validator harus mencakup:

- ID validator
- alamat validator
- kunci publik konsensus
- hak suara atau referensi pasak
- poin dasar komisi validator, jika rantai mengizinkan pembaruan komisi layanan mandiri
- Metadata P2P `node_id` jika rantai menggunakan metadata genesis/validator untuk melakukan preseed peta rekan
- metadata alamat P2P publik
- metadata alamat RPC publik, jika publik
- Metadata bukti kepemilikan BLS saat BLS diaktifkan

Pembaruan validator harus efektif pada tingkat tertentu dan menghasilkan hash set validator baru.

Setelah validator aktif, operator dapat mengekspos status reward melalui modul staking:
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. Verifikasi Pembaruan Set Validator

Setelah ketinggian pembaruan:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
Periksa:

- validator muncul di set ketinggian tertentu
- hak suara benar
- hash set validator berubah seperti yang diharapkan
- bukti finalitas merujuk pada ketinggian set validator yang benar

## 5. Rencanakan Rotasi Kunci Validator

Kunci validator dapat dirotasi dengan menyiapkan dokumen kunci berikutnya dengan metadata `active_from` dan `active_until` yang tidak tumpang tindih, lalu memulai node dengan kunci rotasi tambahan:
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
Pada waktu penandatanganan, node menggunakan kunci yang jendela aktifnya berisi ketinggian konsensus. Dokumen kunci penanda tangan jarak jauh mempertahankan kebijakan, token autentikasi, dan persyaratan perlindungan tanda tangan ganda yang sama.

## 6. Mulai Validator
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
Startup tidak memiliki saklar mode jaringan. Gunakan `config audit --strict` sebelum memulai ketika jaringan diharapkan memenuhi asumsi keamanan jaringan publik.

## 7. Pantau

Tonton:

- latensi proposal/suara
- batas waktu putaran
- kegagalan penandatanganan validator
- larangan teman sebaya
- ukuran mempool
- melakukan latensi
- kesehatan snapshot/putar ulang

Gunakan:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## Catatan Keamanan

- Jangan pernah menggunakan kembali kunci validator di seluruh rantai independen.
- Tetap mengaktifkan kebijakan penandatanganan jarak jauh untuk validator produksi.
- Jangan menerima validator BLS tanpa bukti kepemilikan atau pertahanan kunci palsu yang setara.
- Jangan memangkas atau memenjarakan validator tanpa bukti terverifikasi yang terkait dengan kumpulan validator tinggi bukti yang benar.

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: 1. Initialize Validator Home — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: 2. Configure Network Addresses and Peers — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: 3. Submit Validator Admission — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: 4. Verify Validator Set Update — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: 5. Plan Validator Key Rotation — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: 6. Start Validator — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: 7. Monitor — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Safety Notes — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `VEXO_KEY_PASSPHRASE` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--passphrase` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `bls_pop` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `blst-bls12381-minpk-v1` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `node.key.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `network_config.json:p2p.node_id` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `.vexo-validator-new/network_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `network_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.listen_address` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc.address` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.node_id` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.node_key_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.peers` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p_address` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc_address` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `node_id` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `active_from` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `active_until` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `config audit --strict` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
