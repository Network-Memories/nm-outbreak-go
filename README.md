<div align="center">

# NM-Outbreak Go

**Serveur unifié Resident Evil Outbreak (File #1 & File #2)**  
*Support natif PS2 occidental (SNAP) et japonais (KDDI) — sans patch client*

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue?style=for-the-badge)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker&logoColor=white)](docker-compose.yml)
[![Status](https://img.shields.io/badge/Status-Work%20in%20Progress-orange?style=for-the-badge)](#)

[🇫🇷 Français](README.md) • [🇬🇧 English](README.en.md) • [🇷🇺 Русский](README.ru.md)

</div>

---

> [!NOTE]
> Ce dépôt est un travail en cours, amené à être amélioré et complété au fil des mesures et validations en laboratoire.

## Démarrage rapide

```bash
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

| Service | Protocole / Port | Rôle |
|---|---|---|
| **SNAP Bootstrap** | `9090/udp` | Redirection initiale des clients US/PAL |
| **SNAP File #1 & #2** | `9091/udp`, `9092/udp` | Lobby & jeu PS2 US / PAL |
| **KDDI File #1** | `8300/tcp`, `8690/tcp` | Lobby & jeu PS2 JP (`SLPM_654.28`) |
| **KDDI File #2** | `8200/tcp`, `8590/tcp` | Lobby & jeu PS2 JP (`SLPM_656.92`) |
| **API & Portail Web** | `18090/tcp`, `18091/tcp` | API JSON (`/state`) & portail natif |
| **Tableau de bord Web** | `8080/tcp` | Interface de statut en direct (`web/index.html`) |

## Documentation

- 📘 [Installation & configuration](docs/INSTALL.md)
- 🏗️ [Architecture & modules](docs/ARCHITECTURE.md)
- 🔌 [Protocole SNAP & KDDI](docs/PROTOCOL.md)
- 📄 [Notices & licences tierces](THIRD_PARTY_NOTICES.md)

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
