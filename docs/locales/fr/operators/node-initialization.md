> Locale: fr · Français

# Initialisation du nœud

Ce guide explique comment initialiser les maisons de nœuds de validation et d'archivage, les démarrer, vérifier qu'ils sont sains et connecter les clients.

La connectivité homologue doit être configurée dans `network_config.json`, et non transmise à plusieurs reprises sur la ligne de commande `start`.

Le comportement d'exécution qui affecte les comptes de consensus, RPC, P2P, de journalisation ou Web3 gérés concerne uniquement le fichier de configuration. `vexod start` rejette les indicateurs tels que `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env` et `--evm-account-key` ; modifiez plutôt les fichiers de configuration fractionnés afin que chaque opérateur examine le même comportement de nœud déterministe.

Il n'y a pas de commutateur de mode nœud. Un nœud home est défini par ses fichiers de configuration, sa genèse, ses clés et la présence ou non de `validator_id` et d'un signataire.

## Ce que vous construisez

Un nœud Home Vexo est un répertoire qui contient tout ce dont un nœud a besoin pour démarrer :
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
La règle importante est simple : initialisez une fois, modifiez les fichiers de configuration, puis démarrez. Ne cachez pas le comportement du réseau dans les indicateurs du shell.

## Exécution locale de cinq minutes

Utilisez ce flux lorsque vous souhaitez prouver le fonctionnement du binaire avant de penser au déploiement multi-hôtes.
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
Dans un autre terminal :
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
Forme de statut attendue :
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
La dernière hauteur peut rester à zéro lors d'une exécution à nœud unique ou avec un pool de mémoire vide lorsque la création de blocs vides est désactivée. Cela ne veut pas dire que le processus est interrompu. Cela signifie que le nœud ne produit pas de blocs vides. Ajoutez des transactions ou exécutez un réseau de test multi-validateur pour observer les validations continues.

## Réseau local à quatre validateurs

Utilisez ce flux lorsque vous souhaitez une connectivité entre pairs, une rotation des proposants, des journaux de validation de bloc et une croissance en hauteur.
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
Vérifications utiles :
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
Si la journalisation des validations de bloc est activée dans `log_config.json`, les journaux du validateur incluent des événements tels que :
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
Arrêtez le réseau local généré avec :
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 et Remix

Le JSON-RPC de style Ethereum réside au niveau du point de terminaison Web3, et non sous l'espace de noms de l'API opérationnelle Vexo versionné.

Pour le validateur Docker à hôte unique 1, l'URL du fournisseur personnalisé Remix est :
```text
http://127.0.0.1:28657/web3
```
Pour un nœud local direct avec le port RPC par défaut :
```text
http://127.0.0.1:26657/web3
```
Testez le même appel que Remix fait :
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
Si un navigateur indique que la récupération de l'ID de chaîne a échoué, vérifiez ces éléments dans l'ordre :

1. L'URL se termine par le chemin du point de terminaison Web3.
2. Le navigateur peut atteindre le port hôte. Les exemples Docker exposent `28657`, `28667`, `28677` et `28687` ; à l'intérieur du conteneur, le port RPC est toujours `26657`.
3. Le serveur RPC est en cours d'exécution ; interrogez le point de terminaison d’état sur le même hôte et le même port.
4. CORS est autorisé par la configuration `network_config.json`/RPC. Le gestionnaire par défaut autorise le contrôle en amont du navigateur lorsqu'aucune liste CORS personnalisée n'est définie.
5. La chaîne a un ID de chaîne EVM différent de zéro dans `module_config.json`.

## Nœud de validation

Utilisez `init validator` lorsque le nœud proposera, votera, signera des messages de consensus et participera à la rotation des validateurs.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
Définissez `VEXO_KEY_PASSPHRASE` avant d'exécuter cette commande, ou transmettez `--passphrase` pour une configuration locale unique. `--encrypt-keys` crypte `validator.key.json`, `node.key.json` et `validator.vrf.key.json`.

Règle générale de garde des clés :

