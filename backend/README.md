# TimeSync Backend

Go HTTP API server for the TimeSync application.

## Technology Stack

| Layer | Choice | Reason |
|-------|--------|--------|
| Language | Go 1.21 | Excellent concurrency model, low memory footprint, fast compile |
| HTTP | `net/http` (stdlib) | No unnecessary dependencies; easy to audit |
| Session store | `sync.Map` / `sync.RWMutex` | In-memory, zero-dependency, safe for concurrent use |
| Async sync | `goroutines` + `channels` | Native Go concurrency; per-system isolation |
| Real-time updates | Server-Sent Events | Simple, HTTP-native, works through most proxies |

## Running Locally

```bash
# From the /backend directory

# Download dependencies
go mod download

# Run (hot-reload with Air)
go install github.com/cosmtrek/air@latest
air

# Run directly
go run ./cmd/server

# Build binary
go build -o bin/timesync-server ./cmd/server
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Listen port |
| `TLS_ENABLED` | `false` | Enable TLS |
| `TLS_CERT_FILE` | – | PEM cert path |
| `TLS_KEY_FILE` | – | PEM key path |
| `ALLOWED_ORIGINS` | `http://localhost:5173,...` | CORS origins |
| `MACONOMY_BASE_URL` | – | Default Maconomy URL |
| `MACONOMY_COMPANY` | – | Default Maconomy company |

## Running Tests

```bash
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Adapter Interface

Every time-reporting system implements seven methods:

```go
type Adapter interface {
    SystemID() string
    SystemName() string
    Description() string
    AuthFields() []models.AuthField
    Authenticate(ctx context.Context, fields map[string]string) (*models.AuthResult, error)
    RefreshAuth(ctx context.Context, auth *models.AuthResult) (*models.AuthResult, error)
    ValidateAuth(ctx context.Context, auth *models.AuthResult) (bool, error)
    GetAvailableRows(ctx context.Context, auth *models.AuthResult) ([]models.SystemRow, error)
    SubmitEntries(ctx context.Context, auth *models.AuthResult, entries []models.SystemTimeEntry) (*models.SubmitResult, error)
}
```

See `internal/adapters/interface.go` for full documentation.

## Security Notes

- Tokens are stored **only in RAM** – never written to disk or logs
- Session IDs are UUIDs validated by the `RequireSession` middleware
- All responses carry OWASP-recommended security headers
- Request bodies are capped at 64 KB (auth) / 1 MB (sync) to prevent DoS
- The Docker image runs as UID 65532 (nobody) from a `scratch` base image
- Server timeouts are set to resist Slowloris-style attacks

## Code Organisation

```
cmd/server/main.go            ← Wire-up and startup
internal/
  config/config.go            ← Env-var config
  models/models.go            ← All shared types
  adapters/
    interface.go              ← Adapter contract
    registry.go               ← Runtime registry
    maconomy/adapter.go       ← Maconomy X-Reconnect auth
    fieldglass/adapter.go     ← Fieldglass OAuth2
  session/store.go            ← In-memory session store + TTL eviction
  sync/engine.go              ← Async sync dispatcher
  api/
    router.go                 ← Route registration
    handlers/
      auth.go                 ← POST/GET/DELETE /api/auth/{systemId}
      systems.go              ← GET /api/systems
      timesheets.go           ← GET /api/timesheets/{systemId}/rows
      sync.go                 ← POST /api/sync + SSE /api/sync/status
    middleware/
      security.go             ← CORS, HSTS, CSP, session validation
```
