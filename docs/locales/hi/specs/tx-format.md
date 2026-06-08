# Transaction Format

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी canonical documentation पर आधारित हिन्दी अनुवाद गाइड है। protocol, security और release से जुड़े निर्णयों के लिए अंग्रेज़ी मूल पाठ ही मानक रहेगा।

## उद्देश्य

यह दस्तावेज़ transaction format, signing, fee और gas rulesको समझाता है। Implementation और operation में उपयोग होने वाले commands, JSON fields, RPC names, config key और code identifiers compatibility के लिए अंग्रेज़ी में ही रहेंगे।

## मुख्य दायरा

- इस दस्तावेज़ को पढ़ते समय नीचे दिए बिंदु अवश्य जाँचें। commands, JSON fields, RPC methods, config keys और code identifiers compatibility के लिए अंग्रेज़ी में ही रखे जाते हैं।
- विस्तृत normative भाषा के लिए अंग्रेज़ी मूल दस्तावेज़ देखें।
- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/hi/specs/tx-format.md`

## संरक्षित identifier

- `fee`
- `gas`
- `gas_limit`
- `signer`
- `nonce`
- `priority`
- `vexo`
- `vexovaloper`
- `vexovalcons`
- `signer=<address>`
- `0x`
- `evm_chain_id`
- `EVMChainID`
- `chain_id`
- `auth`
- `1`
- `N`
- `N+1`

## अंग्रेज़ी मूल अनुभाग

- Transaction Format
- Scope
- Canonical Payload
- Address Format
- Signed Envelope
- Required Ante Metadata
- CheckTx Requirements
- Fee and Gas
- Load Test Payloads
- CLI Examples

## ऑपरेशनल नोट

- `MUST`, `SHOULD`, `MAY`, कमांड उदाहरण, JSON उदाहरण और RPC नाम अंग्रेज़ी वर्तनी में ही रहेंगे।
- इस translation को बदलने के बाद `make docs-check` चलाएँ।
- यदि यह पेज अंग्रेज़ी source से अलग हो, तो अंग्रेज़ी source मानें और उसी change में इस locale file को update करें।

## Canonical स्रोत

- [English canonical document](../../en/specs/tx-format.md)
