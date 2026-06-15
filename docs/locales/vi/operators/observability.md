> Locale: vi · Tiếng Việt

# Hướng dẫn quan sát

Hướng dẫn này giải thích cách nhận biết liệu nút Vexo có hoạt động tốt hay không dựa trên RPC, số liệu, nhật ký và bằng chứng phát hành.

Nó được viết cho những người vận hành cần các tín hiệu thực tế: cần xem gì, ý nghĩa của từng con số và khi nào một giá trị được coi là nguy hiểm.

## Sơ lược

Nếu một nút có vẻ sai, hãy kiểm tra các nút này theo thứ tự:

1. `running` và `latest_height` trong `/v1/status`
2. `latest_finalized_height` và số lượng ngang hàng
3. `round_timeout`, độ trễ đề xuất/bỏ phiếu, kích thước nhóm bộ nhớ và số liệu độ trễ cam kết
4. lỗi của người ký, tình trạng ảnh chụp nhanh và tình trạng phát lại
5. lệnh cấm ngang hàng và lỗi quay số ngang hàng

Thứ tự đó quan trọng vì nó tách biệt “quy trình đang hoạt động” với “chuỗi thực sự đang đạt được tiến bộ an toàn”.

## Điểm cuối cốt lõi

| Điểm cuối | Sử dụng |
|---|---|
| `/v1/status` | Quy trình nhanh, chiều cao, hàm băm ứng dụng, tính hữu hạn và tóm tắt ngang hàng |
| `/v1/metrics` | Số liệu JSON cho bảng thông tin và tự động hóa |
| `/metrics/text` | Số liệu văn bản tương thích với Prometheus |
| `/v1/diagnostics` | Kiểm tra tính sẵn sàng, khả năng, trạng thái, ngang hàng, lưu trữ và số liệu kết hợp |
| `/v1/finality/latest` | Bằng chứng cuối cùng mới nhất về kiểm tra an toàn và khách hàng nhẹ |
| `/v1/state/latest` | Ràng buộc trạng thái gốc và trình xác thực mới nhất |
| `/v1/recovery/report` | Chẩn đoán sự cố/khởi động lại tính nhất quán |
| `/v1/snapshot` | Ảnh chụp nhanh tình trạng và xuất siêu dữ liệu |

Các điểm cuối của quản trị viên như cắt tỉa, phát lại và kiểm soát đồng thuận thường chỉ có thể truy cập được thông qua loopback, mạng của nhà điều hành, mTLS hoặc cổng được xác thực. Mã thông báo quản trị có phạm vi vẫn là tùy chọn và được thực thi khi được định cấu hình.

## Đang đọc `/v1/status`

Các trường quan trọng:

| Lĩnh vực | Ý nghĩa | Lưu ý của nhà điều hành |
|---|---|---|
| `running` | Quá trình nút đã bắt đầu và sở hữu trạng thái thời gian chạy | `true` tự nó không chứng minh được sự đồng thuận trong sự sống động |
| `latest_height` | Chiều cao ứng dụng được cam kết cục bộ mới nhất | Phải tăng theo thời gian trên mạng xác thực trực tiếp |
| `latest_finalized_height` | Chiều cao hoàn thiện ba chuỗi HotStuff mới nhất | Không nên tụt hậu vô thời hạn so với chiều cao được thực hiện/đã cam kết |
| `latest_app_hash` | Băm cam kết ứng dụng | Nên kết hợp với các bạn cùng lứa tuổi |
| `peer_count` | Tóm tắt ngang hàng được kết nối/chấm điểm tương thích ngược | Thích các trường ngang hàng cụ thể hơn bên dưới |
| `active_peer_count` | Các phiên vận chuyển đang hoạt động, khi quá trình vận chuyển có thể báo cáo chúng | Tín hiệu nhanh nhất cho kết nối P2P trực tiếp |
| `configured_peer_count` | Địa chỉ ngang hàng được định cấu hình hoặc học được | Khả năng tiếp cận không được đảm bảo |
| `scored_peer_count` | Đồng nghiệp biết đến bảng điểm | Hữu ích cho lịch sử cấm/giới hạn tỷ lệ, không phải bằng chứng về các phiên trực tiếp |
| `banned_peers` | Các đồng nghiệp hiện bị cấm theo chính sách tính điểm | Mức tăng đột biến biểu thị sự tấn công, cấu hình ngang hàng kém hoặc giới hạn quá nghiêm ngặt |

