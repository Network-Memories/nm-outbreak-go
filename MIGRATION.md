# Reprise de la fusion Go — 20 novembre 2026

Cible : un seul service, transports natifs SNAP et KDDI, registre en mémoire
commun, modules File 1 et File 2. Préserver les services et les données NM.

## Increment réalisé

- Copies profondes des profils, règles et réglages sur entrée/sortie du registre.
- Diffusion non bloquante sous verrou de lecture ; fermeture des canaux sous
  verrou exclusif. Départ invité et fermeture hôte réveillent les lecteurs.
- Chaque destinataire possède sa trame. Connexions absentes du roster refusées.
- Désabonnement chat explicite, sûr pendant la publication.
- Ordre déterministe des salons ; les emplacements natifs stables par session
  restent à porter séparément.
- Commandes KDDI corrigées selon le Commands.java existant : SLOTSTATUS 6404,
  SLOTPLRSTATUS 6403, SLOTTITLE 6402, SLOTATTRIB2 640b, SLOTPWDPROT 6405,
  SLOTSCENTYPE 650a, PLAYERSTATS 640a, CHATIN 6701 ; serveur 18.
  Test d'enveloppe avec opcode littéral indépendant et titre vide.

## Moteur fusionné, suite du 20 novembre

`internal/core.Engine` possède maintenant l'autorité, les sessions logiques et
le cycle CreateRoom/JoinRoom/LeaveRoom/Logout, les profils et le relais. Le programme
construit une seule instance et la fournit à tous les `NewTransport`. Les anciens
constructeurs créant chacun leur état ont été retirés.

SNAP ne conserve plus de copie du profil, de l'identité ou de l'appartenance au
salon : ses Binding ne portent que l'identifiant natif, l'adresse UDP, le lien
vers la session du cœur et le numéro de lobby. Les mutations utilisent le cœur.
Le relais KDDI reçoit une session du cœur, pas un couple arbitraire salon/joueur.
Sa fermeture détache seulement le transport ; la place reste occupée jusqu'au
Leave/Logout explicite. La fermeture par l'hôte libère les sessions invitées.

Les connexions portent un propriétaire prouvé et une région mesurée. PAL File1
est refusé ; File1 et File2 restent séparés dans ce moteur commun. Un profil non
reçu n'est pas remplacé par une fiche de zéros. Le champ owner_id identifie le
propriétaire de l'autorité sélectionnée ; aucun identifiant NM n'est inventé pour
un compte autonome.

Le faux login KICS basé sur un pseudo a été supprimé. Une adoption native exige
une session déjà authentifiée ; l'adresse du paquet doit correspondre au binding.
La vérification des défis bootstrap n'étant pas portée, ce chemin est testé via
un handoff authentifié explicitement construit dans les fixtures, pas via le jeu.
Le chat KDDI brut non authentifié a également été retiré du handler provisoire.
Les commandes SNAP étaient aussi partiellement erronées : corrigées selon la
référence locale commands.py (login01, logout02, statut0d, send0f, ack00, wrapper28,
résultat KICS29). Le résultat KICS porte les trois mots mesurés, pas un ACK générique.

Tests : cycle commun hôte JAP/invité occidental, profil mis à jour et isolé,
relais direct, détachement sans perte de place, fermeture hôte et logout,
absence de preuve, mauvais jeu/région/profil, refus du faux login KICS et d'une
session révoquée. Ce sont des tests reconstitués ; les flux natifs complets,
la fiabilité et les séquences UDP restent à terminer avant validation originale.

Les limites de région/profil/zone/scénario sont déclarées par chaque module
(`file1.Definition`, `file2.Definition`), copiées à l'enregistrement dans le cœur.
Le cœur ne contient plus de condition File1/File2 pour ces variantes. Le travail
de mot de passe ou d'autorité distante se fait hors verrou du moteur : un test
bloque volontairement une authentification tout en laissant passer une trame de
partie établie. L'arrêt du listener KDDI ferme aussi ses lectures inactives.

## Parcours à porter avant toute bascule

1. Identité NM : client identity non raccordé aux transports, route actuelle à
   remplacer par le contrat NM mesuré. Préserver les comptes historiques courts,
   les alias et secrets existants. Une session en mémoire ne prouve pas un compte NM.
2. KDDI : LOGIN, chiffrement des charges utiles, cycle natif lobby ; distinguer
   les sockets de jeu, traiter GSLOGIN et brancher RelayGameConnection.
3. SNAP : bootstrap/authentification, fiabilité UDP, Capcom APP (configuré mais
   pas démarré par main).
4. Projections de profils par jeu/région, admission atomique, lancement, moteur,
   départs et après-partie. Mesurer File 1 sans recopier File 2 aveuglément.
5. STUN : la sonde ne prouve pas le transport réel ; le relais est explicitement non implémenté.
6. Amis/messages NM, site, Station et statistiques ; régressions et clients
   originaux avant toute activation avec sauvegarde et retour arrière.

Les échanges TCP/UDP des tests sont reconstruits : aucune nouvelle validation
avec client original ni activation Go en production pendant cet increment.

