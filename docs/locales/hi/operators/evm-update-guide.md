# EVM अपडेट गाइड

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी स्रोत का हिंदी अनुवाद है। प्रोटोकॉल, सुरक्षा, और रिलीज़ निर्णय अंग्रेज़ी स्रोत के अनुसार लिए जाते हैं।

यह गाइड बताता है कि built-in EVM stack को chain ID handling, Web3 compatibility, और release evidence को तोड़े बिना कैसे अपडेट करें। यह उन operators और maintainers के लिए है जिन्हें go-ethereum upgrade करना हो, fork presets बदलने हों, या controlled release में EVM behavior संशोधित करना हो।

## कौन सा बदलाव EVM अपडेट माना जाएगा

अगर कोई बदलाव Ethereum-style execution या Web3-facing behavior को प्रभावित कर सकता है, तो उसे release-sensitive feature update समझें:

- `modules/evm/backend/geth` में `go-ethereum` version bump
- `modules/evm/ethcompat` में बदलाव
- `modules/evm` में बदलाव
- `execution.evm_fork_preset` में बदलाव
- `execution.evm_chain_config_json` में बदलाव
- raw transaction admission, gas accounting, receipts, traces, proofs, या block response fields में बदलाव
- `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction`, `eth_sendTransaction` जैसे managed Web3 account handling में बदलाव

## सुरक्षित अपडेट क्रम

कोड, config, और docs को aligned रखने के लिए यह क्रम अपनाएँ:

1. पहले isolated geth-backed adapter को अपडेट करें।
2. फिर fixture corpus और conformance tests अपडेट करें।
3. semantic change होने पर `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md`, और `docs/sdk/rpc-api-versioning.md` अपडेट करें।
4. release evidence format बदलने पर `docs/release/release-pipeline.md` अपडेट करें।
5. operator-facing knobs बदलने पर node configuration docs अपडेट करें।
6. merge से पहले validation matrix फिर से चलाएँ।

जब तक conformance suites, RPC smoke checks, और Docker deployment checks पास न हों, EVM runtime version bump करके तुरंत ship न करें।

## अपडेट workflow

### 1. बदलाव का दायरा तय करें

अपडेट का exact intent लिखें:

- fork behavior only
- transaction admission only
- execution semantics only
- RPC compatibility only
- blob / receipt / trace handling only
- managed account या wallet behavior only

इससे review focused रहता है और unrelated code साथ नहीं हिलता।

### 2. सबसे narrow layer में बदलाव करें

इन boundaries को prefer करें:

- `modules/evm/backend/geth` के लिए upstream go-ethereum integration changes
- `modules/evm/ethcompat` के लिए raw transaction decoding, hash preservation, और fixture handling
- `modules/evm` के लिए state transition, receipts, logs, storage, snapshot behavior
- `rpc` के लिए Web3 request/response surface changes
- `cmd/vexod` केवल तब जब CLI या release workflow को नया behavior दिखाना हो

अगर बदलाव application modules तक पहुँचता है, तो module boundary स्पष्ट रखें और deterministic state writes बनाए रखें।

### 3. default configuration refresh करें

जब semantics बदलें, उसी patch में default config भी अपडेट करें:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- ज़रूरत हो तो `network_config.json` के managed account RPC fields
- `module_config.json` का EVM chain ID

Runtime behavior को समझाने के लिए hidden CLI flag पर निर्भर न रहें। config files से ही node behavior साफ दिखना चाहिए।

### 4. conformance stack चलाएँ

कम से कम यह चलाएँ:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

फिर उन user-visible flows को verify करें जो सबसे पहले टूटते हैं:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

अगर Docker single-host deployment है, तो यह भी देखें:

```text
http://127.0.0.1:28657/web3
```

कम से कम ये behaviors जांचें:

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

फिर एक simple contract deploy, proxy contract deploy, और UUPS upgrade path को production में उपयोग होने वाले RPC endpoint के साथ टेस्ट करें।

### 5. proxy और upgrade verify करें

EVM update तब तक complete नहीं है जब तक यह सब true न हो:

- normal contract deploy सफल हो
- proxy deploy सफल हो
- UUPS upgrade call सफल हो
- upgrade के बाद storage और code expected हों
- nonce tracking monotonic रहे
- block producer transactions को unsafe proposal errors के बिना accept करे

अगर proxy deploy हो जाए लेकिन upgrade fail हो, तो अभी publish नहीं किया जा सकता। इसे warning नहीं, release blocker मानें।

### 6. evidence refresh करें

जब EVM surface बदले, release evidence bundle भी अपडेट करें:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- pinned SHA-256 fixture references

release evidence में लिखें कि क्या बदला, क्या टेस्ट किया, और कौन सा commit या version verify हुआ। अगर evidence और real executed code match नहीं करते, तो EVM update को complete न कहें।

## validation matrix

इस table को merge gate की तरह इस्तेमाल करें।

| Check | क्यों ज़रूरी है |
| --- | --- |
| `make evm-conformance` | fork rule और execution regressions पकड़ता है |
| `go test ./modules/evm -count=1` | receipts, logs, storage, balances, snapshots verify करता है |
| `go test ./rpc -count=1` | Web3 request/response compatibility verify करता है |
| `make network-e2e` | पुष्टि करता है कि node अभी भी शुरू होता है, peers बनाता है, और commit करता है |
| Docker single-host smoke | Remix और browser tools के actual path की पुष्टि करता है |
| Contract deploy | transaction admission और receipt generation verify करता है |
| Proxy deploy | ABI और storage layout assumptions verify करता है |
| UUPS upgrade | upgrade semantics और post-upgrade reads verify करता है |

अगर कोई भी check red हो, तो update को done न कहें।

## rollback criteria

इनमें से कुछ भी हो तो EVM update rollback करें:

- `eth_chainId` अचानक बदल जाए
- `eth_sendRawTransaction` valid transactions reject करने लगे
- `eth_call` या `eth_estimateGas` expected fork rules से अलग हो जाए
- receipts, logs, proofs committed state से मेल न खाएँ
- proxy या upgrade transactions fail होने लगें
- release evidence current code path से match न करे

rollback को last known good adapter version, config defaults, और fixture set एक साथ restore करने चाहिए।

## technical parity appendix

यह appendix update guide को बाकी documentation tree के साथ aligned रखता है।

- `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc`, और `cmd/vexod` को stable implementation boundaries बनाए रखें।
- `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction`, और `eth_sendTransaction` की spelling न बदलें।
- `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures`, और `--evm-web3-conformance-evidence` भी वैसे ही रखें।
- operational question सरल है: क्या यह update Ethereum-style execution को preserve करता है और साथ ही Vexo consensus तथा release safety के अनुरूप है?

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
