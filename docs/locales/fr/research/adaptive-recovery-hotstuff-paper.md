# HotStuff adaptatif avec garde de reprise pour les réseaux Proof-of-Stake modulaires

> Locale: fr · Français  
> Type de document : manuscrit de recherche et protocole de reproductibilité  
> État : brouillon fondé sur l'implémentation ; toute affirmation de performance exige des mesures archivées.

## Résumé

Ce manuscrit étudie une réplication de machine à états BFT de style HotStuff pour des réseaux Proof-of-Stake modulaires. L'implémentation associe la finalité à trois chaînes et des ensembles de validateurs versionnés par hauteur à trois mécanismes opérationnels. Un contrôleur borné adapte le délai de round à partir des latences p95 de proposition, de vote et de commit ainsi que de la santé des pairs actifs. Une garde de finalité liée à la reprise retarde un commit applicatif finalisé lorsque l'historique durable des blocs et celui de l'état applicatif divergent au-delà d'une hauteur commune sûre. Enfin, un ordonnancement déterministe retire l'ordre d'arrivée local du mempool pour un même ensemble de transactions tout en conservant les dépendances de nonce de chaque signataire.

La contribution ne consiste pas à déclarer nouveaux PoS, BFT, HotStuff, la synchronisation adaptative des vues ou l'équité d'ordre. La question est plus précise : cette composition bornée de contrôle, de reprise et d'ordonnancement réduit-elle les timeouts évitables et les incohérences de reprise sans modifier la règle de sécurité HotStuff sous-jacente ? Le texte sépare les faits implémentés, les hypothèses réfutables et les conclusions qui nécessitent encore des expériences. Aucun gain de débit ou de latence ne doit être annoncé avant des répétitions utilisant un binaire, une configuration, une topologie et un jeu de données identifiés.

## Questions de recherche

RQ1 compare le contrôleur adaptatif au même système avec timeout fixe lorsque le délai réseau varie, en mesurant le nombre de timeouts et la latence p95 de commit. RQ2 injecte des défauts de stockage et de redémarrage afin de vérifier que la garde de reprise interdit à l'état applicatif de dépasser la hauteur durable commune. RQ3 permute un ensemble identique de transactions et vérifie à la fois l'identité de l'ordre proposé et la monotonie des nonces par signataire. RQ4 mesure les coûts CPU, mémoire, réseau et latence dans un réseau stable sans faute.

H1 à H4 sont des hypothèses directionnelles et falsifiables, non des résultats. La présence du code ne démontre pas son avantage. Une absence d'amélioration significative constitue un résultat négatif ou une limite de domaine qui doit être publiée telle quelle.

## État de l'art et limite de nouveauté

HotStuff a déjà présenté une BFT menée par un leader sous synchronie partielle, des certificats de quorum, une règle de commit chaînée, une communication linéaire sur le chemin favorable et la réactivité. LibraBFT/DiemBFT et AptosBFT ont déjà combiné des descendants de HotStuff avec une gouvernance de validateurs pondérée par le stake. Jolteon et Ditto étudient la réduction de latence, l'adaptation réseau et un repli asynchrone ; Fever traite la synchronisation réactive des vues. Tendermint appartient à une autre lignée BFT PoS par rounds. Narwhal/Tusk sépare la diffusion fiable des transactions de leur ordre. Aequitas, Wendy et Themis définissent des propriétés d'équité plus fortes que l'ordre déterministe par hash utilisé ici.

Il est donc incorrect de parler de « première blockchain PoS+BFT », de « premier réseau PoS utilisant HotStuff », d'un protocole identique à AptosBFT, d'une vivacité asynchrone ou d'une complexité optimale sans preuve, d'une suppression complète du MEV, ou d'une préparation production démontrée par un test Docker sur une seule machine. La contribution système candidate est plus étroite : intégrer dans un nœud PoS modulaire en Go un contrôleur borné, une garde locale de l'historique durable et un ordre déterministe sensible aux nonces, puis les comparer de manière reproductible à des variantes fixes et sans garde.

