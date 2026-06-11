# Sổ tay ra mắt

> Locale: vi · Tiếng Việt
> Tài liệu này là tài liệu đồng hành tiếng Việt để đọc cùng nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.

## Tổng quan

Tài liệu này giúp hiểu checklist vận hành và quy trình trước khi khởi chạy mạng và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/vi/release/launch-runbook.md`

## Vì sao nên đọc tài liệu này

- checklist vận hành và quy trình trước khi khởi chạy mạng
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

- `MaxScore`
- `release gate`
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--evm-default-fixtures`
- `chain_id`

## Cấu trúc nguồn tiếng Anh

- Sổ tay ra mắt
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## VRF audit evidence SHA-256

Khi kiểm tra release candidate, hãy truyền cả BLS và VRF audit evidence digest cho `release gate`. Tối thiểu dùng `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256` và `--evidence-manifest`, rồi xác nhận mọi evidence file khớp SHA-256 trong manifest.

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../../en/release/launch-runbook.md)
- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
