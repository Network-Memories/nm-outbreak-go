# Notices de sources tierces

## openSNAP

Projet amont : https://github.com/GrenderG/openSNAP.
Le découpage des datagrammes SNAP dans internal/snap/wire.go est porté depuis
la logique de opensnap/protocol/codec.py et les constantes du projet existant,
avec adaptation à Go et validation sur des captures natives privées. Le défi
SNAP de internal/protocol/snapauth et le traitement bootstrap sont portés depuis
opensnap/core/bootstrap/handlers.py ; les vecteurs de test comparent les octets
à la référence Python. Les mécanismes de réception ordonnée, réémission,
fenêtre et maintien de session suivent core/sessions.py, core/engine.py et
udp_server.py. Le codec, les flux, le décodeur de résultat occidental et les
fichiers de configuration NETBIO Capcom APP sont portés depuis opensnap_app/capcom.
Les recherches, listes et relais SNAP suivent plugins/common.py et les plugins
Outbreak. L'adaptateur du contrôle natif reprend l'intégration NM locale existante.
Les notifications de membres SNAP utilisent les enveloppes des plugins Outbreak ;
la distribution des événements et les files bornées sont implémentées dans le cœur Go.
Licence amont : GNU Affero General Public License version 3. Le texte distribué
par l'amont est conservé dans licenses/openSNAP-AGPL-3.0.txt.

## BioServer

Les sources Java de la lignée obsrv.org / no23, historique corbin-zip/bioserver,
sont utilisées pour le cadrage et les commandes KDDI. Le début de connexion,
le masque des champs, la sélection des HN et les pages de démarrage du navigateur
sont portés/adaptés vers Go. Les routes CRS-top.jsp et enterareas, leurs marqueurs
et le CSV suivent ViewCRSTopAction.php, ViewEnterareasAction.php et les templates
Twig existants ; les entrées du catalogue sont déclarées par module. Les variantes
LOGIN seize octets et VERSION33 sont issues d'un nouveau relevé du client File2
original. Le Java et PHP ne sont pas embarqués dans le runtime Go.
La navigation zones/salons/emplacements est portée depuis PacketHandler.java ;
les formes distinctes des annuaires File1/File2 viennent de Areas.java, Rooms.java
et Slots.java. L'allocation stable des coordonnées et le rattachement au registre
commun sont implémentés dans le cœur Go.
Les enveloppes d'entrée/sortie et de consultation des membres sont adaptées de
PacketHandler.java, Client.java et ClientList.java ; le cadrage des refus suit
PacketString.java. Le contrôle d'admission partagé et la protection contre un
aperçu de salle périmé sont implémentés dans le cœur Go.
Les enveloppes6506 et670e de préparation suivent PacketHandler.java et les
mesures du signal occidental dans admission_host.py. Le timer6409 suit
Slot.java ; son origine temporelle est conservée dans le registre Go.
Le maintien de connexion reprend HeartBeatThread.java et PacketHandler.java ;
la réponse vide au message du jour suit MessageOfTheDay.getPacket. La requête
614c après sélection du HN et les réponses de maintien ont été observées avec
le client File2 japonais original en laboratoire.
Le catalogue6881 reprend seulement la forme nombre de blocs décrite par
PacketHandler.java ; les tables opaques de Packet6881.java ne sont pas reprises.
La commande6145 répond indisponible et ne reprend aucun score de démonstration.
La réponse opaque6412 suit PacketHandler.java et une capture native historique ;
l'ordre greeting670e avant6504 a été corrigé d'après cette même capture.
Le corps nul de6912 et la sortie6209 à six octets nuls proviennent des relevés
natifs File2. Le logout6006 suit PacketHandler.sendLogout ; les comptes restent
dans l'autorité commune et seule la session explicitement quittée est révoquée.
Les étapes de réservation/configuration/validation et les réponses aux requêtes
de règles sont adaptées de PacketHandler.java et RuleSet.java. Les contraintes
et formes propres à File1/File2 résident dans leurs modules Go ; les effets fixes
File2 reprennent la configuration commune APP existante plutôt que des valeurs
divergentes de l'ancienne implémentation.
Les diffusions KDDI d'arrivée, départ et état de salle suivent également
PacketHandler.java ; leur transport concurrent est adapté au serveur Go.
Les sources originales consultées portent Copyright 2013-2019 obsrv.org (no23@deathless.net), AGPL v3
ou ultérieure. Leurs notices doivent rester conservées lors d'autres portages.

