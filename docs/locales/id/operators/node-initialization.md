> Locale: id · Bahasa Indonesia

# Inisialisasi Node

Panduan ini menjelaskan cara menginisialisasi validator dan mengarsipkan rumah node, memulainya, memverifikasi kondisinya, dan menghubungkan klien.

Konektivitas rekan harus dikonfigurasi di `network_config.json`, tidak diteruskan berulang kali pada baris perintah `start`.

Perilaku runtime yang memengaruhi konsensus, RPC, P2P, logging, atau akun Web3 terkelola hanyalah file konfigurasi. `vexod start` menolak tanda seperti `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env`, dan `--evm-account-key`; edit file konfigurasi terpisah sehingga setiap operator meninjau perilaku node deterministik yang sama.

Tidak ada saklar mode simpul. Rumah node ditentukan oleh file konfigurasinya, asal-usulnya, materi kuncinya, dan apakah ada `validator_id` plus penanda tangan.

## Apa yang Anda Bangun

Rumah node Vexo adalah direktori yang berisi semua yang dibutuhkan node untuk memulai:
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
Aturan pentingnya sederhana: inisialisasi sekali, edit file konfigurasi, lalu mulai. Jangan sembunyikan perilaku jaringan di dalam flag shell.

## Lari Lokal Lima Menit

Gunakan alur ini ketika Anda ingin membuktikan biner berfungsi sebelum memikirkan penerapan multi-host.
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
Di terminal lain:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
Bentuk status yang diharapkan:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
Ketinggian terbaru mungkin tetap nol pada node tunggal atau mempool kosong yang dijalankan ketika pembuatan blok kosong dinonaktifkan. Bukan berarti prosesnya terputus. Artinya node tersebut tidak menghasilkan blok kosong. Tambahkan transaksi atau jalankan jaringan pengujian multi-validator untuk mengamati komitmen berkelanjutan.

## Jaringan Lokal Empat Validator

Gunakan alur ini ketika Anda menginginkan konektivitas rekan, rotasi pengusul, log penerapan blok, dan pertumbuhan tinggi badan.
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
Pemeriksaan yang berguna:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
Jika pencatatan log komit blok diaktifkan di `log_config.json`, log validator menyertakan peristiwa seperti:
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
Hentikan jaringan lokal yang dihasilkan dengan:
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 dan Remix

JSON-RPC bergaya Ethereum berada di titik akhir Web3, bukan di bawah namespace API operasional Vexo yang berversi.

Untuk validator host tunggal Docker 1, URL penyedia khusus Remix adalah:
```text
http://127.0.0.1:28657/web3
```
Untuk node lokal langsung dengan port RPC default:
```text
http://127.0.0.1:26657/web3
```
Uji panggilan yang sama yang dilakukan Remix:
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
Jika browser mengatakan pengambilan ID rantai gagal, periksa hal ini secara berurutan:

1. URL diakhiri dengan jalur titik akhir Web3.
2. Browser dapat mencapai port host. Contoh Docker mengekspos `28657`, `28667`, `28677`, dan `28687`; di dalam container port RPC masih `26657`.
3. Server RPC sedang berjalan; menanyakan titik akhir status pada host dan port yang sama.
4. CORS diizinkan oleh konfigurasi `network_config.json`/RPC. Penangan default mengizinkan preflight browser ketika tidak ada daftar CORS khusus yang disetel.
5. Rantai memiliki ID rantai EVM bukan nol di `module_config.json`.

## Simpul Validator

Gunakan `init validator` ketika node akan mengusulkan, memberikan suara, menandatangani pesan konsensus, dan berpartisipasi dalam rotasi validator.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
Setel `VEXO_KEY_PASSPHRASE` sebelum menjalankan perintah ini, atau teruskan `--passphrase` untuk pengaturan lokal satu kali. `--encrypt-keys` mengenkripsi `validator.key.json`, `node.key.json`, dan `validator.vrf.key.json`.

Aturan praktis utama hak asuh:

- `validator.key.json` menandatangani proposal konsensus, pemungutan suara, pemungutan suara batas waktu, dan pesan terkait finalitas.
- `node.key.json` hanya menandatangani jabat tangan P2P; kunci tersebut tidak boleh digunakan kembali sebagai kunci konsensus validator.
- `validator.vrf.key.json` membuktikan keacakan panitia dan harus diperlakukan seperti materi hak asuh validator.
- Pendengar publik harus menggunakan dokumen kunci lokal terenkripsi atau dokumen kunci bergaya KMS/penanda tangan jarak jauh. Jika sebuah node mengekspos RPC publik atau P2P publik yang diautentikasi saat `require_network_safety=true`, startup akan menolak kunci validator lokal teks biasa.
- Kunci yang dihasilkan ditulis dengan mode sistem file `0600`; masih lebih memilih penandatangan jarak jauh/KMS untuk validator yang berumur panjang.

