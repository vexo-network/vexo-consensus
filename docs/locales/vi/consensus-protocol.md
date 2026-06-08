# Consensus Protocol Overview

> Locale: vi · Tiếng Việt
> Tài liệu này là hướng dẫn dịch dựa trên tài liệu tiếng Anh chuẩn. Các quyết định về giao thức, bảo mật và phát hành vẫn lấy bản tiếng Anh làm chuẩn.

## Mục đích

Tài liệu này trình bày mô hình đồng thuận, thuật ngữ execution/finality và ranh giới an toàn. Lệnh, trường JSON, tên RPC, config key và định danh mã dùng trong triển khai và vận hành được giữ bằng tiếng Anh để đảm bảo tương thích.

## Phạm vi chính

- Khi đọc tài liệu này, hãy kiểm tra các mục sau. Lệnh, trường JSON, phương thức RPC, khóa cấu hình và định danh mã được giữ nguyên tiếng Anh để đảm bảo tương thích.
- Đối với câu chữ mang tính quy phạm chi tiết, hãy dùng bản tiếng Anh.
- Canonical path: `docs/consensus-protocol.md`
- Locale path: `docs/locales/vi/consensus-protocol.md`

## Định danh cần giữ nguyên

- `FinalizeBlock`
- `consensus_config.json`
- `execution_commit`
- `finalized`
- `qc`
- `require_network_safety`
- `block_committed`
- `deterministic`
- `ed25519`
- `bls`

## Mục trong bản tiếng Anh

- Consensus Protocol Overview
- Model
- Execution Terms
- Safety Boundary
- Crypto Boundary
- Operational Boundary

## Ghi chú vận hành

- `MUST`, `SHOULD`, `MAY`, ví dụ lệnh, ví dụ JSON và tên RPC giữ nguyên cách viết tiếng Anh.
- Sau khi sửa bản dịch này, hãy chạy `make docs-check`.
- Nếu trang này khác với nguồn tiếng Anh, hãy dùng nguồn tiếng Anh và cập nhật file locale này trong cùng thay đổi.

## Nguồn chuẩn

- [English canonical document](../en/consensus-protocol.md)
