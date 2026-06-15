> Locale: vi · Tiếng Việt

# Tổng quan về Giao thức đồng thuận

Trang này là điểm truy cập cấp cao cho tài liệu đồng thuận của Vexo. Để có bản đồ tài liệu rộng hơn, hãy xem [Tài liệu](./README.md).

Để biết chi tiết quy chuẩn, hãy sử dụng các tệp thông số kỹ thuật:

- [Thông số đồng thuận](./specs/consensus-spec.md)
- [Định dạng bằng chứng cuối cùng](./specs/finality-proof-format.md)
- [Vòng đời của trình xác thực](./specs/validator-lifecycle.md)
- [Sơ đồ lưu trữ](./specs/storage-schema.md)
- [Thông số mạng](./specs/networking-spec.md)
- [Định dạng giao dịch](./specs/tx-format.md)

## Người mẫu

Vexo sử dụng lõi BFT kiểu HotStuff với các đề xuất, phiếu bầu, chứng chỉ đại biểu, chứng chỉ hết thời gian, an toàn QC bị khóa và tính hữu hạn ba chuỗi.

Một khối chỉ an toàn để bỏ phiếu khi nó mở rộng QC bị khóa hoặc mang QC biện minh ít nhất là mới như khóa. Một khối sẽ được hoàn thiện khi quy tắc ba chuỗi chứng minh sự mở rộng chuỗi cha mẹ/ông bà an toàn.

Việc triển khai liên kết quyết định ba chuỗi với các chiều cao rõ ràng của khối, cha mẹ và ông bà. QC khối phải chứng nhận chiều cao/băm gốc và QC gốc phải chứng nhận chiều cao/băm gốc; Chuỗi QC tổng hợp hoặc bỏ qua độ cao sẽ bị từ chối trước khi quyết định cuối cùng được ghi lại.

## Điều khoản thực hiện

Vexo sử dụng các thuật ngữ này một cách nhất quán:

- **QC Certified**: một khối có đủ số phiếu bầu để hình thành chứng chỉ đại biểu.
- **Hoàn thiện**: quy tắc ba chuỗi HotStuff hoàn thiện khối tổ tiên.
- **Đã thực thi**: ứng dụng đã chạy `FinalizeBlock` cho một khối.
- **Trạng thái đã cam kết**: ứng dụng ghi KV, bản ghi khối, bản ghi trạng thái và gốc trạng thái mô-đun đã được cam kết lâu dài.

Đường dẫn thực thi nút sử dụng hai ranh giới riêng biệt:

- **Ranh giới cam kết thực thi**: một khối được chứng nhận QC có thể được thực thi và tồn tại nguyên tử khi ghi ứng dụng + bản ghi khối + bản ghi trạng thái + gốc trạng thái.
- **Ranh giới cuối cùng đồng thuận**: quy tắc ba chuỗi hoàn thiện tổ tiên và là nguồn duy nhất cho các bằng chứng cuối cùng về client nhẹ.

`consensus_config.json` hiển thị lựa chọn này thông qua `execution_commit`. Các nhà xác thực được tạo mặc định là `finalized`, chỉ thực thi tổ tiên được chọn theo quy tắc cuối cùng ba chuỗi để trạng thái cam kết phù hợp với ranh giới cuối cùng chặt chẽ hơn. Ranh giới có độ trễ thấp hơn `qc` vẫn khả dụng cho việc triển khai tùy chỉnh nhưng `require_network_safety` từ chối ranh giới đó. Người vận hành và người dùng SDK nên coi nhật ký `block_committed` là sự kiện cam kết trạng thái cho ranh giới thực thi đã định cấu hình. Bằng chứng cuối cùng mô tả tính hữu hạn đồng thuận ở mức độ xác thực do người xác thực đặt.

## Ranh giới an toàn

An toàn phụ thuộc vào:

- ít hơn một phần ba quyền biểu quyết của Byzantine
- đề xuất, phiếu bầu, phiếu hết thời gian chờ và chữ ký cuối cùng được phân tách theo tên miền
- liên kết băm do trình xác nhận đặt ở độ cao bằng chứng có liên quan
- người ký duy nhất được biết đến trong QC và bằng chứng cuối cùng
- bằng chứng có trách nhiệm về sự không rõ ràng của người xác nhận
- từ chối các quyết định cam kết xung đột ở cùng độ cao cuối cùng

## Ranh giới tiền điện tử

- `deterministic` chỉ mang tính thử nghiệm và không xác thực được an toàn mạng.
- `ed25519` được hỗ trợ để thử nghiệm mạng công cộng và chuẩn bị ra mắt.
- `bls` mặc định là `blst-bls12381-minpk-v1` và yêu cầu bằng chứng sở hữu hoặc bảo vệ khóa lừa đảo tương đương, kiểm tra nhóm phụ, xác thực khóa công khai, bằng chứng kiểm tra sự phụ thuộc và bằng chứng về cổng phát hành. Bộ điều hợp CIRCL tích hợp vẫn là sự tích hợp tham chiếu cho giao diện thời gian chạy và không phải là sự từ bỏ an toàn sản xuất.
- Xác thực an toàn mạng yêu cầu siêu dữ liệu bộ điều hợp VRF để lựa chọn ủy ban VRF. Bộ điều hợp ECVRF tích hợp có thể đáp ứng giao diện thời gian chạy; VRF xác định vẫn chỉ ở dạng thử nghiệm và không nên được sử dụng cho các mạng mang lại giá trị.

## Ranh giới hoạt động

Mã này bao gồm các bước kiểm tra theo định hướng sản xuất, nhưng việc triển khai công khai vẫn yêu cầu:

- kiểm tra cấu hình nghiêm ngặt cho mọi nhà xác nhận
- bằng chứng về cổng phát hành
- đánh giá an ninh bên ngoài
- bằng chứng hỗn loạn và lâu dài trên nhiều máy chủ
- bằng chứng về chính sách của người ký/KMS
- đánh giá chính sách kinh tế và quản trị theo chuỗi cụ thể

Xem [Tính sẵn sàng kiểm tra bảo mật](./security/audit-readiness.md) và [Dòng phát hành](./release/release-pipeline.md) trước khi coi bản phát hành là sẵn sàng sản xuất.

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Model — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Execution Terms — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Safety Boundary — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Crypto Boundary — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Operational Boundary — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `consensus_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `execution_commit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `require_network_safety` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `block_committed` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blst-bls12381-minpk-v1` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