## Travail NM préexistant de lancement interrégions

Le décodeur et la machine d'états SNAP03..08 sont adaptés du module Python
snap_launch.py de l'implémentation NM antérieure, elle-même fondée sur des
captures PAL/US et l'analyse du client original. Les enveloppes04 suivent
admission_host.py ; les contrôles de règles File2 suivent ForeignRoomRules.java.
Les codecs de scénario sont adaptés de snap_game.py et kddi_game.py ; les
sondes, phases et la correspondance keepalive reprennent les mesures portées
dans ForeignGameBridge.java. La distribution neutre, les baux de transport et
les files bornées sont implémentés dans le cœur Go. Les règles de fiabilité
SNAP déjà créditées sont prolongées pour conserver les lots différés sans trous
de séquence lorsque la destination est saturée.
Les requêtes natives6914/15/16 et6002 suivent PacketHandler.java de la lignée
BioServer déjà créditée ci-dessus. Le contexte de tour, les identités et les
files d'événements sont réunis dans le cœur Go ; aucun coordinateur HTTP de
l'ancienne architecture n'est embarqué dans ce parcours.

## Go x/crypto

Dépendance golang.org/x/crypto v0.38.0, paquet blowfish utilisé pour le protocole
SNAP historique. Licence BSD conservée dans licenses/golang-x-crypto-BSD.txt.
Cette dépendance ne sert pas au chiffrement du magasin local (AES-GCM standard Go).

## Adaptateur TLS des essais japonais

Le composant stunnel historique existant sert de terminaison TLS dans le
laboratoire. Il reste un composant séparé ; son binaire et ses bibliothèques
ne sont pas copiés dans l'image du dorsal Go. Conserver les licences et notices
de cet adaptateur lors de sa distribution séparée.

## Sources et références

- [openSNAP](https://github.com/GrenderG/openSNAP) : code porté du transport, du défi et de Capcom APP ; callbacks de zone Outbreak2 `_area_callbacks` consultés pour les notifications de population06/07.
- [Go x/crypto](https://pkg.go.dev/golang.org/x/crypto/blowfish) : dépendance Blowfish.
- [BioServer / obsrv](https://obsrv.org/viewtopic.php?start=15&t=5251) : référence
  du protocole japonais et base du portage des débuts de connexion.
- [BioServer / corbin-zip](https://github.com/corbin-zip/bioserver) : historique
  de la lignée Java consultée.

- [stunnel](https://www.stunnel.org/) : dépendance séparée de terminaison TLS historique utilisée pour les essais avec le client japonais original.

- [BioServer2 / gh0stl1ne](https://gitlab.com/gh0stl1ne/Bioserver2) : PacketHandler.java consulté et enveloppes natives d’après-partie portées (6210–6215, demande6138, EVENTDAT670d/670e, CHATOUT6702, filtre broadcastInArea) ; ClientList.countPlayersInArea et cadre AREAPLAYERCNT6205 consultés pour la distinction hall/répertoire, AGPL-3.0 ; cycle commun réimplémenté en Go.

- [BioServer2 / corbin-zip](https://github.com/corbin-zip/bioserver) : référence pour les enveloppes 6881/6882 et la fixture de catalogue optionnelle utilisée en laboratoire. La fixture reste externe au binaire.

- [Go x/text — japanese](https://pkg.go.dev/golang.org/x/text/encoding/japanese), [width](https://pkg.go.dev/golang.org/x/text/width) : dépendance v0.31.0 compatible Go1.24, décodage/encodage Shift-JIS et rendu des largeurs ; documentation officielle consultée. Licence BSD conservée séparément.
