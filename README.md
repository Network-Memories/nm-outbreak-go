<p align="center"><img src="assets/banner.svg" alt="NM-OUTBREAK-GO — Resident Evil Outbreak" width="100%" /></p>

<p align="center"><strong>English</strong> · <a href="docs/i18n/README.fr.md">Français</a> · <a href="docs/i18n/README.ru.md">Русский</a></p>

<h3 align="center">Self-hosted game server software · Resident Evil Outbreak (File #1 &amp; File #2)</h3>

<p align="center">Go 1.24 · Docker Compose · SNAP (US/PAL) &amp; KDDI (JP)</p>

[Game compatibility](docs/COMPATIBILITY.md) · [Installation](docs/INSTALL.md) · [Architecture](docs/ARCHITECTURE.md) · [Protocol](docs/PROTOCOL.md) · [License](LICENSE)

> **Work in progress.** This repository is continuously improved as protocol measurements and lab validations advance. Standalone installation checks are tracked separately. This is not a blanket claim that every game, mode, region or NAT is supported.

Unified Go server supporting original Western (**SNAP**) and Japanese (**KDDI**) PS2 clients without client patches.

## Confirmed & tested paths

| Game | Platform | Status |
| --- | --- | --- |
| Resident Evil Outbreak (File #1) | PS2 (NTSC-J ↔ NTSC-U) | Mixed JAP → US launch &amp; gameplay confirmed in lab |
| Resident Evil Outbreak: File #2 | PS2 (PAL / NTSC-U ↔ NTSC-J) | Mixed lobby, launch &amp; gameplay confirmed in lab |

<details><summary><strong>Other tested and partial paths</strong></summary>

| Game | Platform | Status |
| --- | --- | --- |
| Resident Evil Outbreak (File #1) | PS2 (NTSC-U host → NTSC-J) | Reverse join &amp; `StatWord` confirmed; US-host launch validation pending |
| Resident Evil Outbreak: File #2 | PS2 (3–4 players, special modes) | `MixedEnginePeers` currently bounded to 2 pending multi-client measurement |

</details>

**[Complete game and edition catalogue](docs/COMPATIBILITY.md)**

## Start a local instance

Run from the repository root. Generate your own secrets; keep databases and control interfaces private.

```sh
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

[Read the installation guide before opening any game ports.](docs/INSTALL.md)

## Repository map

| Path | Description |
| --- | --- |
| `cmd/nm-outbreak/` | Server entrypoint |
| `internal/` | Core engine, SNAP/KDDI transports, File #1 &amp; File #2 modules |
| `web/` | Live status dashboard (`http://localhost:8080`) |
| `tools/` | Local bootstrap and protocol verification scripts |
| `docs/` | Installation, architecture, compatibility and protocol notes |

## Sources & References

Credits and thanks to the creators and contributors of the **BHO / KDDI / Outbreak** projects: **panzer punk**, **no23 / obsrv.org** (`BioServer`), **gh0stl1ne** (`BioServer2`), **GrenderG** (`openSNAP`), and **Ninjacornix** (`REO`).

- **bioserver-docker / panzer punk** (https://github.com/panzer-punk/bioserver-docker): BioServer containerization and reference integration.
- **openSNAP** (https://github.com/GrenderG/openSNAP): SNAP protocol reference and base for the Go codec and native challenge port (`SLUS_207.65`, `SLES_533.19`, `SLUS_209.84`).
- **obsrv / BioServer** (https://obsrv.org): Java code adapted to Go for KDDI framing, login, and directory navigation (`SLPM_654.28`, `SLPM_656.92`).
- **BioServer2 / gh0stl1ne** (https://gitlab.com/gh0stl1ne/Bioserver2): reference code and native envelopes ported under AGPL-3.0 (`EVENTDAT`, `CHATOUT`).
- **REO** (https://github.com/Ninjacornix/reo): historical Outbreak protocol research and references.
- **RFC 5389** (https://datatracker.ietf.org/doc/html/rfc5389): STUN specification.
- **stunnel** (https://www.stunnel.org/): separate legacy TLS termination component used for original Japanese client testing.
- **golang.org/x/crypto** &amp; **golang.org/x/text**: Blowfish and Shift-JIS encoding (BSD licenses preserved in `licenses/`).
