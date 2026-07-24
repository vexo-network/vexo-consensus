# HotStuff thích nghi có cổng phục hồi cho mạng Proof-of-Stake mô-đun

> Locale: vi · Tiếng Việt  
> Loại tài liệu: bản thảo nghiên cứu và giao thức tái lập  
> Trạng thái: bản thảo dựa trên triển khai; mọi tuyên bố hiệu năng phải có hiện vật đo lường.

## Tóm tắt

Tài liệu này nghiên cứu cơ chế sao chép state machine BFT theo phong cách HotStuff cho mạng Proof-of-Stake mô-đun. Triển khai kết hợp three-chain finality và validator set được phiên bản hóa theo height với ba cơ chế vận hành. Bộ điều khiển thích nghi có giới hạn điều chỉnh round timeout dựa trên độ trễ xử lý p95 của proposal, vote và commit cùng tình trạng active peer. Recovery finality gate trì hoãn finalized application commit khi lịch sử block bền vững và lịch sử application state khác nhau phía trên height an toàn chung. Deterministic transaction ordering loại bỏ ảnh hưởng của thứ tự đến mempool cục bộ đối với cùng một tập transaction nhưng vẫn giữ quan hệ nonce của từng signer.

Đóng góp không phải là tuyên bố PoS, BFT, HotStuff, adaptive view synchronization hay order fairness là phát minh mới. Câu hỏi hẹp hơn là liệu tổ hợp bộ điều khiển, cổng phục hồi và ordering có giới hạn này có giảm timeout không cần thiết và bất nhất khi phục hồi mà không thay đổi quy tắc an toàn HotStuff cơ sở hay không. Tài liệu tách riêng sự thật đã triển khai, giả thuyết có thể bác bỏ và kết luận còn cần thí nghiệm. Không công bố số cải thiện throughput hay latency trước khi có các lần lặp với binary, config, topology và workload được ghim.

## Câu hỏi nghiên cứu

RQ1 so sánh chính sách thích nghi với cùng hệ thống dùng fixed timeout khi network delay thay đổi, dựa trên số timeout và p95 commit latency. RQ2 tiêm storage/restart fault để xác minh gate ngăn application state vượt quá height bền vững chung của block/state. RQ3 hoán vị cùng một tập transaction và yêu cầu proposal order giống nhau cùng nonce tăng dần cho mỗi signer. RQ4 đo chi phí CPU, memory, network và latency trong điều kiện ổn định không lỗi.

H1 đến H4 là giả thuyết có hướng và có thể phủ định, không phải kết quả. Việc code path tồn tại không chứng minh lợi ích. Nếu khác biệt không có ý nghĩa, kết quả âm hoặc giới hạn áp dụng phải được báo cáo trung thực.

## Công trình trước và ranh giới tính mới

HotStuff đã đưa ra leader-based BFT dưới partial synchrony, quorum certificate, chained commit, giao tiếp tuyến tính trên happy path và responsiveness. LibraBFT/DiemBFT cùng AptosBFT đã kết hợp hậu duệ HotStuff với stake-weighted validator governance. Jolteon và Ditto nghiên cứu latency thấp hơn, network adaptation và asynchronous fallback; Fever nghiên cứu responsive view synchronization. Tendermint là một dòng round-based PoS BFT khác. Narwhal/Tusk tách reliable transaction dissemination khỏi ordering. Aequitas, Wendy và Themis định nghĩa order fairness mạnh hơn hash-based determinism dùng ở đây.

Vì vậy không được tuyên bố “blockchain PoS+BFT đầu tiên”, “mạng PoS đầu tiên dùng HotStuff”, “giống hệt AptosBFT”, “asynchronous liveness” hoặc “optimal communication” khi chưa có proof, “bảo vệ MEV hoàn toàn”, hay “production-ready” chỉ từ single-host test. Systems contribution có thể bảo vệ hẹp hơn: tích hợp bounded feedback controller, local durable-history commit gate và nonce-aware deterministic ordering vào node PoS mô-đun viết bằng Go, rồi đánh giá tái lập với baseline fixed và gate-disabled.

## Mô hình và cơ chế

Tại height h, Vh là active validator set và Ph là tổng voting power. QC hợp lệ khi các signer đã biết và không trùng đóng góp ít nhất hai phần ba Ph. Set và hash được phiên bản hóa theo height. Admission có thể permissionless với minimum stake, bị giới hạn số lượng hoặc restricted theo config. Lớp này xử lý Sybil resistance và governance; nó không thay đổi BFT fault threshold.

Network được giả định partially synchronous. Safety cần Byzantine voting power dưới một phần ba, signature hợp lệ, validator-set binding đúng và durable store tin cậy. Liveness còn cần delay cuối cùng trở nên bounded, honest quorum có thể liên lạc, signer sẵn sàng và peer connectivity đủ. Không có bảo đảm tiến triển cho network luôn asynchronous.

EVM là application workload bên dưới Vexo consensus. Thực thi Ethereum bytecode và tương thích tooling `/web3` không đồng nghĩa với Ethereum fork choice hay devp2p consensus.

Quy tắc an toàn cơ sở theo dõi `locked_qc` và `high_qc`. Proposal chỉ an toàn nếu mở rộng lock hoặc mang justify QC mới ít nhất bằng lock. Validator không thể vote cho block khác nhau tại cùng height/round. Ba certified link liên tiếp được ràng buộc height và hash sẽ finalize grandparent. Adaptive controller không đổi predicate, quorum threshold, QC verification hay three-chain rule.

