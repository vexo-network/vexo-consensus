# EVM and Native Accounting

> Locale: vi · Tiếng Việt
> Tài liệu này là hướng dẫn dịch dựa trên tài liệu tiếng Anh chuẩn. Các quyết định về giao thức, bảo mật và phát hành vẫn lấy bản tiếng Anh làm chuẩn.

## Mục đích

Tài liệu này trình bày liên kết nhất quán native coin với EVM gas/accounting. Lệnh, trường JSON, tên RPC, config key và định danh mã dùng trong triển khai và vận hành được giữ bằng tiếng Anh để đảm bảo tương thích.

## Phạm vi chính

- Khi đọc tài liệu này, hãy kiểm tra các mục sau. Lệnh, trường JSON, phương thức RPC, khóa cấu hình và định danh mã được giữ nguyên tiếng Anh để đảm bảo tương thích.
- Đối với câu chữ mang tính quy phạm chi tiết, hãy dùng bản tiếng Anh.
- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/vi/specs/evm-native-accounting.md`

## Định danh cần giữ nguyên

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

## Mục trong bản tiếng Anh

- EVM and Native Accounting
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM Execution
- Compatibility Boundary
- Failure Modes

## Ghi chú vận hành

- `MUST`, `SHOULD`, `MAY`, ví dụ lệnh, ví dụ JSON và tên RPC giữ nguyên cách viết tiếng Anh.
- Sau khi sửa bản dịch này, hãy chạy `make docs-check`.
- Nếu trang này khác với nguồn tiếng Anh, hãy dùng nguồn tiếng Anh và cập nhật file locale này trong cùng thay đổi.

## Nguồn chuẩn

- [English canonical document](../../en/specs/evm-native-accounting.md)
