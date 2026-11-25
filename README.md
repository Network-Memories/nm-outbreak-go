# nm-outbreak-go

Prototype de service unifié **Network Memories** en Go pour **Resident Evil Outbreak File #1** et **File #2** (`dorsal → modules de jeu`), fusionnant dans un seul binaire natif :

- Le dorsal **SNAP** (UDP `9090` bootstrap, `9091` File #1, `9092` File #2)
- Le dorsal **KDDI / BioServer** (TCP `8300`/`8690` File #1, `8200`/`8590` File #2)
- Le registre unifié en mémoire (`internal/world`) : salles, rosters, primitives de chat inter-régions (`area:` / `world:`) et relais direct de jeu sans pont HTTP/JSON
- La liaison **STUN NM immuable** (`internal/stun`) avec sonde RFC 5389 au démarrage, vérification périodique et état dégradé ; transport de jeu et relais de secours restant à valider
- L'API HTTP de présence et de santé (`internal/httpapi`) pour le site NM, NM Station et le bot Discord

## État de la migration

Le prototype n'est pas un remplacement validé des services de production.
Deux clients originaux File1 US ont validé en laboratoire connexion, création,
entrée à deux, lancement, jeu et retour après abandon en salle d'après-partie2P.
Deux originaux File2 PAL (hôte) et JAP (invité) ont validé Wild Things, abandon,
résultats, sauvegardes et retour2P dans les deux ordres, avec les portraits,
les identités et les départs (2P→1P). Les journaux natifs sont échangés sans
modifier leurs2048octets. Les messages en alphabet latin passent dans les deux
sens en salle d'attente et en après-partie, sans double écho, sur le même binaire
Go. Aucun client n'est patché. Une comparaison originale PAL + US valide aussi
les deux salles2P, les fiches du partenaire et le départ.

Les rapports sont rattachés à une manche canonique et à leurs propriétaires
prouvés, sans doubler les résultats. Les rapports japonais encore opaques sont
archivés ; aucun score ni classement japonais n'en est inventé. Les autres
paires, le chat des zones et de File1, les caractères hors du latin mesuré,
les parties à3–4 et une réussite complète de scénario restent à qualifier.
L'identité NM complète, les amis, la messagerie personnelle et les raccordements
restent à terminer. La sonde RFC et les sockets UDP du service natif commun sont
observés séparément ; NAT distincts et relais de secours restent non qualifiés.

Un seul `internal/core.Engine` possède les sessions authentifiées, les profils,
les entrées/sorties de salon et le relais. SNAP et KDDI reçoivent la même instance :
leurs fichiers sont des adaptateurs natifs, pas deux dorsaux reliés par HTTP.
Un simple pseudo reçu dans un paquet ne crée plus de session. Le login commun
utilise une autorité explicite ; la preuve SNAP et le ticket web KDDI sont portés
pour le mode autonome. Le contrat NM reste à terminer.

Le registre commun fournit des instantanés isolés, ferme les abonnements à la sortie,
isole File 1 de File 2 et protège la diffusion des trames contre les déconnexions
concurrentes. Voir [MIGRATION.md](MIGRATION.md) pour la reprise.

## Identité autonome ou NM

`NM_IDENTITY_MODE=standalone` est le mode par défaut : les comptes sont locaux,
dans `NM_ACCOUNT_FILE` (défaut `state/accounts.json`). Aucune URL, clé ou connexion
au service d'identité NM n'est nécessaire. SNAP et KDDI reçoivent la même autorité.
La création locale attribue automatiquement un alias occidental et un HN aléatoire
à six caractères, conservés après redémarrage. Le nom principal n'est pas tronqué.
Les noms et alias sont réservés sans distinction de casse ; les nouveaux noms ont
5 à 32 lettres ASCII, chiffres ou underscores, les nouveaux mots de passe de
5 à 10 caractères inclus. Les secrets historiques restent acceptés à la connexion,
sans troncature. Les formats natifs restent à vérifier selon le client.

`NM_IDENTITY_MODE=nm` sélectionne uniquement l'autorité NM, avec `NM_IDENTITY_URL`
et `NM_IDENTITY_KEY_FILE`. Une configuration manquante provoque une erreur : aucune
bascule silencieuse vers des comptes locaux et aucune fusion de propriétaires.
Le client Go utilise les opérations internes KDDI et SN@P de l’identité NM.
L’hébergeur NM doit déployer les opérations correspondantes et fournir la clé
du client autorisé aux deux jeux.

L'autorité autonome conserve des mots de passe dérivés avec PBKDF2-HMAC-SHA256,
sel aléatoire individuel et fichier privé, remplacé atomiquement. Une instance
serveur possède ce fichier ; ne pas le partager entre plusieurs processus.
L'outil suivant sert aux essais et à la préparation hors ligne, serveur arrêté :

```bash
NM_IDENTITY_MODE=standalone NM_ACCOUNT_FILE=state/accounts.json \
  ./bin/nm-outbreak account-create PlayerOne < /chemin/vers/fichier-mot-de-passe
```

Le mot de passe est lu sur l'entrée standard, jamais dans les arguments ni la sortie.
L'attribution alias/HN est implémentée. Le navigateur KDDI utilise cette autorité
commune pour la création et la preuve du mot de passe. La création native SNAP,
les amis et la messagerie restent à raccorder. Le parcours original File1 US à deux
est validé jusqu'au retour après abandon ; une réussite de scénario et les autres
variantes restent à qualifier. Le transport STUN et son relais restent nécessaires ;
l'hébergeur peut configurer son adresse avec `NM_STUN_ADDR`.

Pour SN@P, `OPENSNAP_BOOTSTRAP_PORT` règle l’écoute UDP (9090 par défaut).
Lorsqu’une redirection de ports donne au client une entrée différente, régler
`OPENSNAP_ADVERTISE_BOOTSTRAP_PORT` sur le port accessible au client. Sans
cette option, le port annoncé reprend celui de l’écoute. Le défi et la preuve
doivent parvenir au serveur depuis le même endpoint UDP.

Pour les parcours KDDI avec les disques originaux, fournir les catalogues natifs
avec `NM_OUTBREAK_FILE1_JP_CATALOGUE` et `NM_OUTBREAK_FILE2_JP_CATALOGUE`.
Ces fichiers ne sont pas inclus dans l’image Docker. Monter des copies en
lecture seule et accessibles à l’UID 10001 du conteneur.


## Construction & tests

```bash
go test -race ./...
go vet ./...
go build -o bin/nm-outbreak ./cmd/nm-outbreak
```

## Validation du 21 novembre 2026

Le passage sur des captures natives a révélé un mauvais format SNAP dans le
prototype : CRC fictif, longueur absente et messages groupés non traités.
Le codec corrigé correspond à 7 731 datagrammes capturés. Le parseur KDDI correspond
à 958 trames lobby et 942 trames moteur capturées ; ces tests n'exécutent pas une partie.
Les propriétés hôte correspondent à 2 400 combinaisons de règles de référence.
Les corpus privés ne sont pas distribués ; leurs tests sont explicitement ignorés
sans les variables `NM_SNAP_CAPTURE_FIXTURE`, `NM_KDDI_CAPTURE_FIXTURE` et
`NM_PROFILE_REFERENCE_FIXTURE`.

Le test runtime se lance avec `python3 tools/runtime_check.py` et ne contacte pas
NM : un seul binaire Go et un répondant STUN simulé, sur boucle locale et ports
jetables. Il vérifie démarrage, panne/reprise et arrêt. Il expose aussi les
limites qui empêchent encore une partie complète et une bascule.
`/health` indique donc `degraded`, `ready=false` et relais non implémenté.
Ce test reconstruit ne qualifie pas les clients originaux : leurs essais distincts
sont décrits ci-dessus. Les NAT distincts restent à valider.

## Bootstrap SNAP autonome — 21 novembre 2026

Le défi natif chiffré est porté vers le même cœur Go : identification du titre,
preuve du mot de passe, transmission vers la socket de jeu, profil et création
de salle. Un client Python indépendant valide File 1 US et File 2 US/PAL avec
`python3 tools/bootstrap_check.py` (dépendance de test : Python cryptography).
Ce sont des échanges reconstitués, pas une validation avec une PS2 ou un émulateur.
Fiabilité SNAP, KDDI LOGIN/GSLOGIN et Capcom APP sont maintenant portés en partie ;
leur parcours original reste à qualifier hors File1 US à deux jusqu'au retour
après abandon. Le fournisseur NM n'est pas raccordé à ce défi ; aucun compte
local de secours n'est créé en mode NM. L'état reste `ready=false`.

Configurer `NM_OUTBREAK_ADVERTISE_HOST` avec l'IPv4 joignable par les clients.
Une adresse d'écoute indéterminée ne constitue pas une destination de connexion.
La reprise sur une autre socket exige l'identifiant de session authentifiée ;
un nom et une adresse IP seuls ne prouvent pas le propriétaire.

Les nouveaux comptes locaux possèdent le matériel nécessaire au défi, chiffré
au repos par AES-GCM. Sauvegarder ensemble le fichier de comptes et son fichier
`.native-key`, privés : perdre cette clé rend ce matériel inutilisable. Les
comptes du format historique restent conservés ; préparer leur défi, serveur
arrêté, avec preuve du mot de passe actuel :

```bash
NM_IDENTITY_MODE=standalone NM_ACCOUNT_FILE=state/accounts.json \
  ./bin/nm-outbreak account-enable-snap PlayerOne < /chemin/vers/fichier-mot-de-passe
```

Cette opération conserve propriétaire, alias, HN et dérivation du mot de passe.
Le format du magasin devient version 2. Aucun secret n'est tronqué pour un client ;
les limites de saisie natives doivent encore être vérifiées dans le parcours complet.

## État des transports et des résultats

Le processus sert maintenant Capcom APP et le démarrage KDDI reconstitué, en plus
du bootstrap SNAP. `python3 tools/bootstrap_check.py` contrôle les trois variantes
occidentales, APP et les deux connexions japonaises reconstituées, en boucle locale.
Le maintien fiable et la pagination des listes sont également portés. Le jeu
original File1 US entre dans une zone, crée une salle et joue à deux en laboratoire.
La sortie du hall répond sur son canal natif, distinct de celui des salles ; ce
traitement est également testé pour File2 PAL/US. Le retour après abandon affiche
2P sur les deux clients, leurs personnages et leurs journaux respectifs. Le départ
de l'invité ramène le compteur de l'hôte à 1P.

Les rapports SNAP et KDDI sont conservés dans un même journal privé. Le registre
canonique compte une seule fois une mesure par manche et propriétaire prouvés,
même lors des reprises d'envoi. File1 et File2 gardent leurs propres résultats.
Le résumé japonais reste opaque : son numéro de session et ses pairs sont
conservés, sans fabriquer de points ou de temps. Le retour d'un client ne doit
pas séparer la manche commune des autres participants.

Deux rapports APP 6301 des clients originaux File1 US sont attribués aux deux
propriétaires d'une même manche. Deux répétitions d'un rapport réel ajoutent des
archives, sans ajouter de résultat canonique. Sur cet abandon, le temps de réussite
natif vaut zéro ; ce champ ne fournit pas le temps passé affiché par le jeu.

Le raccordement UDP au service natif partagé se configure par `NM_STUN_CONTROLE`.
Il est obligatoire en mode NM et ne bascule pas vers un socket direct sur erreur.
Le fichier de contrôle doit être accessible au processus et partager le même
répertoire de canaux natifs. `/health` distingue cette connexion de la sonde RFC,
avec les échanges réellement observés. Le relais de secours et les parcours sous
NAT distincts restent à qualifier ; l'état global reste non prêt.

Le navigateur japonais écoute sur `NM_OUTBREAK_WEB_ADDR` (boucle locale par
défaut), pour être placé derrière l'entrée native de l'hébergeur. Le contrat
complet d'identité NM et la reprise des données doivent être terminés avant une
bascule. Les détails de progression et les limitations sont dans MIGRATION.md.

La navigation KDDI zones/salons/emplacements est portée pour les deux modules.
Les coordonnées de partie sont stables dans chaque annuaire et isolées entre
jeux et zones ; fermer une partie ne renumérote pas les autres. Les zones/salons
désactivés sont refusés. Fermer le transport du lobby expire sa présence de
navigation tout en conservant la place logique en jeu et le contexte de résultat.
Ces parcours japonais sont testés avec des échanges reconstitués ; le client
File2 japonais original atteint aussi une salle d'attente PAL reconstituée2/2
et en sort proprement. La création originale6509FFFF/FFFF reste à comprendre,
sans choisir un scénario à la place du joueur. L'admission
KDDI vers une salle occidentale utilise les contrôles communs de capacité,
mot de passe et fermeture, avec consultation des personnages et places du même
roster. La création KDDI réserve un emplacement sans exposer de salle incomplète,
reçoit les choix natifs puis publie une seule salle après validation. Les règles
restent définies par module et les effets fixes File2 suivent la configuration
commune APP. Le client de test indépendant vérifie également l'entrée occidentale
dans ces salles japonaises. Les arrivées et départs sont publiés par le cœur commun
puis traduits en notifications natives SNAP et KDDI, avec des tests indépendants
dans les deux sens. Les notifications de lobby sont suspendues pendant le scénario.
Le dialogue de lancement File2 d'un hôte occidental vers un invité KDDI est
porté et testé avec des clients reconstitués, en conservant le tour commun des
statistiques. La conversion des enveloppes de scénario à deux participants est
raccordée au cœur et comparée aux captures existantes, avec maintien des messages
pendant l'ouverture du socket. Les variantes natives restantes et la charge du transport restent à qualifier ;
les limites précises figurent dans MIGRATION.md.

## Suite des mesures du 21 novembre : retour inverse et journal

Deux clients originaux File2, hôte PAL et invité JAP, ont aussi terminé le
parcours dans l’ordre inverse : retour PAL seul (1P), puis arrivée JAP et passage
automatique à 2P sur les deux versions, avec portraits et pseudos corrects.
Les rapports des deux clients restent associés à une même manche canonique,
distincte de la précédente. Les scores du rapport japonais restent opaques.

Un essai de saisie a ensuite révélé une requête japonaise EVENTDAT 0x670d sans
réponse ; le client a envoyé son logout environ 121 secondes après, tout en
répondant aux battements. Il s’agit du journal d’après-partie, mesuré dans les
anciens essais, et non du texte du chat. Le portage Go assemble désormais les
blocs natifs (32 octets SNAP, 64 KDDI), sans modifier les 2048 octets du journal.
Le socle vérifie les propriétaires, la même manche, les retours effectifs et le
destinataire ; il attend que le destinataire ait commencé son propre échange
avant de lui transmettre le journal. Le module File2 porte les enveloppes.
Les tests vérifient aussi troncatures, destinations étrangères, fin prématurée,
départ et saturation. L’image de laboratoire 4c5e10f1d1d5 a été validée avec les
deux originaux : 32 requêtes/32 réponses japonaises, journaux de 2048 octets
identiques dans les deux sens, événements réellement affichés par chaque jeu
dans sa langue, et présence conservée pendant au moins 235 secondes après le
premier bloc, y compris pendant la saisie au clavier. Le départ JAP réduit
ensuite le compteur PAL de 2P à 1P. Les rapports sont corrélés à une même manche
avec deux propriétaires distincts ; les scores japonais restent opaques.

Un durcissement ultérieur conserve le journal complet dans la manche tant que
la file du destinataire est saturée, puis le transmet quand elle se vide. Il
est couvert par les tests avec détection de courses, et son parcours normal a été revalidé avec les originaux sur l’image
f37af088359f : journaux identiques dans les deux sens, portraits et compteur2P
conservés. La saturation elle-même reste testée en laboratoire reconstitué. La fiabilité du transport sortant sous
saturation reste à qualifier. Le message natif japonais de chat a été capturé :
6701 BROADCAST, champ chiffré de 10 octets pour cinq lettres pleine largeur
Shift-JIS, suivi de 0x0c. Le chat des zones et la production restent à faire.

## Portage du chat de salle File2

Le module reconnaît les enveloppes de texte occidentales de salle mesurées (04/0c),
vérifie le nom avec l’identité authentifiée et décode le champ Shift-JIS natif
japonais. Le socle route les messages entre participants de la même salle et,
en après-partie, de la même manche avec retour effectif. Les clients encore en
jeu et les messages en attente après un départ sont exclus. L’écho japonais
passe par CHATOUT ; l’écho occidental reste celui du client. Les caractères
pleine largeur restent dans le texte commun ; leur rendu occidental rapproche
les lettres latines de leur forme étroite. Aucun remplacement de caractères
japonais par des points d’interrogation ni aucune troncature ne sont ajoutés.
Le type02 appartient au chat de zone ; il reste isolé. Le chat des zones, les autres bandes de texte et File1 ne sont pas qualifiés
par ce portage. La salle d’attente a été validée avec deux clients originaux PAL et JAP sur
l’image f37af088359f : messages reçus dans les deux sens sous les identités
authentifiées, une seule ligne par envoi et bons caractères latins. La capture
confirme le type04 occidental et le trailer04 japonais en attente ; le type0c
reste propre à la mesure d’après-partie. Le texte partagé conserve l’Unicode
pleine largeur, adapté seulement au rendu occidental. Les glyphes japonais
occidentaux, les accents et le chat de zone ne sont pas qualifiés. Le chat après-partie a également été validé avec les deux originaux, une
seule ligne par envoi sous les bons noms. Les journaux durcis restent identiques
bit à bit dans chaque sens ; la salle reste connectée au moins191secondes
après le premier bloc, clavier et messagerie compris. Les deux rapports sont
liés à la même manche canonique sous deux propriétaires prouvés distincts,
sans fabriquer un score japonais encore opaque. Le départ JAP fait passer
le PAL de2P à1P.
Aucune activation en production.

## Chat de zone libre : validation native PAL/JAP

Le type02 est maintenant décodé séparément des salles04 et de l’après-partie0c.
Le module File2 déclare uniquement la zone native1 pour ce premier portage.
Le socle exige une présence authentifiée, hors partie et hors répertoire de
salons KDDI (room0). Cette portée suit les anciens essais de la zone libre et
le filtre explicite de BioServer2 broadcastInArea. Les autres zones et File1
ne sont pas activés par analogie. Les paquets de contrôle APP restent opaques.

Les notifications utilisent la même file bornée que celles des salles. Une
révision de présence rend caducs les messages en attente après navigation,
déconnexion ou passage dans une partie, même après retour au même endroit.
Le nom vient de l’autorité partagée. Le cadre occidental de zone utilise la
source neutre0 historique ; aucun joueur ni identifiant de partie n’est inventé.
Les tests de portée, de saturation et de cadrage passent. La variante est
activée en laboratoire et vérifiée avec les originaux File2 PAL et JAP :
messages dans les deux sens, identités correctes et une seule ligne par message.
Le message envoyé pendant que le JAP consulte le répertoire des salles reste
absent après son retour ; un nouveau message au hall est reçu normalement.
La capture confirme les cadres natifs02 et les cinq caractères latins japonais
rendus en largeur simple côté PAL. La saturation reste testée par reconstruction.
Les compteurs de population des halls restent propres à chaque protocole dans
cet essai : leur affichage commun demeure à traiter séparément.
Aucune activation en production.

## Population commune du hall : correctif en laboratoire

Le module File2 déclare la zone native1 pour le premier compteur commun.
La réponse SNAP USER/liste des zones ajoute les présences JAP réellement
attachées au hall ; la réponse JAP6205 ajoute les présences SNAP au même hall.
Les tickets web sans transport JAP attaché sont exclus. Les départs, la
navigation et l'entrée en partie retirent le supplément de hall correspondant.
Les autres zones et File1 conservent leur portée précédente. Les compteurs
JAP de répertoire/partie et les notifications spontanées restent à mesurer ;
ce correctif ne constitue pas leur fusion complète.

Tests du socle, cadre JAP6205 reconstitué, suite avec détecteur de courses
et analyse statique passent. Image dédiée activée uniquement en laboratoire,
avec retour arrière conservé. Le compteur sur demande a ensuite été vérifié avec les originaux PAL/JAP :
2P des deux côtés après rafraîchissement, puis1P après le départ et un nouveau
rafraîchissement. L'écran déjà ouvert ne se met pas encore à jour spontanément
sur cette image. Les notifications d'arrivée/départ sont maintenant raccordées
dans le code : callbacks de zone SNAP06/07, paquet JAP6205/BROADCAST16, sur
la même file bornée que les autres événements. Aucun polling supplémentaire.
Leur émission est limitée à la portée interprotocole déclarée ; les anciens
contextes du destinataire sont invalidés. La nouvelle image a été vérifiée avec les deux originaux PAL/JAP : les
arrivées, départs et retours font évoluer le compteur1/2 dans les deux sens
sans rafraîchissement manuel. Fermer le client JAP retire aussi sa présence
du compteur PAL. Les échanges de chat des deux côtés restent opérationnels
et ne présentent pas de doublon observé. Ces preuves portent sur deux clients
au hall de la zone libre ; les autres zones et le cycle en partie restent
à qualifier séparément pour ce changement.
Aucune activation en production.

## Sources et références

- **openSNAP** (https://github.com/GrenderG/openSNAP) : référence de protocole SNAP et base du portage du codec et du défi natif Go (longueurs, marqueur final, messages multiples) pour les versions occidentales (`SLUS_207.65`, `SLES_533.19`, `SLUS_209.84`).
- **obsrv / BioServer** (https://obsrv.org) : code Java adapté vers Go pour le cadrage, la connexion et la navigation KDDI des versions japonaises (`SLPM_654.28`, `SLPM_656.92`) ; référence des variantes natives.
- **REO** (https://github.com/Ninjacornix/reo) : recherche et références historiques sur le protocole Outbreak.
- **RFC 5389** (https://datatracker.ietf.org/doc/html/rfc5389) : spécification STUN utilisée pour la sonde de liaison STUN NM.

- **Go crypto/pbkdf2, crypto/rand** (https://pkg.go.dev/crypto/pbkdf2,
  https://pkg.go.dev/crypto/rand) : dépendances standard de l'autorité autonome,
  consultées via la documentation locale `go doc` ; dérivation des mots de passe
  et allocation aléatoire des HN. Aucun service NM requis par cette implémentation.

- **golang.org/x/crypto** (https://pkg.go.dev/golang.org/x/crypto/blowfish) : dépendance v0.38.0, Blowfish pour le protocole historique SNAP ; licence BSD conservée dans `licenses/`.

Les notices et textes de licence des sources portées sont conservés séparément dans [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

- **stunnel** (https://www.stunnel.org/) : composant séparé de terminaison TLS historique employé pour les essais du client japonais original ; aucun binaire TLS embarqué dans le dorsal Go.

- [Go x/text — japanese](https://pkg.go.dev/golang.org/x/text/encoding/japanese), [width](https://pkg.go.dev/golang.org/x/text/width) : dépendance v0.31.0 compatible Go1.24, décodage/encodage Shift-JIS et rendu des largeurs ; documentation officielle consultée. Licence BSD conservée séparément.

- [BioServer2 / gh0stl1ne](https://gitlab.com/gh0stl1ne/Bioserver2) : code de référence et enveloppes natives portées sous AGPL-3.0, notamment EVENTDAT et CHATOUT ; les crédits et licences sont conservés séparément.
