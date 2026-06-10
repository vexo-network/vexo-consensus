# Adding a Validator

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।

## सारांश

यह दस्तावेज़ validator जोड़ने की प्रक्रिया, config validation और staking checks को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/hi/operators/add-validator.md`

## यह दस्तावेज़ क्यों पढ़ें

- validator जोड़ने की प्रक्रिया, config validation और staking checks
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

- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `bls_pop`
- `.vexo-validator-new/network_config.json`
- `network_config.json`
- `p2p.listen_address`
- `rpc.address`
- `p2p.peers`
- `p2p_address`
- `rpc_address`
- `active_from`
- `active_until`
- `config audit --strict`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## अंग्रेज़ी source की संरचना

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## Canonical source

- [English canonical document](../../en/operators/add-validator.md)
