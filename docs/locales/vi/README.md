> Locale: vi · Tiếng Việt

# Tài liệu

Thư mục này là hướng dẫn thực hành cho `vexo-consensus`. Nội dung dành cho developer, operator, người phụ trách release và reviewer cần hiểu mạng mà không suy đoán hành vi chỉ từ source code.

Mỗi trang phải giải thích trách nhiệm của component, file, command, config key và API triển khai nó, điều kiện an toàn và bằng chứng cần có trước mạng thực. Tiếng Anh vẫn là nguồn chuẩn tắc cho protocol, security, release, SDK, command, config và RPC; bản dịch hỗ trợ đọc nhưng không thay thế nguồn tiếng Anh trong quyết định audit.

Để bắt đầu, chạy các command bên dưới rồi đọc `Node Initialization`, `Docker Deployment`, `Observability Guide` và `RPC API Versioning`.

| Nhiệm vụ | Đường dẫn lệnh |
|---|---|
| Xây dựng hệ nhị phân cục bộ | __ VEXO_CODE_0__ |
| Tạo một nhà xác thực | `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` |
| Xác thực một nhà | __ VEXO_CODE_2__ và `vexod config audit --home .vexo-validator-1 --strict` |
| Chạy một nút | `vexod start --home .vexo-validator-1` |
| Truy vấn một nút | `curl -s http://127.0.0.1:26657/v1/status` |
| Chạy mạng bốn xác thực Docker | __ VEXO_CODE_5__ tiếp theo là __ VEXO_CODE_6__ |
| Connect Remix | Sử dụng trình xác thực Docker 1 URL Web3 `__ VEXO_URL_1__ |
| Kiểm tra ID chuỗi Web3 | `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` |

## Bắt đầu nhanh

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## Bắt đầu ở đây

| Tài liệu | Mục đích |
|---|---|
| [Hướng dẫn sẵn sàng sản xuất](./production-readiness.md) | Bản đồ duy nhất về giao thức, thời gian chạy, hoạt động, bằng chứng và mức độ sẵn sàng phát hành |

## Thông số kỹ thuật của giao thức

- [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md) và [Validator Lifecycle](./specs/validator-lifecycle.md) mô tả an toàn, finality và thay đổi validator set.
- [Networking Spec](./specs/networking-spec.md), [Storage Schema](./specs/storage-schema.md) và [Transaction Format](./specs/tx-format.md) bao phủ transport, durable recovery và transaction admission.
- [EVM and Native Accounting](./specs/evm-native-accounting.md) xác định ranh giới accounting native và EVM.

## SDK và mở rộng

[App Module Guide](./sdk/app-module-guide.md), [Custom Crypto Backend](./sdk/custom-crypto-backend.md), [Custom Storage and Transport](./sdk/custom-storage-transport.md) và `RPC API Versioning` giải thích cách mở rộng runtime mà không phá vỡ hợp đồng consensus hay RPC.

## Vận hành, release và bảo mật

`Node Initialization`, [Adding a Validator](./operators/add-validator.md), `Observability Guide`, [Sổ tay ra mắt](./release/launch-runbook.md), `Release Pipeline` và [Version Compatibility Matrix](./release/version-compatibility.md) tạo thành lộ trình operator. [Security Audit Readiness](./security/audit-readiness.md) ghi lại threat model và bằng chứng bắt buộc.

## Quy tắc trưởng thành

Chỉ có code không chứng minh sẵn sàng production. Cần unit, adversarial và E2E test, artefact vận hành, giả định, failure mode và kết quả release gate. Command, phương thức RPC và config key giữ nguyên trong mọi bản dịch.

## Nghiên cứu và công bố

Khi chuẩn bị bài báo, hãy bắt đầu với [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md). Tài liệu phân biệt các cơ chế đã thực sự được triển khai, gồm timeout vòng thích ứng, cổng finality khi phục hồi và thứ tự giao dịch tất định, với các công trình trước đó. Tài liệu tập hợp câu hỏi nghiên cứu, giả thuyết, quy trình thực nghiệm, hiện vật tái lập và nguyên tắc đạo đức nghiên cứu. Hiệu năng chưa được đo không được trình bày như kết quả, và PoS, BFT hay HotStuff không được tuyên bố là đóng góp mới.

Các tên tài liệu chuẩn được giữ nguyên để điều hướng xuyên ngôn ngữ gồm `Node Initialization`, `Docker Deployment`, `Observability Guide`, `RPC API Versioning`, `Production Readiness`, `Release Pipeline` và `Adaptive Recovery-Gated HotStuff Research Draft`.

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
