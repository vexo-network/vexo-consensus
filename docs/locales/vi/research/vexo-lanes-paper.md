# Vexo Lanes: phát tán lô được chứng nhận cho HotStuff two-chain có trọng số theo stake

> Locale: vi · Tiếng Việt  
> Loại tài liệu: bản thảo nghiên cứu và giao thức tái lập  
> Trạng thái: bản nháp dựa trên mã nguồn; kết quả được đo trên một máy với liên kết giả lập và phải được lặp lại trên mạng thật trước khi công bố

## Tóm tắt

Một giao thức BFT dựa trên leader thuộc họ HotStuff bị giới hạn bởi đường lên của leader: mọi giao dịch trong khối rời một máy một lần cho mỗi validator, nên thông lượng giảm theo `1/(n−1)` trong khi đường truyền của các validator khác để trống. Vexo Lanes chuyển việc phát tán ra khỏi leader mà không thay giao thức sắp thứ tự. Mỗi validator niêm phong các giao dịch nó sở hữu thành các lô có chữ ký trên làn của mình; các validator nắm hai phần ba stake xác nhận đã lưu, khiến lô được chứng nhận khả dụng; leader đề xuất khối bằng cách nêu tên các lô đã chứng nhận. Định dạng khối, hash, gốc giao dịch, các byte được ký, quy tắc bỏ phiếu và quy tắc commit two-chain đều không đổi.

Trên một máy với liên kết giả lập 100 Mbit/s, 25 ms cho mỗi validator, Lanes commit 60.080, 45.323 và 37.099 tx/s với 4, 7 và 10 validator, so với 21.073, 10.909 và 8.297 tx/s khi cùng nút phát cả khối: 2,9×, 4,2× và 4,5×, khoảng cách tăng theo `n`. Dưới công suất (4.000 tx/s), Lanes thêm 35 ms độ trễ trung vị với 4 validator và bớt 25 ms, 130 ms với 7 và 10. Khi không giới hạn băng thông, Lanes mất 4% thông lượng. Quá trình đánh giá phát hiện chín lỗi, trong đó có lịch leader khiến chỉ validator chỉ số chẵn đề xuất khi số validator là chẵn.

## 1. Câu hỏi nghiên cứu

Có thể thêm phát tán lô được chứng nhận bên dưới một giao thức HotStuff có trọng số sẵn có, mà không đổi đối tượng bỏ phiếu và cách commit khối, để thông lượng tỷ lệ với tổng đường lên của mọi validator thay vì của leader không? Các câu hỏi con xét nút thắt leader (RQ1), khả năng mở rộng (RQ2), việc giữ an toàn, thứ tự nonce và khả năng chống kiểm duyệt (RQ3) và chi phí khi băng thông dồi dào (RQ4). Giả thuyết: H1, lượng tải lên lớn nhất của đường cơ sở trên mỗi giao dịch tăng tuyến tính theo `n` còn của Lanes gần kích thước giao dịch; H2, mức tăng tốc tăng theo `n`; H3, khi không giới hạn liên kết Lanes không nhanh hơn và tốn tối đa một vòng khứ hồi.

## 2. Công trình trước và ranh giới mới

Tách phát tán khỏi sắp thứ tự là hướng đã có: Narwhal và Tusk, Bullshark và Shoal, Stratus, Mir-BFT và ISS, Autobahn, Quorum Store của Aptos. Bản thảo không tuyên bố sự tách này, chứng chỉ khả dụng, làn theo validator hay kết quả WAN thật là mới. Đóng góp mang tính kỹ thuật: cải tạo giữ nguyên từng bit khối, header, chữ ký và quy tắc commit; quyền sở hữu làn theo người gửi kèm cơ chế dự phòng chống kiểm duyệt; xác thực đề xuất trước mọi lần tải; và bộ đo tái lập được cùng các lỗi nó phát hiện.

## 3. Mô hình hệ thống

`n` validator có quyền biểu quyết theo stake; validator Byzantine nắm ít hơn một phần ba. Quorum là `>= 2/3` quyền biểu quyết. Đồng bộ một phần, kênh được xác thực, chữ ký tách miền. Validator có thể sập, ký mâu thuẫn, giữ lô, làm ngập làn của mình hoặc gửi đề xuất giả.

## 4. Giao thức

