# Claude Code Plugin Marketplace Aggregator — Design

**Date**: 2026-06-11
**Status**: Draft (awaiting user review)
**Author**: brainstorming session output

## 1. Purpose

Build a Go HTTP service that aggregates multiple Claude Code plugin
marketplaces (Git repos and HTTP endpoints) into a single, stable URL that team
members can point their Claude Code at. The service ships a web UI for adding,
editing, removing, and refreshing the upstream sources.

**Use case**: team-internal multi-source aggregation, deployed on the
intranet without authentication.

## 2. Goals and Non-Goals

### Goals
- Configure one aggregated marketplace URL for the whole team.
- Support two source types: Git repositories and HTTP endpoints that serve a
  `marketplace.json`.
- Provide a web UI and a JSON admin API for managing sources
  (add / edit / delete / refresh).
- Mirror upstream plugin files locally so that consumers never need direct
  access to upstream repositories.
- Run as a single Go binary with no external runtime dependencies (no DB, no
  message broker, no system git binary).

### Non-Goals
- Public-internet multi-tenant hosting.
- Authentication / authorization (deployment is trusted intranet).
- Automatic scheduled refresh (manual refresh only).
- Plugin-level authorization or rate-limiting.
- Caching layer shared across multiple service instances.

## 3. Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Go single binary                         │
│                                                             │
│  ┌─────────┐   ┌─────────┐   ┌──────────┐   ┌───────────┐  │
│  │ Config  │──▶│ Fetcher │──▶│  Store   │──▶│Aggregator │  │
│  │ (YAML)  │   │ (Git/   │   │ (local   │   │ (merge +  │  │
│  └─────────┘   │  HTTP)  │   │  disk)   │   │  rewrite) │  │
│       │        └─────────┘   └──────────┘   └─────┬─────┘  │
│       │                                          │         │
│       ▼                                          ▼         │
│  ┌─────────┐                              ┌──────────┐     │
│  │ Admin   │◀───── HTML UI + JSON API ────▶│ Reader   │     │
│  │ API     │                              │ (static  │     │
│  │         │                              │  files)  │     │
│  └─────────┘                              └──────────┘     │
└─────────────────────────────────────────────────────────────┘
            ▲                                      ▲
            │ HTTP (admin)                         │ HTTP (read)
            │                                      │
       admins                              Claude Code clients
```

### Core principles
- Single binary deployment. No external dependencies beyond local disk and
  the YAML config file.
- Config is read at startup. Runtime writes go through the admin API, which
  persists back to YAML.
- Source repos live under `data/sources/{name}/`. The aggregated output is
  cached at `data/aggregated/marketplace.json`.
- Write operations (add / delete / refresh) and read operations
  (the marketplace served to Claude Code) use distinct route groups so they
  can be secured or rate-limited independently in the future.

## 4. Data Model and Storage

### 4.1 Configuration file (`config.yaml`)

```yaml
server:
  listen: ":8080"          # HTTP listen address
  data_dir: "./data"       # runtime data directory
  base_url: ""             # optional; if empty, derive from request Host

sources:
  - name: "anthropic-official"
    type: "git"
    url: "https://github.com/anthropics/claude-code.git"
    ref: "main"
    enabled: true

  - name: "team-internal"
    type: "http"
    url: "https://internal.example.com/marketplace.json"
    enabled: true
```

`name` is unique, matches `^[a-z0-9][a-z0-9-]{0,62}$`, and is used as the
URL path segment for plugin file serving.

### 4.2 On-disk layout

```
data/
├── sources/
│   ├── anthropic-official/      # Git shallow clone
│   │   ├── .git/
│   │   └── plugins/...
│   └── team-internal/           # HTTP fetch: marketplace.json + plugin files
│       ├── marketplace.json
│       └── plugins/...
└── aggregated/
    ├── marketplace.json         # merged main entry served to Claude Code
    └── index.json               # internal index: source → local path map