- `validator.key.json` signe les propositions de consensus, les votes, les votes de délai d'attente et les messages liés à la finalité.
- `node.key.json` signe uniquement les poignées de main P2P ; elle ne doit jamais être réutilisée comme clé de consensus du validateur.
- `validator.vrf.key.json` prouve le caractère aléatoire du comité et doit être traité comme un matériel de garde du validateur.
- Les auditeurs publics doivent utiliser des documents de clé locale chiffrés ou des documents de clé de type signataire distant/KMS. Si un nœud expose un RPC public ou un P2P public authentifié pendant `require_network_safety=true`, le démarrage rejette les clés de validation locales en texte brut.
- Les clés générées sont écrites en mode système de fichiers `0600` ; préférez toujours un signataire/KMS distant pour les validateurs de longue durée.

Pour une clé de consensus BLS :
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` écrit un document clé `blst-bls12381-minpk-v1` BLS et copie la preuve de possession dans les métadonnées du validateur `genesis.json` sous le nom `bls_pop`.

Cela crée :

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` est le signataire du consensus. `node.key.json` est le signataire de la négociation P2P référencé par `network_config.json:p2p.node_key_path`. Ils sont délibérément séparés afin que les nœuds d'archives et les validateurs puissent utiliser le même transport sans donner à chaque homologue une clé de signature de validateur.

Démarrez-le avec un réseau basé sur la configuration :
```bash
vexod start --home .vexo-validator-1
```
Après le démarrage, lisez les journaux. Un validateur sain doit émettre des événements d'exécution de nœud, d'écoute RPC, d'écoute P2P et, une fois les blocs validés, de validation de bloc. Si la création de blocs vides est désactivée, les journaux manquants des blocs validés peuvent simplement signifier qu'il n'y a aucune transaction.

## Nœud d'archive

Utilisez `init archive` lorsque le nœud doit conserver les données de la chaîne, exposer RPC, se synchroniser avec ses pairs et éviter la signature du validateur.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
Cela crée :

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

Cela ne crée **pas** `validator.key.json`.

Commencez-le avec :
```bash
vexod start --home .vexo-archive-1
```
Les nœuds d'archives ne signent pas les votes de consensus. Ils sont utiles pour le RPC, l’indexation, la synchronisation d’état, la diffusion de preuves historiques et la conservation d’un historique de requêtes plus large que celui des validateurs d’élagage.

## Diviser les fichiers de configuration

Les maisons de nœuds utilisent des fichiers de configuration distincts afin que les opérateurs puissent modifier un sous-système sans mélanger des paramètres non liés :

- `config.json` contient l'identité du nœud, l'ID de chaîne, le chemin des données et des pointeurs vers les fichiers de configuration fractionnés.
- `module_config.json` contient la sélection du module d'application, la politique d'exécution/ante et la politique de gouvernance au niveau du module.
- `network_config.json` contient RPC, l'identité du nœud P2P, les paramètres d'écoute/peer/seed, les paramètres TLS/auth et la politique de notation par les pairs.
- `consensus_config.json` contient le timing de la boucle de consensus, la politique de bloc vide, le backend cryptographique, le VRF, l'admission du validateur et la politique du comité.
- `mempool_config.json` contient la taille du pool de mémoire, les frais, la priorité, les WAL, les doublons et la politique TTL.
- `log_config.json` contient le format de journal, le niveau, la journalisation des événements de validation de bloc et la journalisation des événements homologues.
- `genesis.json` contient des validateurs de genèse immuables, des métadonnées du validateur et l'état du module de genèse.

Les paramètres `network_config.json` RPC incluent également `shutdown_timeout`, `web3_max_subscriptions_per_connection` et `web3_idle_timeout`. `shutdown_timeout` limite l'arrêt progressif de la boucle de consensus, du serveur RPC et du transport de nœuds afin que les opérateurs n'attendent pas indéfiniment sur un chemin d'arrêt bloqué. La valeur par défaut générée est `10s` ; Les abonnements Web3 sont par défaut de 256 par connexion avec un délai d'inactivité de `2m` afin que les points de terminaison RPC publics ne puissent pas accumuler d'abonnements inactifs illimités.