Untuk kunci konsensus BLS:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` menulis dokumen kunci BLS `blst-bls12381-minpk-v1` dan menyalin bukti kepemilikan ke dalam metadata validator `genesis.json` sebagai `bls_pop`.

Hal ini menciptakan:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` adalah penandatangan konsensus. `node.key.json` adalah penanda tangan jabat tangan P2P yang direferensikan oleh `network_config.json:p2p.node_key_path`. Mereka sengaja dipisahkan sehingga node arsip dan validator dapat menggunakan transportasi yang sama tanpa memberikan kunci penandatanganan validator kepada setiap rekan.

Mulailah dengan jaringan berbasis konfigurasi:
```bash
vexod start --home .vexo-validator-1
```
Setelah memulai, baca log. Validator yang sehat harus memancarkan peristiwa yang menjalankan node, mendengarkan RPC, mendengarkan P2P, dan, setelah blok dikomit, peristiwa yang dikomit blok. Jika pembuatan blok kosong dinonaktifkan, log komitmen blok yang hilang berarti tidak ada transaksi.

## Arsip Node

Gunakan `init archive` ketika node harus menyimpan data rantai, mengekspos RPC, melakukan sinkronisasi dari rekan, dan menghindari penandatanganan validator.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
Hal ini menciptakan:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

Itu **tidak** membuat `validator.key.json`.

Mulailah dengan:
```bash
vexod start --home .vexo-archive-1
```
Node arsip tidak menandatangani suara konsensus. Mereka berguna untuk RPC, pengindeksan, sinkronisasi status, penyajian bukti historis, dan menyimpan riwayat kueri yang lebih luas daripada memangkas validator.

## Pisahkan File Konfigurasi

Rumah node menggunakan file konfigurasi terpisah sehingga operator dapat mengedit satu subsistem tanpa mencampurkan pengaturan yang tidak terkait:

- `config.json` berisi identitas node, ID rantai, jalur data, dan penunjuk ke file konfigurasi terpisah.
- `module_config.json` berisi pemilihan modul aplikasi, kebijakan eksekusi/ante, dan kebijakan tata kelola tingkat modul.
- `network_config.json` berisi RPC, identitas node P2P, pengaturan pendengaran/peer/seed, pengaturan TLS/auth, dan kebijakan penilaian rekan.
- `consensus_config.json` berisi waktu putaran konsensus, kebijakan blok kosong, backend kripto, VRF, penerimaan validator, dan kebijakan komite.
- `mempool_config.json` berisi ukuran mempool, biaya, prioritas, WAL, duplikat, dan kebijakan TTL.
- `log_config.json` berisi format log, level, pencatatan peristiwa komit blok, dan pencatatan peristiwa rekan.
- `genesis.json` berisi validator genesis yang tidak dapat diubah, metadata validator, dan status modul genesis.

`network_config.json` Pengaturan RPC juga mencakup `shutdown_timeout`, `web3_max_subscriptions_per_connection`, dan `web3_idle_timeout`. `shutdown_timeout` membatasi penghentian yang baik untuk loop konsensus, server RPC, dan transportasi node sehingga operator tidak menunggu selamanya di jalur penghentian yang macet. Default yang dihasilkan adalah `10s`; Langganan Web3 defaultnya adalah 256 per koneksi dengan batas waktu menganggur `2m` sehingga titik akhir RPC publik tidak dapat mengumpulkan langganan menganggur tanpa batas.

`network_config.json` Pengaturan P2P mencakup `auth_replay_path`, `require_auth_replay_store`, dan `dial_timeout`. Default yang dihasilkan menulis bukti pemutaran ulang nonce ke `data/p2p_auth_replay.jsonl` dan menggunakan batas waktu panggilan keluar `10s`. Untuk pengujian loopback pribadi, toko replay sebagian besar merupakan pembukuan yang tidak berbahaya; untuk P2P yang diautentikasi publik, ini merupakan persyaratan keamanan karena mencegah nonce jabat tangan bertanda tangan yang ditangkap diputar ulang setelah dimulai ulang. `dial_timeout` harus cukup panjang untuk TLS, verifikasi jabat tangan yang ditandatangani, dan latensi lintas wilayah; menyetelnya terlalu rendah akan membuat rekan-rekan yang sehat terlihat tidak stabil dan dapat memperlambat keaktifan setelah dimulai ulang.