Adaptive timeout dùng base budget T0, current budget Tt, tổng proposal/vote/commit p95 latency và floor theo peer deficit. Sau timeout giá trị tăng về 1,5×Tt; sau progress giảm về 0,8×Tt. Ba lần observed latency tạo candidate floor, kết quả bị giới hạn giữa T0 và 8×T0. Khi không có active peer, peer floor là 2×T0. Idle không có pending work và local execution/storage error không tiêu thụ round. Đây là bounded operational controller, không phải pacemaker được chứng minh tối ưu.

Recovery gate tính Hsafe=min(Hs,Hb) khi durable state height Hs và block-index height Hb tồn tại. Trong lúc hai giá trị khác nhau, finalized application commit trên Hsafe bị trì hoãn. Đây là local persistence restriction, không phải vote phase bổ sung hay network certificate.

Deterministic ordering tạo salt từ chain ID và height. Transaction có signer/nonce metadata được nhóm theo signer chain, sắp nonce tăng dần rồi trộn các chain head theo salted transaction hash. Điều này bỏ arrival-order dependence cho cùng candidate set. Nó không bảo đảm first-seen fairness, censorship resistance, confidentiality hay strong order-fairness vì proposer vẫn ảnh hưởng inclusion.

Consensus vote path hiện dùng full height-versioned validator set và deterministic proposer. ECVRF committee selector có ở component và query nhưng chưa kết nối quorum formation hoặc proposal eligibility. VRF committee consensus vẫn là future work.

## Phương pháp thí nghiệm

Mọi treatment dùng cùng binary và application config. So sánh fixed với adaptive off/gate on, adaptive với cả hai on, và gate-disabled ablation chỉ trong mạng nghiên cứu cô lập có thể hủy. Khi đủ tài nguyên dùng 4, 7, 16 và 31 validator; single-host chỉ làm smoke test.

Điều kiện gồm latency 10, 50, 100 và 250 ms, step delay, jitter, loss 0/1/5/10%, restart validator thường và current proposer, unavailability ngay dưới một phần ba voting power, minority partition/heal, signer delay và injected durable-history mismatch. Workload gồm native transfer, EVM transfer, contract creation, event log, proxy deployment và UUPS upgrade.

Metric gồm committed/finalized height, proposal/vote/commit p50/p95/p99, end-to-end finality latency, timeout count, round distribution, current adaptive timeout, peer count, recovery deferral, throughput, gas, CPU, RSS, disk/network bytes, rejection, double-sign và invalid nonce. Performance run chỉ hợp lệ khi mọi validator đồng ý app hash và finalized block hash, transaction/receipt/block locations nhất quán, deployed code tồn tại và proxy state còn đúng sau upgrade.

Sau warm-up, mỗi điều kiện thực hiện ít nhất ba mươi lần lặp độc lập trừ khi số nhỏ hơn được biện minh trước bằng power analysis. Ngẫu nhiên hóa thứ tự treatment và lưu seed. Báo cáo median, IQR, p95, confidence interval và effect size. Không chỉ chọn run tốt nhất; exclusion rule phải xác định trước khi xem kết quả.

## Tính đúng, tái lập và đạo đức

Chính sách thích nghi chỉ thay đổi khi nào thử timeout vote, không thay đổi điều gì làm vote hoặc QC an toàn. Gate chỉ siết commit và không thể cho phép commit bị quy tắc cơ sở từ chối. Deterministic ordering hỗ trợ execution input giống nhau nhưng không thay thế proof chống conflicting finality.

Proof đủ chất lượng xuất bản phải hình thức hóa stake-weighted quorum intersection, lock monotonicity, tính duy nhất của finalized block theo height, validator-set transition, vote WAL crash recovery và tính safety-neutral của controller/gate. Unit tests và adversarial simulations là evidence, không thay formal proof hay independent audit.

Mỗi experiment lưu commit, dirty-tree status, Go/OS/CPU/memory/container, topology, genesis, split configs, binary SHA-256, workload seed, raw JSON/JSONL/CSV, validator logs, final app hashes, analysis scripts và failed-run ledger. Không đổi tên cơ chế đã biết rồi tuyên bố phát minh, không dựng số liệu và phải tách hypothesis, observation, interpretation.

AI assistance được công bố theo chính sách venue, còn tác giả chịu trách nhiệm cho mọi claim, citation, experiment và proof. Fault injection chỉ chạy trên isolated system thuộc sở hữu hoặc được cho phép. Private key, operator token, participant data và production endpoint không đưa vào artifact. Phát hiện security tuân theo coordinated vulnerability disclosure.

Trước submission, manuscript phải khớp pinned source revision, prior-art search được lưu, baseline reproducible, multi-host fault measurements hoàn tất và mọi table/figure tái tạo được từ raw data cùng script. Negative result, limitation, proof wording phù hợp và external methodology review vẫn phải có trong bản gửi. Trước đó, cách gọi chính xác là “bản thảo nghiên cứu dựa trên triển khai”, không phải “consensus mới đã được chứng minh”.

<!-- vexo-docs:technical-parity -->

## Phụ lục tương đương kỹ thuật

Các tên sau giữ nguyên:

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
