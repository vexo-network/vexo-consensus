# Lược đồ lưu trữ

> Locale: vi · Tiếng Việt
> Đây là bản dịch trực tiếp sang tiếng Việt của nguồn tiếng Anh. Các quyết định về giao thức, bảo mật và phát hành lấy bản tiếng Anh làm chuẩn.


## Thứ tự nên đọc

Tài liệu này giải thích đặc tả quy phạm của Storage Schema. Nếu đây là lần đọc đầu, hãy theo thứ tự này.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Thứ tự này khớp với cách đọc đúng: trước hết là phạm vi và trạng thái, sau đó là quy tắc message, safety và liveness, và cuối cùng là evidence.

## Tổng quan

Tài liệu này giúp hiểu namespace durable storage, key schema và recovery marker và liên hệ nội dung đó với quyết định triển khai, vận hành.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/vi/specs/storage-schema.md`

## Vì sao nên đọc tài liệu này

- namespace durable storage, key schema và recovery marker
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

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Cấu trúc nguồn tiếng Anh

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## Nguồn chuẩn

- [Tài liệu chuẩn tiếng Anh](../../en/specs/storage-schema.md)

## Backend Pebble và bản ghi nhị phân

Backend mặc định là Pebble (`github.com/cockroachdb/pebble/v2`), mở với block cache 256 MiB và memtable 64 MiB. Bản ghi được lưu bằng mã hóa nhị phân chuẩn của gói `types`: uvarint tối thiểu, chuỗi byte và danh sách có tiền tố độ dài, hash dưới dạng 32 byte của nó, giá trị tùy chọn sau một byte hiện diện `0` hoặc `1`, và các trường theo thứ tự của struct khai báo chúng. Bộ giải mã từ chối mọi thứ khác, kể cả byte thừa, nên bản ghi hỏng được báo lỗi thay vì bị đọc thành giá trị khác; mỗi bản ghi chỉ có một cách mã hóa. Thư mục dữ liệu do bản build trước (LevelDB, bản ghi JSON) ghi không đọc được và không được chuyển đổi: Pebble từ chối mở nó với lỗi `format major version 1`. Hãy khôi phục node như vậy từ snapshot hoặc đồng bộ vào một thư mục dữ liệu trống.

Định danh liên quan:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## Lịch sử state (undo log) và cache cây

Lịch sử state là undo log các phiên bản theo key. Khi một key thay đổi ở một độ cao (các lần ghi của khối ở độ cao của nó, ghi trực tiếp (`Set`, `Delete`, `SetBatch`) sau state committed đầu tiên ở độ cao committed kế tiếp), store ghi lại giá trị key có trước đó, kể cả vắng mặt, làm phiên bản của key ở độ cao đó, một lần cho mỗi key và độ cao; các lần ghi trực tiếp trước khối commit độ cao của chúng giữ phiên bản mà lần đầu tiên ghi lại (`hdirect` chỉ độ cao đó). Giá trị của key ở một độ cao committed là giá trị mà phiên bản thấp nhất của nó phía trên độ cao đó ghi lại, hoặc giá trị hiện tại nếu không có. Ghi một thay đổi chỉ đọc giá trị hiện tại của key, không bao giờ đọc lịch sử, và một lần đọc lịch sử là một lần seek vào các phiên bản của key. `PruneBelow` xóa mọi phiên bản tại hoặc dưới độ cao mà nó giữ từ đó. Node của cây được lưu phiên bản theo cùng cách. Hai cấu trúc trong bộ nhớ giảm công việc cây của mỗi khối; không cấu trúc nào được lưu bền vững hay thay đổi kết quả: cache node cây chứa tối đa 2^18 node đã mã hóa của các cây hiện tại (mọi batch của store cập nhật nó, và generation lẻ trong khi một batch như vậy commit khiến reader chỉ dùng cache ở state của snapshot của mình), và theo từng namespace, root và thay đổi node của lần tính root gần nhất, khóa theo generation đó và digest SHA-256 của các lần ghi của namespace, nên app hash, state roots và commit chỉ tính cùng các lần ghi một lần.

## Nén WAL của mempool và batch genesis

Commit chỉ nén WAL của mempool khi nó chứa ít nhất 4 MiB và gấp đôi lượng nó chứa sau lần nén hoặc mở gần nhất. Vì vậy mỗi byte được ghi thêm trung bình chỉ được ghi lại khoảng một lần, và log mà lần khởi động lại phát lại luôn nằm trong gấp đôi nội dung sống hoặc 4 MiB. Nén tường minh (`Node.Compact`, hoặc lần khởi động bỏ các giao dịch phát lại mà ứng dụng từ chối) luôn ghi lại nó. `InitChain` ghi state genesis của mọi module thành một batch, nên genesis nhiều tài khoản chỉ tốn một lần ghi bền vững và genesis thất bại không để lại gì trong store.

## Finality proof trong batch commit và việc xóa khỏi mempool được trì hoãn

Đường commit của node ghi các finality proof mà QC của block hoàn tất vào cùng batch block/state/app-write (`CommitBlockStateWithWrites` nhận chúng làm tham số cuối). Vì vậy một commit chỉ tốn một lần ghi durable thay vì thêm một lần cho mỗi proof. Proof mà store sẽ từ chối được để ngoài batch và do bước sau commit lưu, bước này báo lỗi. Commit xóa các giao dịch của mình khỏi pool và đưa record xóa vào hàng đợi mà không chờ sync. Commit kế tiếp chờ record đó durable rồi mới thực thi block của mình, vì khi khởi động lại node chỉ làm lại việc xóa cho block được commit cuối cùng. Record này thường đi cùng lần sync của một admission sau đó.

## Băm trì hoãn của state tree

Việc tính state tree trì hoãn việc băm: mỗi write chỉ đánh dấu các nửa nó đã đổi, và mỗi node bị đổi được băm một lần, từ dưới lên, khi root, proof hoặc các thay đổi được yêu cầu. Nhờ vậy các node gần root, vốn bị mọi write trong block ghi lại, chỉ được băm một lần mỗi block thay vì mỗi write. Hash, node và root vẫn giống như khi băm sau mỗi write.

## Sync định kỳ của mempool WAL

Mempool WAL ghi mỗi thay đổi vào log và sync mỗi 100 ms (`walSyncInterval`). Admission chỉ chờ việc ghi, không chờ sync. Khi tiến trình crash, mọi record vẫn còn; khi máy crash, chỉ mất tối đa các admission của khoảng cuối, và client sẽ gửi lại. Record xóa của block đã commit được sync trước commit kế tiếp (`WaitDurable`).

## Tái sử dụng lần đọc cho history

Commit ghi giá trị trước đó của key bị đổi làm version, thường không cần đọc lại: store nhớ các giá trị `Get` đã đọc kể từ lần commit batch gần nhất, tối đa 2^16, vì việc thực thi block đọc phần lớn các key nó thay đổi. Giá trị chỉ được dùng khi chưa có batch nào commit kể từ lúc đọc, nên đó là giá trị hiện tại; key không tồn tại không được nhớ và vẫn được đọc.

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Scope — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Backend — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Records — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Indexes — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: EVM Records — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Recovery Rules — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Snapshot Validation — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Schema Migration — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `store.Store` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_getBalance` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_getProof` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `bank/{0x_address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `auth/nonce/{0x_address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm/code/{0x_address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm/storage/{0x_address}/{slot}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `seen_ttl` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `code/{address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `storage/{address}/{slot}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `receipts/{tx_hash}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `logs/{address}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm_receipts` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `receipt_index/{tx_hash}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blob_sidecars/by_tx/{tx_hash}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blob_sidecars/by_blob/{blob_hash}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `tree:` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `receipt_height/{height}/{tx_hash}` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `receipt_pruned_below` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
