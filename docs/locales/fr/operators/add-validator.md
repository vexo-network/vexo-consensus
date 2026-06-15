> Locale: fr · Français

# Ajout d'un validateur

Ce guide décrit le flux de l'opérateur pour ajouter un validateur à un réseau Vexo.

Le chemin d'admission exact dépend de la politique de jalonnement et de gouvernance de la chaîne. Au minimum, le validateur doit être représenté dans un état de chaîne, disposer d'informations d'identification valides et faire partie d'une mise à jour de l'ensemble de validateurs versionnée en hauteur.

## 1. Initialiser l'accueil du validateur
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
Pour une clé de validateur BLS :
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
Définissez `VEXO_KEY_PASSPHRASE` avant d'exécuter ces commandes, ou transmettez `--passphrase` pour une configuration locale unique.

Lors de l'admission d'un validateur BLS dans une chaîne existante, incluez les métadonnées `bls_pop` générées dans la proposition de mise à jour du validateur.
Le chemin de clé BLS par défaut utilise `blst-bls12381-minpk-v1` ; utilisez `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` uniquement pour les tests de référence/compatibilité.

Archivez la clé publique générée :
```bash
vexod keys show --home .vexo-validator-new --json
```
Conservez également le `node.key.json` généré. Il signe les poignées de main P2P pour `network_config.json:p2p.node_id` ; il ne s'agit pas d'une clé de consensus de validateur et ne doit pas être réutilisée comme clé de compte.

## 2. Configurer les adresses réseau et les pairs

Modifiez `.vexo-validator-new/network_config.json` et définissez les adresses d'écoute locales ainsi que les homologues persistants :
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
Ne comptez pas sur des remplacements de mise en réseau en ligne de commande de longue durée pour les validateurs de production. Conservez les adresses homologues persistantes dans `network_config.json`.

Utilisez des rôles d'adresse distincts :

- `p2p.listen_address` et `rpc.address` sont des adresses de liaison locales pour cette machine ou ce conteneur.
- `p2p.node_id` est l'identité homologue de ce nœud. Gardez-le stable une fois que vos pairs l'ont appris.
- `p2p.node_key_path` pointe vers la clé de signature de prise de contact locale pour cette identité d'homologue.
- `p2p.peers` contient les cibles de numérotation que ce nœud utilise pour atteindre d'autres pairs ; les clés de mappage doivent être les valeurs `p2p.node_id` des nœuds distants.
- Les métadonnées du validateur `p2p_address` et `rpc_address` doivent contenir des adresses publiques annoncées, et non des noms de service Docker uniquement, à moins que le réseau ne soit intentionnellement privé.

## 3. Soumettre l'admission du validateur

Par exemple, des flux de staking, créez une transaction de staking :
```bash
vexod staking --help
```
La transaction d’admission du validateur doit inclure :

- identifiant du validateur
- adresse du validateur
- clé publique consensuelle
- pouvoir de vote ou référence d'enjeu
- points de base de commission du validateur, si la chaîne permet des mises à jour de commission en libre-service
- Métadonnées P2P `node_id` si la chaîne utilise des métadonnées de genèse/validateur pour pré-amorcer les cartes homologues
- métadonnées d'adresse publique P2P
- métadonnées d'adresse RPC publique, si publiques
- Métadonnées de preuve de possession BLS lorsque BLS est activé

La mise à jour du validateur doit devenir effective à une hauteur spécifique et produire un nouveau hachage d'ensemble de validateur.

Une fois le validateur actif, les opérateurs peuvent exposer l'état de la récompense via le module de jalonnement :
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. Vérifier la mise à jour de l'ensemble de validateurs

Après la mise à jour de la hauteur :
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
Vérifiez :

- le validateur apparaît dans l'ensemble spécifique à la hauteur
- le pouvoir de vote est correct
- Le hachage du validateur a changé comme prévu
- les preuves de finalité font référence à la hauteur correcte définie par le validateur

## 5. Rotation des clés du validateur de plan

Les clés du validateur peuvent être alternées en préparant un document de clé suivant avec des métadonnées `active_from` et `active_until` qui ne se chevauchent pas, puis en démarrant le nœud avec la clé de rotation supplémentaire :
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
Au moment de la signature, le nœud utilise la clé dont la fenêtre active contient la hauteur de consensus. Les documents de clé de signataire distant conservent les mêmes exigences en matière de politique, de jeton d'authentification et de protection contre la double signature.

## 6. Démarrer le validateur
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
Le démarrage n'a pas de commutateur de mode réseau. Utilisez `config audit --strict` avant le démarrage lorsque le réseau est censé satisfaire aux hypothèses de sécurité du réseau public.

## 7. Surveiller

Regardez :

- latence proposition/vote
- délais d'attente des tours
- échecs de signature du validateur
- interdictions par les pairs
- taille de la mémoire
- latence de validation
- santé de l'instantané/relecture

Utilisation :
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## Notes de sécurité

- Ne réutilisez jamais les clés du validateur sur des chaînes indépendantes.
- Gardez la politique de signataire distant activée pour les validateurs de production.
- N'admettez pas un validateur BLS sans preuve de possession ou défense équivalente contre les clés malveillantes.
- Ne sabrez pas ou n'emprisonnez pas un validateur sans preuves vérifiées liées au bon ensemble de validateurs de hauteur de preuve.

<!-- vexo-docs:technical-parity -->
## Annexe de parité technique

Cette annexe garantit que la traduction conserve les interfaces exécutables et les sections clés du document canonique anglais. Les commandes, clés de configuration, méthodes RPC et noms de paquets restent inchangés dans toutes les langues.

### Suivi des sections
- section: 1. Initialize Validator Home — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: 2. Configure Network Addresses and Peers — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: 3. Submit Validator Admission — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: 4. Verify Validator Set Update — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: 5. Plan Validator Key Rotation — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: 6. Start Validator — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: 7. Monitor — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.
- section: Safety Notes — Cette section doit être lue avec les valeurs de configuration, les preuves de vérification, les conditions d’échec et les actions opérateur.

### Interfaces conservées telles quelles
- `VEXO_KEY_PASSPHRASE` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `--passphrase` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `bls_pop` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `blst-bls12381-minpk-v1` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `node.key.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `network_config.json:p2p.node_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `.vexo-validator-new/network_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `network_config.json` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.listen_address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc.address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.node_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.node_key_path` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p.peers` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `p2p_address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `rpc_address` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `node_id` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `active_from` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `active_until` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
- `config audit --strict` — Ce nom est utilisé tel quel dans les exemples exécutables et la validation de configuration; il ne doit pas être traduit.
