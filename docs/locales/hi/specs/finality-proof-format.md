# Finality Proof Format

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी canonical documentation पर आधारित हिन्दी अनुवाद गाइड है। protocol, security और release से जुड़े निर्णयों के लिए अंग्रेज़ी मूल पाठ ही मानक रहेगा।

## उद्देश्य

यह दस्तावेज़ finality proof fields, verification order और validator set bindingको समझाता है। Implementation और operation में उपयोग होने वाले commands, JSON fields, RPC names, config key और code identifiers compatibility के लिए अंग्रेज़ी में ही रहेंगे।

## मुख्य दायरा

- इस दस्तावेज़ को पढ़ते समय नीचे दिए बिंदु अवश्य जाँचें। commands, JSON fields, RPC methods, config keys और code identifiers compatibility के लिए अंग्रेज़ी में ही रखे जाते हैं।
- विस्तृत normative भाषा के लिए अंग्रेज़ी मूल दस्तावेज़ देखें।
- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/hi/specs/finality-proof-format.md`

## संरक्षित identifier

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## अंग्रेज़ी मूल अनुभाग

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## ऑपरेशनल नोट

- `MUST`, `SHOULD`, `MAY`, कमांड उदाहरण, JSON उदाहरण और RPC नाम अंग्रेज़ी वर्तनी में ही रहेंगे।
- इस translation को बदलने के बाद `make docs-check` चलाएँ।
- यदि यह पेज अंग्रेज़ी source से अलग हो, तो अंग्रेज़ी source मानें और उसी change में इस locale file को update करें।

## Canonical स्रोत

- [English canonical document](../../en/specs/finality-proof-format.md)
