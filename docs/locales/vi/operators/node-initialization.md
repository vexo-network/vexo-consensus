> Locale: vi · Tiếng Việt

# Khởi tạo nút

Hướng dẫn này giải thích cách khởi tạo trình xác thực và lưu trữ các nhà nút, khởi động chúng, xác minh chúng hoạt động tốt và kết nối máy khách.

Kết nối ngang hàng phải được định cấu hình trong `network_config.json`, không được truyền nhiều lần trên dòng lệnh `start`.

Hành vi trong thời gian chạy ảnh hưởng đến sự đồng thuận, RPC, P2P, ghi nhật ký hoặc tài khoản Web3 được quản lý chỉ là tệp cấu hình. `vexod start` từ chối các cờ như `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env` và `--evm-account-key`; Thay vào đó, hãy chỉnh sửa các tệp cấu hình phân tách để mọi toán tử xem xét hành vi nút xác định giống nhau.

Không có chuyển đổi chế độ nút. Nút chủ được xác định bởi các tệp cấu hình, nguồn gốc, tài liệu chính và liệu `validator_id` cùng với người ký có mặt hay không.

## Những gì bạn đang xây dựng

Trang chủ nút Vexo là một thư mục chứa mọi thứ mà nút cần để khởi động:
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
Quy tắc quan trọng rất đơn giản: khởi tạo một lần, chỉnh sửa tệp cấu hình, sau đó bắt đầu. Không ẩn hành vi mạng bên trong cờ shell.

## Chạy cục bộ năm phút

Sử dụng quy trình này khi bạn muốn chứng minh nhị phân hoạt động trước khi nghĩ đến việc triển khai nhiều máy chủ.
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
Trong một thiết bị đầu cuối khác:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
Hình dạng trạng thái dự kiến:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
Chiều cao mới nhất có thể ở mức 0 khi chạy một nút hoặc bộ nhớ trống khi việc tạo khối trống bị vô hiệu hóa. Điều đó không có nghĩa là quá trình này bị hỏng. Nó có nghĩa là nút không tạo ra các khối trống. Thêm giao dịch hoặc chạy mạng thử nghiệm đa trình xác thực để quan sát các cam kết liên tục.

## Mạng cục bộ bốn trình xác thực

Sử dụng quy trình này khi bạn muốn kết nối ngang hàng, xoay vòng người đề xuất, chặn nhật ký cam kết và tăng chiều cao.
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
Kiểm tra hữu ích:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
Nếu tính năng ghi nhật ký cam kết khối được bật trong `log_config.json` thì nhật ký của trình xác thực sẽ bao gồm các sự kiện như:
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
Dừng mạng cục bộ được tạo bằng:
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 và Remix

JSON-RPC kiểu Ethereum tồn tại ở điểm cuối Web3, không nằm trong không gian tên API hoạt động Vexo được phiên bản.

Đối với trình xác thực máy chủ đơn Docker 1, URL nhà cung cấp tùy chỉnh Remix là:
```text
http://127.0.0.1:28657/web3
```
Đối với nút cục bộ trực tiếp có cổng RPC mặc định:
```text
http://127.0.0.1:26657/web3
```
Kiểm tra cuộc gọi tương tự mà Remix thực hiện:
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
Nếu trình duyệt cho biết quá trình tìm nạp ID chuỗi không thành công, hãy kiểm tra những điều sau theo thứ tự:

1. URL kết thúc bằng đường dẫn điểm cuối Web3.
2. Trình duyệt có thể truy cập cổng máy chủ. Các ví dụ về Docker hiển thị `28657`, `28667`, `28677` và `28687`; bên trong vùng chứa, cổng RPC vẫn là `26657`.
3. Máy chủ RPC đang chạy; truy vấn điểm cuối trạng thái trên cùng một máy chủ và cổng.
4. Cấu hình `network_config.json`/RPC cho phép CORS. Trình xử lý mặc định cho phép duyệt trước trình duyệt khi không có danh sách CORS tùy chỉnh nào được đặt.
5. Chuỗi có ID chuỗi EVM khác 0 trong `module_config.json`.

## Nút xác thực

