# Tài liệu

> Locale: vi · Tiếng Việt
> Tài liệu này là tài liệu đồng hành tiếng Việt để đọc cùng nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.

## Bắt đầu nhanh

- Dùng `make build` để biên dịch binary.
- Tạo validator home bằng `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`, rồi kiểm tra bằng `vexod validate --home .vexo-validator-1` và `vexod config audit --home .vexo-validator-1 --strict`, sau đó chạy `vexod start --home .vexo-validator-1`.
- Chạy mạng Docker trước với `docker compose -f deployments/docker/compose.single-host-init.yml up`, rồi `docker compose -f deployments/docker/compose.single-host.yml up`.
- Remix phải trỏ tới `http://127.0.0.1:28657/web3`; để kiểm tra chain ID dùng `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`.
- Sau khi sửa tài liệu, chạy `make docs-check` để kiểm tra locale tree và translation guards.
## Tổng quan

Tài liệu này giúp hiểu mục lục tài liệu và thứ tự đọc khuyến nghị và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/vi/README.md`

## Vì sao nên đọc tài liệu này

- mục lục tài liệu và thứ tự đọc khuyến nghị
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

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## Cấu trúc nguồn tiếng Anh

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../en/README.md)

## Danh sách thuật ngữ giữ nguyên

Những thuật ngữ dưới đây không được dịch.

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: How to Read This Set — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Start Here — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Protocol Specs — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: SDK and Extension Guides — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Operations and Release — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Security — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Localized Documentation — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Writing New Docs — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Production Claim Rule — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Documentation Review Checklist — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `vexo-consensus` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/*` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make docs-check` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod status --json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `feature_assurance` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `network_config.json:p2p.auth_replay_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `network_config.json:p2p.node_key_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `module_config.json:governance.RequireDeposit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `module_config.json:governance.MinDeposit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `consensus_config.json:consensus.execution_commit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `mempool_config.json:mempool.WALPath` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
