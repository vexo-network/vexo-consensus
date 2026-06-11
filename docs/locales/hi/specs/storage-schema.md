# Storage Schema

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।

## सारांश

यह दस्तावेज़ durable storage namespace, key schema और recovery marker को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/hi/specs/storage-schema.md`

## यह दस्तावेज़ क्यों पढ़ें

- durable storage namespace, key schema और recovery marker
- पहले अंग्रेज़ी source में MUST/SHOULD/MAY statements जाँचें।
- यह स्थानीयकृत दस्तावेज़ समझने में मदद करता है; audit, release और security decisions अंग्रेज़ी source से तय होते हैं।

## पढ़ने के बाद क्या कर पाना चाहिए

- समझाना कि यह दस्तावेज़ किस implementation या operation decision में मदद करता है।
- अंग्रेज़ी source की normative requirements को वर्तमान network configuration से जोड़ना।
- examples copy करने से पहले chain ID, validator ID, fee/gas और peer addresses जाँचना।

## सुरक्षित उपयोग checklist

- पहले अंग्रेज़ी source में MUST/SHOULD/MAY statements जाँचें।
- commands, config key, RPC names, JSON fields और code identifiers का अनुवाद न करें।
- examples copy करने से पहले chain ID, validator ID, fee/gas और peer addresses अपनी network settings से मिलाएँ।
- documentation बदलने के बाद `make docs-check` चलाकर locale tree और translation guards जाँचें।

## ध्यान रखने योग्य बातें

- यह स्थानीयकृत दस्तावेज़ समझने में मदद करता है; audit, release और security decisions अंग्रेज़ी source से तय होते हैं।
- implementation बदलने पर अंग्रेज़ी source और सभी स्थानीयकृत दस्तावेज़ को उसी change में update करें।

## ज्यों-का-त्यों रखने वाले interfaces

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
- `evm_ethstate/{height}/meta`
- `evm_ethstate/{height}/accounts/{0x_address}`
- `eth_getProof`
- `stateRoot`
- `evm_ethstate/{height}`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## अंग्रेज़ी source की संरचना

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

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../../en/specs/storage-schema.md)
