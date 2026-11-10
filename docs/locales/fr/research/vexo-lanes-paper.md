# Vexo Lanes : diffusion de lots certifiés pour un HotStuff à deux chaînes pondéré par la mise

> Locale: fr · Français  
> Type de document : manuscrit de recherche et protocole de reproductibilité  
> Statut : brouillon ancré dans l'implémentation ; les résultats proviennent d'une seule machine aux liens émulés et doivent être répétés sur un vrai réseau avant publication

## Résumé

Un protocole BFT à leader de la famille HotStuff est borné par la liaison montante de son leader : chaque transaction d'un bloc quitte une machine une fois par validateur, si bien que le débit chute comme `1/(n−1)` pendant que les liens des autres validateurs restent inactifs. Vexo Lanes retire la diffusion au leader sans remplacer le protocole d'ordonnancement. Chaque validateur scelle les transactions qui lui appartiennent en lots signés sur sa propre voie ; les validateurs détenant deux tiers de la mise accusent réception du stockage, ce qui certifie la disponibilité du lot ; le leader propose ensuite un bloc en nommant des lots certifiés. Le format du bloc, son hachage, sa racine de transactions, les octets signés, la règle de vote et la règle de validation à deux chaînes sont inchangés.

Sur une machine aux liens émulés de 100 Mbit/s et 25 ms par validateur, Lanes valide 60 080, 45 323 et 37 099 tx/s avec 4, 7 et 10 validateurs, contre 21 073, 10 909 et 8 297 tx/s lorsque le même nœud diffuse des blocs entiers : 2,9×, 4,2× et 4,5×, un écart qui croît avec `n`. Sous la capacité (4 000 tx/s), Lanes ajoute 35 ms de latence médiane avec 4 validateurs et en retire 25 ms et 130 ms avec 7 et 10. Sans limite de bande passante, Lanes perd 4 % de débit. L'évaluation a révélé neuf défauts, dont un ordonnancement des leaders sous lequel seuls les validateurs d'indice pair proposaient avec un nombre pair de validateurs.

## 1. Question de recherche

Peut-on ajouter une diffusion de lots certifiés sous un protocole HotStuff pondéré existant, sans changer l'objet des votes ni la façon dont les blocs sont validés, afin que le débit suive la somme des liaisons montantes des validateurs plutôt que celle du leader ? Les sous-questions portent sur le goulot du leader (RQ1), le passage à l'échelle (RQ2), la préservation de la sûreté, de l'ordre des nonces et de la résistance à la censure (RQ3) et le coût lorsque la bande passante abonde (RQ4). Hypothèses : H1, l'envoi de la référence par transaction croît linéairement avec `n` alors que celui de Lanes reste proche de la taille d'une transaction ; H2, l'accélération croît avec `n` ; H3, sans contrainte de lien Lanes n'est pas plus rapide et coûte au plus un aller-retour.

## 2. Travaux antérieurs et limite de nouveauté

La séparation de la diffusion et de l'ordonnancement est établie : Narwhal et Tusk, Bullshark et Shoal, Stratus, Mir-BFT et ISS, Autobahn et le Quorum Store d'Aptos. Ce manuscrit ne revendique comme nouveaux ni cette séparation, ni les certificats de disponibilité, ni les voies par validateur, ni des résultats sur un vrai réseau étendu. La contribution relève de l'ingénierie : une adaptation qui conserve bit à bit le bloc, l'en-tête, la signature et la règle de validation ; la propriété des voies par expéditeur avec un repli contre la censure ; l'authentification de la proposition avant toute récupération ; et un banc de mesure reproductible avec les défauts qu'il a révélés.

## 3. Modèle du système

`n` validateurs au pouvoir de vote pondéré ; les validateurs byzantins détiennent moins d'un tiers. Les quorums représentent `>= 2/3` du pouvoir. Synchronie partielle, canaux authentifiés et signatures séparées par domaine. Les validateurs peuvent tomber en panne, équivoquer, retenir des lots, inonder leur voie ou envoyer des propositions falsifiées.

