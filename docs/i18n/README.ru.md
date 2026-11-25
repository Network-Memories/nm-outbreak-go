<p align="center"><img src="../../assets/banner.svg" alt="NM-OUTBREAK-GO — Resident Evil Outbreak" width="100%" /></p>

<p align="center"><a href="../../README.md">English</a> · <a href="README.fr.md">Français</a> · <a href="README.de.md">Deutsch</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a> · <strong>Русский</strong> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.zh-CN.md">简体中文</a></p>

<h3 align="center">Самостоятельный игровой сервер · Resident Evil Outbreak (File #1 и File #2)</h3>

<p align="center">Go 1.24 · Docker Compose · SNAP (US/PAL) и KDDI (JP)</p>

[Совместимость](../../docs/COMPATIBILITY.md) · [Установка](../../docs/INSTALL.md) · [Архитектура](../../docs/ARCHITECTURE.md) · [Протокол](../../docs/PROTOCOL.md) · [Лицензия](../../LICENSE)

> **В разработке.** Этот репозиторий постоянно улучшается по мере проведения измерений протокола и проверок в лаборатории. Проверки автономной установки отслеживаются отдельно. Это не означает полную поддержку всех игр, режимов, регионов или типов NAT.

Единый сервер на Go, поддерживающий оригинальные западные (**SNAP**) и японские (**KDDI**) клиенты PS2 без патчей клиента.

## Подтверждённые и проверенные сценарии

| Игра | Платформа | Статус |
| --- | --- | --- |
| Resident Evil Outbreak (File #1) | PS2 (NTSC-J ↔ NTSC-U) | Смешанный запуск JAP → US и игра подтверждены в лаборатории |
| Resident Evil Outbreak: File #2 | PS2 (PAL / NTSC-U ↔ NTSC-J) | Смешанное лобби, запуск и игра подтверждены в лаборатории |

<details><summary><strong>Другие проверенные и частичные сценарии</strong></summary>

| Игра | Платформа | Статус |
| --- | --- | --- |
| Resident Evil Outbreak (File #1) | PS2 (хост NTSC-U → NTSC-J) | Обратный вход и `StatWord` подтверждены; запуск с хоста US ожидает проверки |
| Resident Evil Outbreak: File #2 | PS2 (3–4 игрока, особые режимы) | `MixedEnginePeers` пока ограничен 2 до замеров с несколькими клиентами |

</details>

**[Полный каталог игр и изданий](../../docs/COMPATIBILITY.md)**

## Запуск локального экземпляра

Запускайте из корня репозитория. Генерируйте собственные секреты; держите базы данных и панели управления закрытыми.

```sh
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

[Прочитайте руководство по установке перед открытием игровых портов.](../../docs/INSTALL.md)

## Структура репозитория

| Путь | Описание |
| --- | --- |
| `cmd/nm-outbreak/` | Точка входа сервера |
| `internal/` | Общее ядро, транспорты SNAP/KDDI, модули File #1 и File #2 |
| `web/` | Панель статуса в реальном времени (`http://localhost:8080`) |
| `tools/` | Локальные скрипты настройки и проверки протокола |
| `docs/` | Установка, архитектура, совместимость и заметки по протоколу |

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