`network_config.json` Les paramètres P2P incluent `auth_replay_path`, `require_auth_replay_store` et `dial_timeout`. La valeur par défaut générée écrit les preuves de relecture occasionnelles dans `data/p2p_auth_replay.jsonl` et utilise un délai d'expiration de numérotation sortante `10s`. Pour les tests de bouclage privé, le magasin de relecture est pour l'essentiel une comptabilité inoffensive ; pour le P2P authentifié publiquement, il s'agit d'une exigence de sécurité car elle empêche une poignée de main signée capturée d'être rejouée après le redémarrage. `dial_timeout` doit être suffisamment long pour TLS, la vérification de la négociation signée et la latence inter-régions ; un réglage trop bas donne un aspect floconneux aux pairs en bonne santé et peut ralentir la vivacité après les redémarrages.

`network_config.json` possède également la synchronisation de l'état de démarrage. Ceci est utile pour les nœuds d'archive, les validateurs de remplacement ou les nœuds restaurés sur une machine propre. Lorsque `state_sync.enabled` est vrai, `vexod start` télécharge le premier instantané valide de `state_sync.snapshot_urls`, vérifie l'ID de chaîne, la somme de contrôle, les racines d'état et les espaces de noms KV, le restaure dans LevelDB, reconstruit les index et démarre ensuite le nœud. Si l'état local satisfait déjà à `state_sync.min_height` et que `state_sync.trust_local_higher` est vrai, le démarrage enregistre `state_sync_skipped` et conserve le magasin local.

Exemple de bloc `state_sync` :
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
Journaux de démarrage `state_sync_candidate_failed` pour une erreur de récupération, `state_sync_candidate_rejected` pour un instantané invalide ou obsolète et `state_sync_applied` après une restauration vérifiée. Gardez `max_snapshot_bytes` en dessous du plus grand instantané que votre infrastructure sert intentionnellement, mais suffisamment élevé pour une croissance normale de l'état. Ne pointez pas les nœuds publics vers une source d'instantanés tierce non authentifiée, sauf si l'opérateur dispose d'une politique de confiance hors bande et d'une preuve de finalité/client léger pour cette source.

Si un champ modifie le comportement du réseau, modifiez le fichier de configuration fractionné et validez ou distribuez ce fichier révisé. Ne vous fiez pas aux longs indicateurs `vexod start` pour le comportement d'exécution. La commande start rejette intentionnellement les indicateurs de synchronisation de consensus, de bloc vide, d'authentification P2P, d'administrateur RPC et de clé Web3 gérée afin que les opérateurs n'exécutent pas accidentellement un comportement différent de la configuration examinée.

## Quel fichier dois-je modifier ?

| Objectif | Fichier | Champ |
|---|---|---|
| Changer le port de liaison RPC | `network_config.json` | `rpc.address` |
| Changer le port de liaison P2P | `network_config.json` | `p2p.listen_address` |
| Ajouter des pairs persistants | `network_config.json` | `p2p.peers` |
| Ajouter des pairs de départ | `network_config.json` | `p2p.seeds` |
| Activer/désactiver les blocs vides | `consensus_config.json` | champ de bloc vide de consensus |
| Ajuster les délais d'attente du consensus | `consensus_config.json` | champs de proposition, de prévote, de pré-validation et de délai d'expiration de validation |
| Exiger une exécution finalisée | `consensus_config.json` | champ d'exécution-engagement de consensus |
| Activer/désactiver les modules | `module_config.json` | liste des modules d'application |
| Modifier l'ID de la chaîne EVM | `module_config.json` | champ d'ID de chaîne EVM d'exécution |
| Régler les frais de base/gaz | `module_config.json` | champs de frais de base d'exécution, de frais dynamiques, de gaz cible et de gaz maximum |
| Configurer le pool de mémoire WAL | `mempool_config.json` | chemin WAL du pool de mémoire |
| Journaux de validation du bloc de contrôle | `log_config.json` | champ des événements de validation du journal |
| Contrôler les journaux des pairs | `log_config.json` | enregistrer le champ des événements homologues |

En cas de doute, exécutez :
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## Types de clés

