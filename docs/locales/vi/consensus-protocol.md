> Locale: vi · Tiếng Việt

# Tổng quan giao thức đồng thuận

Đây là điểm vào cấp cao của tài liệu đồng thuận Vexo. Chi tiết chuẩn tắc nằm trong [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md) và [Transaction Format](./specs/tx-format.md).

## Mô hình

Vexo dùng lõi BFT kiểu HotStuff với proposal, vote, quorum certificate(QC), timeout certificate, an toàn locked-QC và finality ba chuỗi. Chỉ an toàn khi bỏ phiếu cho block nếu nó mở rộng locked QC hoặc mang justify QC ít nhất mới bằng lock. Chuỗi QC tổng hợp hoặc bỏ qua height mà không ràng buộc rõ height và hash của block, parent và grandparent sẽ bị từ chối trước quyết định finality.

## Danh tính giao thức và ranh giới nghiên cứu

Vexo không phải tên mới của HotStuff nguyên bản, cũng không phải cùng giao thức hoặc implementation với AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint hay CometBFT. Một Go runtime riêng kết hợp khái niệm an toàn HotStuff với thời gian vòng thích ứng, phục hồi bền vững, thứ tự giao dịch tất định, thực thi mô-đun và validator set được phiên bản hóa theo height.

Đường vote đang hoạt động dùng toàn bộ validator set của height và proposer tất định. VRF committee selector có trong component và query nhưng chưa quyết định proposal eligibility hoặc quorum formation. Vì vậy phải mô tả đây là nghiên cứu tương lai. Xem [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) về phạm vi đóng góp và quy trình thực nghiệm.

## Ranh giới thực thi và phục hồi

QC certification, HotStuff finalization, application execution và state commit là các sự kiện riêng. Mặc định `execution_commit=finalized` chỉ thực thi ancestor do quy tắc ba chuỗi chọn. Pacemaker thích ứng và `recovery_finality_gate_enabled` điều khiển độ trễ và phục hồi, không đổi proposer, quorum power, safe-vote hay finality.

## Ranh giới an toàn

- ít hơn một phần ba quyền biểu quyết của Byzantine
- đề xuất, bỏ phiếu, bỏ phiếu theo thời gian chờ và chữ ký cuối cùng được phân tách theo miền
- liên kết băm bộ xác thực ở độ cao bằng chứng có liên quan
- những người ký tên độc đáo được biết đến trong QC và bằng chứng cuối cùng
- bằng chứng chịu trách nhiệm về sự tương đương của người xác nhận
- từ chối các quyết định cam kết xung đột ở cùng độ cao đã hoàn thành

## Ranh giới tiền điện tử

- Backend `deterministic` chỉ dành cho thử nghiệm và không vượt qua kiểm tra network safety.
- `ed25519` được hỗ trợ cho thử nghiệm mạng công khai và chuẩn bị ra mắt.
- `bls` mặc định dùng `blst-bls12381-minpk-v1` và yêu cầu proof-of-possession, kiểm tra subgroup, xác thực public key, audit dependency và bằng chứng release-gate.
- Kiểm tra yêu cầu metadata VRF adapter, nhưng điều đó không có nghĩa VRF committee nằm trên đường đồng thuận đang hoạt động.

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
