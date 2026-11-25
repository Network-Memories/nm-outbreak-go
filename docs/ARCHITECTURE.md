# Architecture & Modules

[Français](#français) | [English](#english)

---

## Français

Le serveur suit une architecture **socle commun (`internal/core`) → transports (`internal/snap`, `internal/kddi`) → modules de jeu (`internal/modules/file1`, `internal/modules/file2`)** :

- `internal/core` : moteur d'état partagé (sessions, salles, présence, événements, lancement mixte, relais moteur).
- `internal/snap` : transport UDP fiable SNAP (bootstrap `9090`, File #1 `9091`, File #2 `9092`).
- `internal/kddi` : transport TCP KDDI (lobby `8200`/`8300`, jeu `8590`/`8690`).
- `internal/modules/file1` : règles, `StatWord`, profils (`208`/`240` octets) et scénarios File #1 (`0..4` ↔ `1..5`).
- `internal/modules/file2` : règles (`nightmare`, `infinity`, `friendly_fire`), chat, journaux et scénarios File #2.
- `internal/httpapi` : API JSON `/state` consommée par `web/index.html`.

---

## English

The server follows a **shared core (`internal/core`) → transports (`internal/snap`, `internal/kddi`) → game modules (`internal/modules/file1`, `internal/modules/file2`)** architecture:

- `internal/core`: shared state engine (sessions, rooms, presence, events, mixed launch, engine relay).
- `internal/snap`: reliable SNAP UDP transport (bootstrap `9090`, File #1 `9091`, File #2 `9092`).
- `internal/kddi`: KDDI TCP transport (lobby `8200`/`8300`, game `8590`/`8690`).
- `internal/modules/file1`: rules, `StatWord`, profiles (`208`/`240` bytes), and File #1 scenarios (`0..4` ↔ `1..5`).
- `internal/modules/file2`: rules (`nightmare`, `infinity`, `friendly_fire`), chat, journals, and File #2 scenarios.
- `internal/httpapi`: JSON `/state` API consumed by `web/index.html`.