L'initialisation du validateur est par défaut `--key-type bls` car la validation de la sécurité du réseau nécessite une finalité globale BLS auditée. `--key-type ed25519` reste disponible pour les expériences privées et les déploiements personnalisés en dehors de la barrière de sécurité du réseau. `--encrypt-keys` doit être utilisé pour tout nœud domestique non jetable. La génération de clés autonome prend également en charge les clés VRF :
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
Les clés VRF ne sont pas des signataires de consensus. Ils sont utilisés pour la sélection des comités soutenus par le VRF et doivent être référencés de `consensus_config.json` à `vrf_key_paths` plus la clé de métadonnées du validateur `vrf_public_key` lorsque ce backend est activé.

`config.json` pointe vers les fichiers de configuration fractionnés :
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
Chaque chemin peut être absolu ou relatif au nœud home. En cas d'omission, `vexod` utilise le fichier `<home>/<name>_config.json` par défaut.

Exemple `module_config.json` :
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
La politique de gouvernance réside également dans `module_config.json`. Les configurations sécurisées pour le réseau générées nécessitent un dépôt de proposition :
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
Le dépôt est le solde natif déposé par le soumissionnaire de la proposition. Les propositions réussies remboursent l'acompte ; les propositions rejetées le déplacent vers `RejectedDeposits`. Utilisez une adresse contrôlée par votre module de trésorerie/pool communautaire si les dépôts rejetés doivent financer une trésorerie au lieu du compte de module par défaut.

Exemple `network_config.json` :
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` et `rpc.evm_account_private_keys` sont des méthodes de compte géré Web3 facultatives et inverses telles que `eth_accounts`, `eth_sign`, `eth_signTransaction` et `eth_sendTransaction`. Préférez `evm_account_key_envs` pour que la clé privée soit injectée par l'environnement de processus ou le gestionnaire de secrets au lieu d'être stockée dans JSON. Gardez les deux listes vides pour le fonctionnement normal du validateur, à moins que ce nœud n'agisse intentionnellement comme un point de terminaison de portefeuille chaud Web3 local. La sécurité de démarrage rejette les touches de raccourci EVM gérées sur les écouteurs RPC publics.

Exemple `consensus_config.json` :
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` sont résolus par rapport au répertoire contenant `consensus_config.json`. Utilisez des documents de clé chiffrés et fournissez `VEXO_KEY_PASSPHRASE` au processus de nœud lorsque la conservation des clés VRF locales est inévitable. Ne placez pas les scalaires privés VRF bruts directement dans `consensus_config.json` pour les réseaux gérés par l'opérateur.

Utilisez `vexod config paths --home <home>` pour inspecter tous les chemins résolus.

La configuration de l'archive contient :
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
L'archive `consensus_config.json` désactive la boucle de consensus local :
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
Les maisons de validation générées définissent `"require_network_safety": true` dans `config.json` par défaut. Ce n’est pas un mode ; il s'agit d'une barrière de sécurité de démarrage qui rejette la cryptographie déterministe, les transactions non signées/non-officielles, les planchers de frais/gaz manquants, les WAL de pool de mémoire durables manquants, la politique de remplacement manquante pour les transactions du même signataire/nonce, le caractère aléatoire du comité dangereux et les valeurs `execution_commit` autres que `finalized`.

Lorsque `require_network_safety` est activé, exécutez :
```bash
vexod config audit --home <home> --strict
```
avant de démarrer le nœud. L'audit doit réussir pour chaque validateur et maison d'archives qui participe au même réseau.

## Pairs basés sur la configuration

