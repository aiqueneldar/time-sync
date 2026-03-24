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
  │  /api/*  → proxy_pass
  ▼
Go backend (API server)
  ├── Session Store (in-memory, keyed by UUID)
  ├── Adapter Registry
  │     ├── Maconomy adapter
  │     ├── Fieldglass adapter
  │     └── (your adapter here)
  └── Sync Engine (goroutine per system)
        │  SSE stream
        └──────────────────→ Browser status chips
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
| **SAP Fieldglass** | OAuth 2.0 Client Credentials | ✅ Included |
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

# Option B – Set TLS_ENABLED=true in .env and mount certs directly into the backend.
# See .env.example for the exact variables.
```

---

## Development Setup

### Backend (Go)

```bash
cd backend

# Install dependencies
go mod download

# Run with live reload (uses 'air' watcher – install once with: go install github.com/cosmtrek/air@latest)
air

# Or run directly:
go run ./cmd/server

# Run tests
go test ./...
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

func (a *Adapter) AuthFields() []models.AuthField {
    return []models.AuthField{
        { Key: "apiUrl", Label: "API URL", Type: models.AuthFieldTypeURL, Required: true },
        { Key: "apiKey", Label: "API Key", Type: models.AuthFieldTypePassword, Required: true },
    }
}

func (a *Adapter) Authenticate(ctx context.Context, fields map[string]string) (*models.AuthResult, error) {
    // Exchange credentials for a token.
    // Store the token in the returned AuthResult.
    // NEVER store it anywhere else.
}

func (a *Adapter) RefreshAuth(ctx context.Context, auth *models.AuthResult) (*models.AuthResult, error) {
    // Return a fresh AuthResult, or an error if refresh is unsupported.
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

That's it. The frontend automatically picks up the new system from `GET /api/systems` and renders the login form using the `authFields` you declared.

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
| `TLS_ENABLED` | `false` | Enable HTTPS on the backend |
| `TLS_CERT_FILE` | _(empty)_ | Path to TLS certificate PEM |
| `TLS_KEY_FILE` | _(empty)_ | Path to TLS private key PEM |
| `ALLOWED_ORIGINS` | `http://localhost,...` | Comma-separated CORS allow-list |
| `MACONOMY_BASE_URL` | _(empty)_ | Pre-fill Maconomy URL for all users |
| `MACONOMY_COMPANY` | _(empty)_ | Pre-fill Maconomy company name |
| `FRONTEND_PORT` | `80` | Host port for the nginx container |

---

## Security Design

TimeSync follows OWASP Top 10 guidelines throughout.

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
│       ├── session/           # In-memory session store with TTL eviction
│       ├── sync/              # Async sync engine
│       └── api/
│           ├── router.go      # Route registration
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

All endpoints require the `X-Session-ID` header (UUID v4) except `GET /api/systems` and `GET /health`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/systems` | List all registered systems |
| `GET` | `/api/auth/{systemId}` | Get auth status for a system |
| `POST` | `/api/auth/{systemId}` | Authenticate with credentials |
| `DELETE` | `/api/auth/{systemId}` | Log out (remove server-side token) |
| `GET` | `/api/timesheets/{systemId}/rows` | Fetch bookable rows from a system |
| `POST` | `/api/sync` | Start async sync (returns 202 Accepted) |
| `GET` | `/api/sync/status` | **SSE stream** of real-time sync updates |
| `GET` | `/api/sync/status/poll` | One-shot sync status snapshot (SSE fallback) |
| `GET` | `/health` | Health check |

---

## License

MIT
