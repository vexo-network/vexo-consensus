# Storage Schema

> Locale: vi · Tiếng Việt
> Tài liệu này là hướng dẫn dịch dựa trên tài liệu tiếng Anh chuẩn. Các quyết định về giao thức, bảo mật và phát hành vẫn lấy bản tiếng Anh làm chuẩn.

## Mục đích

Tài liệu này trình bày namespace durable storage, key schema và recovery marker. Lệnh, trường JSON, tên RPC, config key và định danh mã dùng trong triển khai và vận hành được giữ bằng tiếng Anh để đảm bảo tương thích.

## Phạm vi chính

- Khi đọc tài liệu này, hãy kiểm tra các mục sau. Lệnh, trường JSON, phương thức RPC, khóa cấu hình và định danh mã được giữ nguyên tiếng Anh để đảm bảo tương thích.
- Đối với câu chữ mang tính quy phạm chi tiết, hãy dùng bản tiếng Anh.
- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/vi/specs/storage-schema.md`

## Định danh cần giữ nguyên

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

## Mục trong bản tiếng Anh

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

## Ghi chú vận hành

- `MUST`, `SHOULD`, `MAY`, ví dụ lệnh, ví dụ JSON và tên RPC giữ nguyên cách viết tiếng Anh.
- Sau khi sửa bản dịch này, hãy chạy `make docs-check`.
- Nếu trang này khác với nguồn tiếng Anh, hãy dùng nguồn tiếng Anh và cập nhật file locale này trong cùng thay đổi.

## Nguồn chuẩn

- [English canonical document](../../en/specs/storage-schema.md)