## Autorité autonome ajoutée

Le module identity propose maintenant une autorité locale et un sélecteur explicite
standalone/nm. Le programme ouvre une seule autorité et transmet la même instance
aux transports. La méthode Register attribue un identifiant, un alias compatible
et un HN aléatoire, atomiquement persistés. Authenticate reprend cette identité,
sans recréer ni renommer quoi que ce soit. Les secrets NM ne sont jamais importés.
Le mode NM échoue explicitement si sa configuration manque, sans compte local de
secours. L'outil account-create lit le secret sur stdin, avant de démarrer un port.

Le magasin JSON est privé et prévu pour un seul processus. L'outil de préparation
hors ligne exige l'arrêt du serveur. Avant une exposition aux joueurs, raccorder
les méthodes aux véritables sessions natives, leurs défis d'authentification et
leurs limites mesurées, ainsi que la limitation des tentatives et les parcours
amis/messages. Ne pas annoncer ces parcours validés parce que le stockage passe
les tests. Le login commun appelle cette autorité ; la preuve issue des défis natifs doit encore y être raccordée.

## Résultat de la validation isolée du 21 novembre

Les captures ont invalidé le CRC et l'en-tête SNAP initiaux ; découpage natif,
longueur et marqueur final corrigés puis comparés à 7 731 datagrammes réels.
958 trames lobby et 942 enregistrements moteur KDDI capturés passent aussi ;
2 400 combinaisons de profils passent face aux modules de référence.
49 cas Go passent avec race detector, vet et compilation. Le test runtime expose
correctement l'état non prêt : bootstrap/LOGIN/GSLOGIN et APP manquent encore.
La sonde STUN exige maintenant une adresse mappée ; elle ne déclare plus de
transport/relais de secours opérationnel sans implémentation.
Priorité suivante : faire fonctionner les vrais débuts de connexion avant de
faire croire qu'un assemblage de codecs et registre constitue une partie native.

## Bootstrap porté le 21 novembre — état actuel

Le blocage bootstrap SNAP décrit plus haut est maintenant levé pour les tests
reconstitués autonomes : défi chiffré, preuve, handoff authentifié, profil/salle
et logout passent File1 US, File2 US et PAL avec un client Python indépendant.
Les preuves invalides, expirées, rejouées après nouvelle session et étrangères
sont refusées. Le cœur conserve l'autorité commune ; aucun repli sur IP/pseudo.
Le magasin v2 protège le matériel natif avec une clé privée séparée ; les anciens
comptes se préparent avec account-enable-snap après preuve du secret existant,
sans changer propriétaire, alias ni HN. Sauvegarder comptes et .native-key ensemble.

Régressions Go avec race detector, vet et build : PASS ; image de laboratoire
compilée avec Go 1.24 : PASS. Le client Python vérifie aussi panne/reprise STUN
simulée et arrêt propre. Aucun client original ni production contacté.
Restent : fournisseur NM natif, fiabilité UDP complète, LOGIN/GSLOGIN KDDI,
APP, lancement/après-partie, relais et transport NAT réellement raccordés.
Ne pas déployer ce prototype comme remplacement jouable. Les constats précédents
sont historiques ; cette section précise uniquement le progrès bootstrap.

## Suite vers la production — 21 novembre 2026

Les paragraphes précédents décrivent les étapes historiques. État actuel :

- Fiabilité SNAP : fenêtre d'envoi 16, attente des séquences manquantes,
  détection des doublons, ACK exact, réémission avec estimation de délai et
  maintien par paquet fiable vide. Les relais distinguent le destinataire du
  SID de l'expéditeur. Listes paginées sous la limite native de 896 octets.
- Capcom APP dans le même processus : ouverture, annonce sans patch,
  téléchargement NETBIO, annuaire et pages minimales. 45 réponses comparées au
  Python de référence ; client Python indépendant File1 US / File2 US/PAL.
- KDDI : ticket issu de la preuve web, LOGIN, version, HN, profil et entrée
  GSLOGIN portés. Essais reconstitués File1/File2 ; salons/règles/admission et
  démarrage natifs complets restent à porter. Les HN historiques ne sont pas
  régénérés au choix de personnage.
- Rapports des deux protocoles dans le journal privé commun. Décodeur APP
  occidental et structure File2 JAP portés séparément. Résumé et fin du rapport
  JAP restent opaques ; aucun classement japonais inventé.
- Registre canonique par jeu, manche serveur et propriétaire prouvé, champs
  absents distincts de zéro. Reprises d'envoi sans crédit double, fusion des
  mesures compatibles et refus des contradictions. Le parcours APP se rattache
  à une session SNAP prouvée et sa transition native, pas à une IP seule.
- Adaptateur UDP du service commun STUN : réservation des vrais ports,
  datagrammes transmis par le canal natif, suivi et reconnexion. État visible
  distinct de la sonde RFC. Pas de repli direct silencieux en mode NM.

