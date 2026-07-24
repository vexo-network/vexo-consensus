> Locale: fr · Français

# Documentation

Ce répertoire est le manuel pratique de `vexo-consensus`. Il s'adresse aux développeurs, opérateurs, responsables de publication et auditeurs qui doivent comprendre le réseau sans déduire son comportement uniquement à partir du code.

Chaque page doit expliquer la responsabilité du composant, les commandes, fichiers, clés de configuration et API qui l'implémentent, les conditions de sûreté et les preuves exigées avant un réseau réel. L'anglais reste la source normative pour le protocole, la sécurité, la publication, le SDK, les commandes, la configuration et RPC; cette traduction facilite la lecture mais ne remplace pas la source anglaise pour une décision d'audit.

Pour un premier parcours, utilisez les commandes ci-dessous, puis lisez `Node Initialization`, `Docker Deployment`, `Observability Guide` et `RPC API Versioning`.

| Tâche | Chemin de commande |
|---|---|
| Build local binary | `make build` |
| Créer un validateur home | `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` |
| Valider un logement | `vexod validate --home .vexo-validator-1` et `vexod config audit --home .vexo-validator-1 --strict` |
| Exécuter un nœud | `vexod start --home .vexo-validator-1` |
| Interroger un nœud | `curl -s http://127.0.0.1:26657/v1/status` |
| Exécuter le réseau à quatre validateurs Docker | `docker compose -f deployments/docker/compose.single-host-init.yml up` suivi de `docker compose -f deployments/docker/compose.single-host.yml up` |
| Connect Remix | Utiliser le validateur Docker 1 URL Web3 `http://127.0.0.1:28657/web3` |
| Vérifier l'ID de la chaîne Web3 | `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'` |

## Démarrage rapide

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## Commencez ici

| Document | Objet |
|---|---|
| [Guide de préparation à la production](./production-readiness.md) | Carte unique du protocole, de l'exécution, des opérations, des preuves et de la préparation à la mise en production |

## Spécifications du protocole

- [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md) et [Validator Lifecycle](./specs/validator-lifecycle.md) décrivent la sûreté, la finalité et les changements du validator set.
- [Networking Spec](./specs/networking-spec.md), [Storage Schema](./specs/storage-schema.md) et [Transaction Format](./specs/tx-format.md) couvrent transport, récupération durable et admission des transactions.
- [EVM and Native Accounting](./specs/evm-native-accounting.md) définit la frontière entre les soldes natifs et EVM.

## SDK et extensions

[App Module Guide](./sdk/app-module-guide.md), [Custom Crypto Backend](./sdk/custom-crypto-backend.md), [Custom Storage and Transport](./sdk/custom-storage-transport.md) et `RPC API Versioning` expliquent comment étendre le runtime sans rompre les contrats de consensus ou RPC.

## Exploitation, publication et sécurité

`Node Initialization`, [Adding a Validator](./operators/add-validator.md), `Observability Guide`, [Runbook de lancement](./release/launch-runbook.md), `Release Pipeline` et [Version Compatibility Matrix](./release/version-compatibility.md) forment le parcours opérateur. [Security Audit Readiness](./security/audit-readiness.md) énumère le modèle de menace et les preuves obligatoires.

## Règle de maturité

Le code seul ne prouve pas qu'une fonction est prête pour la production. Il faut des tests unitaires, adversariaux et E2E, des artefacts d'exploitation, les hypothèses et modes d'échec, ainsi que les résultats du release gate. Les commandes, méthodes RPC et clés de configuration restent identiques dans toutes les traductions.

## Recherche et publication

Pour préparer un article, commencez par [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md). Ce document distingue les mécanismes réellement implémentés, notamment le délai de ronde adaptatif, la barrière de finalité pendant la récupération et l'ordre déterministe des transactions, des travaux antérieurs. Il rassemble les questions de recherche, les hypothèses, le protocole expérimental, les artefacts reproductibles et les règles d'éthique. Il ne présente pas une performance non mesurée comme un résultat et ne revendique pas PoS, BFT ou HotStuff comme une invention.

Les noms normatifs conservés pour la navigation multilingue sont `Node Initialization`, `Docker Deployment`, `Observability Guide`, `RPC API Versioning`, `Production Readiness`, `Release Pipeline` et `Adaptive Recovery-Gated HotStuff Research Draft`.

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
