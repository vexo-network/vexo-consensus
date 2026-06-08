# Transaction Format

> Locale: fr · Français
> Ce document est un guide traduit à partir de la documentation anglaise canonique. Les décisions de protocole, de sécurité et de publication restent normatives en anglais.

## Objectif

Ce document couvre transaction format, signing, fee et règles de gas. Les commandes, champs JSON, noms RPC, config key et identifiants de code utilisés par l’implémentation et l’exploitation restent en anglais pour préserver la compatibilité.

## Périmètre essentiel

- Vérifiez les points suivants lors de la lecture. Les commandes, champs JSON, méthodes RPC, clés de configuration et identifiants de code restent en anglais pour préserver la compatibilité.
- Pour les formulations normatives détaillées, utilisez le document anglais.
- Canonical path: `docs/specs/tx-format.md`
- Locale path: `docs/locales/fr/specs/tx-format.md`

## Identifiants à conserver

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

## Sections anglaises

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

## Notes opérationnelles

- `MUST`, `SHOULD`, `MAY`, les exemples de commande, les exemples JSON et les noms RPC conservent l’orthographe anglaise.
- Après modification de cette traduction, exécutez `make docs-check`.
- Si cette page contredit la source anglaise, utilisez la source anglaise et mettez à jour ce fichier locale dans le même changement.

## Source canonique

- [English canonical document](../../en/specs/tx-format.md)