Client original File1 US non patché : APP/NETBIO, preuve chiffrée, KICS, hall
et liste des zones atteints dans le laboratoire. Raccordement réel des sockets
SNAP au service natif commun constaté. Blocage ensuite avant l'entrée dans une
zone ; maintien de session et détails du protocole encore en investigation.
Aucune partie complète Go ni interopération originale JAP/PAL/US validée.
L'image de laboratoire ne constitue pas une activation de production.

Avant bascule : terminer et vérifier les transports natifs, après-partie,
identité NM (contrat HTTP encore incomplet), amis/messages, stats japonaises,
reprise des données historiques et raccordements site/Station/bot existants.
Vérifier sous NAT distincts et le relais de secours. Conserver `ready=false`.
Sauvegarder comptes, clé native et historiques avant toute activation ; aucun
service OVH remplacé pendant cette étape.

## Sortie de hall native — 21 novembre

Le client original File1 US envoyait `07/b000` avant d'entrer dans sa zone.
Le serveur répondait sur le canal des salles (`e000`), puis le jeu attendait sans
envoyer le `06` suivant. Le résultat respecte désormais le canal demandé (`f000`
pour cette sortie fiable de hall), et la présence de zone est libérée. Une sortie
de salle conserve au contraire la présence dans la zone. Le résultat précède les
notifications aux autres membres, comme dans les gestionnaires amont portés.

Après correction : original File1 US connecté, entrée AREA 01, liste des salles,
création d'une salle, Kevin et attente 1/2P observés. Maintien fiable reçu et
acquitté toutes les vingt secondes pendant le silence. Aucun patch du jeu.
Régression des deux canaux sur File1/File2 et parcours indépendant de sortie puis
réentrée File1 US, File2 US/PAL réussis ; captures privées, tests `-race` et `vet`
réussis. Cela ne valide pas encore une partie complète ou l'interopérabilité JAP.

## File1 US original à deux et rapports réels — 21 novembre

Deux clients originaux US v2, sans patch, ont effectué : création et entrée en
salle, Kevin/Mark, compteur 2/2P, lancement commun, cinématique synchronisée et jeu.
Après abandon, chacun reçoit son résultat, sauvegarde et rejoint MEETING ROOM 2P.
R1 affiche le personnage et le journal de l'autre. Le départ de l'invité revient
au hall et passe le compteur de l'hôte à 1P ; la fermeture de l'hôte est vérifiée.

APP 6301 reçoit les deux rapports : deux propriétaires prouvés, même identifiant
de manche. Le premier rapport réel est rejoué deux fois depuis le transport
du client du laboratoire : quatre archives au total, deux résultats canoniques,
deux propriétaires et une seule manche. L'essai concerne un abandon, pas la
réussite du scénario ; les points zéro sont reçus. Le temps de réussite zéro de
ce rapport n'est pas le temps de jeu affiché à l'écran et ne doit pas l'usurper.

Un défaut de projection est également corrigé dans le code : l'entrée native en
scénario positionne Playing dans le registre commun ; il reste positionné jusqu'au
retour de tous les membres du roster. Cette évolution supplémentaire est testée
sur le cœur mais n'est pas encore dans l'image du parcours original ci-dessus.

La validation des nouvelles créations impose maintenant les mots de passe de 5
à 10 caractères, avec message/formulaire alignés. Les connexions et la reprise
des secrets historiques restent distinctes, testées avec secrets courts et longs
persistés. Le fournisseur NM distant reste à porter selon le contrat réel.

## Annuaire KDDI et coordonnées stables — 21 novembre

Portage des commandes natives 6203/04/05/06/0a/07/09, 6301/02/03/04/05/08,
6401/08 : zones, descriptions, compteurs, statut, sélection, salons et sorties.
Les modules déclarent leur propre annuaire (File1 : 2 zones et 10 salons ;
File2 : 10 zones et 6 salons ; 20 emplacements par salon). File2 réutilise les
libellés et états activés du fichier de réglages APP. Les constantes opaques des
réponses natives ne deviennent pas des statistiques publiques interprétées.

Le cœur possède la navigation authentifiée et les coordonnées stables des
parties, indexées par jeu/zone/salon. Les requêtes de slot ne parcourent plus
une liste globale qui change les numéros après une fermeture. File1 libre mappe
la zone2/salonN vers la zone partagée N ; File2 mappe zoneN/salon1 vers N.
Le mode automatique File1 n'est pas annoncé comme porté par cette correspondance.
Une zone/salon désactivé, inexistant ou une requête tronquée ne change pas l'état.
L'emplacement vide actif porte le statut natif1 (libre), pas0 (désactivé).

Une seule connexion lobby possède sa présence de navigation ; sa fermeture la
retire sans supprimer la place logique ou la manche en cours. Un test couvre
explicitement le scénario en cours et la possibilité de rattachement ultérieur.
Les notifications de population spontanées, création/admission/règles/lancement
KDDI et parcours original japonais restent à porter/mesurer avant activation.
Tests -race et vet complets réussis, parcours indépendant SNAP/KDDI/APP vérifié.
Aucune nouvelle activation native ou production pour cet increment.