Regardez et écoutez les adresses en direct dans `network_config.json` :
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` charge automatiquement ces pairs :
```bash
vexod start --home .vexo-archive-1
```
Les homologues et graines persistants sont configurés dans `network_config.json` ; `vexod start` n'accepte pas les remplacements d'homologues ou d'hôtes de départ.

Ne placez pas les paramètres d'hôte de longue durée ou `host:port` sur la ligne de commande `vexod start`. Modifiez plutôt `rpc.address`, `p2p.listen_address`, `p2p.peers` et `p2p.seeds` dans `network_config.json`.

Gardez `p2p.node_id` stable pendant toute la durée de vie du nœud home. `p2p.node_key_path` doit pointer vers `node.key.json` ou un autre document de clé local/géré utilisé uniquement pour la signature de prise de contact entre homologues. Les mappages homologues doivent utiliser des ID de nœud homologue, et non des adresses de compte ou des noms d’opérateurs de validation, à moins qu’ils ne soient intentionnellement identiques.

Pour le transport homologue gRPC chiffré et authentifié, définissez également `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path` et éventuellement `p2p.tls_server_name` dans `network_config.json`. Les chemins TLS relatifs sont résolus à partir du répertoire de base du nœud. Conservez `p2p.dial_timeout` dans le même fichier afin que chaque opérateur utilise le même comportement de reconnexion ; ne cachez pas le timing des pairs dans les scripts shell.

## Calendrier du consensus

Le timing de la boucle de consensus se trouve dans `consensus_config.json` :
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` contrôle combien de temps un tour attend une proposition.
- `timeout_prevote` contrôle la fenêtre de collecte des votes.
- `timeout_precommit` contrôle la fenêtre de collecte du certificat de validation.
- `timeout_commit` contrôle le délai minimum après un bloc validé.
- `create_empty_blocks: false` signifie que le nœud ne propose que lorsque les transactions sont disponibles.
- `execution_commit: "finalized"` attend la décision de finalité à trois chaînes HotStuff avant d'exécuter l'ancêtre finalisé et est la valeur par défaut du validateur généré. `execution_commit: "qc"` exécute et conserve immédiatement les blocs certifiés QC, mais la barrière de sécurité les rejette.

`round_timeout` est conservé uniquement comme agrégat de compatibilité. Préférez les champs de délai d'attente de style Tendermint ci-dessus.

Lorsque `create_empty_blocks` est faux, la hauteur peut rester inchangée tant que le pool de mémoire est vide. C'est normal : la chaîne attend un travail utile au lieu de valider des blocs vides. Lorsqu'une transaction apparaît et que l'état du cycle de consensus local a dépassé un autre proposant, le nœud passe au tour suivant où son validateur est le proposant et construit à partir du mempool. Ce chemin de récupération maintient la vivacité déclenchée par la transaction sans réactiver le spam en bloc vide.

## Réseau multi-validateur

Pour un réseau généré :
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
Chaque maison de validateur générée reçoit :

- son propre `validator.key.json`
- ses propres fichiers de configuration fractionnés : `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` et `log_config.json`
- un `genesis.json` partagé
- `network_config.json` entrées homologues pour les autres validateurs

`vexod network up` et `make network-e2e` utilisent un délai d'attente au niveau du processus en attendant que tous les validateurs démarrent, soumettent la transaction de fumée et observent la croissance en hauteur. Le délai d'expiration de la commande par défaut est intentionnellement plus long que l'intervalle de consensus, car il couvre le démarrage du processus, l'ouverture de LevelDB, les poignées de main signées P2P, les vérifications TLS/auth, l'admission des transactions et la finalité. Si vous réduisez de manière agressive les délais d'attente du consensus, gardez le délai d'attente du réseau suffisamment grand pour diagnostiquer les erreurs de démarrage au lieu de tuer le faisceau trop tôt.