## Modèle système et mécanismes

À la hauteur h, Vh désigne l'ensemble actif des validateurs et Ph sa puissance de vote totale. Un QC n'est valide que si des signataires connus et uniques apportent au moins deux tiers de Ph. L'ensemble et son hash sont versionnés par hauteur. L'admission peut être libre sous condition de stake minimal, plafonnée ou restreinte par configuration. Cette couche traite la résistance Sybil et la gouvernance ; elle ne change pas le seuil de faute BFT.

Le réseau est supposé partiellement synchrone. La sécurité exige moins d'un tiers de puissance byzantine ainsi que des signatures, une liaison au bon ensemble de validateurs et un stockage durable corrects. La vivacité exige en plus qu'un délai borné finisse par s'établir, qu'un quorum honnête soit joignable, que les signataires soient disponibles et que la connectivité des pairs soit suffisante. Aucun progrès n'est promis dans un réseau définitivement asynchrone.

L'EVM est une charge applicative sous le consensus Vexo. L'exécution de bytecode Ethereum et la compatibilité des outils `/web3` ne signifient ni fork choice Ethereum ni consensus devp2p Ethereum.

La règle de sécurité de base suit `locked_qc` et `high_qc`. Une proposition n'est sûre que si elle étend le verrou ou porte un justify QC au moins aussi récent. Un validateur ne peut pas voter pour deux blocs différents à la même hauteur et au même round. Trois liens certifiés consécutifs et liés par hauteur et hash finalisent le grand-parent. Le contrôleur adaptatif ne modifie ni ce prédicat, ni le seuil de quorum, ni la validation des QC, ni la règle à trois chaînes.

Le timeout adaptatif utilise le budget de base T0, le budget courant Tt, la somme des latences p95 de proposal/vote/commit et un plancher lié au déficit de pairs. Après timeout, la valeur croît vers 1,5×Tt ; après progrès, elle décroît vers 0,8×Tt. Trois fois la latence observée forme un plancher candidat. Le résultat est borné entre T0 et 8×T0. Sans pair actif, le plancher vaut 2×T0. Une période idle sans travail et une erreur locale d'exécution ou de stockage ne consomment pas de round. Il s'agit d'un contrôleur opérationnel borné, pas d'un pacemaker théoriquement optimal.

La garde de reprise calcule Hsafe=min(Hs,Hb) lorsque la hauteur durable d'état Hs et la hauteur de l'index de blocs Hb existent. Tant qu'elles diffèrent, tout commit applicatif finalisé au-dessus de Hsafe est différé. Cette règle est une restriction locale de persistance, et non une phase de vote supplémentaire ou un certificat réseau.

L'ordre déterministe produit un sel à partir du chain ID et de la hauteur. Les transactions dotées d'un signataire et d'un nonce sont regroupées par chaîne de signataire et triées par nonce croissant. Les têtes de chaînes sont fusionnées par hash salé. Le résultat est indépendant de l'arrivée locale pour un ensemble candidat identique, mais ne garantit ni équité first-seen, ni résistance à la censure, ni confidentialité, ni forte order-fairness. Le proposer peut encore influencer l'inclusion dans l'ensemble candidat.

Le chemin de vote actif utilise actuellement l'ensemble complet versionné par hauteur et une sélection déterministe du proposer. Le sélecteur de comité ECVRF existe comme composant et comme requête, mais n'est pas relié à la formation du quorum ni à l'éligibilité des propositions. Un consensus par comité VRF reste donc un travail futur.

## Plan expérimental

Les traitements emploient le même binaire et la même configuration applicative. Ils comparent une base fixe avec adaptation désactivée et garde activée, la politique adaptative avec garde activée, et une ablation avec garde désactivée uniquement dans un réseau de recherche jetable. Selon les ressources, les tailles visées sont 4, 7, 16 et 31 validateurs ; une seule machine ne sert qu'au smoke test.