## Admission KDDI vers les salles occidentales — 21 novembre

Les commandes natives 6406 (entrée), 640a (aperçu des membres), 6501 (départ),
6911/12/13 (nombre, place et fiche) utilisent le même cœur et roster que SNAP.
Le mot de passe de salle est décodé dans son champ natif après le numéro de slot,
sans troncature ni assimilation au mot de passe du compte. La résolution du slot
et l'admission sont effectuées sous le même verrou, avec les contrôles communs
de capacité, profil reçu, jeu, fermeture et mot de passe. Un refus ne change pas
le roster ; rejouer une entrée valide ne crée pas un deuxième membre.

Lorsqu'un client a consulté les personnages, l'aperçu conserve l'identifiant de
la partie. Si celle-ci ferme et que le slot est réutilisé, l'ancienne sélection
ne rejoint pas silencieusement sa remplaçante : il faut consulter de nouveau.
La sortie/navigation/déconnexion retire ce contexte de prévisualisation.

Les fiches natives portent les HN/pseudos prouvés et les 208 octets natifs reçus
(préfixe mesuré du profil SNAP240). Une identité ou un profil absent ne devient
pas une fiche artificielle. L'encodage historique non ASCII reste à porter.
Les refus utilisent le cadrage PacketString avec le drapeau natif ff.

Tests Go sur File1 et File2 : mauvaise clé de salle, salle fermée/pleine, entrée
répétée, aperçu devenu périmé, places et sortie sans perte de l'hôte occidental.
Client Python indépendant : trois variantes SNAP créent une salle et un second
propriétaire KDDI consulte, rejoint deux fois sans doublon, lit les places et
repart. Échanges reconstitués, pas une preuve avec un jeu japonais original.
Les notifications vers l'hôte occidental ne sont pas encore raccordées ; ne pas
confondre l'admission dans le cœur avec un lancement mixte opérationnel.
La création japonaise exige encore son cycle réservé/règles/validation6504,
et le lancement exige les adaptations natives mesurées. Production inchangée.

## Création KDDI réservée puis publiée — 21 novembre

Portage du cycle 6407 réservation, 6509 scénario/type natif, 6609 titre, 660a
mot de passe de salle, 660b choix de règle, 6504 validation. Les requêtes
6601/02/03/04/05/06/07/08/0e et 660c sont cadrées selon le Java. La validation
6504 d'un invité déjà admis reste distincte de la création par l'hôte.

Le cœur réserve l'emplacement sous verrou, sans publier de world incomplet.
Une deuxième création ne peut pas le prendre et une nouvelle salle SNAP reçoit
un autre numéro. Scénario déclaré, profil effectivement reçu et règles valides
sont exigés avant publication. La réservation est remplacée par une salle unique,
au même numéro ; rejouer la validation ne crée pas de doublon. Annulation,
déconnexion ou logout nettoient la réservation et son secret temporaire.
La vue de création n'expose pas ce secret ; la salle publiée utilise la protection
commune par empreinte salée. Le type natif DVD/HDD déclaré est conservé.

Les schémas et projections de règles résident dans les modules, copiés sans
alias mutable. Les valeurs fixes File2 suivent les réglages communs APP ; cela
corrige notamment le défaut du Java où Survie déclarait le tir allié désactivé
alors que la configuration commune l'impose activé. Les effets en jeu de chaque
zone restent à qualifier ; cette concordance de configuration n'en est pas une
preuve. Le mode automatique File1 reste distinct du mode libre porté ici.

Tests Go : pas de salle avant validation, réservation exclusive, publication
concurrente SNAP dans un autre slot, choix invalide/verrouillé, mot de passe,
rejeu, annulation et déconnexion. Client Python indépendant : création native
File1JAP puis entrée File1US ; création File2JAP puis entrées File2US et PAL.
Les transports consultent deux membres et deux propriétaires prouvés dans la
même salle, avec projection des fiches dans les deux sens. Puis départ invité et
fermeture hôte. Échanges reconstitués, aucune nouvelle validation originale JAP.

Restent notamment le timer et le lancement,
la projection des règles d'une salle SNAP reçue sans RuleValues vers l'UI KDDI,
les encodages historiques non ASCII (titres limités ici à 16 octets ASCII),
l'identité NM et les autres prérequis de bascule. Production inchangée.

## Événements communs de salle

Le cœur publie les arrivées, changements de profil et départs à partir des
mutations autoritaires du même registre. Chaque destinataire reçoit un instantané
indépendant ; les adaptateurs SNAP et KDDI émettent leurs enveloppes natives sans
entrée/sortie réseau sous le verrou du cœur. Les anciens callbacks SNAP directs
sont retirés pour éviter les notifications en double.

Les files sont bornées. Une entrée ou modification de profil est refusée avant
mutation lorsqu'un destinataire est saturé ; les départs disposent de places
réservées. Un dépassement résiduel devient une erreur explicite du transport.
Aucune notification de lobby n'est injectée dans un scénario. Le transport KDDI
sérialise les trames entières, même en présence d'écritures partielles concurrentes.
Les mises à jour de profil seules ne sont pas encore diffusées vers KDDI : leur
place dans les étapes natives de préparation et d'après-partie reste à mesurer.