## 4. Protocole

**Voies et certificats.** Chaque validateur scelle ses transactions en un `lanes.Batch` à 512 KiB ou 5 ms après la première, borné par `lanes.MaxBatchTxs` et `lanes.MaxBatchBytes`. Une voie peu chargée, dont le lot précédent faisait moins de 16 KiB, attend aussi le certificat de ce lot, au plus 50 ms. Les validateurs accusent réception dans le domaine `DomainLaneBatchAck` ; les accusés de `>= 2/3` du pouvoir forment un `BatchCert`, de sorte qu'au moins un validateur honnête détient le lot. Les retransmissions utilisent un recul exponentiel.

**Propriété et repli.** Une transaction appartient à la voie `lanes.Owner(members, sender)`. Toutes les transactions d'un expéditeur passent par une seule voie dans l'ordre d'admission, donc les voies ne réordonnent jamais une suite de nonces. Si aucun lot stocké ne porte une transaction étrangère après `lanes.FallbackDelay` (3 s), son détenteur la met lui-même en lot ; un propriétaire fautif peut retarder mais pas censurer.

**Propositions par voies.** Le leader ne prend que des lots certifiés, par voie dans l'ordre jusqu'au premier lot non certifié, en alternant les voies jusqu'à la limite du bloc, signe le bloc comme avant et diffuse un `lane_proposal` avec les identifiants de lots, les positions refusées (`Excluded`) et les transactions hors lot (`Extra`).

**Authentification avant récupération.** Le destinataire vérifie le leader et la signature avant de récupérer quoi que ce soit. Si l'ensemble des validateurs dépend d'un bloc pas encore exécuté, la proposition est conservée (au plus 16) et jugée à nouveau après l'exécution suivante. Le bloc est reconstruit à partir des lots stockés et n'est jugé que si son hachage et sa racine égalent l'en-tête signé ; sinon le validateur demande le `proposal` entier par `proposal_request`.

**Ordonnancement proportionnel à la mise.** Le leader de la vue `v` est `validator.SelectProposer(validators, v)` : les validateurs possèdent `VotingPower` emplacements consécutifs de `[0, W)`, la vue `v` tombe sur l'emplacement `v·S mod W`, avec un pas premier avec `W` à partir de `ceil(W·(φ−1))`. Toute fenêtre de `W` vues donne à chaque validateur exactement sa part. L'ordonnancement ne dépend que de la vue.

**Bornes de ressources.** 64 MiB par auteur dans un stockage de 256 MiB ; expiration après 30 s ; au plus deux envois par lot et par pair ; au plus `consensus.MaxLaneRefs` lots par proposition ; un encodage binaire versionné.

## 5. Argument de correction

Sûreté : les règles de vote et de validation sont inchangées, et un validateur ne vote que pour un bloc dont le hachage reconstruit égale l'en-tête signé ; les voies peuvent retarder un vote mais non en changer l'objet. Disponibilité : chaque lot nommé est détenu par un validateur honnête, et la proposition entière est récupérée en dernier recours. Vivacité : après stabilisation, les lots honnêtes sont certifiés puis proposés ; un validateur en retard juge les propositions juste après l'exécution. C'est un argument, pas une preuve vérifiée par machine.

## 6. Méthode expérimentale

Traitements : Lanes (nœud par défaut) contre la référence avec `DisseminateByProposal`, où le leader diffuse des blocs entiers ; tout le reste est partagé. Tous les validateurs tournent dans un processus sur un Apple M4 ; chaque envoi passe par un lien virtuel à débit et latence fixes, les messages de consensus avant le trafic de masse, sans perte ni gigue. Les clients soumettent des transactions de 89 octets ; les essais de débit soumettent 150 000 transactions d'un coup, les essais de latence 20 000 en boucle ouverte à 4 000 tx/s (`VEXO_RATE`). Les cellules de débit à 10 validateurs utilisent un délai de 5 s pour les deux traitements, car la référence se bloque après la hauteur 6 avec la valeur par défaut. Chaque cellule a été mesurée trois fois.

