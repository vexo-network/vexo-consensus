# EVM et comptabilité native

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.


## Ordre de lecture

Ce document explique la spécification normative de Evm Native Accounting. Si c'est votre première lecture, suivez cet ordre.

1. Core Rule
2. Amount Encoding
3. Fee Accounting
4. EVM Execution
5. State Root Policy
6. Compatibility Boundary
7. Failure Modes

Cet ordre correspond à la manière de lire le document : d'abord le périmètre et l'état, puis les règles des messages, de sûreté et de vivacité, et enfin les preuves.

## Vue d’ensemble

Ce document aide à comprendre l’alignement entre native coin et EVM gas/accounting et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/fr/specs/evm-native-accounting.md`

## Pourquoi lire ce document

- l’alignement entre native coin et EVM gas/accounting
- Vérifiez d’abord les phrases MUST/SHOULD/MAY dans la source anglaise.
- Ce document localisé aide à la compréhension ; l’audit, le release et la sécurité se décident sur la source anglaise.

## Ce que vous devez savoir faire

- Expliquer quelle décision d’implémentation ou d’exploitation ce document soutient.
- Relier les exigences normatives de la source anglaise à la configuration réseau actuelle.
- Vérifier chain ID, validator ID, fee/gas et adresses peer avant de copier les exemples.

## Checklist d’utilisation sûre

- Vérifiez d’abord les phrases MUST/SHOULD/MAY dans la source anglaise.
- Ne traduisez pas les commandes, config key, noms RPC, champs JSON ni identifiants de code.
- Avant de copier des exemples, adaptez chain ID, validator ID, fee/gas et adresses peer à votre réseau.
- Après modification, exécutez `make docs-check` pour vérifier le locale tree et les garde-fous de traduction.

## Points d’attention

- Ce document localisé aide à la compréhension ; l’audit, le release et la sécurité se décident sur la source anglaise.
- Quand l’implémentation change, mettez à jour la source anglaise et tous les documents localisés dans le même changement.

## Interfaces à conserver telles quelles

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`
- `bank query balance`

## Structure de la source anglaise

- EVM et comptabilité native
- Core Rule
- Amount Encoding
- Fee Accounting
- Exécution EVM
- State Root Policy
- Compatibility Boundary
- Failure Modes

## Source canonique

- [Document canonique anglais](../../en/specs/evm-native-accounting.md)

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Core Rule — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Amount Encoding — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Fee Accounting — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: EVM Execution — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: State Root Policy — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Compatibility Boundary — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Failure Modes — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `base_fee * gas` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `contract.Invocation` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `value_hex` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `gas_price_hex` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `max_fee_per_gas_hex` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `max_priority_fee_per_gas_hex` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_getBalance` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_sendRawBlobTransaction` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexo_sendRawBlobTransaction` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_sendRawTransaction` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `execution.strict_evm_state_root` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
