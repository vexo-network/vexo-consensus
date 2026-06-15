> Locale: vi · Tiếng Việt

# Thêm trình xác thực

Hướng dẫn này mô tả quy trình vận hành để thêm trình xác nhận vào mạng Vexo.

Con đường tiếp nhận chính xác phụ thuộc vào chính sách quản trị và đặt cọc của chuỗi. Tối thiểu, trình xác thực phải được thể hiện ở trạng thái chuỗi, có thông tin xác thực hợp lệ và trở thành một phần của bản cập nhật bộ trình xác thực có phiên bản cao.

## 1. Khởi tạo Trang chủ Trình xác thực
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
Đối với khóa xác thực BLS:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
Đặt `VEXO_KEY_PASSPHRASE` trước khi chạy các lệnh này hoặc chuyển `--passphrase` để thiết lập cục bộ một lần.

Khi thừa nhận trình xác thực BLS vào chuỗi hiện có, hãy bao gồm siêu dữ liệu `bls_pop` đã tạo trong đề xuất cập nhật trình xác thực.
Đường dẫn khóa BLS mặc định sử dụng `blst-bls12381-minpk-v1`; chỉ sử dụng `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` để kiểm tra tính tương thích/tham chiếu.

Lưu trữ khóa công khai được tạo:
```bash
vexod keys show --home .vexo-validator-new --json
```
Đồng thời giữ `node.key.json` đã tạo. Nó ký kết bắt tay P2P cho `network_config.json:p2p.node_id`; nó không phải là khóa đồng thuận của người xác nhận và không được sử dụng lại làm khóa tài khoản.

## 2. Cấu hình địa chỉ mạng và các mạng ngang hàng

Chỉnh sửa `.vexo-validator-new/network_config.json` và đặt địa chỉ nghe cục bộ cộng với các địa chỉ ngang hàng liên tục:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
Không dựa vào các phần ghi đè mạng dòng lệnh tồn tại lâu dài cho trình xác thực sản xuất. Giữ các địa chỉ ngang hàng liên tục trong `network_config.json`.

Sử dụng vai trò địa chỉ riêng biệt:

- `p2p.listen_address` và `rpc.address` là địa chỉ liên kết cục bộ cho máy hoặc vùng chứa này.
- `p2p.node_id` là danh tính ngang hàng của nút này. Giữ nó ổn định sau khi các đồng nghiệp đã học được nó.
- `p2p.node_key_path` trỏ đến khóa ký kết bắt tay cục bộ cho danh tính ngang hàng đó.
- `p2p.peers` chứa các mục tiêu quay số mà nút này sử dụng để tiếp cận các nút ngang hàng khác; khóa bản đồ phải là giá trị `p2p.node_id` của các nút từ xa.
- siêu dữ liệu của trình xác thực `p2p_address` và `rpc_address` phải chứa các địa chỉ được quảng cáo công khai, không phải tên dịch vụ chỉ dành cho Docker, trừ khi mạng được đặt ở chế độ riêng tư có chủ ý.

## 3. Gửi Người xác nhận nhập học

Ví dụ: luồng đặt cược, xây dựng giao dịch đặt cược:
```bash
vexod staking --help
```
Giao dịch tiếp nhận trình xác thực phải bao gồm:

- ID người xác thực
- địa chỉ xác thực
- Khóa công khai đồng thuận
- quyền biểu quyết hoặc tham chiếu cổ phần
- điểm cơ bản hoa hồng xác thực, nếu chuỗi cho phép cập nhật hoa hồng tự phục vụ
- Siêu dữ liệu P2P `node_id` nếu chuỗi sử dụng siêu dữ liệu gốc/trình xác thực để chèn sẵn các bản đồ ngang hàng
- siêu dữ liệu địa chỉ P2P công cộng
- siêu dữ liệu địa chỉ RPC công khai, nếu công khai
- Siêu dữ liệu bằng chứng sở hữu BLS khi BLS được bật

Bản cập nhật trình xác thực phải có hiệu lực ở một độ cao cụ thể và tạo ra hàm băm do trình xác thực mới đặt.

Sau khi trình xác thực hoạt động, người vận hành có thể hiển thị trạng thái phần thưởng thông qua mô-đun đặt cược:
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. Xác minh cập nhật bộ trình xác thực

Sau khi cập nhật chiều cao:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
Kiểm tra:

- trình xác thực xuất hiện trong tập hợp chiều cao cụ thể
- quyền biểu quyết là chính xác
- hàm băm của bộ xác thực đã thay đổi như mong đợi
- bằng chứng cuối cùng tham chiếu chiều cao được xác thực chính xác

## 5. Lập kế hoạch xoay khóa trình xác thực

Bạn có thể xoay khóa trình xác thực bằng cách chuẩn bị tài liệu khóa tiếp theo với siêu dữ liệu `active_from` và `active_until` không chồng chéo, sau đó bắt đầu nút bằng khóa xoay bổ sung:
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
Tại thời điểm ký, nút sử dụng khóa có cửa sổ hoạt động chứa chiều cao đồng thuận. Các tài liệu chính của người ký từ xa giữ nguyên chính sách, mã thông báo xác thực và các yêu cầu bảo vệ ký kép.

## 6. Bắt đầu Trình xác thực
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
Khởi động không có chuyển đổi chế độ mạng. Sử dụng `config audit --strict` trước khi khởi động khi mạng dự kiến ​​sẽ đáp ứng các giả định về an toàn mạng công cộng.

## 7. Màn hình

Xem:

- độ trễ đề xuất/bỏ phiếu
- thời gian chờ của vòng
- lỗi ký xác thực
- lệnh cấm ngang hàng
- kích thước mempool
- độ trễ cam kết
- ảnh chụp nhanh / phát lại sức khỏe

sử dụng:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## Lưu ý an toàn

- Không bao giờ sử dụng lại khóa xác thực trên các chuỗi độc lập.
- Luôn bật chính sách người ký từ xa cho người xác nhận sản xuất.
- Không thừa nhận trình xác thực BLS mà không có bằng chứng sở hữu hoặc biện pháp bảo vệ khóa lừa đảo tương đương.
- Không chém hoặc bỏ tù người xác thực mà không có bằng chứng được xác minh gắn với bộ trình xác thực có chiều cao bằng chứng chính xác.

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: 1. Initialize Validator Home — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: 2. Configure Network Addresses and Peers — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: 3. Submit Validator Admission — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: 4. Verify Validator Set Update — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: 5. Plan Validator Key Rotation — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: 6. Start Validator — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: 7. Monitor — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Safety Notes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `VEXO_KEY_PASSPHRASE` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--passphrase` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `bls_pop` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blst-bls12381-minpk-v1` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `node.key.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `network_config.json:p2p.node_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `.vexo-validator-new/network_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `network_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.listen_address` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc.address` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.node_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.node_key_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.peers` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p_address` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc_address` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `node_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `active_from` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `active_until` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `config audit --strict` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
