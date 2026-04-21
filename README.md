# ⏱ TimeSync

> **Report your hours once. Sync to every time-reporting system automatically.**

TimeSync is a self-hosted web application that lets you enter a week of time once in a clean browser UI and synchronise it into multiple time-reporting back-end systems (Maconomy, SAP Fieldglass, and any future system you plug in) with a single click.

---

## Table of Contents

- [Features](#features)
- [Architecture Overview](#architecture-overview)
- [Supported Systems](#supported-systems)
- [Quick Start (Docker)](#quick-start-docker)
- [Development Setup](#development-setup)
- [Adding a New Time-Reporting System](#adding-a-new-time-reporting-system)
- [Configuration Reference](#configuration-reference)
- [Security Design](#security-design)
- [Project Structure](#project-structure)
- [API Reference](#api-reference)

---

## Features

- 🗓 **Weekly time entry** – enter Mon–Sun hours for multiple rows/projects in one table
- 🔀 **Multi-system sync** – map each row to one or more back-end system rows and sync simultaneously
- ⚡ **Async sync with live status** – the Sync button dispatches background goroutines; the UI updates in real time via Server-Sent Events (golden-amber chip → green "Synced")
- 🧩 **Modular adapter system** – adding a new time-reporting system requires creating one Go file and registering one line
- 🔒 **Secure by default** – API tokens never leave the server, full OWASP Top-10 mitigations, optional TLS
- 🐳 **Docker-first** – single `docker compose up` starts everything; runs equally well on a laptop or a cloud server

---

## Architecture Overview

```
Browser (React SPA)
  │
  │  HTTP/S + X-Session-ID header
  ▼
nginx (frontend container)
  │  /api/v1/*  → proxy_pass
  ▼
Go backend (API server)
  ├── Session Store (in-memory, keyed by UUID)
  ├── Adapter Registry
  │     ├── Maconomy adapter
  │     ├── Fieldglass adapter
  │     └── (your adapter here)
  └── Sync Engine (To be Decided)
```

**Key design choices:**

| Concern | Decision | Reason |
|---------|----------|--------|
| Session identity | UUID in `sessionStorage`, sent as `X-Session-ID` | No cookies → no CSRF; cleared on tab close |
| Token storage | Backend RAM only | Tokens never touch disk, never leave the server |
| Multi-user isolation | Each UUID is a separate in-memory session | Users cannot see each other's data |
| Async sync | One goroutine per system per sync call | Slow systems don't block the user or each other |
| Real-time status | Server-Sent Events (`/api/sync/status`) | Uni-directional; works through most proxies |

---

## Supported Systems

| System | Auth Method | Status |
|--------|-------------|--------|
| **Deltek Maconomy** | Proprietary `X-Reconnect` token | ✅ Included |
| **SAP Fieldglass** | OAuth 2.0 Client Credentials | 🚧 Work in progress |
| _Your system_ | Any | See [Adding a New System](#adding-a-new-time-reporting-system) |

---

## Quick Start (Docker)

**Prerequisites:** Docker ≥ 24, Docker Compose v2.

```bash
# 1. Clone the repository
git clone https://github.com/aiqueneldar/time-sync.git
cd time-sync

# 2. Create your environment file
cp .env.example .env
# Edit .env if you want to pre-configure Maconomy URL or change ports

# 3. Build and start
docker compose up -d --build

# 4. Open the app
open http://localhost
```

Logs:
```bash
docker compose logs -f
```

Stop:
```bash
docker compose down
```

### HTTPS / Internet Deployment

The recommended approach for internet-facing deployments is to put a TLS-terminating reverse proxy (e.g. **Caddy** or **Traefik**) in front of the nginx container:

```bash
# Option A – Caddy (auto TLS via Let's Encrypt)
# Add a Caddyfile alongside docker-compose.yml:
#   timesync.example.com {
#     reverse_proxy frontend:80
#   }
```

---

## Development Setup

### Backend (Go)

```bash
cd backend

# Install dependencies
go mod download

# Run with live reload (uses 'air' watcher – install once with: go install github.com/cosmtrek/air@latest) OPTIONAL
air

# Or run directly:
cd backend && go run ./cmd/server
```

The backend starts on `http://localhost:8080`.

### Frontend (Node / Vite)

```bash
cd frontend

# Install dependencies
npm install

# Start dev server (proxies /api/* to localhost:8080 automatically)
npm run dev
```

The frontend starts on `http://localhost:5173`.

### Running both together (development)

Open two terminal windows:

```
Terminal 1:  cd backend && go run ./cmd/server
Terminal 2:  cd frontend && npm run dev
```

Then open `http://localhost:5173`.

---

## Adding a New Time-Reporting System

TimeSync is designed so that adding a new system touches exactly **two files**:

### Step 1 – Create the adapter package

```
backend/internal/adapters/mysystem/adapter.go
```

Implement the `adapters.Adapter` interface (7 methods):

```go
package mysystem

import (
    "context"
    "github.com/timesync/backend/internal/models"
)

type Adapter struct{ /* your fields */ }

func New() *Adapter { return &Adapter{} }

func (a *Adapter) SystemID() string    { return "mysystem" }
func (a *Adapter) SystemName() string  { return "My System" }
func (a *Adapter) Description() string { return "Short description shown in UI" }

func (a *Adapter) Authenticate(c *gin.Context, fields map[string]string) (uuid.UUID, error) {
    // Exchange credentials for a token.
    // Store the token in the Auth struct and return Auth structs UUID in the in-memory store.
    // NEVER store it anywhere else.
}

func (a *Adapter) ValidateAuth(ctx context.Context, auth *models.AuthResult) (bool, error) {
    // Lightweight check that the token is still valid.
}

func (a *Adapter) GetAvailableRows(ctx context.Context, auth *models.AuthResult) ([]models.SystemRow, error) {
    // Fetch the list of rows the user can book time against.
}

func (a *Adapter) SubmitEntries(ctx context.Context, auth *models.AuthResult, entries []models.SystemTimeEntry) (*models.SubmitResult, error) {
    // Convert SystemTimeEntry → your API format and submit.
}
```

### Step 2 – Register the adapter

In `backend/cmd/server/main.go`, add one line:

```go
registry.Register(mysystem.New())
```

That's it. The frontend automatically picks up the new system from `GET /api/v1/systems`.

Optionally add an icon in `frontend/src/components/SystemSelector/SystemSelector.jsx`:

```js
const SYSTEM_ICONS = {
  // ... existing icons ...
  mysystem: <MySystemIcon />,
};
```

---

## Configuration Reference

All configuration is via environment variables. See `.env.example` for the full list.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend listen port |
| `ALLOWED_ORIGINS` | `http://localhost,...` | Comma-separated CORS allow-list |
| `FRONTEND_PORT` | `80` | Host port for the nginx container |

---

## Security Design

TimeSync tries to follow OWASP Top 10 guidelines throughout.

### Token / credential handling

```
Browser                   nginx                  Go backend
  │                          │                       │
  │── POST /api/auth/xxx ───►│── proxy_pass ────────►│
  │   body: {user, pass}     │                       │── adapter.Authenticate()
  │                          │                       │      ↓
  │◄── {authenticated:true} ─│◄─────────────────────│   token stored in RAM
  │   (NO token in response) │                       │   session[sessionID].Auth
```

- Credentials are transmitted to the backend and immediately discarded after the token exchange
- Tokens are stored only in the server's in-memory session store, keyed by a random UUID
- The UUID is held in `sessionStorage` — it expires when the browser tab closes
- The server never writes tokens to disk, logs, or any persistent store

### OWASP mitigations

| # | Risk | Mitigation |
|---|------|-----------|
| A01 | Broken Access Control | Session UUID required on every API call; separate in-memory store per session |
| A02 | Cryptographic Failures | TLS support; tokens in RAM only; crypto.randomUUID() for session IDs |
| A03 | Injection | All inputs validated; JSON-only API; no SQL/shell execution |
| A04 | Insecure Design | Secrets never leave the server; architecture reviewed against threat model |
| A05 | Security Misconfiguration | Full security header suite; non-root Docker containers; scratch image |
| A06 | Vulnerable Components | Minimal dependencies; scratch Docker image; `go mod` pinned versions |
| A07 | Auth Failures | 24-hour session TTL; token expiry + refresh; logout endpoint |
| A08 | Software Integrity | Multi-stage Docker builds; dependency checksums via `go.sum` / `package-lock.json` |
| A09 | Logging Failures | Structured logging (no sensitive data); error detail never leaked to client |
| A10 | SSRF | Adapter HTTP clients use user-supplied URLs — add URL allowlisting for production |

### CSRF protection

TimeSync uses the **custom header pattern**: every mutating request must carry `X-Session-ID`. Browsers cannot add custom headers to cross-origin requests automatically, making CSRF attacks impossible without an additional CSRF token.

---

## Project Structure

```
timesync/
├── .env.example               # Configuration template
├── .gitignore
├── docker-compose.yml         # Full stack orchestration
│
├── backend/
│   ├── Dockerfile             # Multi-stage Go build → scratch image
│   ├── go.mod
│   ├── README.md              # Backend-specific docs
│   ├── cmd/
│   │   └── server/
│   │       └── main.go        # Entry point: wires all components
│   └── internal/
│       ├── config/            # Environment-based configuration
│       ├── models/            # Shared data types (the API contract)
│       ├── adapters/
│       │   ├── interface.go   # The Adapter interface (add systems here)
│       │   ├── registry.go    # Runtime adapter registry
│       │   ├── maconomy/      # Deltek Maconomy adapter
│       │   └── fieldglass/    # SAP Fieldglass adapter
│       ├── store/             # In-memory session store with TTL eviction
│       ├── sync/              # Async sync engine
│       └── api/
│           ├── handlers/      # HTTP handler functions
│           └── middleware/    # Security, CORS, session validation
│
└── frontend/
    ├── Dockerfile             # Node build → nginx
    ├── nginx.conf             # Hardened nginx config with SSE support
    ├── package.json
    ├── vite.config.js         # Dev proxy + build config
    ├── tailwind.config.js
    ├── README.md              # Frontend-specific docs
    └── src/
        ├── main.jsx           # React entry point
        ├── App.jsx            # Root component / page router
        ├── index.css          # Tailwind + global styles
        ├── context/
        │   └── AppContext.jsx # Global state (useReducer)
        ├── services/
        │   ├── api.js         # Typed API client
        │   └── tokenStore.js  # Session ID in sessionStorage
        ├── pages/
        │   ├── SetupPage.jsx  # System selection + auth flow
        │   └── TimesheetPage.jsx # Main time entry + sync UI
        └── components/
            ├── SystemSelector/  # System card grid
            ├── AuthModal/       # Dynamic credential form
            ├── TimeTable/       # Week entry grid
            ├── RowMapper/       # Backend row mapping panel
            └── SyncStatus/      # Real-time status chips
```

---

## API Reference

See OpenAPI specification here until such a time it gets incorporated in Gin and rendered as part of the project.
https://app.swaggerhub.com/apis/aiquen-private/time-sync/v1.1

---

## License

GPL-V3
