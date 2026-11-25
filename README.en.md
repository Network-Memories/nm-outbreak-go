<div align="center">

# NM-Outbreak Go

**Unified Resident Evil Outbreak Server (File #1 & File #2)**  
*Native Western (SNAP) and Japanese (KDDI) PS2 client support — no client patches*

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue?style=for-the-badge)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker&logoColor=white)](docker-compose.yml)
[![Status](https://img.shields.io/badge/Status-Work%20in%20Progress-orange?style=for-the-badge)](#)

[🇫🇷 Français](README.md) • [🇬🇧 English](README.en.md) • [🇷🇺 Русский](README.ru.md)

</div>

---

> [!NOTE]
> This repository is an active work in progress, continuously improved as protocol measurements and lab validations advance.

## Quick Start

```bash
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

| Service | Protocol / Port | Role |
|---|---|---|
| **SNAP Bootstrap** | `9090/udp` | Initial US/PAL client redirection |
| **SNAP File #1 & #2** | `9091/udp`, `9092/udp` | PS2 US / PAL lobby & gameplay |
| **KDDI File #1** | `8300/tcp`, `8690/tcp` | PS2 JP lobby & gameplay (`SLPM_654.28`) |
| **KDDI File #2** | `8200/tcp`, `8590/tcp` | PS2 JP lobby & gameplay (`SLPM_656.92`) |
| **API & Web Portal** | `18090/tcp`, `18091/tcp` | JSON state API (`/state`) & native portal |
| **Web Status UI** | `8080/tcp` | Live status dashboard (`web/index.html`) |

## Documentation

- 📘 [Installation & Configuration](docs/INSTALL.md)
- 🏗️ [Architecture & Modules](docs/ARCHITECTURE.md)
- 🔌 [SNAP & KDDI Protocol](docs/PROTOCOL.md)
- 📄 [Third-Party Notices](THIRD_PARTY_NOTICES.md)

## License

Licensed under **AGPL-3.0-or-later**. See [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Sources & References

Credits and thanks to the creators and contributors of the **BHO / KDDI / Outbreak** projects: **panzer punk**, **no23 / obsrv.org** (`BioServer`), **gh0stl1ne** (`BioServer2`), **GrenderG** (`openSNAP`), and **Ninjacornix** (`REO`).

- **bioserver-docker / panzer punk** (https://github.com/panzer-punk/bioserver-docker): BioServer containerization and reference integration.
- **openSNAP** (https://github.com/GrenderG/openSNAP): SNAP protocol reference and base for the Go codec and native challenge port (`SLUS_207.65`, `SLES_533.19`, `SLUS_209.84`).
- **obsrv / BioServer** (https://obsrv.org): Java code adapted to Go for KDDI framing, login, and directory navigation (`SLPM_654.28`, `SLPM_656.92`).
- **BioServer2 / gh0stl1ne** (https://gitlab.com/gh0stl1ne/Bioserver2): reference code and native envelopes ported under AGPL-3.0 (`EVENTDAT`, `CHATOUT`).
- **REO** (https://github.com/Ninjacornix/reo): historical Outbreak protocol research and references.
- **RFC 5389** (https://datatracker.ietf.org/doc/html/rfc5389): STUN specification.
- **stunnel** (https://www.stunnel.org/): separate legacy TLS termination component used for original Japanese client testing.
- **golang.org/x/crypto** & **golang.org/x/text**: Blowfish and Shift-JIS encoding (BSD licenses preserved in `licenses/`).