**Làn và chứng chỉ.** Mỗi validator niêm phong giao dịch thành `lanes.Batch` khi đạt 512 KiB hoặc 5 ms sau giao dịch đầu, giới hạn bởi `lanes.MaxBatchTxs` và `lanes.MaxBatchBytes`. Làn tải nhẹ, có lô trước nhỏ hơn 16 KiB, còn chờ chứng chỉ của lô đó, tối đa 50 ms. Validator xác nhận trong miền `DomainLaneBatchAck`; xác nhận của `>= 2/3` quyền biểu quyết tạo thành `BatchCert`, nên ít nhất một validator trung thực giữ lô. Gửi lại dùng backoff lũy thừa.

**Sở hữu và dự phòng.** Giao dịch thuộc làn `lanes.Owner(members, sender)`. Mọi giao dịch của một người gửi đi trên một làn theo thứ tự tiếp nhận, nên làn không đảo thứ tự nonce. Nếu sau `lanes.FallbackDelay` (3 giây) không lô nào đã lưu chứa giao dịch của làn khác, người giữ sẽ tự đóng lô; chủ làn lỗi chỉ có thể làm chậm chứ không kiểm duyệt được.

**Đề xuất theo làn.** Leader chỉ lấy lô đã chứng nhận, theo thứ tự mỗi làn đến trước lô chưa chứng nhận đầu tiên, luân phiên các làn đến giới hạn khối, ký khối như trước và phát `lane_proposal` gồm ID lô, vị trí bị từ chối (`Excluded`) và giao dịch ngoài lô (`Extra`).

**Xác thực trước khi tải.** Bên nhận kiểm tra leader và chữ ký trước khi tải bất cứ thứ gì. Nếu tập validator phụ thuộc vào khối chưa thực thi, đề xuất được giữ lại (tối đa 16) và được xét lại sau lần thực thi kế tiếp. Khối được dựng lại từ các lô đã lưu và chỉ được xét khi hash và gốc khớp header đã ký; nếu không, validator yêu cầu toàn bộ `proposal` bằng `proposal_request`.

**Lịch leader theo tỷ lệ stake.** Leader của view `v` là `validator.SelectProposer(validators, v)`: validator sở hữu `VotingPower` ô liên tiếp trong `[0, W)`, view `v` ứng với ô `v·S mod W`, bước là số đầu tiên từ `ceil(W·(φ−1))` nguyên tố cùng nhau với `W`. Mọi cửa sổ `W` view cho mỗi validator đúng phần của nó. Lịch chỉ phụ thuộc vào view.

**Giới hạn tài nguyên.** 64 MiB mỗi tác giả trong kho 256 MiB; hết hạn sau 30 giây; mỗi lô phục vụ mỗi peer tối đa hai lần; mỗi đề xuất tối đa `consensus.MaxLaneRefs` lô; mã hóa nhị phân có phiên bản.

## 5. Lập luận về tính đúng

An toàn: quy tắc bỏ phiếu và commit không đổi, validator chỉ bỏ phiếu cho khối có hash dựng lại khớp header đã ký; làn có thể làm chậm phiếu nhưng không đổi đối tượng của nó. Khả dụng: mỗi lô được nêu tên đều do một validator trung thực giữ, và cuối cùng có thể tải toàn bộ đề xuất. Tính sống: sau khi ổn định, lô trung thực được chứng nhận và đề xuất; validator chậm xét đề xuất ngay sau khi thực thi. Đây là lập luận, không phải chứng minh được máy kiểm tra.

## 6. Phương pháp thực nghiệm

Hai phương án: Lanes (nút mặc định) và đường cơ sở với `DisseminateByProposal`, trong đó leader phát cả khối; mọi phần khác dùng chung. Mọi validator chạy trong một tiến trình trên Apple M4; mỗi luồng tải lên đi qua liên kết ảo có tốc độ và độ trễ cố định, thông điệp đồng thuận đi trước lưu lượng lớn, không mất gói và không jitter. Client gửi giao dịch 89 byte; lần đo thông lượng gửi 150.000 giao dịch cùng lúc, lần đo độ trễ gửi 20.000 theo vòng hở ở 4.000 tx/s (`VEXO_RATE`). Ô thông lượng 10 validator dùng timeout 5 giây cho cả hai, vì với mặc định đường cơ sở dừng sau độ cao 6. Mỗi ô đo ba lần.

## 7. Kết quả

| Validator | Lanes tx/s | Đường cơ sở tx/s | Hệ số | Lanes B/tx | Đường cơ sở B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