`network_config.json` juga memiliki sinkronisasi status startup. Ini berguna untuk mengarsipkan node, validator pengganti, atau node yang dipulihkan ke mesin yang bersih. Ketika `state_sync.enabled` benar, `vexod start` mengunduh snapshot valid pertama dari `state_sync.snapshot_urls`, memverifikasi ID rantai, checksum, akar status, dan namespace KV, memulihkannya ke LevelDB, membangun kembali indeks, dan baru kemudian memulai node. Jika keadaan lokal sudah memenuhi `state_sync.min_height` dan `state_sync.trust_local_higher` benar, startup mencatat `state_sync_skipped` dan menyimpan penyimpanan lokal.

Contoh blok `state_sync`:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
Log startup `state_sync_candidate_failed` untuk kesalahan pengambilan, `state_sync_candidate_rejected` untuk snapshot yang tidak valid atau basi, dan `state_sync_applied` setelah pemulihan terverifikasi. Pertahankan `max_snapshot_bytes` di bawah gambaran terbesar yang ingin dilayani oleh infrastruktur Anda, namun cukup tinggi untuk pertumbuhan keadaan normal. Jangan arahkan node publik ke sumber snapshot pihak ketiga yang tidak diautentikasi kecuali operator memiliki kebijakan kepercayaan out-of-band dan bukti finalitas/klien ringan untuk sumber tersebut.

Jika suatu bidang mengubah perilaku jaringan, edit file konfigurasi terpisah dan komit atau distribusikan file yang ditinjau tersebut. Jangan mengandalkan tanda `vexod start` yang panjang untuk perilaku waktu proses. Perintah start dengan sengaja menolak penandaan waktu konsensus, blok kosong, autentikasi P2P, admin RPC, dan kunci Web3 terkelola sehingga operator tidak secara tidak sengaja menjalankan perilaku berbeda dari konfigurasi yang ditinjau.

## File Mana yang Saya Edit?

| Sasaran | Berkas | Bidang |
|---|---|---|
| Ubah port pengikatan RPC | `network_config.json` | `rpc.address` |
| Ubah port pengikatan P2P | `network_config.json` | `p2p.listen_address` |
| Tambahkan rekan persisten | `network_config.json` | `p2p.peers` |
| Tambahkan rekan benih | `network_config.json` | `p2p.seeds` |
| Aktifkan/nonaktifkan blok kosong | `consensus_config.json` | bidang blok kosong konsensus |
| Sesuaikan batas waktu konsensus | `consensus_config.json` | kolom proposal, prevote, precommit, dan commit timeout |
| Memerlukan eksekusi yang diselesaikan | `consensus_config.json` | bidang komitmen eksekusi konsensus |
| Aktifkan/nonaktifkan modul | `module_config.json` | daftar modul aplikasi |
| Ubah ID rantai EVM | `module_config.json` | bidang ID rantai EVM eksekusi |
| Tune biaya dasar/gas | `module_config.json` | bidang biaya dasar eksekusi, biaya dinamis, gas target, dan gas maksimal |
| Konfigurasikan mempool WAL | `mempool_config.json` | mempool jalur WAL |
| Log komit blok kontrol | `log_config.json` | bidang log komit-peristiwa |
| Kontrol log rekan | `log_config.json` | mencatat bidang acara rekan |

Jika ragu, jalankan:
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## Jenis Kunci

Validator init defaultnya adalah `--key-type bls` karena validasi keamanan jaringan memerlukan finalitas agregat BLS yang diaudit. `--key-type ed25519` tetap tersedia untuk eksperimen pribadi dan penerapan khusus di luar gerbang keamanan jaringan. `--encrypt-keys` harus digunakan untuk node rumah yang tidak dapat dibuang. Pembuatan kunci mandiri juga mendukung kunci VRF:
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
Kunci VRF bukanlah penandatangan konsensus. Mereka digunakan untuk pemilihan komite yang didukung VRF dan harus direferensikan dari `consensus_config.json` hingga `vrf_key_paths` ditambah kunci metadata validator `vrf_public_key` ketika backend tersebut diaktifkan.

