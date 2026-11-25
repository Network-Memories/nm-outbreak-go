<div align="center">

# NM-Outbreak Go

**Единый сервер Resident Evil Outbreak (File #1 и File #2)**  
*Нативная поддержка западных (SNAP) и японских (KDDI) клиентов PS2 — без патчей клиента*

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue?style=for-the-badge)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker&logoColor=white)](docker-compose.yml)
[![Status](https://img.shields.io/badge/Status-Work%20in%20Progress-orange?style=for-the-badge)](#)

[🇫🇷 Français](README.md) • [🇬🇧 English](README.en.md) • [🇷🇺 Русский](README.ru.md)

</div>

---

> [!NOTE]
> Этот репозиторий находится в активной разработке и постоянно улучшается по мере проведения измерений и проверок в лаборатории.

## Быстрый старт

```bash
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

| Сервис | Протокол / Порт | Назначение |
|---|---|---|
| **SNAP Bootstrap** | `9090/udp` | Начальное перенаправление клиентов US/PAL |
| **SNAP File #1 & #2** | `9091/udp`, `9092/udp` | Лобби и игра PS2 US / PAL |
| **KDDI File #1** | `8300/tcp`, `8690/tcp` | Лобби и игра PS2 JP (`SLPM_654.28`) |
| **KDDI File #2** | `8200/tcp`, `8590/tcp` | Лобби и игра PS2 JP (`SLPM_656.92`) |
| **API и веб-портал** | `18090/tcp`, `18091/tcp` | JSON API (`/state`) и нативный портал |
| **Веб-статус** | `8080/tcp` | Панель статуса в реальном времени (`web/index.html`) |

## Документация

- 📘 [Установка и настройка](docs/INSTALL.md)
- 🏗️ [Архитектура и модули](docs/ARCHITECTURE.md)
- 🔌 [Протокол SNAP и KDDI](docs/PROTOCOL.md)
- 📄 [Уведомления и сторонние лицензии](THIRD_PARTY_NOTICES.md)

## Лицензия

Распространяется под лицензией **AGPL-3.0-or-later**. См. [LICENSE](LICENSE) и [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Источники и ссылки

Благодарность создателям и участникам проектов **BHO / KDDI / Outbreak**: **panzer punk**, **no23 / obsrv.org** (`BioServer`), **gh0stl1ne** (`BioServer2`), **GrenderG** (`openSNAP`) и **Ninjacornix** (`REO`).

- **bioserver-docker / panzer punk** (https://github.com/panzer-punk/bioserver-docker): контейнеризация и эталонная интеграция BioServer.
- **openSNAP** (https://github.com/GrenderG/openSNAP): спецификация протокола SNAP и основа для порта кодека и нативного challenge на Go (`SLUS_207.65`, `SLES_533.19`, `SLUS_209.84`).
- **obsrv / BioServer** (https://obsrv.org): Java-код, адаптированный на Go для фрейминга, входа и навигации KDDI (`SLPM_654.28`, `SLPM_656.92`).
- **BioServer2 / gh0stl1ne** (https://gitlab.com/gh0stl1ne/Bioserver2): эталонный код и нативные конверты под AGPL-3.0 (`EVENTDAT`, `CHATOUT`).
- **REO** (https://github.com/Ninjacornix/reo): исследования и исторические материалы по протоколу Outbreak.
- **RFC 5389** (https://datatracker.ietf.org/doc/html/rfc5389): спецификация STUN.
- **stunnel** (https://www.stunnel.org/): отдельный компонент терминации TLS для тестирования оригинального японского клиента.
- **golang.org/x/crypto** и **golang.org/x/text**: Blowfish и кодировка Shift-JIS (лицензии BSD сохранены в `licenses/`).