Les conditions incluent 10, 50, 100 et 250 ms de latence, des changements en paliers, du jitter, 0/1/5/10 % de pertes, le redémarrage d'un validateur puis du proposer courant, une indisponibilité juste sous un tiers de la puissance, une partition minoritaire suivie d'une guérison, un retard de signer et une divergence injectée des historiques durables. Les charges comprennent transferts natifs, transferts EVM, création de contrat, event logs, déploiement de proxy et upgrade UUPS.

Les mesures couvrent hauteurs committed/finalized, latences p50/p95/p99 de proposal/vote/commit, latence de finalité de bout en bout, nombre de timeouts, distribution des rounds, timeout adaptatif courant, pairs, reports de reprise, débit, gas, CPU, RSS, écritures disque, octets réseau, rejets, double-sign et invalid nonce. Un run de performance n'est valide que si tous les validateurs ont le même app hash et le même hash finalisé à la hauteur comparée, si transactions, receipts et positions de bloc concordent, si le code du contrat existe et si l'état du proxy survit à l'upgrade UUPS.

Après warm-up, chaque condition doit avoir au moins trente répétitions indépendantes, sauf justification préalable par analyse de puissance. L'ordre des traitements et les seeds sont enregistrés. Le rapport contient médiane, intervalle interquartile, p95, intervalles de confiance et taille d'effet. Il est interdit de ne conserver que le meilleur run ; les règles d'exclusion sont fixées avant observation des résultats.

## Correction, reproductibilité et éthique

La politique adaptative change le moment d'une tentative de timeout vote, jamais la définition d'un vote sûr ou d'un QC valide. La garde ne peut que restreindre les commits et ne peut pas autoriser un commit refusé par la règle de base. L'ordre déterministe aide l'exécution identique, mais ne remplace pas la preuve d'absence de finalités contradictoires.

Une preuve publiable doit formaliser l'intersection des quorums pondérés, la monotonie du lock, l'unicité d'un bloc finalisé par hauteur, les transitions d'ensembles, la reprise du vote WAL et la neutralité de sécurité du contrôleur et de la garde. Les tests et simulations adversariales sont des éléments de preuve empirique, non un remplacement d'une preuve formelle ou d'un audit indépendant.

Chaque expérience archive commit, état dirty, versions Go/OS/CPU/mémoire/conteneur, topologie, genesis, configurations séparées, SHA-256 du binaire, seed, données JSON/JSONL/CSV brutes, logs, app hashes finaux, scripts d'analyse et registre des échecs. Un mécanisme connu ne doit pas être renommé puis présenté comme une invention. Débit, latence et nombre de validateurs ne sont jamais fabriqués ; hypothèse, observation et interprétation restent séparées.

L'aide de l'IA est déclarée selon la politique du lieu de publication, les auteurs restant responsables de chaque affirmation, citation, expérience et preuve. L'injection de fautes se déroule uniquement sur des systèmes isolés possédés ou autorisés. Clés privées, tokens opérateur, données des participants et endpoints de production sont exclus des artefacts. Les vulnérabilités découvertes suivent une divulgation coordonnée.

Avant soumission, le manuscrit doit correspondre à une révision source épinglée, la recherche d'antériorité doit être archivée, les bases doivent être reproductibles, les mesures multi-hôtes achevées et chaque tableau ou figure régénérable à partir des données brutes. Les résultats négatifs, limites, formulations de preuve appropriées et revue méthodologique externe restent dans la version soumise. Jusque-là, le terme exact est « brouillon de recherche fondé sur l'implémentation », pas « nouveau consensus prouvé ».

<!-- vexo-docs:technical-parity -->

## Annexe de parité technique

Les noms suivants restent inchangés :

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
