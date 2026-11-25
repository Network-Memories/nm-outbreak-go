Notes de décodage SNAP — Resident Evil Outbreak (File #1 & File #2)
==================================================================

1. Ports UDP & TCP associés
---------------------------
- 9090/udp : Bootstrap / redirection SNAP
- 9091/udp : Lobby & jeu SNAP File #1 (NTSC-U)
- 9092/udp : Lobby & jeu SNAP File #2 (PAL / NTSC-U)
- 8300/tcp : Lobby KDDI File #1 (NTSC-J)  | 8690/tcp : GameServer KDDI File #1
- 8200/tcp : Lobby KDDI File #2 (NTSC-J)  | 8590/tcp : GameServer KDDI File #2


2. Format d'un datagramme SNAP
------------------------------
Un datagramme UDP contient 1 ou plusieurs messages SNAP contigus, suivis d'un
marqueur de fin fixe sur 4 octets (big-endian) :

  [ Message 1 ] [ Message 2 (si FlagMulti) ] ... [ 0xba 0x47 0x66 0x11 ]

FooterMarker = 0xba476611


3. En-tête d'un message SNAP (16 octets, big-endian)
----------------------------------------------------
Offset  Taille   Champ             Description
0x00    2        TypeFlags | Size  6 bits hauts = TypeFlags (0xfc00)
                                   10 bits bas = taille totale du message (0x03ff, min 16)
0x02    1        Sequence          Numéro de séquence applicatif de la commande
0x03    1        Command           Code commande SNAP
0x04    4        SessionID         Identifiant de session SNAP (WireID)
0x08    4        PacketSeq         Numéro de séquence transport (0 dans un sous-message FlagMulti)
0x0c    4        AckNumber         Numéro de paquet acquitté (PacketSeq reçu)
0x10    Size-16  Payload           Corps du message

Bits de TypeFlags (0xfc00) :
- 0x8000 (FlagReliable)    : paquet fiable (ACK + retransmission)
- 0x4000 (FlagResponse)    : réponse / notification serveur -> client
- 0x2000 (FlagRoom)        : canal salle (room)
- 0x1000 (FlagChannelBits) : canal lobby / aire
- 0x0800 (FlagMulti)       : un autre message suit dans le même datagramme
- 0x0400                   : bit relais chat (sur CmdSend / CmdSendTarget)


4. Échanges Requête / Réponse SNAP (Client <-> Serveur)
-------------------------------------------------------
A) Connexion & Bootstrap (9090/udp -> 9091/9092/udp)
  1. Client -> Serveur (9090) : CmdLoginToKics (0x01)
     Serveur -> Client (9090) : CmdResultLogin (0x03)
       Payload (12 octets) : port_jeu[4 BE] + 0x01234567[4 BE] + SessionID[4 BE]
  2. Client -> Serveur (9091/9092) : CmdLoginToKics (0x01)
     Serveur -> Client (9091/9092) : CmdResultLogin (0x03)
       Payload (12 octets) : port_local[4 BE] + 0x01234567[4 BE] + SessionID[4 BE]

B) Profil & Aires
  - Client -> Serveur : CmdChangeUserProperty (0x0c), Payload = 240 octets (profil)
    Serveur -> Client : ResultOK (0x00000000) sur FlagRoom
  - Client -> Serveur : CmdJoin (0x06) avec FlagChannelBits (0x1000), Payload = lobbyID[4 BE]
    Serveur -> Client : ResultOK sur FlagChannelBits
  - Client -> Serveur : CmdQueryGameRooms (0x14) avec lobbyID[4 BE]
    Serveur -> Client : liste des salles (FlagChannelBits | FlagResponse)

C) Création, entrée et sortie de salle
  - Création :
    Client -> Serveur : CmdCreateGameRoom (0x04)
      Payload : title[16] + maxPlayers[4 BE] + password[16] + unknown[4] + statWord[4 BE]
    Serveur -> Client : Result = roomNumericID[4 BE] (ou ErrorRefused)
  - Mise à jour STAT :
    Client -> Serveur : CmdChangeAttribute (0x09), Payload = "STAT" + statWord[4 BE]
    Serveur -> Client : ResultOK
  - Entrée en salle :
    Client -> Serveur : CmdJoin (0x06) avec FlagRoom, Payload = roomNumericID[4 BE] (+ password)
    Serveur -> Client : ResultOK
    Serveur -> Membres : CmdJoin (0x06, FlagReliable|FlagRoom|FlagResponse) avec BuildMemberRecord
  - Sortie de salle / déconnexion :
    Client -> Serveur : CmdLeave (0x07) -> Serveur -> Client : ResultOK
      (+ CmdLeave vers les autres membres)
    Client -> Serveur : CmdLogoutClient (0x0b) -> Serveur -> Client : ResultOK

D) Salle d'attente mixte (Hôte US <-> Invité JAP)
  1. Dès que l'invité JAP rejoint la salle :
     Hôte US -> Serveur : CmdSendTarget (0x10), target=WireID_JAP, tag=1 (20 octets, body=8 zéros)
     Serveur -> Invité JAP (TCP KDDI) : 0x670e (handle[6] + 8 zéros)
     Invité JAP -> Serveur (TCP KDDI) : 0x6504 (1 octet = 1)
     Serveur -> Invité JAP (TCP KDDI) : 0x6506 (0x00, seat+1, 0x00, 0x00)
  2. Requêtes d'attente de l'invité US (lorsque JAP est hôte) :
     Invité US -> Serveur : CmdSendTarget (0x10), 12 octets, tag=10 / 2 / 14
     Serveur -> Invité US : CmdSendTarget (0x10), tag=11 (wait*30) / tag=12 (2) / tag=15

E) Lancement mixte (US hôte -> JAP invité)
  1. Hôte US -> Serveur : CmdSendTarget (0x10), tag=3 (12 octets)
     Serveur -> Invité JAP (TCP KDDI) : 0x6404 (slot, 4) + 0x6910 (vide)
  2. Invité JAP -> Serveur (TCP KDDI) : 0x6911 (bloqué jusqu'à ACK)
     Serveur -> Hôte US (UDP SNAP) : CmdSendTarget (0x10), tag=4 (12 + 768 zéros)
  3. Hôte US -> Serveur (UDP SNAP) :
     - tag=5 (number, count)
     - tag=6 (noms des joueurs, 1 par joueur)
     - tag=7 (profils 240 octets en 5 morceaux : 50+50+50+50+40 par joueur)
     - tag=8 (fin de transfert)
  4. Serveur débloque JAP (TCP KDDI) :
     - 0x6911 -> count
     - 0x6912 -> number+1
     - 0x6913 -> profils 208 octets
     - 0x6914 -> règles (18 octets File #1 / 34 octets File #2)
     - 0x6915 -> NativeNumber
     - 0x6916 -> IP + port GameServer (8690/8590)
     - 0x6917 -> EmptyPlayerScore
     - 0x6002 -> confirmation entrée en scénario
  5. Hôte US -> Serveur : CmdChangeUserStatus (0x0d), Payload = 1 (entrée en scénario)

F) En jeu & Fin de partie
  - Relais moteur : CmdSend (0x0f) sur FlagRoom (sans bit chat 0x0400)
  - Sortie de scénario : CmdChangeUserStatus (0x0d), Payload = 0xf7e00001
  - Keepalive fiable : CmdAck (0x00) avec FlagReliable après ~20s de silence
