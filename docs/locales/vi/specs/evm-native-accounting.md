# EVM và kế toán native

> Locale: vi · Tiếng Việt
> Tài liệu này là tài liệu đồng hành tiếng Việt để đọc cùng nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.


## Thứ tự nên đọc

Tài liệu này giải thích đặc tả quy phạm của Evm Native Accounting. Nếu đây là lần đọc đầu, hãy theo thứ tự này.

1. Core Rule
2. Amount Encoding
3. Fee Accounting
4. EVM Execution
5. State Root Policy
6. Compatibility Boundary
7. Failure Modes

Thứ tự này khớp với cách đọc đúng: trước hết là phạm vi và trạng thái, sau đó là quy tắc message, safety và liveness, và cuối cùng là evidence.

## Tổng quan

Tài liệu này giúp hiểu liên kết nhất quán native coin với EVM gas/accounting và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/vi/specs/evm-native-accounting.md`

## Vì sao nên đọc tài liệu này

- liên kết nhất quán native coin với EVM gas/accounting
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

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`
- `bank query balance`

## Cấu trúc nguồn tiếng Anh

- EVM và kế toán native
- Core Rule
- Amount Encoding
- Fee Accounting
- Thực thi EVM
- State Root Policy
- Compatibility Boundary
- Failure Modes

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../../en/specs/evm-native-accounting.md)

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Core Rule — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Amount Encoding — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Fee Accounting — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: EVM Execution — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: State Root Policy — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Compatibility Boundary — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Failure Modes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `base_fee * gas` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `contract.Invocation` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `value_hex` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `gas_price_hex` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `max_fee_per_gas_hex` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `max_priority_fee_per_gas_hex` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_getBalance` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_sendRawBlobTransaction` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_sendRawBlobTransaction` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_sendRawTransaction` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `execution.strict_evm_state_root` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
