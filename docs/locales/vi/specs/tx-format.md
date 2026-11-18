# Định dạng giao dịch

> Locale: vi · Tiếng Việt
> Đây là bản dịch trực tiếp sang tiếng Việt của nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.


## Thứ tự nên đọc

Tài liệu này giải thích đặc tả quy phạm của Tx Format. Nếu đây là lần đọc đầu, hãy theo thứ tự này.

1. Scope
2. Canonical Payload
3. Address Format
4. Signed Envelope
5. Required Ante Metadata
6. CheckTx Requirements
7. Fee and Gas
8. Load Test Payloads
9. CLI Examples

Thứ tự này khớp với cách đọc đúng: trước hết là phạm vi và trạng thái, sau đó là quy tắc message, safety và liveness, và cuối cùng là evidence.

## Tổng quan

Tài liệu này giúp hiểu quy tắc transaction format, signing, fee và gas và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/vi/specs/tx-format.md`

## Vì sao nên đọc tài liệu này

- quy tắc transaction format, signing, fee và gas
- Trước hết hãy kiểm tra các câu MUST/SHOULD/MAY trong nguồn tiếng Anh.
- Tài liệu bản địa hóa này hỗ trợ hiểu nội dung; audit, release và security decisions được quyết định theo nguồn tiếng Anh.

## Sau khi đọc cần làm được

- Giải thích tài liệu này hỗ trợ quyết định triển khai hoặc vận hành nào.
- Liên hệ yêu cầu chuẩn trong nguồn tiếng Anh với cấu hình mạng hiện tại.
- Kiểm tra chain ID, validator ID, fee/gas và địa chỉ peer trước khi sao chép ví dụ.

## Checklist sử dụng an toàn

- Trước hết hãy kiểm tra các câu MUST/SHOULD/MAY trong nguồn tiếng Anh.
- Không dịch lệnh, config key, tên RPC, trường JSON hoặc định danh mã.
- Trước khi sao chép ví dụ, hãy chỉnh chain ID, validator ID, fee/gas và địa chỉ peer theo mạng của bạn.
- Sau khi sửa tài liệu, chạy `make docs-check` để kiểm tra cây tài liệu bản địa và các kiểm tra dịch thuật.

## Điểm cần chú ý

- Tài liệu bản địa hóa này hỗ trợ hiểu nội dung; audit, release và security decisions được quyết định theo nguồn tiếng Anh.
- Khi implementation thay đổi, cập nhật nguồn tiếng Anh và tất cả tài liệu bản địa hóa trong cùng một thay đổi.

## Giao diện cần giữ nguyên

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`
- `CheckTx`
- `avxo`
- `gvxo`
- `base_fee`

## Cấu trúc nguồn tiếng Anh

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../../en/specs/tx-format.md)

## Envelope ký dạng nhị phân

Giao dịch đã ký là dữ liệu nhị phân (`txcodec.EncodeEnvelope`): tiền tố hai byte `0x89 'V'` (`txcodec.SignedPrefix`), byte phiên bản `1`, rồi bốn trường, mỗi trường có độ dài uvarint đứng trước: chain ID (tối đa `txcodec.MaxChainIDLength`, 128 byte), khóa công khai (tối đa `txcodec.MaxPublicKeyLength`), chữ ký (tối đa `txcodec.MaxSignatureLength`) và payload. Bộ giải mã từ chối độ dài không tối thiểu, trường vượt giới hạn, phiên bản không biết và byte thừa ở cuối, nên một giao dịch đã ký chỉ có một cách mã hóa. `0x89` không phải UTF-8 hợp lệ, nên không payload văn bản nào bị nhầm là envelope. Node từ chối khi khởi động chain ID dài hơn `txcodec.MaxChainIDLength` (`config.ErrChainIDTooLong`).

Các giao diện văn bản truyền giao dịch đã ký dưới dạng base64. `vexod keys sign-tx` in ra `tx: <base64>` và `encoding: base64`; `POST /v1/tx` đọc `tx` dạng base64 trừ khi yêu cầu đặt `"encoding": "plain"`; `vexod tx parse --encoding base64` đọc giao dịch đã ký, còn `--encoding plain` (mặc định) đọc payload văn bản. Cả ba giải mã qua `txcodec.DecodeTxEncoding`.

Ante handler chỉ ghi nhớ các lần xác thực thành công (`app.VerifyCache`, tối đa `app.VerifyCacheCapacity`), với khóa là digest của byte giao dịch và mọi thứ mà phán quyết phụ thuộc, như chain ID; thất bại không bao giờ được ghi nhớ. Chữ ký của một khối được kiểm tra song song trước khi khối được xử lý theo thứ tự.

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Scope — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Canonical Payload — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Address Format — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Signed Envelope — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Required Ante Metadata — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: CheckTx Requirements — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Fee and Gas — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Load Test Payloads — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: CLI Examples — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `gas_limit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm_chain_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `chain_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `base_fee` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `max(min_fee, base_fee * gas)` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blob_base_fee` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blob_gas` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blob_gas_fee_cap` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_sendRawBlobTransaction` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blob_hashes` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_getBlobSidecarByTxHash` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_getBlobSidecarByBlobHash` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_chainId` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `net_version` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_sendRawTransaction` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `dynamic_base_fee` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `target_gas` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `dynamic_blob_base_fee` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `target_blob_gas` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `bank:send` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `bank:send:<from>:<to>:<amount>` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `bank:mint:<to>:<amount>` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_raw` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_hash` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `max_txs` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `max_total_bytes` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `max_txs_per_sender` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
