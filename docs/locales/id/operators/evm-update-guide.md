# Panduan Pembaruan EVM

> Locale: id · Bahasa Indonesia
> Dokumen ini adalah terjemahan bahasa Indonesia dari sumber bahasa Inggris. Keputusan protokol, keamanan, dan rilis mengikuti sumber bahasa Inggris.

Panduan ini menjelaskan cara memperbarui stack EVM bawaan tanpa merusak penanganan chain ID, kompatibilitas Web3, atau bukti rilis. Ini ditujukan untuk operator dan maintainer yang perlu menaikkan go-ethereum, menyesuaikan fork presets, atau mengubah perilaku EVM dalam rilis yang terkontrol.

## Apa yang Termasuk Pembaruan EVM

Anggap sebagai pembaruan sensitif rilis jika ada perubahan yang dapat memengaruhi eksekusi gaya Ethereum atau perilaku yang terlihat oleh Web3:

- kenaikan versi `go-ethereum` di `modules/evm/backend/geth`
- perubahan pada `modules/evm/ethcompat`
- perubahan pada `modules/evm`
- perubahan pada `execution.evm_fork_preset`
- perubahan pada `execution.evm_chain_config_json`
- perubahan pada admission raw transaction, gas accounting, receipts, traces, proofs, atau field respons blok
- perubahan pada penanganan managed Web3 account seperti `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction`, atau `eth_sendTransaction`

## Urutan Pembaruan yang Aman

Gunakan urutan ini agar kode, konfigurasi, dan dokumentasi tetap selaras:

1. Perbarui dulu adapter geth yang terisolasi.
2. Perbarui kemudian corpus fixtures dan conformance tests.
3. Jika semantik berubah, perbarui `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md`, dan `docs/sdk/rpc-api-versioning.md`.
4. Jika bentuk release evidence berubah, perbarui `docs/release/release-pipeline.md`.
5. Jika pengatur yang terlihat operator berubah, perbarui dokumentasi konfigurasi node.
6. Jalankan ulang validation matrix sebelum merge.

Jangan menaikkan versi runtime EVM lalu langsung merilisnya pada saat yang sama, kecuali conformance suites, RPC smoke checks, dan pengecekan Docker sudah lolos.

## Alur Pembaruan

### 1. Kunci Ruang Lingkup

Catat niat pembaruan dengan tepat:

- fork behavior saja
- transaction admission saja
- execution semantics saja
- RPC compatibility saja
- penanganan blob / receipt / trace saja
- perilaku managed account atau wallet saja

Pemecahan ini menjaga review tetap fokus dan mencegah kode yang tidak terkait ikut bergerak.

### 2. Ubah di Lapisan Tersempit

Utamakan batas berikut:

- `modules/evm/backend/geth` untuk perubahan integrasi upstream go-ethereum
- `modules/evm/ethcompat` untuk raw transaction decoding, preservation hash, dan penanganan fixture
- `modules/evm` untuk state transition, receipts, logs, storage, dan snapshot behavior
- `rpc` untuk perubahan surface Web3 request/response
- `cmd/vexod` hanya jika CLI atau release workflow perlu menampilkan perilaku baru

Jika perubahan mencapai application modules, jaga batas module tetap jelas dan pertahankan deterministic state writes.

### 3. Segarkan Konfigurasi Default

Ketika semantik berubah, perbarui default config dalam patch yang sama:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- jika perlu, field RPC managed account di `network_config.json`
- EVM chain ID di `module_config.json`

Jangan bergantung pada hidden CLI flag untuk menjelaskan perilaku runtime. File konfigurasi harus cukup untuk menunjukkan perilaku node.

### 4. Jalankan Conformance Stack

Minimal jalankan:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Lalu periksa alur yang paling sering rusak terlebih dahulu:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Untuk Docker single-host deployment, verifikasi juga:

```text
http://127.0.0.1:28657/web3
```

Periksa setidaknya perilaku berikut:

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

Kemudian deploy kontrak sederhana, deploy proxy contract, dan uji jalur UUPS upgrade dengan endpoint RPC yang sama seperti yang akan dipakai wallet atau tool di production.

### 5. Konfirmasi Proxy dan Upgrade

Pembaruan EVM belum selesai sampai semua hal ini benar:

- deploy kontrak biasa berhasil
- deploy proxy berhasil
- panggilan UUPS upgrade berhasil
- setelah upgrade, pembacaan storage dan code sesuai harapan
- nonce tracking tetap monotonic
- block producer menerima transaksi yang dihasilkan tanpa unsafe proposal errors

Jika deploy proxy berhasil tetapi upgrade gagal, belum layak dirilis. Anggap itu sebagai release blocker, bukan peringatan.

### 6. Segarkan Evidence

Saat surface EVM berubah, perbarui juga release evidence bundle:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- referensi SHA-256 fixture yang dipin

Release evidence harus menjelaskan apa yang berubah, apa yang diuji, dan commit atau version mana yang diverifikasi. Jangan menyebut pembaruan EVM selesai jika evidence tidak cocok dengan kode yang benar-benar dijalankan.

## Matriks Validasi

Gunakan tabel ini sebagai merge gate.

| Check | Mengapa penting |
| --- | --- |
| `make evm-conformance` | menangkap regresi fork rule dan execution |
| `go test ./modules/evm -count=1` | memverifikasi receipts, logs, storage, balances, dan snapshots |
| `go test ./rpc -count=1` | memverifikasi kompatibilitas Web3 request/response |
| `make network-e2e` | memastikan node masih bisa start, bertetangga dengan peer, dan melakukan commit |
| Docker single-host smoke | memastikan jalur yang dipakai Remix dan browser tools |
| Contract deploy | memastikan admission transaksi dan pembuatan receipt |
| Proxy deploy | memastikan asumsi ABI dan storage layout |
| UUPS upgrade | memastikan semantik upgrade dan pembacaan setelah upgrade |

Jika ada satu saja yang merah, jangan bilang pembaruan selesai.

## Kriteria Rollback

Rollback pembaruan EVM jika salah satu hal berikut terjadi:

- `eth_chainId` berubah secara tidak terduga
- `eth_sendRawTransaction` mulai menolak transaksi valid
- `eth_call` atau `eth_estimateGas` menyimpang dari fork rules yang diharapkan
- receipts, logs, atau proofs tidak lagi cocok dengan committed state
- transaksi proxy atau upgrade mulai gagal
- release evidence tidak lagi cocok dengan jalur kode saat ini

Rollback harus mengembalikan versi adapter yang terakhir diketahui baik, default config, dan fixture set secara bersama-sama.

## Lampiran Paritas Teknis

Lampiran ini menjaga panduan tetap selaras dengan tree dokumentasi lainnya.

- Pertahankan `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc`, dan `cmd/vexod` sebagai batas implementasi yang stabil.
- Pertahankan penulisan `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction`, dan `eth_sendTransaction` tanpa perubahan.
- Pertahankan juga `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures`, dan `--evm-web3-conformance-evidence` tanpa perubahan.
- Pertanyaan operasionalnya sederhana: apakah pembaruan ini mempertahankan execution gaya Ethereum sambil tetap cocok dengan keamanan Vexo consensus dan release?

<!-- vexo-docs:technical-parity -->
