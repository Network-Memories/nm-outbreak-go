# Installation & Configuration

[Français](#français) | [English](#english)

---

## Français

### Prérequis
- Docker & Docker Compose (ou Go 1.24+ pour compiler localement)

### Lancement avec Docker Compose
```bash
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

### Variables d'environnement principales
| Variable | Défaut | Description |
|---|---|---|
| `NM_IDENTITY_MODE` | `standalone` | Mode d'authentification (`standalone` pour usage autonome) |
| `NM_OUTBREAK_BIND_HOST` | `0.0.0.0` | Adresse d'écoute locale |
| `NM_OUTBREAK_ADVERTISE_HOST` | `127.0.0.1` | IPv4 annoncée aux clients lors du bootstrap SNAP |
| `NM_OUTBREAK_STATE_ADDR` | `0.0.0.0:18090` | API JSON d'état (`/state`) |
| `NM_OUTBREAK_WEB_ADDR` | `0.0.0.0:18091` | Portail web natif |

---

## English

### Prerequisites
- Docker & Docker Compose (or Go 1.24+ for local builds)

### Run with Docker Compose
```bash
OUTBREAK_HOST=127.0.0.1 docker compose up --build -d
```

### Main Environment Variables
| Variable | Default | Description |
|---|---|---|
| `NM_IDENTITY_MODE` | `standalone` | Authentication mode (`standalone` for self-hosted use) |
| `NM_OUTBREAK_BIND_HOST` | `0.0.0.0` | Local bind address |
| `NM_OUTBREAK_ADVERTISE_HOST` | `127.0.0.1` | IPv4 advertised to clients during SNAP bootstrap |
| `NM_OUTBREAK_STATE_ADDR` | `0.0.0.0:18090` | JSON state API (`/state`) |
| `NM_OUTBREAK_WEB_ADDR` | `0.0.0.0:18091` | Native web portal |
