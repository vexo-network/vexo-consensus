# App Module Guide

> Locale: hi · हिन्दी
> यह दस्तावेज़ अंग्रेज़ी source के साथ पढ़ने के लिए हिन्दी सहायक दस्तावेज़ है। protocol, security और release निर्णयों के लिए अंग्रेज़ी source ही मानक है।

## सारांश

यह दस्तावेज़ नया app module बनाना और CLI/RPC/state storage से जोड़ना को समझने और उसे implementation व operation decisions से जोड़ने में मदद करता है।

- Canonical path: `docs/sdk/app-module-guide.md`
- Locale path: `docs/locales/hi/sdk/app-module-guide.md`

## यह दस्तावेज़ क्यों पढ़ें

- नया app module बनाना और CLI/RPC/state storage से जोड़ना
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

- `app.Module`
- `app.QueryHandler`
- `app.ValidatorUpdateProvider`
- `app.TxEventEmitter`
- `app.PruneHook`
- `bank`
- `bank:`
- `module_config.json`
- `config.json`
- `module_config_path`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `app.Context.Store`
- `ctx.GoContext()`
- `CheckTx`
- `PrepareProposal`
- `ProcessProposal`
- `FinalizeBlock`
- `Query`
- `params`

## अंग्रेज़ी source की संरचना

- App Module Guide
- Goal
- Module Interface
- Transaction Routing
- Module Configuration
- State
- Events and Query Proofs
- IBC and Contract Extension Points
- Genesis
- Ante Handling
- CLI Commands
- Tests

## प्रामाणिक स्रोत

- [अंग्रेज़ी प्रामाणिक दस्तावेज़](../../en/sdk/app-module-guide.md)
