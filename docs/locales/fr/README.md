# Documentation

> Locale: fr · Français
> Ce document est un document d’accompagnement français à lire avec la source anglaise. Les décisions de protocole, de sécurité et de release restent normatives en anglais.

## Démarrage rapide

- Utilisez `make build` pour compiler le binaire.
- Créez un validator home avec `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`, puis vérifiez-le avec `vexod validate --home .vexo-validator-1` et `vexod config audit --home .vexo-validator-1 --strict` avant de lancer `vexod start --home .vexo-validator-1`.
- Pour le réseau Docker, exécutez d’abord `docker compose -f deployments/docker/compose.single-host-init.yml up`, puis `docker compose -f deployments/docker/compose.single-host.yml up`.
- Remix doit pointer vers `http://127.0.0.1:28657/web3`; pour vérifier l’ID de chaîne, utilisez `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`.
- Après toute modification, lancez `make docs-check` pour valider l’arborescence locale et les garde-fous de traduction.
## Vue d’ensemble

Ce document aide à comprendre l’index de documentation et l’ordre de lecture recommandé et à relier ce sujet aux décisions d’implémentation et d’exploitation.

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/fr/README.md`

## Pourquoi lire ce document

- l’index de documentation et l’ordre de lecture recommandé
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

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## Structure de la source anglaise

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## Source canonique

- [Document canonique anglais](../en/README.md)

## Liste des termes à conserver

Les termes ci-dessous ne doivent pas être traduits.

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: How to Read This Set — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Start Here — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Protocol Specs — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: SDK and Extension Guides — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Operations and Release — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Security — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Localized Documentation — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Writing New Docs — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Production Claim Rule — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Documentation Review Checklist — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `vexo-consensus` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `/v1/*` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `make docs-check` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod status --json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `feature_assurance` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `network_config.json:p2p.auth_replay_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `network_config.json:p2p.node_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `module_config.json:governance.RequireDeposit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `module_config.json:governance.MinDeposit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `consensus_config.json:consensus.execution_commit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `mempool_config.json:mempool.WALPath` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