Sử dụng `init validator` khi nút sẽ đề xuất, bỏ phiếu, ký thông báo đồng thuận và tham gia vòng quay trình xác thực.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
Đặt `VEXO_KEY_PASSPHRASE` trước khi chạy lệnh này hoặc chuyển `--passphrase` để thiết lập cục bộ một lần. `--encrypt-keys` mã hóa `validator.key.json`, `node.key.json` và `validator.vrf.key.json`.

Nguyên tắc chung về quyền giám hộ chính:

- `validator.key.json` ký các đề xuất đồng thuận, phiếu bầu, phiếu hết thời gian chờ và các thông báo liên quan đến quyết định cuối cùng.
- `node.key.json` chỉ ký kết bắt tay P2P; nó không bao giờ được sử dụng lại làm khóa đồng thuận của người xác thực.
- `validator.vrf.key.json` chứng minh tính ngẫu nhiên của ủy ban và phải được coi như tài liệu lưu ký của người xác nhận.
- Người nghe công khai phải sử dụng tài liệu khóa cục bộ được mã hóa hoặc tài liệu khóa kiểu người ký từ xa/KMS. Nếu một nút hiển thị RPC công khai hoặc P2P công khai đã được xác thực trong khi `require_network_safety=true`, thì quá trình khởi động sẽ từ chối các khóa trình xác thực cục bộ ở dạng văn bản gốc.
- Khóa đã tạo được ghi bằng chế độ hệ thống tệp `0600`; vẫn thích người ký từ xa/KMS cho người xác nhận tồn tại lâu dài.

Đối với khóa đồng thuận BLS:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` ghi tài liệu khóa `blst-bls12381-minpk-v1` BLS và sao chép bằng chứng sở hữu vào siêu dữ liệu của trình xác thực `genesis.json` dưới dạng `bls_pop`.

Điều này tạo ra:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` là người ký đồng thuận. `node.key.json` là người ký bắt tay P2P được tham chiếu bởi `network_config.json:p2p.node_key_path`. Chúng được tách biệt một cách có chủ ý để các nút lưu trữ và trình xác thực có thể sử dụng cùng một phương thức vận chuyển mà không cần cung cấp cho mỗi thiết bị ngang hàng một khóa ký xác thực.

Bắt đầu với mạng điều khiển cấu hình:
```bash
vexod start --home .vexo-validator-1
```
Sau khi khởi động, đọc nhật ký. Trình xác thực lành mạnh sẽ phát ra các sự kiện chạy nút, nghe RPC, nghe P2P và sau khi các khối được cam kết, các sự kiện được cam kết theo khối. Nếu việc tạo khối trống bị vô hiệu hóa, việc thiếu nhật ký cam kết khối có thể đơn giản có nghĩa là không có giao dịch nào.

## Nút lưu trữ

Sử dụng `init archive` khi nút cần lưu giữ dữ liệu chuỗi, hiển thị RPC, đồng bộ hóa từ các nút ngang hàng và tránh việc ký xác thực.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
Điều này tạo ra:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

Nó **không** tạo `validator.key.json`.

Bắt đầu nó với:
```bash
vexod start --home .vexo-archive-1
```
Các nút lưu trữ không ký phiếu đồng thuận. Chúng hữu ích cho RPC, lập chỉ mục, đồng bộ hóa trạng thái, cung cấp bằng chứng lịch sử và lưu giữ lịch sử truy vấn rộng hơn so với việc cắt bớt trình xác thực.

## Tách các tập tin cấu hình

Nhà nút sử dụng các tệp cấu hình riêng biệt để người vận hành có thể chỉnh sửa một hệ thống con mà không trộn lẫn các cài đặt không liên quan:

- `config.json` chứa danh tính nút, ID chuỗi, đường dẫn dữ liệu và con trỏ tới các tệp cấu hình được phân tách.
- `module_config.json` chứa lựa chọn mô-đun ứng dụng, chính sách thực thi/trước và chính sách quản trị cấp mô-đun.
- `network_config.json` chứa RPC, nhận dạng nút P2P, cài đặt nghe/ngang hàng/hạt giống, cài đặt TLS/xác thực và chính sách chấm điểm ngang hàng.
- `consensus_config.json` chứa thời gian vòng lặp đồng thuận, chính sách khối trống, phụ trợ tiền điện tử, VRF, tiếp nhận trình xác thực và chính sách ủy ban.
- `mempool_config.json` chứa kích thước mempool, phí, mức độ ưu tiên, WAL, trùng lặp và chính sách TTL.
- `log_config.json` chứa định dạng nhật ký, cấp độ, ghi nhật ký sự kiện cam kết khối và ghi nhật ký sự kiện ngang hàng.
- `genesis.json` chứa trình xác thực nguồn gốc bất biến, siêu dữ liệu của trình xác thực và trạng thái mô-đun nguồn gốc.