Vérifications : tests Go avec détecteur de courses et vet ; client Python
indépendant dans les deux sens File1 US/JAP et File2 US/PAL/JAP, avec attente
explicite des notifications d'arrivée et de départ, identifiant stable et compteurs.
Ces échanges reconstitués ne qualifient pas un lancement entre jeux originaux.

Une observation complémentaire utilise File1 US v2 original sans patch comme
hôte et un invité KDDI reconstitué d'un propriétaire distinct : l'écran passe de
1/2P à 2/2P, la fiche du second membre montre son pseudo et Mark, puis le départ
revient à 1/2P et retire la fiche. La fermeture de l'hôte libère le slot et le
registre. Les preuves restent privées. Cet essai ne valide ni le client japonais
original ni un scénario mixte.

## Lancement File2 occidental vers un invité KDDI

Le transport SNAP reconnaît les messages ciblés 03 à 08 du lancement File2
mesuré. Le cœur fige les participants et un identifiant de tour commun avant
l'entrée en scénario. Le message03 de l'hôte déclenche KDDI6910 ; seule une
requête de lancement du vrai transport KDDI produit l'acquittement occidental04.
Les messages05 à08 de l'hôte fournissent la table : nombre et place conformes aux
participants prouvés, noms conformes à leurs alias, profils reconstitués par
fragments ordonnés et règles interprétées par le module File2. Les requêtes KDDI
attendent cette table pendant un délai borné, sans verrou réseau dans le cœur.

Les réponses6911/12/13/14/15/16 portent cette même table, les règles reçues,
un numéro natif stable sur quinze chiffres et l'adresse du listener de jeu
configuré. 6002 exige une table complète avant la transition en scénario.
La fermeture du lobby conserve la place et le contexte du tour. Les résultats
exigent toujours une entrée réelle par participant ; préparer le lancement ne
produit aucun score. Les valeurs de démonstration de l'ancien6917 ne sont pas
reprises : la requête est explicitement refusée tant qu'une réponse native
fondée sur les résultats reçus n'est pas qualifiée.

Vérifié : tests Go avec détecteur de courses et vet ; un client Python indépendant
exécute le cycle complet des messages de lancement pour un hôte US puis PAL,
avec preuve KDDI avant04, profils et règles, numéro stable, adresse/port, refus
des faux scores, confirmation6002 et maintien des deux places après fermeture
du lobby. Les tests vérifient aussi les messages hors ordre, noms substitués,
compteurs fictifs, départ avant fin et l'identifiant de statistiques commun.
Un retour ancien répété ne peut plus effacer le nouveau tour préparé.

Cette étape n'a pas été activée dans le laboratoire des jeux originaux ni en
production. Elle ne valide pas le dialogue préalable de préparation de la salle,
les requêtes6917 effectivement exigées par le client original, le transport de
scénario mixte, son après-partie, un hôte japonais ni le lancement File1. Les
adaptations restent limitées aux variantes mesurées du module File2.

## Transport de scénario File2 à deux participants

Le module File2 active la variante de transport mixte mesurée à deux places.
Le cœur associe chaque message au même tour et à la place du véritable émetteur.
Les adaptateurs décodent les enveloppes SNAP et KDDI en corps de message neutres,
sans les interpréter comme des statistiques. Le sens inverse reconstruit le
mot natif index/longueur, le keepalive et la forme japonaise à padding pour un
corps de 36 octets. Le routage entre familles passe directement en mémoire.
Le relais occidental entre clients de même famille reste conservé.

Chaque destinataire dispose d'une file bornée de 256 messages, ouverte avant son
socket de jeu pour conserver les premiers échanges. Un lot dont un émetteur est
substitué ou dont la taille est incompatible est refusé intégralement. Un seul
transport consomme chaque place ; un ancien détachement ne révoque pas son
remplaçant. Départ et fermeture ferment les files. La fermeture du seul socket
conserve la place logique, comme auparavant.

Côté japonais, l'adaptateur de transport n'annonce son extrémité occidentale
qu'après les deux entrées natives prouvées. Il traite les phases et sondes
mesurées ; huit échos distincts et identiques aux sondes émises sont nécessaires
avant sa phase2. Ce sont des sondes du serveur adaptateur, pas des échanges STUN
attribués au jeu ni une preuve de progression dans le scénario.

La saturation est explicite : les lots SNAP fiables restent dans la file de
transport bornée de 1 024 datagrammes et sont réessayés dans l'ordre. Un datagramme
non conservé n'est pas acquitté. L'allocation du numéro de sortie est faite après
le contrôle de capacité, pour éviter un trou lors d'une nouvelle tentative. Le
transport japonais suspend sa lecture pendant une attente bornée si la file de
sortie est pleine. Les écritures de jeu japonaises sont complètes et sérialisées.

