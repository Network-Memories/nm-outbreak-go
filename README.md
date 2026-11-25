# NM-Outbreak Go

[Français](README.md) | [English](README.en.md)

> [!NOTE]
> Ce dépôt est un travail en cours, amené à être amélioré et complété au fil des mesures et validations en laboratoire.

Serveur unifié **Resident Evil Outbreak** (File #1 & File #2) écrit en Go, prenant en charge les clients originaux PS2 occidentaux (**SNAP**) et japonais (**KDDI**) sans patch client.

## Sommaire

- [Démarrage rapide](#démarrage-rapide)
- [Documentation](#documentation)
- [Page de statut web](#page-de-statut-web)
- [Licence](#licence)
- [Sources et références](#sources-et-références)

## Démarrage rapide

```bash
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

- **SNAP (PS2 US/PAL)** : `9090/udp` (bootstrap), `9091/udp` (File #1), `9092/udp` (File #2)
- **KDDI (PS2 JP)** : `8300/tcp` & `8690/tcp` (File #1), `8200/tcp` & `8590/tcp` (File #2)
- **API & Web** : `18090/tcp` (état JSON), `18091/tcp` (portail natif), `8080/tcp` (page de statut)

## Documentation

- [Installation & configuration](docs/INSTALL.md)
- [Architecture & modules](docs/ARCHITECTURE.md)
- [Protocole SNAP & KDDI](docs/PROTOCOL.md)
- [Notices & licences tierces](THIRD_PARTY_NOTICES.md)

## Page de statut web

Une interface web légère est servie sur `http://localhost:8080` (`web/index.html`) et affiche en temps réel les salles actives et les joueurs en ligne via `GET /state` (`18090/tcp`).

## Licence

Distribué sous licence **AGPL-3.0-or-later**. Voir [LICENSE](LICENSE) et [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Sources et références

Remerciements et crédits aux créateurs et contributeurs des projets **BHO / KDDI / Outbreak** : **panzer punk**, **no23 / obsrv.org** (`BioServer`), **gh0stl1ne** (`BioServer2`), **GrenderG** (`openSNAP`) et **Ninjacornix** (`REO`).

- **bioserver-docker / panzer punk** (https://github.com/panzer-punk/bioserver-docker) : conteneurisation et intégration de référence BioServer.
- **openSNAP** (https://github.com/GrenderG/openSNAP) : référence de protocole SNAP et base du portage du codec et du défi natif Go (`SLUS_207.65`, `SLES_533.19`, `SLUS_209.84`).
- **obsrv / BioServer** (https://obsrv.org) : code Java adapté vers Go pour le cadrage, la connexion et la navigation KDDI (`SLPM_654.28`, `SLPM_656.92`).
- **BioServer2 / gh0stl1ne** (https://gitlab.com/gh0stl1ne/Bioserver2) : code de référence et enveloppes natives portées sous AGPL-3.0 (`EVENTDAT`, `CHATOUT`).
- **REO** (https://github.com/Ninjacornix/reo) : recherche et références historiques sur le protocole Outbreak.
- **RFC 5389** (https://datatracker.ietf.org/doc/html/rfc5389) : spécification STUN.
- **stunnel** (https://www.stunnel.org/) : terminaison TLS historique séparée pour les essais du client japonais original.
- **golang.org/x/crypto** & **golang.org/x/text** : Blowfish et encodage Shift-JIS (licences BSD conservées dans `licenses/`).
