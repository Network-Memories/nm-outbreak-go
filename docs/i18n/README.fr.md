<p align="center"><img src="../../assets/banner.svg" alt="NM-OUTBREAK-GO — Resident Evil Outbreak" width="100%" /></p>

<p align="center"><a href="../../README.md">English</a> · <strong>Français</strong> · <a href="README.de.md">Deutsch</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <a href="README.ru.md">Русский</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.zh-CN.md">简体中文</a></p>

<h3 align="center">Serveur de jeu auto-hébergé · Resident Evil Outbreak (File #1 &amp; File #2)</h3>

<p align="center">Go 1.24 · Docker Compose · SNAP (US/PAL) &amp; KDDI (JP)</p>

[Compatibilité](../../docs/COMPATIBILITY.md) · [Installation](../../docs/INSTALL.md) · [Architecture](../../docs/ARCHITECTURE.md) · [Protocole](../../docs/PROTOCOL.md) · [Licence](../../LICENSE)

> **Travail en cours.** Ce dépôt est amélioré continuellement au fil des mesures de protocole et des validations en laboratoire. Les vérifications d'installation autonome sont suivies séparément. Ceci ne constitue pas une garantie globale que chaque jeu, mode, région ou NAT soit pris en charge.

Serveur unifié en Go prenant en charge les clients PS2 originaux occidentaux (**SNAP**) et japonais (**KDDI**) sans patch client.

## Parcours confirmés et testés

| Jeu | Plateforme | Statut |
| --- | --- | --- |
| Resident Evil Outbreak (File #1) | PS2 (NTSC-J ↔ NTSC-U) | Lancement mixte JAP → US et partie confirmés en labo |
| Resident Evil Outbreak: File #2 | PS2 (PAL / NTSC-U ↔ NTSC-J) | Salon mixte, lancement et partie confirmés en labo |

<details><summary><strong>Autres parcours testés et partiels</strong></summary>

| Jeu | Plateforme | Statut |
| --- | --- | --- |
| Resident Evil Outbreak (File #1) | PS2 (hôte NTSC-U → NTSC-J) | Join inverse et `StatWord` confirmés ; validation du lancement hôte US en attente |
| Resident Evil Outbreak: File #2 | PS2 (3–4 joueurs, modes spéciaux) | `MixedEnginePeers` actuellement borné à 2 en attente de mesure multi-clients |

</details>

**[Catalogue complet des jeux et éditions](../../docs/COMPATIBILITY.md)**

## Démarrer une instance locale

Exécuter depuis la racine du dépôt. Générez vos propres secrets ; gardez les bases de données et interfaces de contrôle privées.

```sh
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

[Lisez le guide d'installation avant d'ouvrir des ports de jeu.](../../docs/INSTALL.md)

## Carte du dépôt

| Chemin | Description |
| --- | --- |
| `cmd/nm-outbreak/` | Point d'entrée du serveur |
| `internal/` | Moteur commun, transports SNAP/KDDI, modules File #1 &amp; File #2 |
| `web/` | Tableau de bord de statut en direct (`http://localhost:8080`) |
| `tools/` | Scripts locaux de bootstrap et de vérification de protocole |
| `docs/` | Installation, architecture, compatibilité et notes de protocole |

## Sources et références

Remerciements et crédits aux créateurs et contributeurs des projets **BHO / KDDI / Outbreak** : **panzer punk**, **no23 / obsrv.org** (`BioServer`), **gh0stl1ne** (`BioServer2`), **GrenderG** (`openSNAP`) et **Ninjacornix** (`REO`).

- **bioserver-docker / panzer punk** (https://github.com/panzer-punk/bioserver-docker) : conteneurisation et intégration de référence BioServer.
- **openSNAP** (https://github.com/GrenderG/openSNAP) : référence de protocole SNAP et base du portage du codec et du défi natif Go (`SLUS_207.65`, `SLES_533.19`, `SLUS_209.84`).
- **obsrv / BioServer** (https://obsrv.org) : code Java adapté vers Go pour le cadrage, la connexion et la navigation KDDI (`SLPM_654.28`, `SLPM_656.92`).
- **BioServer2 / gh0stl1ne** (https://gitlab.com/gh0stl1ne/Bioserver2) : code de référence et enveloppes natives portées sous AGPL-3.0 (`EVENTDAT`, `CHATOUT`).
- **REO** (https://github.com/Ninjacornix/reo) : recherche et références historiques sur le protocole Outbreak.
- **RFC 5389** (https://datatracker.ietf.org/doc/html/rfc5389) : spécification STUN.
- **stunnel** (https://www.stunnel.org/) : composant séparé de terminaison TLS historique pour les essais du client japonais original.
- **golang.org/x/crypto** &amp; **golang.org/x/text** : Blowfish et encodage Shift-JIS (licences BSD conservées dans `licenses/`).
