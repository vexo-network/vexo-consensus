# Cosmos/Tendermint Comparison Gate

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी canonical documentation पर आधारित हिन्दी अनुवाद गाइड है। protocol, security और release से जुड़े निर्णयों के लिए अंग्रेज़ी मूल पाठ ही मानक रहेगा।

## उद्देश्य

यह दस्तावेज़ Cosmos/Tendermint शैली की अपेक्षाओं के विरुद्ध release gateको समझाता है। Implementation और operation में उपयोग होने वाले commands, JSON fields, RPC names, config key और code identifiers compatibility के लिए अंग्रेज़ी में ही रहेंगे।

## मुख्य दायरा

- इस दस्तावेज़ को पढ़ते समय नीचे दिए बिंदु अवश्य जाँचें। commands, JSON fields, RPC methods, config keys और code identifiers compatibility के लिए अंग्रेज़ी में ही रखे जाते हैं।
- विस्तृत normative भाषा के लिए अंग्रेज़ी मूल दस्तावेज़ देखें।
- Canonical path: `docs/release/cosmos-comparison-gate.md`
- Locale path: `docs/locales/hi/release/cosmos-comparison-gate.md`

## संरक्षित identifier

- `release gate`
- `--longrun-evidence`
- `--chaos-evidence`
- `--ops-runbook-evidence`
- `--external-audit`
- `--formal-safety-evidence`
- `--fuzz-evidence`
- `--sdk-conformance-evidence`
- `--p2p-scale-evidence`
- `--state-sync-light-client-evidence`
- `--snapshot-evidence`
- `--validator-economics-evidence`
- `--upgrade-governance-evidence`
- `--mev-fee-market-evidence`
- `--kms-evidence`
- `--bls-audit`

## अंग्रेज़ी मूल अनुभाग

- Cosmos/Tendermint Comparison Gate
- Required Evidence Properties
- Release Rule

## ऑपरेशनल नोट

- `MUST`, `SHOULD`, `MAY`, कमांड उदाहरण, JSON उदाहरण और RPC नाम अंग्रेज़ी वर्तनी में ही रहेंगे।
- इस translation को बदलने के बाद `make docs-check` चलाएँ।
- यदि यह पेज अंग्रेज़ी source से अलग हो, तो अंग्रेज़ी source मानें और उसी change में इस locale file को update करें।

## Canonical स्रोत

- [English canonical document](../../en/release/cosmos-comparison-gate.md)
