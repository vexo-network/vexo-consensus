# Hướng dẫn cập nhật EVM

> Locale: vi · Tiếng Việt
> Tài liệu này là bản dịch tiếng Việt từ nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành dựa trên nguồn tiếng Anh.

Hướng dẫn này giải thích cách cập nhật stack EVM tích hợp mà không làm hỏng xử lý chain ID, khả năng tương thích Web3, hoặc bằng chứng phát hành. Nó dành cho operator và maintainer cần nâng cấp go-ethereum, chỉnh fork presets, hoặc thay đổi hành vi EVM trong một release có kiểm soát.

## Điều gì được tính là cập nhật EVM

Hãy coi là cập nhật nhạy cảm với release nếu có thay đổi nào có thể ảnh hưởng đến execution kiểu Ethereum hoặc hành vi mà Web3 nhìn thấy:

- nâng phiên bản `go-ethereum` trong `modules/evm/backend/geth`
- thay đổi ở `modules/evm/ethcompat`
- thay đổi ở `modules/evm`
- thay đổi ở `execution.evm_fork_preset`
- thay đổi ở `execution.evm_chain_config_json`
- thay đổi ở admission raw transaction, gas accounting, receipts, traces, proofs, hoặc các field phản hồi block
- thay đổi ở cách xử lý managed Web3 account như `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction`, hoặc `eth_sendTransaction`

## Thứ tự cập nhật an toàn

Hãy theo thứ tự này để code, config và docs luôn khớp nhau:

1. Cập nhật trước adapter geth tách riêng.
2. Sau đó cập nhật corpus fixtures và conformance tests.
3. Nếu semantics đổi, cập nhật `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md`, và `docs/sdk/rpc-api-versioning.md`.
4. Nếu hình dạng release evidence đổi, cập nhật `docs/release/release-pipeline.md`.
5. Nếu các nút điều chỉnh cho operator đổi, cập nhật tài liệu cấu hình node.
6. Chạy lại validation matrix trước khi merge.

Đừng nâng version runtime của EVM rồi ship ngay cùng lúc, trừ khi conformance suites, RPC smoke checks, và Docker deployment checks đều đã pass.

## Quy trình cập nhật

### 1. Chốt phạm vi thay đổi

Ghi rõ ý định cập nhật:

- chỉ fork behavior
- chỉ transaction admission
- chỉ execution semantics
- chỉ RPC compatibility
- chỉ xử lý blob / receipt / trace
- chỉ hành vi managed account hoặc wallet

Cách tách này giúp review tập trung và tránh kéo theo code không liên quan.

### 2. Sửa ở lớp hẹp nhất

Ưu tiên các ranh giới sau:

- `modules/evm/backend/geth` cho thay đổi tích hợp upstream go-ethereum
- `modules/evm/ethcompat` cho raw transaction decoding, giữ hash, và xử lý fixtures
- `modules/evm` cho state transition, receipts, logs, storage, và snapshot behavior
- `rpc` cho thay đổi bề mặt Web3 request/response
- `cmd/vexod` chỉ khi CLI hoặc release workflow phải hiển thị hành vi mới

Nếu thay đổi chạm tới application modules, hãy giữ rõ ranh giới module và duy trì deterministic state writes.

### 3. Làm mới default config

Khi semantics đổi, hãy cập nhật default config trong cùng patch:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- nếu cần, các field RPC cho managed account trong `network_config.json`
- EVM chain ID trong `module_config.json`

Đừng dựa vào hidden CLI flag để giải thích runtime behavior. File config phải đủ để thấy cách node hoạt động.

### 4. Chạy conformance stack

Tối thiểu hãy chạy:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Sau đó kiểm tra các luồng người dùng nhìn thấy thường hỏng đầu tiên:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Với Docker single-host deployment, hãy kiểm tra thêm:

```text
http://127.0.0.1:28657/web3
```

Hãy xác minh ít nhất các hành vi sau:

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

Sau đó hãy deploy một contract đơn giản, deploy proxy contract, và thử đường UUPS upgrade bằng cùng RPC endpoint mà wallet hoặc tool sẽ dùng trong production.

### 5. Xác nhận proxy và upgrade

EVM update chưa xong cho đến khi tất cả điều này đúng:

- deploy contract thường thành công
- deploy proxy thành công
- gọi UUPS upgrade thành công
- sau upgrade, đọc storage và code trả về đúng như mong đợi
- nonce tracking vẫn monotonic
- block producer chấp nhận các transaction sinh ra mà không có lỗi unsafe proposal

Nếu deploy proxy chạy được nhưng upgrade thất bại, vẫn chưa thể phát hành. Hãy xem đó là release blocker, không phải cảnh báo.

### 6. Làm mới evidence

Khi EVM surface thay đổi, hãy cập nhật luôn release evidence bundle:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- mọi tham chiếu SHA-256 đã được pin

Release evidence phải nói rõ đã đổi gì, đã test gì, và commit hoặc version nào đã được xác minh. Đừng nói một cập nhật EVM đã xong nếu evidence không khớp với code thực sự được chạy.

## Ma trận xác minh

Dùng bảng này làm merge gate.

| Check | Vì sao quan trọng |
| --- | --- |
| `make evm-conformance` | bắt regression về fork rule và execution |
| `go test ./modules/evm -count=1` | xác minh receipts, logs, storage, balances, và snapshots |
| `go test ./rpc -count=1` | xác minh tương thích Web3 request/response |
| `make network-e2e` | xác nhận node vẫn khởi động, có peers, và commit |
| Docker single-host smoke | xác nhận đường đi mà Remix và browser tools dùng |
| Contract deploy | xác nhận admission transaction và tạo receipt |
| Proxy deploy | xác nhận giả định ABI và storage layout |
| UUPS upgrade | xác nhận semantics upgrade và đọc sau upgrade |

Nếu có bất kỳ mục nào đỏ, đừng nói rằng cập nhật đã hoàn thành.

## Điều kiện rollback

Hãy rollback cập nhật EVM nếu xảy ra bất kỳ điều nào sau đây:

- `eth_chainId` thay đổi bất ngờ
- `eth_sendRawTransaction` bắt đầu từ chối transaction hợp lệ
- `eth_call` hoặc `eth_estimateGas` lệch khỏi fork rules mong đợi
- receipts, logs, hoặc proofs không còn khớp với committed state
- transaction proxy hoặc upgrade bắt đầu lỗi
- release evidence không còn khớp với đường code hiện tại

Rollback phải khôi phục cùng lúc adapter version tốt nhất đã biết, default config, và bộ fixture.

## Phụ lục đồng nhất kỹ thuật

Phụ lục này giữ cho hướng dẫn phù hợp với phần còn lại của cây tài liệu.

- Giữ `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc`, và `cmd/vexod` là các ranh giới triển khai ổn định.
- Giữ nguyên cách viết của `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction`, và `eth_sendTransaction`.
- Giữ nguyên cả `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures`, và `--evm-web3-conformance-evidence`.
- Câu hỏi vận hành vẫn rất đơn giản: bản cập nhật này có giữ được execution kiểu Ethereum mà vẫn phù hợp với Vexo consensus và release safety không?

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
