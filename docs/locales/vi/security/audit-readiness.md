# Chuẩn bị kiểm toán bảo mật

> Locale: vi · Tiếng Việt
> Đây là bản dịch trực tiếp sang tiếng Việt của nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.

## Tổng quan

Tài liệu này giúp hiểu threat model, giả định bảo mật và bằng chứng kiểm toán và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/vi/security/audit-readiness.md`

## Vì sao nên đọc tài liệu này

- threat model, giả định bảo mật và bằng chứng kiểm toán
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

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

## Cấu trúc nguồn tiếng Anh

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- Mục tiêu bảo mật
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

Hồ sơ audit phải có VRF adapter audit evidence ngoài BLS. Pin SHA-256 của file như `docs/security/ecvrf-audit-evidence.json` vào `vrf.audit_evidence_sha256` hoặc `--vrf-audit-sha256`, rồi kiểm tra dependency audit, key custody, TLS/mTLS hoặc pinned CA, auth, replay defense và service availability như một boundary duy nhất.

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../../en/security/audit-readiness.md)
- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`

## WAL của mempool và WAL đồng thuận

Mọi sự kiện tx hoặc batch được chấp nhận đều bền vững trước khi lệnh gửi trả về. Bản ghi có dạng `uvarint(length) | op | payload | CRC-32C`; các lần nhận đồng thời dùng chung một fsync (group commit), và node chờ nó mà không giữ khóa nhận. Replay cắt bỏ bản ghi cuối bị rách hoặc hỏng và từ chối log hỏng trước phần cuối (`ErrWALCorrupt`). Sau lỗi I/O, việc ghi thêm dừng cho đến khi log được ghi lại từ pool. WAL đồng thuận dùng cùng cách đóng khung (độ dài big-endian 4 byte, payload, CRC-32C 4 byte): đuôi bị rách được cắt, hỏng ở giữa bị từ chối.

## Record nhóm của consensus WAL và độ bền của việc xóa khỏi mempool

Một vote và safety state mà nó đưa máy trạng thái tới tạo thành một record nhóm, được ghi bằng một lần fsync và khi khôi phục thì được giữ trọn vẹn hoặc bỏ trọn vẹn. Record proposal không tốn fsync riêng mà được ghi cùng vote của proposer, trước khi proposal rời khỏi node. Record xóa khỏi mempool của block đã commit trở nên durable trước khi block kế tiếp được commit; khi khởi động lại, node làm lại việc xóa cho block được commit cuối cùng.

## Sync định kỳ của mempool WAL

Mempool WAL ghi mỗi thay đổi vào log và sync mỗi 100 ms (`walSyncInterval`). Admission chỉ chờ việc ghi, không chờ sync. Khi tiến trình crash, mọi record vẫn còn; khi máy crash, chỉ mất tối đa các admission của khoảng cuối, và client sẽ gửi lại. Record xóa của block đã commit được sync trước commit kế tiếp (`WaitDurable`).

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Scope — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Threat Model — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Security Assumptions — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Known Limitations — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Formal-ish Safety Argument — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Required Evidence for Audit — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Auditor Focus Areas — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Practical Audit Walkthrough — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Remote Signer Audit Notes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: EVM/Web3 Audit Notes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Snapshot and WAL Audit Notes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `docs/security/blst-audit-evidence.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `remote-vrf-http-v1` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod keys serve-vrf` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release collect-evidence` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/*` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `chain_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `go.mod` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/recovery/report` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `da_mismatch` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `missing_data` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `validator_set_hash` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `app_hash` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `finality.LightClient` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod light verify` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `next_validator_set_hash` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `parent_linkage` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `consensus.MaxBlockBytes`