`network_config.json` Cài đặt RPC cũng bao gồm `shutdown_timeout`, `web3_max_subscriptions_per_connection` và `web3_idle_timeout`. `shutdown_timeout` giới hạn việc tắt máy một cách duyên dáng đối với vòng lặp đồng thuận, máy chủ RPC và vận chuyển nút để người vận hành không phải chờ đợi mãi trên đường dừng bị kẹt. Giá trị mặc định được tạo là `10s`; Đăng ký Web3 mặc định là 256 cho mỗi kết nối với thời gian chờ `2m` không hoạt động để các điểm cuối RPC công khai không thể tích lũy các đăng ký nhàn rỗi không giới hạn.

`network_config.json` Cài đặt P2P bao gồm `auth_replay_path`, `require_auth_replay_store` và `dial_timeout`. Giá trị mặc định được tạo sẽ ghi bằng chứng không phát lại vào `data/p2p_auth_replay.jsonl` và sử dụng thời gian chờ quay số đi `10s`. Đối với thử nghiệm vòng lặp riêng tư, cửa hàng phát lại hầu như không có tính chất ghi sổ; đối với P2P được xác thực công khai, đây là một yêu cầu an toàn vì nó ngăn không cho phát lại lần bắt tay đã ký đã ghi lại sau khi khởi động lại. `dial_timeout` phải đủ dài cho TLS, xác minh bắt tay có chữ ký và độ trễ giữa các khu vực; đặt nó quá thấp sẽ khiến các thiết bị ngang hàng khỏe mạnh trông không ổn định và có thể làm chậm hoạt động sau khi khởi động lại.

`network_config.json` cũng sở hữu tính năng đồng bộ hóa trạng thái khởi động. Điều này hữu ích cho các nút lưu trữ, trình xác thực thay thế hoặc các nút được khôi phục trên máy sạch. Khi `state_sync.enabled` là đúng, `vexod start` tải xuống ảnh chụp nhanh hợp lệ đầu tiên từ `state_sync.snapshot_urls`, xác minh ID chuỗi, tổng kiểm tra, gốc trạng thái và không gian tên KV, khôi phục ảnh chụp nhanh đó vào LevelDB, xây dựng lại chỉ mục và chỉ sau đó khởi động nút. Nếu trạng thái cục bộ đã đáp ứng `state_sync.min_height` và `state_sync.trust_local_higher` là đúng, thì quá trình khởi động sẽ ghi nhật ký `state_sync_skipped` và giữ lại cửa hàng cục bộ.

Ví dụ khối `state_sync`:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
Nhật ký khởi động `state_sync_candidate_failed` cho lỗi tìm nạp, `state_sync_candidate_rejected` cho ảnh chụp nhanh không hợp lệ hoặc cũ và `state_sync_applied` sau khi khôi phục đã được xác minh. Giữ `max_snapshot_bytes` bên dưới ảnh chụp nhanh lớn nhất mà cơ sở hạ tầng của bạn cố ý phục vụ nhưng đủ cao để tăng trưởng trạng thái bình thường. Không trỏ các nút công khai vào nguồn ảnh chụp nhanh của bên thứ ba chưa được xác thực trừ khi nhà điều hành có chính sách tin cậy ngoài băng tần và bằng chứng cuối cùng/máy khách nhẹ cho nguồn đó.

Nếu một trường thay đổi hành vi mạng, hãy chỉnh sửa tệp cấu hình phân tách và cam kết hoặc phân phối tệp đã xem xét đó. Không dựa vào các cờ `vexod start` dài cho hành vi thời gian chạy. Lệnh bắt đầu cố tình từ chối thời gian đồng thuận, khối trống, xác thực P2P, quản trị viên RPC và cờ khóa Web3 được quản lý để người vận hành không vô tình chạy hành vi khác với cấu hình được xem xét.

## Tôi nên chỉnh sửa tệp nào?