Vérifications : client indépendant US puis PAL, connexion KDDI réelle au socket
Go, sondes et échos, message conservé avant ouverture du socket, corps construits
de 36 octets dans les deux sens et keepalive. Le corpus privé original comparé
à la référence Python comprend 1 918 enveloppes occidentales et 410 messages de
données japonais ; les corps restent identiques dans les conversions aller-retour.
Deux autres messages occidentaux ne correspondent pas à la grammaire moteur
et restent hors du corpus qualifié, avec les captures originales conservées.
Les deux flux clients occidentaux qualifiés utilisent chacun une seule place,
respectivement0 et1. Les tests contrôlent aussi les files pleines, les doublons,
les émetteurs substitués, les lots rejetés sans mutation et les baux de transport.

Ce ne sont pas de nouveaux essais joués avec les clients originaux. Le dialogue
de préparation de salle, les requêtes6917 réellement exigées, l'exécution d'un
scénario et l'après-partie mixte restent à valider. File1, l'hôte japonais et les
parties mixtes à trois ou quatre joueurs ne sont pas déclarés qualifiés par cet
incrément. Aucune activation en production ; l'état de disponibilité reste dégradé.

## Préparation de salle et attente native

Le signal ciblé File2 de l'hôte occidental (tag1, huit octets nuls) produit
670e avec le HN de l'hôte réel dès réception du signal authentifié. La capture
native historique montre670e avant le rafraîchissement des membres, puis6504 :
attendre6504 pour transmettre670e bloquait l'invité original après6406. Le
correctif respecte cet ordre sans inventer une préparation depuis l'admission.
La réponse6412, mesurée dans cette capture, conserve le numéro de salle et
quatre octets opaques nuls ; elle exige une salle visible existante. Ces octets
ne sont pas projetés en scores ou en statistiques.
Le6504 réel produit séparément6506 à la place
native du joueur, y compris vers lui-même. Ces étapes sont distinctes de l'entrée
dans le scénario ; leurs rejeux ne créent pas de notifications en double et le
départ retire leur contexte.

Les requêtes de règles KDDI peuvent projeter la propriété occidentale240 et STAT
File2 déjà mesurés. Les choix sont contrôlés contre le schéma du module ; un
champ inconnu ou contradictoire reste un refus. Les drapeaux non affichés dans
une zone japonaise restent conservés pour6914. Le timer6409 utilise l'instant de
création serveur, non public, et le choix d'attente reçu : 3/5/10/15/30 minutes
explicitement déclarées dans les deux modules. Les mises à jour de profil,
l'arrivée d'un membre et les changements de STAT ne réinitialisent pas ce timer.
Un timer à zéro ne simule ni annonce ni acquittement de lancement. File1 SNAP
sans règles d'attente mesurées ne reçoit pas de durée inventée.

Tests : ordre des deux preuves, rejeu, retrait au départ, projection et attente
sans réinitialisation ; le client indépendant US/PAL vérifie6506,670e,660x et6409
avant les étapes de lancement et de transport précédentes. L'essai japonais
original reste distinct de ces échanges reconstitués.

## Première connexion du client japonais original

Un essai avec File2 japonais original, sans patch, a mis en évidence des écarts
que le client reconstitué ne couvrait pas. Le frontal TLS moderne refusait son
chiffrement ; l'adaptateur TLS historique existant a permis d'atteindre le
serveur Go. Le premier chemin réel était CRS-top.jsp, absent du prototype : il
est maintenant servi avec le rechargement vers login attendu par le navigateur.
L'annuaire enterareas répond avec CRS-lbs-info-get et son CSV, selon les entrées
déclarées par module, les ports du même dorsal et les connexions natives réelles.
Les tickets web seuls ne sont pas comptés comme des connexions au lobby.

Le formulaire original a authentifié le compte de laboratoire et son secret
complet, puis affiché Connected. Le client a demandé startsession, enterareas
et ouvert le TCP du lobby Go. Son LOGIN contient seize octets : le champ ticket
habituel suivi du suffixe opaque00020000. Le ticket décodé correspond exactement
à la preuve web et l'identifiant de réponse correspond au défi. Cette variante
est déclarée dans le seul module File2 et exige les octets exacts ; ni File1 ni
GSLOGIN ne sont étendus sur cette seule observation.

Le client original demande ensuite61a0 et quatre sondes600e avant une réponse
VERSION de33octets avec son propre compteur. Le module File2 accepte cette
forme, sans interpréter le contenu opaque comme une version de patch à livrer.
La forme canonique vide et corrélée des tests existants reste disponible.
Les échecs de taille, de corrélation canonique et les fuites de variante vers
File1 sont testés. Le test indépendant exécute désormais ce dialogue mesuré.
Ces observations valident le parcours web et le début de connexion natif ; elles
ne suffisent pas à déclarer une partie originale mixte fonctionnelle.