Mọi lần chạy đều commit đủ 150.000 giao dịch. Riêng đề xuất, đường cơ sở tốn `s·(n−1)` byte mỗi giao dịch, Lanes 0,2–0,4 B/tx. Điều này ủng hộ H1 và H2.

| Validator | Lanes p50 / p99 | Đường cơ sở p50 / p99 | Đề xuất Lanes B/tx | Đề xuất đường cơ sở B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

Với 4 validator, chứng nhận tốn khoảng một vòng khứ hồi; từ 7 validator, đường lên của leader ở đường cơ sở đã làm chậm đề xuất ở mức tải này, và với 10 validator chỉ Lanes theo kịp tải. H3 chỉ đúng với tập nhỏ.

| Phương án | tx/s | Tải lên B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| Đường cơ sở | 114,522 (113,095–116,408) | 147.4–148.3 |

Khi không giới hạn liên kết, đường cơ sở nhanh hơn 4%, nhưng Lanes tải lên chưa đến một nửa số byte. Đường cơ sở bị giới hạn bởi liên kết leader, còn Lanes trong ma trận này bởi máy dùng chung.

## 8. Lỗi do quá trình đánh giá phát hiện

1. **Lịch leader theo chẵn lẻ.** `(height−1+round) mod n` chỉ cho chỉ số chẵn đề xuất khi `n` chẵn; đã thay bằng lịch theo stake (`node/leader_schedule_test.go`).
2. **Phát lại giao dịch đã commit.** Gossip đến muộn được nhận lại (152.629 commit cho 150.000 giao dịch).
3. **Hết hạn bậc hai của seen cache.** Mỗi lần tiếp nhận quét toàn bộ cache; nay hết hạn cũ trước và có giới hạn.
4. **Bão gửi lại.** 200 ms cố định trên liên kết nghẽn; nay dùng backoff lũy thừa.
5. **Bằng chứng trùng lặp không giới hạn.** Nay ghi một lần cho mỗi vi phạm trong cửa sổ 1.024.
6. **Yêu cầu thân commit.** Nay hỏi một nơi và thử lại sau một giây.
7. **Đề xuất chỉ được xét khi gửi lại.** Cứ bốn lần chạy 10 validator thì một lần tụt xuống 2.280 tx/s; nay được giữ và xét sau khi thực thi (`consensus/transport_reactor_test.go`).
8. **Chi phí mỗi lô khi tải nhẹ.** Lô hai giao dịch làm bão hòa máy; đã sửa bằng điều nhịp làn.
9. **Chỉ số độ trễ gắn nhãn sai.** Nay đo độ trễ từ gửi đến commit cho từng giao dịch.

## 9. Mối đe dọa đến tính hợp lệ

- Mạng giả lập trên một máy, không mất gói và không jitter.
- CPU dùng chung giới hạn Lanes khi `n` lớn.
- Đường cơ sở cần timeout 5 giây với 10 validator; mã hóa gọn hơn sẽ thu hẹp khoảng cách.
- Chờ 300 ms trước khi yêu cầu cả đề xuất là quá ngắn trên liên kết nghẽn.
- Chỉ chuyển khoản nhỏ; chỉ ba lần lặp (34,9k–39,4k tx/s với 10 validator).
- Chứng chỉ Ed25519 nối chữ ký; BLS sẽ làm chúng nhỏ hơn.

## 10. Tái lập

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` đổi timeout; bỏ `VEXO_UPLINK_MBIT` và `VEXO_LATENCY_MS` để chạy liên kết không giới hạn. Mọi con số công bố cần ghi revision, máy, phiên bản Go và dòng log đầy đủ.

<!-- vexo-docs:technical-parity -->

## Phụ lục tương đương kỹ thuật

Các tên triển khai và thực nghiệm sau đây không được dịch:

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`, `VerifyBatch`, `VerifyCert`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`, `consensus/transport_reactor_test.go`
- `validator/proposer.go`, `node/lanes.go`, `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `DomainLaneBatchAck`, `BatchCert`, `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `Excluded`, `Extra`
- `lane_proposal`, `proposal_request`, `proposal`, `DisseminateByProposal`
- `validator.SelectProposer(validators, v)`, `VotingPower`, `[0, W)`, `v·S mod W`, `ceil(W·(φ−1))`, `0..W−1`, `W`, `S`, `v`
- `n`, `s`, `>= 2/3`, `1/(n−1)`, `s·(n−1)`, `(height−1+round) mod n`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
- `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