| Mục tiêu | Tập tin | Lĩnh vực |
|---|---|---|
| Thay đổi cổng liên kết RPC | `network_config.json` | `rpc.address` |
| Thay đổi cổng liên kết P2P | `network_config.json` | `p2p.listen_address` |
| Thêm các đồng nghiệp kiên trì | `network_config.json` | `p2p.peers` |
| Thêm đồng nghiệp hạt giống | `network_config.json` | `p2p.seeds` |
| Bật/tắt các khối trống | `consensus_config.json` | trường khối trống đồng thuận |
| Điều chỉnh thời gian chờ đồng thuận | `consensus_config.json` | các trường đề xuất, bỏ phiếu trước, cam kết trước và hết thời gian cam kết |
| Yêu cầu thực hiện cuối cùng | `consensus_config.json` | trường cam kết thực thi đồng thuận |
| Bật/tắt mô-đun | `module_config.json` | danh sách mô-đun ứng dụng |
| Thay đổi ID chuỗi EVM | `module_config.json` | thực thi trường ID chuỗi EVM |
| Điều chỉnh phí cơ sở/gas | `module_config.json` | các trường phí cơ sở thực thi, phí động, khí mục tiêu và khí tối đa |
| Cấu hình mempool WAL | `mempool_config.json` | đường dẫn WAL mempool |
| Nhật ký cam kết khối kiểm soát | `log_config.json` | đăng nhập trường sự kiện cam kết |
| Kiểm soát nhật ký ngang hàng | `log_config.json` | đăng nhập trường sự kiện ngang hàng |

Khi nghi ngờ, hãy chạy:
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## Các loại khóa

Trình xác thực ban đầu được đặt mặc định là `--key-type bls` vì quá trình xác thực an toàn mạng yêu cầu tính chính xác tổng hợp BLS đã được kiểm tra. `--key-type ed25519` vẫn khả dụng cho các thử nghiệm riêng tư và triển khai tùy chỉnh bên ngoài cổng an toàn mạng. `--encrypt-keys` nên được sử dụng cho bất kỳ nút nhà không vứt đi nào. Tạo khóa độc lập cũng hỗ trợ khóa VRF:
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
Khóa VRF không phải là người ký đồng thuận. Chúng được sử dụng để lựa chọn ủy ban được VRF hỗ trợ và phải được tham chiếu từ `consensus_config.json` đến `vrf_key_paths` cộng với khóa siêu dữ liệu của trình xác thực `vrf_public_key` khi phần phụ trợ đó được bật.

`config.json` trỏ đến các tệp cấu hình được chia nhỏ:
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
Mỗi đường dẫn có thể là tuyệt đối hoặc tương đối với nút home. Nếu bị bỏ qua, `vexod` sẽ sử dụng tệp `<home>/<name>_config.json` mặc định.