`config.json` menunjuk ke file konfigurasi terpisah:
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
Setiap jalur mungkin bersifat absolut atau relatif terhadap node asal. Jika dihilangkan, `vexod` menggunakan file `<home>/<name>_config.json` default.

Contoh `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
Kebijakan tata kelola juga ada di `module_config.json`. Konfigurasi aman jaringan yang dihasilkan memerlukan deposit proposal:
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
Deposit tersebut merupakan saldo asli yang disimpan dari pengirim proposal. Melewati proposal mengembalikan deposit; proposal yang ditolak pindahkan ke `RejectedDeposits`. Gunakan alamat yang dikontrol oleh modul perbendaharaan/kumpulan komunitas Anda jika setoran yang ditolak seharusnya mendanai perbendaharaan, bukan akun modul default.

Contoh `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` dan `rpc.evm_account_private_keys` bersifat opsional dan mendukung metode akun terkelola Web3 seperti `eth_accounts`, `eth_sign`, `eth_signTransaction`, dan `eth_sendTransaction`. Lebih suka `evm_account_key_envs` sehingga kunci pribadi dimasukkan oleh lingkungan proses atau manajer rahasia daripada disimpan di JSON. Biarkan kedua daftar tetap kosong untuk operasi validator normal kecuali node ini sengaja bertindak sebagai titik akhir hot-wallet Web3 lokal. Keamanan startup menolak hot key EVM yang dikelola pada pendengar RPC publik.

Contoh `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` diselesaikan relatif terhadap direktori yang berisi `consensus_config.json`. Gunakan dokumen kunci terenkripsi dan berikan `VEXO_KEY_PASSPHRASE` ke proses node ketika penyimpanan kunci VRF lokal tidak dapat dihindari. Jangan letakkan skalar pribadi VRF mentah langsung di `consensus_config.json` untuk jaringan yang dijalankan operator.

Gunakan `vexod config paths --home <home>` untuk memeriksa semua jalur yang diselesaikan.

Konfigurasi arsip memiliki:
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
Arsip `consensus_config.json` menonaktifkan putaran konsensus lokal:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
Rumah validator yang dihasilkan menyetel `"require_network_safety": true` di `config.json` secara default. Ini bukan modus; ini adalah gerbang keamanan startup yang menolak kripto deterministik, transaksi yang tidak ditandatangani/tidak ditandatangani, tidak ada biaya/gas minimum, tidak ada WAL mempool yang tahan lama, tidak ada kebijakan penggantian untuk transaksi penandatangan/nonce yang sama, keacakan komite yang tidak aman, dan nilai `execution_commit` selain `finalized`.

Ketika `require_network_safety` diaktifkan, jalankan:
```bash
vexod config audit --home <home> --strict
```
sebelum memulai node. Audit harus dilakukan untuk setiap validator dan rumah arsip yang berpartisipasi dalam jaringan yang sama.

## Rekan Berbasis Konfigurasi

Rekan dan dengarkan alamat langsung di `network_config.json`:
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` memuat rekan-rekan ini secara otomatis:
```bash
vexod start --home .vexo-archive-1
```
Rekan dan benih yang persisten dikonfigurasikan di `network_config.json`; `vexod start` tidak menerima penggantian host peer atau seed.

Jangan letakkan pengaturan host atau `host:port` yang berumur panjang pada baris perintah `vexod start`. Edit `rpc.address`, `p2p.listen_address`, `p2p.peers`, dan `p2p.seeds` di `network_config.json` sebagai gantinya.

Pertahankan `p2p.node_id` stabil selama masa hidup node di rumah. `p2p.node_key_path` harus mengarah ke `node.key.json` atau dokumen kunci lokal/terkelola lainnya yang hanya digunakan untuk penandatanganan jabat tangan rekan. Peta rekan harus menggunakan ID node rekan, bukan alamat akun atau nama operator validator kecuali keduanya sengaja dibuat sama.

Untuk transportasi rekan gRPC yang terenkripsi dan diautentikasi, tetapkan juga `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path`, dan opsional `p2p.tls_server_name` di `network_config.json`. Jalur TLS relatif diselesaikan dari direktori home node. Simpan `p2p.dial_timeout` dalam file yang sama sehingga setiap operator menggunakan perilaku penyambungan kembali yang sama; jangan sembunyikan waktu rekan di skrip shell.

## Waktu Konsensus

