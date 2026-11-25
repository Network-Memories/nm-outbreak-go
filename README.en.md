# NM-Outbreak Go

[Français](README.md) | [English](README.en.md)

> [!NOTE]
> This repository is an active work in progress, continuously improved as protocol measurements and lab validations advance.

Unified **Resident Evil Outbreak** (File #1 & File #2) server written in Go, supporting original Western (**SNAP**) and Japanese (**KDDI**) PS2 clients without client patches.

## Contents

- [Quick Start](#quick-start)
- [Documentation](#documentation)
- [Web Status Page](#web-status-page)
- [License](#license)
- [Sources & References](#sources--references)

## Quick Start

```bash
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

- **SNAP (PS2 US/PAL)**: `9090/udp` (bootstrap), `9091/udp` (File #1), `9092/udp` (File #2)
- **KDDI (PS2 JP)**: `8300/tcp` & `8690/tcp` (File #1), `8200/tcp` & `8590/tcp` (File #2)
- **API & Web**: `18090/tcp` (JSON state), `18091/tcp` (native portal), `8080/tcp` (status page)

## Documentation

- [Installation & Configuration](docs/INSTALL.md)
- [Architecture & Modules](docs/ARCHITECTURE.md)
- [SNAP & KDDI Protocol](docs/PROTOCOL.md)
- [Third-Party Notices](THIRD_PARTY_NOTICES.md)

## Web Status Page

A lightweight web UI is served on `http://localhost:8080` (`web/index.html`), displaying live active rooms and online players via `GET /state` (`18090/tcp`).

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