```

### 4.3 In-memory state
- `*Config` — loaded from YAML, persisted on write.
- `map[string]*SourceState` — per source: `enabled`, `lastRefresh`,
  `lastError`, `localPath`.
- `aggregatedCache` — bytes of the merged `marketplace.json`, refreshed after
  a successful `Merge`.

## 5. HTTP API

### 5.1 Read API (consumed by Claude Code)

| Method | Path | Description |
|---|---|---|
| `GET` | `/marketplace.json` | Aggregated marketplace manifest |
| `GET` | `/plugins/{source}/{plugin}/...` | Plugin file path (`.claude-plugin/plugin.json`, `commands/`, `agents/`, etc.) |
| `GET` | `/healthz` | Liveness probe (returns `200 OK`) |

### 5.2 Admin API (JSON, intranet-only)

| Method | Path | Description |
|---|---|---|
| `GET` | `/admin/api/sources` | List all sources (with state: enabled, lastRefresh, lastError) |
| `POST` | `/admin/api/sources` | Add a source (body: name, type, url, ref?) |
| `PUT` | `/admin/api/sources/{name}` | Update a source (full replacement) |
| `DELETE` | `/admin/api/sources/{name}` | Delete a source and its local cache |
| `POST` | `/admin/api/sources/{name}/refresh` | Refresh a single source |
| `POST` | `/admin/api/refresh` | Refresh all enabled sources |
| `GET` | `/admin/api/aggregated` | Preview the current merged `marketplace.json` |

### 5.3 Admin UI pages (HTML)

| Path | Description |
|---|---|
| `GET` | `/admin/` | Home: source list with status, refresh / edit / delete buttons |
| `GET` | `/admin/sources/new` | Add-source form |
| `GET` | `/admin/sources/{name}/edit` | Edit-source form |
| `GET` | `/admin/preview` | Preview merged `marketplace.json` |

### 5.4 Plugin path rewriting

A source's `marketplace.json` may list plugin `source` fields as relative
paths (e.g. `./plugins/foo`). During aggregation these are rewritten to:

```
{base_url}/plugins/{source_name}/{plugin_relative_path}
```

`base_url` is the `server.base_url` if set, otherwise the request's
`Host` header. Claude Code then fetches the plugin file from the
aggregator, not from the upstream repo.

## 6. Core Modules

```
cmd/agentsstore/main.go         # entrypoint: wire dependencies, start HTTP
internal/
  config/                       # YAML load/save (concurrency-safe)
  source/
    source.go                   # Source interface
    git.go                      # GitSource: shallow clone/pull via go-git
    http.go                     # HTTPSource: fetch marketplace.json + plugin files
  store/
    store.go                    # filesystem read/write (data/sources/{name}/)
  aggregator/
    aggregator.go               # merge + path rewrite + write aggregated JSON
  server/
    server.go                   # gin engine + route registration
    reader.go                   # read-API handlers
    admin.go                    # admin API handlers
    ui.go                       # HTML template rendering
  ui/
    templates/                  # *.html (go:embed)
    static/                     # *.css, *.js (go:embed)
```

### Key interfaces

```go
type Source interface {
    Name() string
    Type() string  // "git" | "http"
    Fetch(ctx context.Context, destDir string) error
    // After Fetch, destDir/marketplace.json MUST exist.
}

type Aggregator interface {
    Refresh(ctx context.Context, srcs []Source) (*Marketplace, error)
}
```

### Data flows

- **Add source**: `POST /admin/api/sources` → validate (unique name, valid
  URL) → persist to YAML → enqueue an initial async fetch.
- **Refresh source**: `POST .../refresh` → call `source.Fetch(ctx, destDir)`
  → on success call `aggregator.Merge(sourceName, destDir)` → rewrite
  `aggregated/marketplace.json`.
- **Read aggregated**: `GET /marketplace.json` → serve cached
  `data/aggregated/marketplace.json`. If the file does not exist, respond
  with `503 Service Unavailable` and a JSON error body explaining that no
  source has been refreshed yet.
- **Read plugin file**: `GET /plugins/{src}/{plugin}/...` → validate that
  `src` exists and is enabled → resolve the path under
  `data/sources/{src}/` and stream the file.

### Error handling
- Fetch failure: do not update `aggregated`; record `lastError` (visible in
  the admin UI).
- Disabled source: skipped during refresh, omitted from aggregation.
- Config write failure: API returns `500`; the in-memory state is preserved
  and will be persisted on the next successful write.
- Path-traversal attempts in `/plugins/{src}/...` are rejected: the resolved
  path must remain under `data/sources/{src}/` (verified with
  `filepath.Clean` + prefix check).

## 7. Dependencies

| Module | Purpose |
|---|---|
| `github.com/gin-gonic/gin` | HTTP routing |
| `github.com/go-git/go-git/v5` | Pure-Go Git clone/pull (no system git required) |
| `gopkg.in/yaml.v3` | YAML config |
| `github.com/stretchr/testify` | Test assertions |

Everything else uses the Go standard library.

## 8. Testing Strategy

| Level | Tooling | Coverage |
|---|---|---|
| Unit | `testing` + `testify` | config load/save, aggregator merge logic, path rewrite, path-traversal guard |
| Integration | `httptest` | reader and admin handlers end-to-end |
| End-to-end | A local git repo fixture + a local HTTP upstream fixture; run the full add → refresh → read flow | real source fetching |

CI runs `go test ./...`, `go vet ./...`, and `gofmt -l`. No Docker required.

## 9. Implementation Phases

1. **M0 — skeleton**: `go mod init`, main starts an empty HTTP server, `/healthz` returns 200.
2. **M1 — config**: YAML load/save, concurrency-safe.
3. **M2 — source abstraction + GitSource**: shallow clone a real repo via go-git.
4. **M3 — HTTPSource**: fetch `marketplace.json` + download plugin files.
5. **M4 — aggregator**: merge, path rewrite, write `aggregated/marketplace.json`.
6. **M5 — read API**: `/marketplace.json` + `/plugins/{src}/...` with path-traversal guard.
7. **M6 — admin API**: CRUD + refresh endpoints.
8. **M7 — admin UI**: html/template pages for list, forms, preview.
9. **M8 — tests + docs**: complete test coverage, README, optional Dockerfile.

Each phase ends with `go test ./...` green.

## 10. Open Questions

None at design time. Outstanding decisions during implementation:
- exact Gin version pinning
- how to surface partial aggregation when some sources fail (current plan:
  successful sources still appear in the merged JSON, with an `errors`
  field listing failed sources)
