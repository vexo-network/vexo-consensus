# Quy trình phát hành

> Locale: vi · Tiếng Việt
> Tài liệu này là tài liệu đồng hành tiếng Việt để đọc cùng nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.


## Thứ tự nên đọc

Tài liệu này giải thích quy trình release và vận hành của Release Pipeline. Nếu đây là lần đọc đầu, hãy theo thứ tự này.

1. Goals
2. Release Commands
3. CI Gates
4. Evidence Quality Rules
5. Artifacts
6. Reproducibility Notes
7. Signed Binaries
8. SBOM
9. Audit Pack
10. Release Candidate Targets
11. Launch Runbook

Thứ tự này khớp với cách sử dụng thực tế: trước hết là mục tiêu và gate, sau đó là artifact và yêu cầu evidence, và cuối cùng là các bước thực thi.

## Tổng quan

Tài liệu này giúp hiểu pipeline phát hành với binary đã ký, checksums và SBOM và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/release/release-pipeline.md`
- Locale path: `docs/locales/vi/release/release-pipeline.md`

## Vì sao nên đọc tài liệu này

- pipeline phát hành với binary đã ký, checksums và SBOM
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

- `release gate`
- `ok`
- `status`
- `--allow-external-pending`
- `--private-rc`
- `dist/`
- `vexod-<version>-<os>-<arch>`
- `checksums.txt`
- `checksums.txt.asc`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--sdk-conformance-evidence`
- `--evm-web3-conformance-evidence`
- `evm_fixtures`
- `evm_execution`
- `web3_rpc`
- `evm_corpus`
- `RELEASE_CGO_ENABLED=1`
- `go build -trimpath`
- `BUILD_DATE`
- `release-candidate`
- `release-candidate-real`
- `release-candidate-smoke`
- `release-candidate-plan`
- `make release-portable RELEASE_REQUIRE_BLS=0`
- `make network-e2e`
- `RC_DRY_RUN=1`
- `network longrun`
- `allow_noop_migrations=true`
- `vexod upgrade apply --allow-empty-migrations`

## Cấu trúc nguồn tiếng Anh

- Release Pipeline
- Mục tiêus
- Release Commands
- Artifacts
- Reproducibility Notes
- Signed Binaries
- SBOM
- Audit Pack
- Release Candidate Soak Test
- Sổ tay ra mắt

## Bằng chứng tương thích EVM/Web3

`--sdk-conformance-evidence` và `--evm-web3-conformance-evidence` là hai bằng chứng riêng. Một dòng tóm tắt kiểu “EVM passed” là chưa đủ; bằng chứng EVM/Web3 phải có các phần máy đọc được `evm_fixtures`, `evm_execution`, `web3_rpc` và `evm_corpus`, rồi được gắn với `evidence-manifest.json` bằng SHA-256 trước mọi tuyên bố tương thích công khai.

## Chính sách release candidate

Release candidate công khai dùng mặc định `make release-candidate`. Target này là gate thật, đi vào `release-candidate-real` và yêu cầu `RELEASE_CGO_ENABLED=1` để artifact thực sự chứa adapter BLS `supranational/blst` dựa trên cgo. `make release-candidate-plan` chỉ dùng cho PR smoke và lập kế hoạch vận hành; nó dùng fixture tích hợp và dry-run plan, nên không được nộp làm evidence cuối cùng. Nếu cần artifact no-cgo, dùng `make release-portable RELEASE_REQUIRE_BLS=0`, nhưng không được công bố nó là release BLS-capable. Khi `RELEASE_CGO_ENABLED=1` và không đặt `RELEASE_TARGETS`, Makefile chỉ build target của host hiện tại. Nếu cần artifact cho nhiều OS/architecture, hãy đặt `RELEASE_TARGETS` rõ ràng trên runner có cgo cross-compiler phù hợp.

## VRF audit evidence SHA-256

`release gate` không chỉ pin BLS audit evidence; VRF audit evidence cũng phải được pin bằng SHA-256. File `--vrf-audit` phải nằm trong `evidence-manifest.json`, và `--vrf-audit-sha256` phải khớp chính xác với nội dung file. Khi dùng config, `vrf.audit_evidence_sha256` là digest pin mặc định. Quy tắc này buộc VRF service, KMS/HSM custody, TLS/mTLS hoặc pinned CA, auth token và nonce replay defense vào release evidence.

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../../en/release/release-pipeline.md)

## Thuật ngữ attestation cho bằng chứng release

Với bản phát hành công khai, mỗi mục trong `evidence-manifest.json` phải được xác minh bằng chữ ký Ed25519. Giữ nguyên các cờ CLI và trường JSON sau, không dịch.

- `--signing-key`
- `--signing-key-env`
- `signature_algorithm`
- `signature_public_key`
- `vexo-release-evidence-attestation-v1`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
- `vrf.audit_evidence_sha256`
<!-- vexo-docs-ops-update-2026-06 -->

## Đọc kết quả network E2E

`make network-e2e` không chỉ là build test: nó chạy 4 validators bằng binary thật và kiểm tra signed-shape smoke transaction, peer connection, height growth, và clean stop. `NETWORK_E2E_GO_TIMEOUT` là giới hạn Go test bên ngoài và phải lớn hơn network timeout bên trong.

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Goals — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Release Commands — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: CI Gates — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Evidence Quality Rules — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Artifacts — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Reproducibility Notes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Signed Binaries — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: SBOM — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Audit Pack — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Release Candidate Targets — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Launch Runbook — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `network analyze-longrun` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release collect-evidence` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `ops-runbook` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p-scale` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `state-sync-light-client` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `snapshot-replay` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make check` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make fuzz-smoke` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod consensus adversarial` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod ops conformance` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod network longrun` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod network chaos-plan` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make network-e2e` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make race` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `NETWORK_E2E_GO_TIMEOUT` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make test` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make vet` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make docs-check` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make build` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make release-candidate-smoke VERSION=ci`
- `make release-candidate-plan VERSION=ci` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make release-candidate VERSION=<rc> RELEASE_CGO_ENABLED=1 RC_EVM_CONFORMANCE_FLAGS=...` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evidence-manifest.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--allow-external-pending` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--private-rc` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo-release-evidence-attestation-v1` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release evidence-manifest` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--signing-key` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--signing-key-env` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `<evidence-file>.sig` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `<evidence-file>.sig.pub` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `<evidence-file>.pub` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `dist/` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod-<version>-<os>-<arch>` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `checksums.txt` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `checksums.txt.asc` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `sbom-go-modules.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `sbom-go-version.txt` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release-manifest.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release-audit-pack.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `longrun-analysis.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `docs-quality.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `RELEASE_CGO_ENABLED=1` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `supranational/blst` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `go build -trimpath` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `BUILD_DATE` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make release-candidate` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make release-portable RELEASE_REQUIRE_BLS=0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `RELEASE_TARGETS` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release-candidate` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release-candidate-real` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod ops conformance --strict` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `RC_EVM_CONFORMANCE_FLAGS` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `RC_LONGRUN_DURATION` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `release-candidate-plan` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `RELEASE_REQUIRE_BLS=0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `allow_noop_migrations=true` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod upgrade apply --allow-empty-migrations` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--bls-audit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--bls-audit-sha256` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--config <path>` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `crypto.audit_evidence_sha256` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--vrf-audit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--vrf-audit-sha256` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vrf.audit_evidence_sha256` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `docs/security/blst-audit-evidence.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `docs/security/ecvrf-audit-evidence.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