Waktu putaran konsensus ada di `consensus_config.json`:
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` mengontrol berapa lama suatu putaran menunggu proposal.
- `timeout_prevote` mengontrol jendela pengumpulan suara.
- `timeout_precommit` mengontrol jendela pengumpulan sertifikat komit.
- `timeout_commit` mengontrol penundaan minimum setelah blok berkomitmen.
- `create_empty_blocks: false` berarti node hanya mengusulkan ketika transaksi tersedia.
- `execution_commit: "finalized"` menunggu keputusan finalitas tiga rantai HotStuff sebelum mengeksekusi leluhur yang diselesaikan dan merupakan default validator yang dihasilkan. `execution_commit: "qc"` segera mengeksekusi dan mempertahankan blok bersertifikasi QC, tetapi gerbang keselamatan menolaknya.

`round_timeout` disimpan hanya sebagai agregat kompatibilitas. Lebih suka bidang batas waktu gaya Tendermint di atas.

Jika `create_empty_blocks` salah, tinggi tetap tidak berubah selama mempool kosong. Hal ini diharapkan: rantai sedang menunggu pekerjaan yang berguna alih-alih melakukan blok kosong. Ketika sebuah transaksi muncul dan keadaan putaran konsensus lokal telah melewati pengusul lain, node maju ke putaran berikutnya di mana validatornya adalah pengusul dan dibangun dari mempool. Jalur pemulihan ini menjaga keaktifan transaksi yang dipicu tanpa mengaktifkan kembali spam blok kosong.

## Jaringan Multi-Validator

Untuk jaringan yang dihasilkan:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
Setiap rumah validator yang dihasilkan menerima:

- `validator.key.json` miliknya sendiri
- file konfigurasi terpisahnya sendiri: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, dan `log_config.json`
- `genesis.json` yang dibagikan
- `network_config.json` entri rekan untuk validator lainnya

`vexod network up` dan `make network-e2e` menggunakan batas waktu tingkat proses sambil menunggu semua validator memulai, mengirimkan transaksi asap, dan mengamati pertumbuhan tinggi badan. Batas waktu perintah default sengaja dibuat lebih lama dari interval konsensus karena mencakup permulaan proses, pembukaan LevelDB, jabat tangan yang ditandatangani P2P, pemeriksaan TLS/auth, penerimaan transaksi, dan penyelesaian. Jika Anda menurunkan waktu tunggu konsensus secara agresif, pertahankan waktu tunggu jaringan cukup besar untuk mendiagnosis kesalahan startup alih-alih mematikan harness terlalu dini.

Untuk jaringan dalam container atau multi-host, masukkan nilai topologi dalam file JSON:
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` dan `rpc_host_template` adalah target panggilan yang ditulis ke dalam daftar rekan `network_config.json` setiap node. Di Docker, ini bisa berupa nama layanan seperti `validator-%d`.
- `p2p_advertise_host_template` dan `rpc_advertise_host_template` adalah alamat publik yang ditulis ke dalam metadata validator di `genesis.json`. Gunakan nama DNS atau IP publik di sini untuk jaringan publik.
- `p2p_listen_host` dan `rpc_listen_host` adalah host pengikatan lokal. Gunakan `0.0.0.0` untuk container atau server yang harus mendengarkan semua antarmuka.
- Jangan menggunakan kembali nama layanan khusus Docker sebagai alamat publik yang diiklankan kecuali jaringan tersebut sengaja dibuat pribadi.

Kemudian hasilkan rumah simpul dari file itu:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## Pemecahan masalah

| Gejala | Kemungkinan besar penyebab | Apa yang harus diperiksa |
|---|---|---|
| `latest_height` tidak bertambah | Blok kosong dinonaktifkan dan tidak ada pemberitahuan, validator online tidak mencukupi, atau penanda tangan tidak tersedia | `consensus_config.json`, log validator, `/v1/diagnostics` |
| `peer_count` adalah `0` | Alamat rekan tidak dapat dijangkau atau `network_config.json` dibuat untuk nama host yang salah | `p2p.peers`, port host kontainer, DNS, firewall |
| `p2p auth replay store` kesalahan | P2P publik/otentikasi memerlukan penyimpanan replay yang tahan lama | `p2p.auth_replay_path` dan tulis izin di bawah beranda |
| `eth_chainId` gagal di Remix | URL salah, port host salah, atau CORS/preflight browser diblokir oleh konfigurasi khusus | Gunakan URL titik akhir Web3, lalu ikalkan titik akhir yang sama secara langsung |
| `config audit --strict` gagal | Gerbang pengaman menemukan properti konfigurasi yang tidak aman | Baca pemeriksaan yang gagal, lalu edit file konfigurasi terpisah yang diberi nama |
| `no block_committed logs` | Logging dinonaktifkan atau tidak ada blok yang dibuat | `log_config.json`, `create_empty_blocks`, konten mempool |
| `managed EVM key rejected` | Kunci pribadi panas dikonfigurasi pada pendengar RPC publik | Hapus `evm_account_private_keys` atau jaga kerahasiaan RPC |