Le client japonais original a ensuite sélectionné son HN : requête6132,
réponse acceptée et notification6104 observées. Les réponses6001 aux contrôles
périodiques prouvent le maintien du dialogue au-delà d’une minute. Le client
attend ensuite614c (message du jour), absent du prototype. Le serveur répond
désormais par la forme vide de MessageOfTheDay.getPacket de BioServer : numéro
zéro et chaîne de longueur zéro. Aucun contenu de production n’est importé.
Les deux modules et les requêtes malformées sont vérifiés par tests de transport.
La réponse vide a été acceptée par le client original, qui transmet ensuite
son profil de personnage (6190,212octets), puis interroge6881. Les salons et
une partie mixte originale ne sont pas encore validés.

6881 décrit un nombre de blocs et leurs tailles dans BioServer File2. Le module
File2 annonce désormais zéro bloc ; toute demande6882 est refusée. Aucun des
blocs opaques de Packet6881.java n’est transmis au jeu. Le client original a accepté le catalogue
vide et a interrogé ensuite les classements6145 des onze scénarios, sans6882. Le transport vérifie les requêtes vides et le module
File1 ne reçoit pas cette variante. La projection des classements natifs6145
n’est pas encore implémentée : réponse d’indisponibilité, aucun score de
démonstration ni projection des rapports japonais opaques. Le client
original accepte ces refus et affiche les dix zones : parcours jusqu’à la
sélection de zone validé dans la capture15. Cela ne valide ni les classements
natifs, ni une partie mixte.

La navigation originale atteint la zone libre, la liste de vingt emplacements,
les règles, puis la fiche d’un hôte PAL reconstitué avec son HN et son portrait.
L’admission6406 est reçue côté SNAP. L’entrée visuelle après correction de670e
est validée avec l’original japonais et cet hôte reconstitué : salle2/2,
compte à rebours, puis6504 et6506 observés dans la capture17. Ce test ne
constitue pas une partie jouée entre deux clients originaux. La création par l’original japonais révèle séparément une
sélection6509 FFFF/FFFF, refusée : la cause de cette sélection invalide reste
à établir. L’ancien parcours natif réussi envoyait0011/0001. La navigation avec
catalogue vide ne suffit donc pas à qualifier la création d’une partie.

Un essai suivant avec les deux jeux originaux File2 PAL et JAP valide aussi
la salle2/2 et les fiches natives des deux personnages, avec les mêmes données
de sauvegarde reçues des deux côtés. Le lancement transmet la table de l’hôte,
puis le JAP demande6912 avec un octet nul, comme dans l’ancienne capture native.
Cette forme est désormais acceptée sans utiliser cet octet comme numéro de place.
La place provient toujours du tableau de l’hôte. L’ancienne forme vide reste
acceptée, les autres corps sont refusés. La validation en jeu obtenue ensuite est décrite ci-dessous.

La sortie6209 originale File2 porte six octets nuls. Cette variante est déclarée
par le module ; le File1 conserve sa forme qualifiée existante. Le logout6006
explicite reçoit une réponse vide puis révoque la session, en conservant le compte.
Une simple fermeture du socket de lobby pendant le transfert en jeu conserve
ses règles distinctes et sa place. Les tests vérifient ces différences, le rejet
des corps non mesurés et la conservation de l’identité après reconnexion.

L’essai original suivant confirme6912 et les deux6913, puis le refus6917
annule le lancement japonais. Une expérience File2 isolée prépare une réponse
courte6917 `[joueur,00,00]`, sans les cinq mots de score du serveur historique.
Le sens exact du short nul n’est pas décodé. Les deux clients originaux acceptent
la réponse, poursuivent6915/6914/6916/6002 et arrivent en jeu ; aucune projection
ni classement valide ne peut cependant en être déduit.
Les numéros hors du roster figé et les corps non mesurés sont refusés.

## Essai original File2 PAL + JAP : jeu et retour

Les deux jeux originaux non patchés atteignent Wild Things sur le dorsal Go.
Un déplacement de Mark effectué côté PAL est visible côté JAP. Le sens inverse
n’a pas été qualifié séparément, Kevin étant attaqué pendant la manipulation.
Après abandon natif, chaque écran de résultats identifie son partenaire de
l’autre région ; les deux sauvegardes sur cartes mémoire de laboratoire terminent.
Ces observations ne qualifient ni des NAT distincts ni tous les scénarios.

Le retour reste bloqué sur l’image essayée : le client japonais reconnecté
adresse6210 (entrée en salle d’après-partie), jusque-là non traité. Le nouveau
code traite6210/6211/6212/6213 et demande6138 à la sélection du HN lorsqu’une
entrée réelle de ce propriétaire dans la manche est prouvée. Le moteur compte
uniquement les retours effectifs ; un partenaire encore en jeu garde sa place,
mais ne devient pas une présence fictive dans la salle d’après-partie. Les
notifications6215/6214/6212 utilisent ces mêmes identités et retours communs.
Les tests de moteur et d’enveloppes passent avec race detector. Ce correctif a ensuite été activé dans le laboratoire et essayé avec les deux
originaux ; la mesure partielle ci-dessous distingue le retour JAP du défaut
PAL persistant. La consolidation des rapports reste à vérifier. Aucun
déploiement en production.

## Mesure originale de l’après-partie mixte

