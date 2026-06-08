# Launch Runbook

> Locale: vi · Tiếng Việt
> Tài liệu này là hướng dẫn dịch dựa trên tài liệu tiếng Anh chuẩn. Các quyết định về giao thức, bảo mật và phát hành vẫn lấy bản tiếng Anh làm chuẩn.

## Mục đích

Tài liệu này trình bày checklist vận hành và quy trình trước khi khởi chạy mạng. Lệnh, trường JSON, tên RPC, config key và định danh mã dùng trong triển khai và vận hành được giữ bằng tiếng Anh để đảm bảo tương thích.

## Phạm vi chính

- Khi đọc tài liệu này, hãy kiểm tra các mục sau. Lệnh, trường JSON, phương thức RPC, khóa cấu hình và định danh mã được giữ nguyên tiếng Anh để đảm bảo tương thích.
- Đối với câu chữ mang tính quy phạm chi tiết, hãy dùng bản tiếng Anh.
- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/vi/release/launch-runbook.md`

## Định danh cần giữ nguyên

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `chain_id`

## Mục trong bản tiếng Anh

- Launch Runbook
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## Ghi chú vận hành

- `MUST`, `SHOULD`, `MAY`, ví dụ lệnh, ví dụ JSON và tên RPC giữ nguyên cách viết tiếng Anh.
- Sau khi sửa bản dịch này, hãy chạy `make docs-check`.
- Nếu trang này khác với nguồn tiếng Anh, hãy dùng nguồn tiếng Anh và cập nhật file locale này trong cùng thay đổi.

## Nguồn chuẩn

- [English canonical document](../../en/release/launch-runbook.md)