## Daftar Periksa Operator Minimal

Sebelum menyerahkan node ke mesin atau operator lain:

- `vexod validate --home <home>` lolos.
- `vexod config audit --home <home> --strict` tiket masuk ke rumah tersebut.
- `config.json`, file konfigurasi terpisah, `genesis.json`, dan metadata validator publik ditinjau.
- `validator.key.json`, `node.key.json`, dan `validator.vrf.key.json` dienkripsi atau diganti dengan penanda tangan jarak jauh/dokumen kunci KMS.
- `network_config.json:p2p.peers` berisi alamat yang dapat dihubungi dari mesin target, bukan nama khusus Docker kecuali node tersebut benar-benar berjalan di dalam jaringan Docker tersebut.
- `network_config.json` pendengar RPC/P2P publik memiliki materi TLS ketika `require_network_safety` diaktifkan.
- `module_config.json:execution.EVMChainID` diatur sebelum dompet Web3 atau Remix terhubung.
- `mempool_config.json` memiliki jalur WAL jika node harus memulihkan txs yang tertunda setelah restart.
- `log_config.json` mengaktifkan blok komit dan log rekan saat jaringan sedang dibuka.

<!-- vexo-docs:technical-parity -->
## Lampiran Paritas Teknis

Lampiran ini memastikan terjemahan tetap membawa antarmuka yang dapat dijalankan dan bagian penting dari dokumen kanonis bahasa Inggris. Perintah, kunci konfigurasi, metode RPC, dan nama paket dipertahankan sama di semua bahasa.

### Pelacakan bagian
- section: Validator Node — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Archive Node — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Split Configuration Files — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Key Types — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Config-Based Peers — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Consensus Timing — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.
- section: Multi-Validator Network — Bagian ini perlu dibaca bersama nilai konfigurasi, bukti verifikasi, kondisi kegagalan, dan tindakan operator.

### Antarmuka yang dipertahankan apa adanya
- `network_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod start` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--timeout-propose` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--create-empty-blocks` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--p2p-auth-token` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--rpc-admin-token` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-account-key-env` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--evm-account-key` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `validator_id` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `VEXO_KEY_PASSPHRASE` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--passphrase` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--encrypt-keys` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `validator.key.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `node.key.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `validator.vrf.key.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `require_network_safety=true` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--key-type bls` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `blst-bls12381-minpk-v1` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `genesis.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `bls_pop` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `module_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `consensus_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `mempool_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `log_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `data/` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `network_config.json:p2p.node_key_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `shutdown_timeout` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `web3_max_subscriptions_per_connection` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `web3_idle_timeout` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `auth_replay_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `require_auth_replay_store` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `dial_timeout` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `data/p2p_auth_replay.jsonl` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `--key-type ed25519` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vrf_key_paths` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vrf_public_key` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `<home>/<name>_config.json` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc.evm_account_key_envs` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc.evm_account_private_keys` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `eth_accounts` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `eth_sign` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `eth_signTransaction` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `eth_sendTransaction` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `evm_account_key_envs` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod config paths --home <home>` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `"require_network_safety": true` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `execution_commit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `require_network_safety` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `host:port` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc.address` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.listen_address` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.peers` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.seeds` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.node_id` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.node_key_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.tls_cert_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.tls_key_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.tls_ca_path` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.tls_server_name` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p.dial_timeout` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_propose` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_prevote` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_precommit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `timeout_commit` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `create_empty_blocks: false` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `execution_commit: "finalized"` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `execution_commit: "qc"` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `round_timeout` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `create_empty_blocks` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `vexod network up` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `make network-e2e` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p_host_template` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc_host_template` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `validator-%d` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p_advertise_host_template` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc_advertise_host_template` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `p2p_listen_host` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.
- `rpc_listen_host` — Nama ini digunakan apa adanya dalam contoh yang dapat dijalankan dan validasi konfigurasi, jadi jangan diterjemahkan.

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