Ví dụ lành mạnh cho mạng một máy chủ có 4 trình xác thực: `running=true`, `latest_height` tăng, `latest_finalized_height` hiện tại, `active_peer_count` gần `3` và `banned_peers=0`.

## Số liệu của Prometheus

Điểm cuối văn bản hiển thị các thước đo như:

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`

`vexo_peer_count` được giữ lại cho các trang tổng quan cũ hơn. Trang tổng quan mới phải lập biểu đồ `vexo_active_peer_count`, `vexo_configured_peer_count` và `vexo_scored_peer_count` riêng biệt.

## Quy tắc cảnh báo được đề xuất

Điều chỉnh số lượng trình xác thực thực tế, khoảng thời gian chặn, độ trễ và phần cứng. Đây là những điểm bắt đầu, không phải là hằng số phổ quát.

| Cảnh báo | Điều kiện bắt đầu | Tại sao |
|---|---|---|
| Nút xuống | `vexo_node_running == 0` trong 1 phút | Quá trình/thời gian chạy đã dừng |
| Chiều cao bị đình trệ | `latest_height` không thay đổi trong 2-3 khoảng thời gian chặn dự kiến ​​| Sự đồng thuận hoặc việc thực thi bị đình trệ |
| Cuối cùng bị đình trệ | `latest_finalized_height` không thay đổi trong khi các khối tiếp tục thực thi | Vấn đề về đường dẫn cuối cùng hoặc số đại biểu |
| Không có đồng nghiệp tích cực | `vexo_active_peer_count == 0` trong 1 phút trên nút không bị cô lập | P2P ngừng hoạt động, xác thực không khớp hoặc vấn đề về địa chỉ |
| Số lượng ngang hàng quá thấp | các đồng nghiệp tích cực dưới mục tiêu kết nối đại biểu | Vấn đề về phân vùng hoặc bootstrap |
| Tăng đột biến thời gian chờ tròn | bộ đếm thời gian chờ tăng nhanh hơn mức cơ bản bình thường | Độ trễ, lỗi của người đề xuất hoặc phân vùng mạng |
| Cam kết độ trễ cao | p95/p99 tiếp cận ngân sách hết thời gian đồng thuận | Quá tải lưu trữ/thời gian chạy |
| Áp lực Mempool | kích thước mempool tăng lên trong vài phút | Chính sách phí, thư rác hoặc vấn đề về dung lượng chặn |
| Ảnh chụp không tốt | `vexo_snapshot_healthy == 0` | Rủi ro đồng bộ/khôi phục trạng thái |
| Phát lại không lành mạnh | `vexo_replay_healthy == 0` | Tính quyết định hoặc rủi ro nhất quán trạng thái |
| Người ký thất bại | `vexo_validator_signing_failures > 0` | KMS/người ký từ xa/lỗi chính sách |
| Hòa giải thất bại | `vexo_post_commit_reconciliation_failures > 0` | Bằng chứng lâu dài hoặc cam kết sửa chữa cần thiết |
| Bị cấm ngang hàng | đồng nghiệp bị cấm tăng đột ngột | Tấn công, cấu hình sai các đồng nghiệp hoặc vấn đề về ngưỡng ghi điểm |

## Ngưỡng bắt đầu được đề xuất

Sử dụng các giá trị này làm giá trị cảnh báo ban đầu, sau đó điều chỉnh theo đường cơ sở dài hạn thực sự:

| Tín hiệu | Cảnh báo | Quan trọng | Hành động đầu tiên |
|---|---:|---:|---|
| Tỷ lệ chiều cao | dưới 50% dự kiến ​​cho 2 cửa sổ | tăng trưởng bằng không trong khoảng thời gian 2-3 khối | so sánh tất cả các trình xác nhận, kiểm tra nhật ký của người đề xuất/ký/ngang hàng |
| Độ trễ chiều cao cuối cùng | phát triển trong 5 phút | tăng trong khi chiều cao thực hiện tiếp tục tăng trong 10 phút | kiểm tra nhật ký chứng minh QC/cuối cùng và hàm băm của trình xác thực |
| Đồng nghiệp tích cực | dưới mục tiêu kết nối đại biểu | không có đồng nghiệp hoạt động nào | kiểm tra địa chỉ được quảng cáo, TLS/auth, ID gốc/chuỗi không khớp |
| Thời gian chờ của vòng | 3x đường cơ sở bình thường | vòng lặp hết thời gian liên tục | tăng ngân sách thời gian chờ hoặc điều tra độ trễ/phân vùng |
| Độ trễ đề xuất p95 | trên 50% `timeout_propose` | trên 80% `timeout_propose` | người đề xuất hồ sơ, mempool, cam kết DA, đĩa |
| Độ trễ bình chọn p95 | trên 50% ngân sách bỏ phiếu trước/cam kết trước | trên 80% ngân sách | kiểm tra CPU, người ký, vận chuyển, tin đồn ngược áp |
| Cam kết độ trễ p95 | trên 50% khoảng thời gian chặn | trên 80% khoảng thời gian chặn | kiểm tra LevelDB, gốc trạng thái, thực thi EVM, ảnh chụp nhanh |
| Kích thước nhóm | tăng trong 5 phút | gần `max_txs` hoặc thay thế liên tục | kiểm tra phí cơ sở, phí tối thiểu, tính hợp lệ của tx, thư rác |
| Người ký thất bại | mọi giá trị khác 0 | lỗi lặp đi lặp lại trong một cửa sổ chiều cao | dừng trình xác thực nếu xuất hiện dấu bảo vệ kép hoặc khóa không khớp |
| Ảnh chụp sức khỏe | một lần kiểm tra thất bại | xuất/xác minh/khôi phục không thành công nhiều lần | tạm dừng cung cấp đồng bộ hóa trạng thái và chạy báo cáo khôi phục |
| Phát lại sức khỏe | một lỗi phát lại nghiêm ngặt | phát lại không khớp ở độ cao an toàn mới nhất | bảo vệ thư mục dữ liệu và tạm dừng nâng cấp/phát hành không an toàn |
| Đồng nghiệp bị cấm | đột ngột | nhiều đồng nghiệp bị cấm sau khi triển khai cấu hình | kiểm tra giới hạn điểm, TLS CA, danh tính ngang hàng, bằng chứng xác thực tùy chọn và độ lệch đồng hồ |

Quy tắc quan trọng nhất: cảnh báo về **thay đổi theo thời gian**. Một con số có thể gây hiểu nhầm; tỷ lệ chiều cao, độ trễ tài chính, tỷ lệ rời bỏ ngang hàng, tốc độ tăng trưởng của mempool và lỗi của người ký cùng nhau kể lại câu chuyện có thật.

## Ma trận phân loại sự cố

| Tình huống | Lớp có khả năng | Bảo quản những gì | Bước tiếp theo an toàn |
|---|---|---|---|
| Chiều cao chững lại, đồng nghiệp khỏe mạnh | sự đồng thuận/người ký/thời gian chạy | nhật ký đồng thuận, nhật ký người ký, mẫu mempool | xác minh khóa người đề xuất và nhật ký hết thời gian chờ |
| Các thiết bị ngang hàng bị rớt sau khi triển khai | mạng/cấu hình | cấu hình mạng, chứng chỉ TLS, addrbook, nhật ký ngang hàng | khôi phục địa chỉ được quảng cáo/TLS/thay đổi xác thực |
| Băm ứng dụng khác nhau ở cùng độ cao | thực thi/lưu trữ | thư mục dữ liệu, bản ghi khối, nhật ký ứng dụng, đầu ra phát lại | tạm dừng các nút bị ảnh hưởng và chạy phát lại nghiêm ngặt |
| Bằng chứng cuối cùng bị từ chối | bộ tài chính/trình xác nhận | JSON bằng chứng, trình xác thực được đặt ở độ cao bằng chứng | xác minh hàm băm do trình xác thực thiết lập và ký tên miền byte |
| Khôi phục ảnh chụp nhanh không thành công | đồng bộ/lưu trữ trạng thái | tập tin chụp nhanh, tổng kiểm tra, gốc trạng thái, nhật ký khôi phục | không thử lại với dữ liệu trực tiếp; khôi phục vào thư mục sạch |
| Người ký từ xa từ chối yêu cầu | quyền giám hộ chìa khóa | nhật ký kiểm tra người ký, tệp bảo vệ, tệp nonce, nhật ký nút | phân biệt việc từ chối chính sách với việc ngừng vận chuyển |
| Các đồng nghiệp bị cấm tăng đột biến | P2P/bảo mật | ảnh chụp nhanh điểm ngang hàng và lý do cấm | kiểm tra tin đồn không đúng định dạng hoặc chia sẻ cấu hình sai |

Trong trường hợp xảy ra sự cố, hãy ưu tiên bảo toàn dữ liệu hơn là “dọn dẹp”. Việc xóa WAL, sổ bổ sung, bộ bảo vệ người ký hoặc thư mục LevelDB có thể phá hủy bằng chứng cần thiết để phân biệt lỗi với lỗi của người vận hành.

## Ghi lại các sự kiện cần lưu giữ

Nhật ký có cấu trúc phải được giữ lại với ID nút, ID trình xác thực, ID chuỗi, chiều cao, vòng, hàm băm khối và ID ngang hàng nếu có liên quan.

Sự kiện quan trọng:

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

Đối với các ứng cử viên phát hành, hãy lưu trữ nhật ký cùng với các mẫu số liệu, mẫu pprof, tệp cấu hình, nguồn gốc, tổng kiểm tra nhị phân và bảng kê khai bằng chứng.

## Cẩm nang phản hồi đầu tiên

Khi người vận hành thấy có vấn đề:

1. Kiểm tra `/v1/status` trên ít nhất hai trình xác thực.
2. So sánh `latest_height`, `latest_finalized_height`, `latest_app_hash` và số lượng ngang hàng.
3. Kiểm tra `/v1/diagnostics` để biết các khả năng bị thiếu hoặc kiểm tra bộ nhớ/phát lại/ảnh chụp nhanh không lành mạnh.
4. Kiểm tra nhật ký sự kiện ngang hàng để tìm lỗi xác thực, TLS, nguồn gốc, ID chuỗi hoặc lỗi chờ.
5. Kiểm tra số liệu mempool và phí cơ sở nếu không bao gồm tx.
6. Xác minh nhật ký người ký và người ký từ xa nếu chữ ký của người xác thực không thành công.
7. Xuất báo cáo khôi phục trước khi xóa hoặc sửa đổi dữ liệu.
8. Nếu nghi ngờ có xung đột về mục đích cuối cùng, hãy ngừng tự động hóa, lưu giữ nhật ký/bằng chứng và chạy tính năng phát hiện xung đột về mục đích cuối cùng.

## Bố cục trang tổng quan

Một bảng thông tin hữu ích thường có năm hàng:

1. **Tính sống động**: nút đang chạy, chiều cao mới nhất, chiều cao cuối cùng, tỷ lệ chiều cao.
2. **Độ trễ đồng thuận**: hết thời gian chờ, đề xuất/bỏ phiếu/cam kết p95 và p99.
3. **Mạng**: các ngang hàng đang hoạt động/được định cấu hình/được tính điểm, các ngang hàng bị cấm, các thông báo trên cửa sổ ngang hàng.
4. **Thực thi**: kích thước mempool, phí gas/base, số lượng tx, độ trễ cam kết.
5. **Phục hồi và an toàn**: tình trạng ảnh chụp nhanh, tình trạng phát lại, lỗi của người ký, lỗi đối chiếu.

Giữ cho bảng điều khiển nhàm chán. Mục tiêu không phải là hiển thị mọi bộ đếm nội bộ; mục đích là làm cho các trạng thái nguy hiểm trở nên rõ ràng trước khi người xác thực phân kỳ hoặc người dùng nhận thấy các giao dịch bị đình trệ.

## Đưa ra bằng chứng từ khả năng quan sát được

Đối với một ứng cử viên phát hành, khả năng quan sát không chỉ là giám sát trực tiếp. Nó trở thành bằng chứng:

1. Thu thập đường cơ sở `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest` và `/v1/recovery/report` từ mọi trình xác thực.
2. Chạy tải trong khoảng thời gian và tốc độ đã chọn.
3. Thực hiện ít nhất một lần khởi động lại, một lần gián đoạn ngang hàng và một lần diễn tập xuất/xác minh/khôi phục ảnh chụp nhanh.
4. Thu thập số liệu cuối cùng từ mọi người xác thực.
5. Lưu trữ các mẫu trước/sau, nhật ký, mẫu pprof, nhật ký kiểm tra người ký và bảng kê khai bằng chứng trong `dist/`.

Một gói bằng chứng tốt cho phép người đánh giá trả lời: chiều cao có tăng lên không, tiến độ cuối cùng có phát triển không, các đồng nghiệp có phục hồi không, txs có cam kết không, ảnh chụp nhanh có xác minh không, phát lại có hoạt động tốt không, người ký có tránh ký hai lần không và liệu nhị phân phát hành chính xác có tạo ra kết quả không?

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Core Endpoints — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Reading `/v1/status` — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Prometheus Metrics — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Suggested Alert Rules — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Suggested Starting Thresholds — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Incident Triage Matrix — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Log Events to Keep — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: First Response Playbook — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Dashboard Layout — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Release Evidence From Observability — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `/v1/status` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/metrics` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/metrics/text` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/diagnostics` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/finality/latest` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/state/latest` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/recovery/report` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `/v1/snapshot` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `latest_height` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `latest_finalized_height` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `latest_app_hash` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `active_peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `configured_peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `scored_peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `banned_peers` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `banned_peers=0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_node_running` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_latest_height` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_active_peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_configured_peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_scored_peer_count` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_banned_peers` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_height_rate_per_minute` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_round_timeouts` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_proposal_latency_p95_nanos` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_vote_latency_p95_nanos` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_commit_latency_p95_nanos` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_mempool_size` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_snapshot_healthy` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_replay_healthy` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_validator_signing_failures` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_post_commit_reconciliation_failures` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_node_running == 0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_active_peer_count == 0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_snapshot_healthy == 0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_replay_healthy == 0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_validator_signing_failures > 0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexo_post_commit_reconciliation_failures > 0` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `timeout_propose` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `max_txs` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `node_running` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc_listening` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p_listening` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `peer_configured` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `peer_connected` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `peer_disconnected` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `peer_dial_failed` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `peer_banned` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `consensus_loop_running` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `block_committed` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `round_timeout` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `validator_signing_failure` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evidence_received` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evidence_applied` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `snapshot_exported` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `replay_checked` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `upgrade_halt` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `upgrade_applied` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `dist/` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