## 7. Résultats

| Validateurs | Lanes tx/s | Référence tx/s | Facteur | Lanes B/tx | Référence B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

Tous les essais ont validé les 150 000 transactions. La référence paie à elle seule `s·(n−1)` octets par transaction pour les propositions, Lanes 0,2–0,4 B/tx. Cela soutient H1 et H2.

| Validateurs | Lanes p50 / p99 | Référence p50 / p99 | Proposition Lanes B/tx | Proposition référence B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

Avec 4 validateurs, la certification coûte environ un aller-retour ; dès 7 validateurs, la liaison du leader de la référence retarde déjà les propositions à cette charge, et à 10 validateurs seul Lanes a suivi la charge offerte. H3 ne tient que pour de petits ensembles.

| Traitement | tx/s | Envoi B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| Référence | 114,522 (113,095–116,408) | 147.4–148.3 |

Sans contrainte, la référence est 4 % plus rapide ; Lanes envoie pourtant moins de la moitié des octets. La référence est bornée par le lien du leader, Lanes dans cette matrice par la machine partagée.

## 8. Défauts révélés par l'évaluation

1. **Ordonnancement par parité.** `(height−1+round) mod n` ne faisait proposer que les indices pairs avec `n` pair ; remplacé par l'ordonnancement proportionnel (`node/leader_schedule_test.go`).
2. **Rejeu de transactions validées.** Des commérages tardifs étaient réadmis (152 629 validations pour 150 000 transactions).
3. **Expiration quadratique du cache des transactions vues.** Chaque admission parcourait tout le cache ; désormais la plus ancienne d'abord, et bornée.
4. **Tempête de retransmissions.** 200 ms fixes sur un lien saturé ; désormais recul exponentiel.
5. **Preuves dupliquées sans borne.** Désormais une fois par infraction dans une fenêtre de 1 024.
6. **Requêtes de corps de validation.** Désormais une requête, répétée après une seconde.
7. **Propositions jugées seulement à la retransmission.** Un essai à 10 validateurs sur quatre s'effondrait à 2 280 tx/s ; désormais conservées et jugées après l'exécution (`consensus/transport_reactor_test.go`).
8. **Coût par lot à faible charge.** Des lots de deux transactions saturaient la machine ; corrigé par le cadencement des voies.
9. **Métrique de latence mal nommée.** Désormais latence de la soumission à la validation par transaction.

## 9. Menaces à la validité

- Réseau émulé sur une machine, sans perte ni gigue.
- Un CPU partagé borne Lanes pour un grand `n`.
- La référence exige un délai de 5 s à 10 validateurs ; un encodage plus compact réduirait l'écart.
- L'attente de 300 ms avant de récupérer une proposition entière est trop courte sur des liens saturés.
- Petits transferts uniquement ; trois répétitions seulement (34,9k–39,4k tx/s à 10 validateurs).
- Les certificats Ed25519 concatènent les signatures ; BLS les réduirait.

## 10. Reproduction

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` modifie le délai ; sans `VEXO_UPLINK_MBIT` ni `VEXO_LATENCY_MS`, le lien n'est pas contraint. Tout chiffre publié doit indiquer la révision, la machine, la version de Go et les lignes de journal complètes.

<!-- vexo-docs:technical-parity -->

## Annexe de parité technique

Ces noms d'implémentation et d'expérience restent non traduits :

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`, `VerifyBatch`, `VerifyCert`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`, `consensus/transport_reactor_test.go`
- `validator/proposer.go`, `node/lanes.go`, `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `DomainLaneBatchAck`, `BatchCert`, `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `Excluded`, `Extra`
- `lane_proposal`, `proposal_request`, `proposal`, `DisseminateByProposal`
- `validator.SelectProposer(validators, v)`, `VotingPower`, `[0, W)`, `v·S mod W`, `ceil(W·(φ−1))`, `0..W−1`, `W`, `S`, `v`
- `n`, `s`, `>= 2/3`, `1/(n−1)`, `s·(n−1)`, `(height−1+round) mod n`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
- `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