Ví dụ `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
Chính sách quản trị cũng có trong `module_config.json`. Các cấu hình an toàn cho mạng được tạo yêu cầu một khoản tiền gửi đề xuất:
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
Khoản tiền gửi là số dư gốc được ký quỹ từ người gửi đề xuất. Đề xuất được thông qua sẽ hoàn trả tiền đặt cọc; đề xuất bị từ chối sẽ chuyển nó tới `RejectedDeposits`. Sử dụng địa chỉ được kiểm soát bởi mô-đun kho bạc/nhóm cộng đồng của bạn nếu khoản tiền gửi bị từ chối sẽ cấp tiền cho kho bạc thay vì tài khoản mô-đun mặc định.

Ví dụ `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` và `rpc.evm_account_private_keys` là các phương thức tài khoản được quản lý Web3 tùy chọn và quay lại như `eth_accounts`, `eth_sign`, `eth_signTransaction` và `eth_sendTransaction`. Ưu tiên `evm_account_key_envs` để khóa riêng được đưa vào bởi môi trường quy trình hoặc trình quản lý bí mật thay vì được lưu trữ trong JSON. Giữ cả hai danh sách trống để hoạt động xác thực thông thường trừ khi nút này cố tình hoạt động như một điểm cuối ví nóng Web3 cục bộ. An toàn khởi động từ chối các phím nóng EVM được quản lý trên trình nghe RPC công khai.

Ví dụ `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` được giải quyết liên quan đến thư mục chứa `consensus_config.json`. Sử dụng tài liệu khóa được mã hóa và cung cấp `VEXO_KEY_PASSPHRASE` cho quy trình nút khi không thể tránh khỏi việc giám sát khóa VRF cục bộ. Không đặt trực tiếp các giá trị vô hướng riêng VRF thô vào `consensus_config.json` cho các mạng do nhà điều hành điều hành.

Sử dụng `vexod config paths --home <home>` để kiểm tra tất cả các đường dẫn đã giải quyết.

Cấu hình lưu trữ có:
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
Lưu trữ `consensus_config.json` vô hiệu hóa vòng lặp đồng thuận cục bộ:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
Các nhà xác thực đã tạo được đặt `"require_network_safety": true` trong `config.json` theo mặc định. Đây không phải là một chế độ; đó là một cổng an toàn khởi động từ chối tiền điện tử xác định, các giao dịch không được ký/không được xác nhận, thiếu sàn phí/gas, thiếu WAL mempool bền vững, thiếu chính sách thay thế cho cùng một người ký/giao dịch không một lần, tính ngẫu nhiên của ủy ban không an toàn và các giá trị `execution_commit` ngoài `finalized`.

Khi `require_network_safety` được bật, hãy chạy:
```bash
vexod config audit --home <home> --strict
```
trước khi bắt đầu nút. Quá trình kiểm tra phải vượt qua đối với mọi nhà xác thực và nhà lưu trữ tham gia vào cùng một mạng.

## Các thiết bị ngang hàng dựa trên cấu hình

Địa chỉ ngang hàng và nghe nằm trong `network_config.json`:
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` tự động tải các thiết bị ngang hàng này:
```bash
vexod start --home .vexo-archive-1
```
Các hạt giống và ngang hàng liên tục được định cấu hình trong `network_config.json`; `vexod start` không chấp nhận ghi đè máy chủ ngang hàng hoặc máy chủ hạt giống.

Không đặt cài đặt máy chủ tồn tại lâu dài hoặc `host:port` trên dòng lệnh `vexod start`. Thay vào đó, hãy chỉnh sửa `rpc.address`, `p2p.listen_address`, `p2p.peers` và `p2p.seeds` trong `network_config.json`.

Giữ `p2p.node_id` ổn định trong suốt thời gian tồn tại của nút chủ. `p2p.node_key_path` phải trỏ đến `node.key.json` hoặc một tài liệu khóa cục bộ/được quản lý khác chỉ được sử dụng cho ký kết bắt tay ngang hàng. Bản đồ ngang hàng nên sử dụng ID nút ngang hàng, không phải địa chỉ tài khoản hoặc tên nhà điều hành trình xác thực trừ khi chúng cố ý giống nhau.

Để truyền tải ngang hàng gRPC được mã hóa và xác thực, cũng đặt `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path` và tùy chọn `p2p.tls_server_name` trong `network_config.json`. Đường dẫn TLS tương đối được giải quyết từ thư mục chính của nút. Giữ `p2p.dial_timeout` trong cùng một tệp để mọi nhà điều hành sử dụng hành vi kết nối lại giống nhau; không ẩn thời gian ngang hàng trong tập lệnh shell.

## Thời điểm đồng thuận

Thời gian vòng lặp đồng thuận tồn tại trong `consensus_config.json`:
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` kiểm soát thời gian chờ đề xuất của một vòng.
- `timeout_prevote` kiểm soát cửa sổ thu thập phiếu bầu.
- `timeout_precommit` kiểm soát cửa sổ thu thập chứng chỉ cam kết.
- `timeout_commit` kiểm soát độ trễ tối thiểu sau một khối đã cam kết.
- `create_empty_blocks: false` nghĩa là nút chỉ đề xuất khi có giao dịch.
- `execution_commit: "finalized"` chờ quyết định cuối cùng ba chuỗi HotStuff trước khi thực thi tổ tiên cuối cùng và là mặc định của trình xác thực được tạo. `execution_commit: "qc"` thực thi và duy trì các khối được chứng nhận QC ngay lập tức nhưng cổng an toàn từ chối nó.

`round_timeout` chỉ được giữ lại dưới dạng tổng hợp khả năng tương thích. Thích các trường thời gian chờ kiểu Tendermint ở trên.

Khi `create_empty_blocks` sai, chiều cao có thể không thay đổi trong khi mempool trống. Điều đó được mong đợi: chuỗi đang chờ công việc hữu ích thay vì cam kết các khối trống. Khi một giao dịch xuất hiện và trạng thái vòng đồng thuận cục bộ đã vượt qua một người đề xuất khác, nút sẽ chuyển sang vòng tiếp theo nơi người xác thực của nó là người đề xuất và xây dựng từ mempool. Đường dẫn khôi phục này duy trì trạng thái hoạt động do giao dịch kích hoạt mà không kích hoạt lại thư rác khối trống.

## Mạng lưới nhiều trình xác thực

Đối với mạng được tạo:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
Mỗi nhà xác thực được tạo sẽ nhận được:

- `validator.key.json` của chính nó
- các tệp cấu hình được phân chia riêng: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` và `log_config.json`
- một `genesis.json` được chia sẻ
- `network_config.json` mục nhập ngang hàng cho những người xác nhận khác