Pour les réseaux conteneurisés ou multi-hôtes, placez les valeurs de topologie dans un fichier JSON :
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` et `rpc_host_template` sont des cibles de numérotation écrites dans la liste de pairs `network_config.json` de chaque nœud. Dans Docker, il peut s'agir de noms de services tels que `validator-%d`.
- `p2p_advertise_host_template` et `rpc_advertise_host_template` sont des adresses publiques écrites dans les métadonnées du validateur dans `genesis.json`. Utilisez ici des noms DNS ou des adresses IP publiques pour les réseaux publics.
- `p2p_listen_host` et `rpc_listen_host` sont des hôtes de liaison locaux. Utilisez `0.0.0.0` pour les conteneurs ou les serveurs qui doivent écouter sur toutes les interfaces.
- Ne réutilisez pas les noms de services Docker uniquement comme adresses publiques annoncées, sauf si le réseau est intentionnellement privé.

Générez ensuite des maisons de nœuds à partir de ce fichier :
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## Dépannage

| Symptôme | Cause la plus probable | Que vérifier |
|---|---|---|
| `latest_height` n'augmente pas | Blocs vides désactivés et pas de transmission, pas assez de validateurs en ligne ou signataire indisponible | `consensus_config.json`, journaux du validateur, `/v1/diagnostics` |
| `peer_count` est `0` | Les adresses homologues ne sont pas accessibles ou `network_config.json` a été généré pour les mauvais noms d'hôte | `p2p.peers`, ports hôtes du conteneur, DNS, pare-feu |
| Erreur `p2p auth replay store` | Le P2P public/authentifié nécessite un stockage de relecture durable | `p2p.auth_replay_path` et écrivez l'autorisation sous le domicile |
| `eth_chainId` échoue dans Remix | Mauvaise URL, mauvais port hôte ou navigateur CORS/preflight bloqué par une configuration personnalisée | Utilisez l'URL du point de terminaison Web3, puis recourbez directement le même point de terminaison |
| `config audit --strict` échoue | La porte de sécurité a trouvé une propriété de configuration dangereuse | Lisez la vérification qui a échoué, puis modifiez le fichier de configuration fractionné qu'il nomme |
| `no block_committed logs` | Journalisation désactivée ou aucun bloc n'est créé | `log_config.json`, `create_empty_blocks`, contenu du pool de mémoire |
| `managed EVM key rejected` | Les clés privées chaudes sont configurées sur un écouteur RPC public | Supprimez `evm_account_private_keys` ou gardez RPC privé |

## Liste de contrôle minimale de l'opérateur

Avant de confier un nœud à une autre machine ou à un autre opérateur :

- `vexod validate --home <home>` réussit.
- `vexod config audit --home <home> --strict` passe pour cette maison exacte.
- `config.json`, les fichiers de configuration fractionnés, `genesis.json` et les métadonnées du validateur public sont examinés.
- `validator.key.json`, `node.key.json` et `validator.vrf.key.json` sont cryptés ou remplacés par des documents de clé de signataire distant/KMS.
- `network_config.json:p2p.peers` contient des adresses composables à partir de la machine cible, et non des noms Docker uniquement, à moins que le nœud ne s'exécute réellement à l'intérieur de ce réseau Docker.
- Les écouteurs publics RPC/P2P `network_config.json` ont du matériel TLS lorsque `require_network_safety` est activé.
- `module_config.json:execution.EVMChainID` est défini avant la connexion des portefeuilles Web3 ou de Remix.
- `mempool_config.json` a un chemin WAL si le nœud doit récupérer les transmissions en attente après le redémarrage.
- `log_config.json` active la validation de bloc et les journaux d'homologues pendant la mise en place du réseau.

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: Validator Node — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Archive Node — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Split Configuration Files — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Key Types — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Config-Based Peers — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Consensus Timing — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Multi-Validator Network — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `network_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod start` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--timeout-propose` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--create-empty-blocks` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--p2p-auth-token` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--rpc-admin-token` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-account-key-env` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--evm-account-key` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `VEXO_KEY_PASSPHRASE` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--passphrase` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--encrypt-keys` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator.key.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `node.key.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator.vrf.key.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `require_network_safety=true` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--key-type bls` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `blst-bls12381-minpk-v1` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `genesis.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `bls_pop` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `module_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `consensus_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `mempool_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `log_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `data/` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `network_config.json:p2p.node_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `shutdown_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `web3_max_subscriptions_per_connection` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `web3_idle_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `auth_replay_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `require_auth_replay_store` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `dial_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `data/p2p_auth_replay.jsonl` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--key-type ed25519` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vrf_key_paths` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vrf_public_key` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `<home>/<name>_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc.evm_account_key_envs` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc.evm_account_private_keys` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_accounts` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_sign` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_signTransaction` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `eth_sendTransaction` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `evm_account_key_envs` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod config paths --home <home>` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `"require_network_safety": true` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `execution_commit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `require_network_safety` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `host:port` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc.address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.listen_address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.peers` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.seeds` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.node_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.node_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_cert_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_ca_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.tls_server_name` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.dial_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_propose` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_prevote` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_precommit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `timeout_commit` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `create_empty_blocks: false` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `execution_commit: "finalized"` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `execution_commit: "qc"` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `round_timeout` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `create_empty_blocks` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod network up` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `make network-e2e` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `validator-%d` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_advertise_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_advertise_host_template` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_listen_host` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_listen_host` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
