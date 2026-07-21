> Locale: vi · Tiếng Việt

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- ít hơn một phần ba quyền biểu quyết của Byzantine
- đề xuất, bỏ phiếu, bỏ phiếu theo thời gian chờ và chữ ký cuối cùng được phân tách theo miền
- liên kết băm bộ xác thực ở độ cao bằng chứng có liên quan
- những người ký tên độc đáo được biết đến trong QC và bằng chứng cuối cùng
- bằng chứng chịu trách nhiệm về sự tương đương của người xác nhận
- từ chối các quyết định cam kết xung đột ở cùng độ cao đã hoàn thành

## Ranh giới tiền điện tử

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- kiểm tra cấu hình nghiêm ngặt cho mọi nhà xác thực
- bằng chứng cổng thông tin
- đánh giá bảo mật bên ngoài
- bằng chứng hỗn loạn và lâu dài về nhiều chủ nhà
- bằng chứng chính sách của người KÝ/KMS
- đánh giá chính sách kinh tế và quản trị cụ thể theo chuỗi

Xem [Security Audit Readiness](./security/audit-readiness.md) và [Release Pipeline](./release/release-pipeline.md) trước khi coi bản phát hành là sẵn sàng sản xuất.

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này tóm tắt những gì phải giữ nguyên so với bản tiếng Anh: giao diện có thể chạy, khóa cấu hình và ranh giới vận hành. Tên lệnh, đường dẫn RPC và mã định danh trong code không được dịch. Nội dung dưới đây giải thích ý nghĩa bằng tiếng Việt nhưng vẫn giữ nguyên các giá trị mà phần mềm và vận hành cần nhìn thấy chính xác.
`require_network_safety` và `block_committed` là các thuật ngữ quan trọng phải giữ nguyên.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### Theo dõi phần
- section: Model - HotStuff, three-chain finality, QC, timeout certificate và locked-QC safety phải được đọc cùng nhau.
- section: Execution Terms - QC certified, finalized, executed và state committed có ý nghĩa vận hành khác nhau.
- section: Safety Boundary - ít hơn 1/3 quyền Byzantine, domain separation, validator-set hash binding và accountable evidence là yêu cầu an toàn.
- section: Crypto Boundary - `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1` và `ecvrf-p256-sha256-tai-v1` phải được xử lý nhất quán.
- section: Operational Boundary - `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled` và snapshot/replay health là tín hiệu vận hành.

### Giao diện giữ nguyên
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`