`vexod network up` và `make network-e2e` sử dụng thời gian chờ ở cấp quy trình trong khi chờ tất cả trình xác thực bắt đầu, gửi giao dịch khói và quan sát mức tăng trưởng chiều cao. Thời gian chờ lệnh mặc định cố ý dài hơn khoảng thời gian đồng thuận vì nó bao gồm quá trình khởi động, mở LevelDB, bắt tay có chữ ký P2P, kiểm tra TLS/xác thực, chấp nhận giao dịch và tính cuối cùng. Nếu bạn giảm mạnh thời gian chờ đồng thuận, hãy duy trì thời gian chờ kết nối mạng đủ lớn để chẩn đoán lỗi khởi động thay vì tắt khai thác quá sớm.

Đối với các mạng được chứa trong vùng chứa hoặc nhiều máy chủ, hãy đặt các giá trị cấu trúc liên kết vào tệp JSON:
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` và `rpc_host_template` là các mục tiêu quay số được ghi vào danh sách ngang hàng `network_config.json` của mỗi nút. Trong Docker, đây có thể là tên dịch vụ như `validator-%d`.
- `p2p_advertise_host_template` và `rpc_advertise_host_template` là các địa chỉ công khai được ghi vào siêu dữ liệu của trình xác thực trong `genesis.json`. Sử dụng tên DNS hoặc IP công cộng tại đây cho mạng công cộng.
- `p2p_listen_host` và `rpc_listen_host` là máy chủ liên kết cục bộ. Sử dụng `0.0.0.0` cho các vùng chứa hoặc máy chủ sẽ lắng nghe trên tất cả các giao diện.
- Không sử dụng lại tên dịch vụ chỉ dành cho Docker làm địa chỉ công cộng được quảng cáo trừ khi mạng được đặt ở chế độ riêng tư có chủ ý.

Sau đó tạo nút nhà từ tệp đó:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## Khắc phục sự cố

| Triệu chứng | Rất có thể nguyên nhân | Kiểm tra những gì |
|---|---|---|
| `latest_height` không tăng | Khối trống bị vô hiệu hóa và không có tx, không đủ trình xác thực trực tuyến hoặc không có người ký | `consensus_config.json`, nhật ký xác thực, `/v1/diagnostics` |
| `peer_count` là `0` | Không thể truy cập địa chỉ ngang hàng hoặc `network_config.json` được tạo cho tên máy chủ sai | `p2p.peers`, cổng máy chủ container, DNS, tường lửa |
| `p2p auth replay store` lỗi | P2P công khai/xác thực yêu cầu bộ nhớ phát lại bền bỉ | `p2p.auth_replay_path` và viết giấy phép dưới nhà |
| `eth_chainId` không thành công trong Remix | URL sai, cổng máy chủ sai hoặc CORS/preflight trình duyệt bị chặn bởi cấu hình tùy chỉnh | Sử dụng URL điểm cuối Web3, sau đó cuộn trực tiếp điểm cuối đó |
| `config audit --strict` thất bại | Cổng an toàn tìm thấy thuộc tính cấu hình không an toàn | Đọc phần kiểm tra lỗi, sau đó chỉnh sửa tệp cấu hình phân tách có tên |
| `no block_committed logs` | Ghi nhật ký bị vô hiệu hóa hoặc không có khối nào được tạo | `log_config.json`, `create_empty_blocks`, nội dung bộ nhớ |
| `managed EVM key rejected` | Khóa riêng nóng được định cấu hình trên trình nghe RPC công khai | Xóa `evm_account_private_keys` hoặc giữ RPC ở chế độ riêng tư |

## Danh sách kiểm tra người vận hành tối thiểu

Trước khi giao nút gốc cho máy hoặc nhà điều hành khác:

- `vexod validate --home <home>` vượt qua.
- `vexod config audit --home <home> --strict` đã vào đúng nhà đó.
- `config.json`, tệp cấu hình phân tách, `genesis.json` và siêu dữ liệu của trình xác thực công khai được xem xét.
- `validator.key.json`, `node.key.json` và `validator.vrf.key.json` được mã hóa hoặc thay thế bằng tài liệu khóa KMS/người ký từ xa.
- `network_config.json:p2p.peers` chứa các địa chỉ có thể quay số từ máy đích, không phải tên chỉ dành cho Docker trừ khi nút thực sự chạy bên trong mạng Docker đó.
- `network_config.json` trình nghe RPC/P2P công khai có tài liệu TLS khi `require_network_safety` được bật.
- `module_config.json:execution.EVMChainID` được đặt trước khi kết nối ví Web3 hoặc Remix.
- `mempool_config.json` có đường dẫn WAL nếu nút sẽ khôi phục các tx đang chờ xử lý sau khi khởi động lại.
- `log_config.json` cho phép ghi nhật ký ngang hàng và cam kết khối trong khi mạng đang được đưa lên.

<!-- vexo-docs:technical-parity -->
## Phụ lục tương đương kỹ thuật

Phụ lục này bảo đảm bản dịch vẫn giữ các giao diện có thể chạy và các phần quan trọng của tài liệu chuẩn tiếng Anh. Lệnh, khóa cấu hình, phương thức RPC và tên gói được giữ nguyên trong mọi ngôn ngữ.

### Theo dõi mục
- section: Validator Node — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Archive Node — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Split Configuration Files — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Key Types — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Config-Based Peers — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Consensus Timing — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.
- section: Multi-Validator Network — Mục này cần được đọc cùng giá trị cấu hình, bằng chứng xác minh, điều kiện lỗi và hành động của người vận hành.

### Giao diện giữ nguyên
- `network_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod start` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--timeout-propose` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--create-empty-blocks` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--p2p-auth-token` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--rpc-admin-token` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--evm-account-key-env` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--evm-account-key` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `validator_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `VEXO_KEY_PASSPHRASE` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--passphrase` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--encrypt-keys` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `validator.key.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `node.key.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `validator.vrf.key.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `require_network_safety=true` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--key-type bls` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `blst-bls12381-minpk-v1` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `genesis.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `bls_pop` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `module_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `consensus_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `mempool_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `log_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `data/` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `network_config.json:p2p.node_key_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `shutdown_timeout` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `web3_max_subscriptions_per_connection` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `web3_idle_timeout` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `auth_replay_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `require_auth_replay_store` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `dial_timeout` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `data/p2p_auth_replay.jsonl` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `--key-type ed25519` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vrf_key_paths` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vrf_public_key` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `<home>/<name>_config.json` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc.evm_account_key_envs` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc.evm_account_private_keys` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_accounts` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_sign` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_signTransaction` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `eth_sendTransaction` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `evm_account_key_envs` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod config paths --home <home>` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `"require_network_safety": true` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `execution_commit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `require_network_safety` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `host:port` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc.address` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.listen_address` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.peers` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.seeds` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.node_id` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.node_key_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.tls_cert_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.tls_key_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.tls_ca_path` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.tls_server_name` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p.dial_timeout` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `timeout_propose` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `timeout_prevote` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `timeout_precommit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `timeout_commit` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `create_empty_blocks: false` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `execution_commit: "finalized"` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `execution_commit: "qc"` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `round_timeout` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `create_empty_blocks` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `vexod network up` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `make network-e2e` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p_host_template` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc_host_template` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `validator-%d` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p_advertise_host_template` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc_advertise_host_template` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `p2p_listen_host` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
- `rpc_listen_host` — Tên này được dùng nguyên dạng trong ví dụ có thể chạy và kiểm tra cấu hình, nên không dịch.
