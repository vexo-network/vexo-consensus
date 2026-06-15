# Storage Schema

> Locale: vi · Tiếng Việt
> Tài liệu này là tài liệu đồng hành tiếng Việt để đọc cùng nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.


## Thứ tự nên đọc

Tài liệu này giải thích đặc tả quy phạm của Storage Schema. Nếu đây là lần đọc đầu, hãy theo thứ tự này.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Thứ tự này khớp với cách đọc đúng: trước hết là phạm vi và trạng thái, sau đó là quy tắc message, safety và liveness, và cuối cùng là evidence.

## Tổng quan

Tài liệu này giúp hiểu namespace durable storage, key schema và recovery marker và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/vi/specs/storage-schema.md`

## Vì sao nên đọc tài liệu này

- namespace durable storage, key schema và recovery marker
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
- Sau khi sửa tài liệu, chạy `make docs-check` để kiểm tra locale tree và translation guards.

## Điểm cần chú ý

- Tài liệu bản địa hóa này hỗ trợ hiểu nội dung; audit, release và security decisions được quyết định theo nguồn tiếng Anh.
- Khi implementation thay đổi, cập nhật nguồn tiếng Anh và tất cả tài liệu bản địa hóa trong cùng một thay đổi.

## Giao diện cần giữ nguyên

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
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Cấu trúc nguồn tiếng Anh

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

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../../en/specs/storage-schema.md)

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Scope — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Backend — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Records — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Indexes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: EVM Records — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Recovery Rules — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Snapshot Validation — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Schema Migration — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `store.Store` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm_ethstate` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_getBalance` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_getProof` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `bank/{0x_address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `auth/nonce/{0x_address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm/code/{0x_address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm/storage/{0x_address}/{slot}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm_ethstate/{height}/meta` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm_ethstate/{height}/accounts/{0x_address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm_ethstate/{height}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `seen_ttl` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `code/{address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `storage/{address}/{slot}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `receipts/{tx_hash}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `logs/{address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