L’image de laboratoire suivante supprime le blocage japonais6210 : l’original
JAP envoie6138 (105 octets, propriétaire authentifié, structure décodée), entre
dans la salle d’après-partie, affiche2 joueurs et la fiche Mark de l’hôte PAL.
La sortie6211 revient proprement au hall. Le PAL reste cependant à1P alors que
sa réponse native QUERYUSER contient2 membres. Le retour mixte complet reste
non validé ; une mesure originale PAL + US est préparée pour comparer le cycle.
Les champs japonais de résultat restent opaques, sans classement fabriqué.

Le code de collecte rattache désormais le rapport structurel japonais à la
manche commune uniquement si son numéro natif correspond et si l’entrée réelle
de ce propriétaire est prouvée. Cette attribution ne décode aucun score et ne
crée aucune ligne classée. L’ajout est vérifié localement, sans activation dans
l’image utilisée pour cette mesure. Les vérifications globales race/vet passent.

## Comparaison originale PAL + US et marqueur de retour

Deux clients originaux PAL + US exécutent Wild Things, puis l’abandon, les
résultats et les sauvegardes. Les deux salles d’après-partie affichent2P et le
portrait du partenaire ; le départ US ramène le compteur PAL à1P. Leurs deux
rapports APP rejoignent la même manche canonique, avec une ligne par propriétaire.

Les propriétés originales PAL et US portent alors1 à l’octet221 (0xdd).
Le callback de propriété du PAL lit cet octet et le compare à1. La conversion
208→240 du profil japonais le laissait à zéro. Une projection propre au module
File2 est ajoutée : seul ce marqueur est adapté, uniquement pour le propriétaire
KDDI dont l’entrée et le retour dans la même manche sont prouvés. Les données
western d’origine, File1 et les joueurs encore en scénario restent inchangés.
Aucun score, durée ni journal d’événements n’est synthétisé. La projection sert
le roster et la notification de propriété de retour ; les tests globaux race/vet
passent. Cette modification n’est pas encore activée ni qualifiée avec les deux
originaux PAL + JAP. Le défaut1P reste donc un défaut observé de l’image essayée.

## Marqueur validé avec les originaux PAL + JAP

L'essai suivant utilise le correctif et deux originaux File2 non patchés :
hôte PAL Mark, invité JAP Kevin, Wild Things puis abandon. Les résultats et les
deux sauvegardes terminent. Le JAP revient en premier et reste seul tant que le
PAL n'est pas revenu ; les deux salles affichent ensuite2P. Les fiches montrent
Kevin et son alias côté PAL, Mark et son HN côté JAP. Le départ natif du JAP
ramène le compteur PAL à1P ; le membre restant est Mark après navigation R1.

Le rapport japonais6138 et le rapport APP6301 du PAL rejoignent la même manche
canonique avec leurs propriétaires authentifiés distincts. Le rapport japonais
reste opaque pour les scores : ce rattachement n'invente aucune ligne classée.
La validation ne couvre pas encore l'ordre inverse des retours, un hôte japonais,
US + JAP original, trois ou quatre joueurs, une réussite complète de scénario,
les NAT distincts ni le relais de secours. Le chat natif entre les deux dialectes
reste à raccorder. Le service de production est inchangé.

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
Shift-JIS, suivi de 0x0c. Le chat de salle est désormais validé dans la mesure
suivante ; les zones et la production restent à faire.

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

- [openSNAP](https://github.com/GrenderG/openSNAP) : référence du transport occidental
  via les sources locales NM-OUTBREAK déjà intégrées.
- [BioServer / obsrv](https://obsrv.org/viewtopic.php?start=15&t=5251) : lignée Java
  existante, Commands.java consulté pour les valeurs KDDI corrigées.
- [BioServer / corbin-zip](https://github.com/corbin-zip/bioserver) : historique de
  cette lignée. Conserver séparément les obligations de licence du code porté.

- [Go crypto/pbkdf2](https://pkg.go.dev/crypto/pbkdf2) et
  [crypto/rand](https://pkg.go.dev/crypto/rand) : bibliothèque standard utilisée
  pour l'autorité locale, documentation locale go doc consultée.

- [Go x/crypto](https://pkg.go.dev/golang.org/x/crypto/blowfish) : dépendance du défi SNAP historique ; licence BSD distribuée séparément.
- [stunnel](https://www.stunnel.org/) : dépendance séparée utilisée pour la terminaison TLS historique des essais japonais originaux ; aucun binaire TLS ajouté au runtime Go.

- [BioServer2 / gh0stl1ne](https://gitlab.com/gh0stl1ne/Bioserver2) : code Java local consulté et enveloppes d’après-partie et EVENTDAT portées sous AGPL-3.0 ; conserver les crédits et la licence du portage.

- [Go x/text — japanese](https://pkg.go.dev/golang.org/x/text/encoding/japanese), [width](https://pkg.go.dev/golang.org/x/text/width) : dépendance v0.31.0 compatible Go1.24, décodage/encodage Shift-JIS et rendu des largeurs ; documentation officielle consultée. Licence BSD conservée séparément.
