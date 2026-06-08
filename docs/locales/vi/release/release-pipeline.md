# Release Pipeline

> Locale: vi · Tiếng Việt
> Tài liệu này là hướng dẫn dịch dựa trên tài liệu tiếng Anh chuẩn. Các quyết định về giao thức, bảo mật và phát hành vẫn lấy bản tiếng Anh làm chuẩn.

## Mục đích

Tài liệu này trình bày pipeline phát hành với binary đã ký, checksums và SBOM. Lệnh, trường JSON, tên RPC, config key và định danh mã dùng trong triển khai và vận hành được giữ bằng tiếng Anh để đảm bảo tương thích.

## Phạm vi chính

- Khi đọc tài liệu này, hãy kiểm tra các mục sau. Lệnh, trường JSON, phương thức RPC, khóa cấu hình và định danh mã được giữ nguyên tiếng Anh để đảm bảo tương thích.
- Đối với câu chữ mang tính quy phạm chi tiết, hãy dùng bản tiếng Anh.
- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/vi/release/release-pipeline.md`

## Định danh cần giữ nguyên

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `CGO_ENABLED=0`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `make network-e2e`
- `RC_DRY_RUN=1`

## Mục trong bản tiếng Anh

- Release Pipeline
- Goals
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Launch Runbook

## Ghi chú vận hành

- `MUST`, `SHOULD`, `MAY`, ví dụ lệnh, ví dụ JSON và tên RPC giữ nguyên cách viết tiếng Anh.
- Sau khi sửa bản dịch này, hãy chạy `make docs-check`.
- Nếu trang này khác với nguồn tiếng Anh, hãy dùng nguồn tiếng Anh và cập nhật file locale này trong cùng thay đổi.

## Nguồn chuẩn

- [English canonical document](../../en/release/release-pipeline.md)
