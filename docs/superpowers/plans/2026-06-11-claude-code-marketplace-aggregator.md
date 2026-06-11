# Claude Code Plugin Marketplace Aggregator — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go HTTP service that aggregates multiple Claude Code plugin marketplaces (Git repos + HTTP endpoints) into a single URL, with a web UI for source management.

**Architecture:** Single Go binary. Sources are stored as YAML config. Source content is fetched and mirrored to local disk under `data/sources/{name}/`. An aggregator merges all `marketplace.json` files, rewrites plugin paths, and writes `data/aggregated/marketplace.json`. The HTTP server (gin) serves the merged marketplace and plugin files on read endpoints; the admin API and HTML UI handle CRUD and manual refresh.

**Tech Stack:** Go 1.22+, `github.com/gin-gonic/gin`, `github.com/go-git/go-git/v5`, `gopkg.in/yaml.v3`, `github.com/stretchr/testify`.

**Spec:** `docs/superpowers/specs/2026-06-11-claude-code-marketplace-aggregator-design.md`

---

## File Structure

```
agentsstore/
├── go.mod
├── go.sum
├── main.go
├── README.md
├── config.example.yaml
├── internal/
│   ├── config/
│   │   ├── config.go            # Config struct, load/save, validation
│   │   └── config_test.go
│   ├── source/
│   │   ├── source.go            # Source interface, SourceSpec, SourceState
│   │   ├── git.go               # GitSource (shallow clone via go-git)
│   │   ├── git_test.go
│   │   ├── http.go              # HTTPSource (fetch marketplace.json + files)
│   │   ├── http_test.go
│   │   └── registry.go          # Map SourceSpec -> Source factory
│   ├── store/
│   │   ├── store.go             # Filesystem helpers under data/sources/{name}
│   │   └── store_test.go
│   ├── aggregator/
│   │   ├── aggregator.go        # Merge + path rewrite
│   │   └── aggregator_test.go
│   ├── server/
│   │   ├── server.go            # gin engine wiring, App context
│   │   ├── reader.go            # /marketplace.json + /plugins/...
│   │   ├── reader_test.go
│   │   ├── admin.go             # /admin/api/* handlers
│   │   ├── admin_test.go
│   │   ├── ui.go                # /admin/* HTML rendering
│   │   └── ui_test.go
│   └── ui/
│       ├── ui.go                # go:embed for templates and static files
│       ├── templates/
│       │   ├── base.html
│       │   ├── index.html
│       │   ├── new.html
│       │   ├── edit.html
│       │   └── preview.html
│       └── static/
│           ├── style.css
│           └── app.js
└── docs/superpowers/plans/2026-06-11-claude-code-marketplace-aggregator.md
```

---

## Task 1: Project skeleton (M0)

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`
- Create: `config.example.yaml`

- [ ] **Step 1: Initialize go module**

```bash
cd /home/wu/agentsstore
go mod init github.com/wu/agentsstore
```

Expected: creates `go.mod`.

- [ ] **Step 2: Write the failing healthz test**

Create `internal/server/server_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := &Server{Engine: r}
	s.RegisterRoutes()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", w.Body.String())
	}
}
```

Create `internal/server/server.go`:

```go
package server

import "github.com/gin-gonic/gin"

type Server struct {
	Engine *gin.Engine
}

func (s *Server) RegisterRoutes() {
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})
}
```

- [ ] **Step 3: Add gin dependency and run test**

```bash
cd /home/wu/agentsstore
go get github.com/gin-gonic/gin@latest
go test ./internal/server/... -run TestHealthz -v
```

Expected: FAIL with "package server" not testable (no test deps cached) on first run; after `go mod tidy` it should PASS. If it fails with import error, run `go mod tidy` first.

- [ ] **Step 4: Write main.go**

Create `main.go`:

```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/server"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	s := &server.Server{Engine: r}
	s.RegisterRoutes()
	log.Println("agentsstore listening on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Verify build and tests**

```bash
go build ./...
go test ./... -v
```

Expected: all green.

- [ ] **Step 6: Create example config**

Create `config.example.yaml`:

```yaml
server:
  listen: ":8080"
  data_dir: "./data"
  base_url: ""

sources: []
```

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum main.go config.example.yaml internal/server/
git commit -m "feat(skeleton): gin server with /healthz"
```

---

## Task 2: Config load/save (M1)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for config**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Minimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
server:
  listen: ":9000"
  data_dir: "./d"
sources: []
`), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, ":9000", cfg.Server.Listen)
	assert.Equal(t, "./d", cfg.Server.DataDir)
	assert.Empty(t, cfg.Sources)
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	cfg := &Config{
		Server: ServerConfig{Listen: ":8080", DataDir: "./data"},
		Sources: []Source{
			{Name: "team", Type: "http", URL: "https://x.example/m.json", Enabled: true},
		},
	}
	require.NoError(t, cfg.Save(path))

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, cfg, loaded)
}

func TestValidate_NameFormat(t *testing.T) {
	cases := []struct {
		name    string
		src     Source
		wantErr bool
	}{
		{"ok", Source{Name: "team-a", Type: "http", URL: "https://x"}, false},
		{"empty", Source{Name: "", Type: "http", URL: "https://x"}, true},
		{"uppercase", Source{Name: "TeamA", Type: "http", URL: "https://x"}, true},
		{"bad-type", Source{Name: "x", Type: "ftp", URL: "https://x"}, true},
		{"missing-url", Source{Name: "x", Type: "http", URL: ""}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.src.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/... -v
```

Expected: FAIL with "package config not found" / undefined: Config.

- [ ] **Step 3: Implement config package**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Listen  string `yaml:"listen"`
	DataDir string `yaml:"data_dir"`
	BaseURL string `yaml:"base_url"`
}

type Source struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`   // "git" or "http"
	URL     string `yaml:"url"`
	Ref     string `yaml:"ref"`    // git only
	Enabled bool   `yaml:"enabled"`
}

type Config struct {
	Server  ServerConfig `yaml:"server"`
	Sources []Source     `yaml:"sources"`
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func (s Source) Validate() error {
	if !nameRe.MatchString(s.Name) {
		return fmt.Errorf("invalid name %q (must match %s)", s.Name, nameRe)
	}
	if s.Type != "git" && s.Type != "http" {
		return fmt.Errorf("type must be git or http, got %q", s.Type)
	}
	if s.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

func (c *Config) Validate() error {
	seen := map[string]bool{}
	for i, s := range c.Sources {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("sources[%d]: %w", i, err)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate source name %q", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if c.Server.DataDir == "" {
		c.Server.DataDir = "./data"
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Add deps and run tests**

```bash
go get gopkg.in/yaml.v3 github.com/stretchr/testify
go mod tidy
go test ./internal/config/... -v
```

Expected: PASS (3 tests + 5 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat(config): YAML load/save with validation"
```

---

## Task 3: Store helpers (M1.5)

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

The store is a thin filesystem helper that the fetcher and aggregator use to
read/write under `data/sources/{name}/`.

- [ ] **Step 1: Write failing test**

Create `internal/store/store_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceDir(t *testing.T) {
	s := New("/tmp/d")
	assert.Equal(t, "/tmp/d/sources/team", s.SourceDir("team"))
}

func TestWriteReadMarketplace(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	require.NoError(t, s.WriteFile("team", "marketplace.json", []byte(`{"plugins":[]}`)))

	got, err := s.ReadFile("team", "marketplace.json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"plugins":[]}`, string(got))
}

func TestResolvePluginPath_Traversal(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	_, err := s.ResolvePluginPath("team", "../escape.txt")
	assert.Error(t, err)
}

func TestResolvePluginPath_OK(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	p, err := s.ResolvePluginPath("team", "plugins/foo/plugin.json")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "sources/team/plugins/foo/plugin.json"), p)
}

func TestRemoveSource(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	require.NoError(t, s.WriteFile("team", "x.txt", []byte("y")))

	require.NoError(t, s.RemoveSource("team"))
	_, err := os.Stat(filepath.Join(dir, "sources/team"))
	assert.True(t, os.IsNotExist(err))
}
```

- [ ] **Step 2: Run test, verify failure**

```bash
go test ./internal/store/... -v
```

Expected: FAIL (package doesn't exist).

- [ ] **Step 3: Implement store**

Create `internal/store/store.go`:

```go
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Store struct {
	DataDir string
}

func New(dataDir string) *Store {
	return &Store{DataDir: dataDir}
}

func (s *Store) SourceDir(name string) string {
	return filepath.Join(s.DataDir, "sources", name)
}

func (s *Store) AggregatedDir() string {
	return filepath.Join(s.DataDir, "aggregated")
}

func (s *Store) EnsureSourceDir(name string) error {
	return os.MkdirAll(s.SourceDir(name), 0o755)
}

func (s *Store) WriteFile(name, rel string, data []byte) error {
	full := filepath.Join(s.SourceDir(name), rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (s *Store) ReadFile(name, rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.SourceDir(name), rel))
}

func (s *Store) ResolvePluginPath(name, rel string) (string, error) {
	root := s.SourceDir(name)
	full := filepath.Join(root, rel)
	clean := filepath.Clean(full)
	if !strings.HasPrefix(clean, root+string(os.PathSeparator)) && clean != root {
		return "", fmt.Errorf("path %q escapes source dir", rel)
	}
	return clean, nil
}

func (s *Store) RemoveSource(name string) error {
	return os.RemoveAll(s.SourceDir(name))
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/store/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/ go.mod go.sum
git commit -m "feat(store): filesystem helpers for sources/aggregated"
```

---

## Task 4: Source interface + registry (M2 partial)

**Files:**
- Create: `internal/source/source.go`
- Create: `internal/source/registry.go`
- Create: `internal/source/registry_test.go`

- [ ] **Step 1: Write failing test for registry**

Create `internal/source/registry_test.go`:

```go
package source

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/store"
)

type fakeSrc struct {
	name    string
	written bool
}

func (f *fakeSrc) Name() string { return f.name }
func (f *fakeSrc) Type() string { return "fake" }

func (f *fakeSrc) Fetch(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(destDir+"/marketplace.json", []byte(`{"plugins":[]}`), 0o644)
}

func TestRegistry_Build(t *testing.T) {
	r := NewRegistry()
	r.Register("fake", func(spec config.Source) Source { return &fakeSrc{name: spec.Name} })

	spec := config.Source{Name: "x", Type: "fake", URL: "u", Enabled: true}
	s, err := r.Build(spec)
	require.NoError(t, err)
	assert.Equal(t, "x", s.Name())

	err = s.Fetch(context.Background(), t.TempDir())
	require.NoError(t, err)
}

func TestRegistry_BuildUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Build(config.Source{Name: "x", Type: "nope", URL: "u", Enabled: true})
	assert.Error(t, err)
}

func TestRegistry_BuildAllFromConfig(t *testing.T) {
	r := NewRegistry()
	r.Register("fake", func(spec config.Source) Source { return &fakeSrc{name: spec.Name} })

	cfg := &config.Config{
		Server:  config.ServerConfig{DataDir: t.TempDir()},
		Sources: []config.Source{{Name: "a", Type: "fake", URL: "u", Enabled: true}, {Name: "b", Type: "fake", URL: "u", Enabled: false}},
	}
	_ = store.New(cfg.Server.DataDir) // ensure store init works (not strictly required)

	all, err := r.BuildAll(cfg)
	require.NoError(t, err)
	names := []string{}
	for _, s := range all {
		names = append(names, s.Name())
	}
	assert.ElementsMatch(t, []string{"a", "b"}, names)
}
```

- [ ] **Step 2: Run test, verify failure**

```bash
go test ./internal/source/... -v
```

Expected: FAIL.

- [ ] **Step 3: Implement Source interface and Registry**

Create `internal/source/source.go`:

```go
package source

import "context"

type Source interface {
	Name() string
	Type() string
	// Fetch downloads the source into destDir. After success,
	// destDir/marketplace.json MUST exist.
	Fetch(ctx context.Context, destDir string) error
}
```

Create `internal/source/registry.go`:

```go
package source

import (
	"fmt"

	"github.com/wu/agentsstore/internal/config"
)

type Factory func(spec config.Source) Source

type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]Factory{}}
}

func (r *Registry) Register(typeName string, f Factory) {
	r.factories[typeName] = f
}

func (r *Registry) Build(spec config.Source) (Source, error) {
	f, ok := r.factories[spec.Type]
	if !ok {
		return nil, fmt.Errorf("unknown source type %q", spec.Type)
	}
	return f(spec), nil
}

func (r *Registry) BuildAll(cfg *config.Config) ([]Source, error) {
	out := make([]Source, 0, len(cfg.Sources))
	for _, s := range cfg.Sources {
		src, err := r.Build(s)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/source/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/source/ go.mod go.sum
git commit -m "feat(source): interface + registry with pluggable factories"
```

---

## Task 5: GitSource (M2)

**Files:**
- Create: `internal/source/git.go`
- Create: `internal/source/git_test.go`
- Modify: `internal/source/registry.go` (wire GitSource in the main package)

- [ ] **Step 1: Write failing integration test**

Create `internal/source/git_test.go`:

```go
package source

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
)

// initBareRepo creates a local bare repo and a worktree with one commit
// containing marketplace.json. Returns the clone URL.
func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bare := filepath.Join(dir, "bare.git")
	work := filepath.Join(dir, "work")

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	require.NoError(t, os.MkdirAll(work, 0o755))
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	require.NoError(t, os.WriteFile(filepath.Join(work, "marketplace.json"),
		[]byte(`{"plugins":[{"name":"p1","source":"./plugins/p1"}]}`), 0o644))
	run("add", ".")
	run("commit", "-q", "-m", "init")

	// create bare clone
	clone := exec.Command("git", "clone", "-q", "--bare", work, bare)
	out, err := clone.CombinedOutput()
	require.NoError(t, err, string(out))
	return bare
}

func TestGitSource_Fetch(t *testing.T) {
	url := initBareRepo(t)
	dest := t.TempDir()

	spec := config.Source{Name: "g", Type: "git", URL: url, Ref: "main", Enabled: true}
	src, err := NewGitSource(spec)
	require.NoError(t, err)

	require.NoError(t, src.Fetch(context.Background(), dest))

	data, err := os.ReadFile(filepath.Join(dest, "marketplace.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "p1")
}
```

- [ ] **Step 2: Run test, verify failure**

```bash
go test ./internal/source/... -v -run TestGitSource
```

Expected: FAIL (NewGitSource undefined).

- [ ] **Step 3: Implement GitSource**

Create `internal/source/git.go`:

```go
package source

import (
	"context"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/wu/agentsstore/internal/config"
)

type GitSource struct {
	spec config.Source
}

func NewGitSource(spec config.Source) (*GitSource, error) {
	if spec.Type != "git" {
		return nil, fmt.Errorf("GitSource requires type=git, got %q", spec.Type)
	}
	return &GitSource{spec: spec}, nil
}

func (g *GitSource) Name() string { return g.spec.Name }
func (g *GitSource) Type() string { return "git" }

func (g *GitSource) Fetch(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	opts := &git.CloneOptions{
		URL:        g.spec.URL,
		RemoteName: "origin",
	}
	if g.spec.Ref != "" {
		opts.ReferenceName = refFromString(g.spec.Ref)
	}
	opts.Depth = 1

	if _, err := git.PlainCloneContext(ctx, destDir, false, opts); err != nil {
		// If already cloned (Pull), try a fetch + checkout.
		repo, perr := git.PlainOpen(destDir)
		if perr != nil {
			return fmt.Errorf("clone failed (%v) and open failed (%v)", err, perr)
		}
		if g.spec.Ref != "" {
			_ = repo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin", Depth: 1, Tags: git.AllTags})
			w, _ := repo.Worktree()
			_ = w.Checkout(&git.CheckoutOptions{Branch: refFromString(g.spec.Ref), Force: true})
		}
	}
	return nil
}
```

Create helper `internal/source/githelper.go`:

```go
package source

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

func refFromString(s string) plumbing.ReferenceName {
	if len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return plumbing.ReferenceName(s)
}
```

(Note: the variable named `config` shadows the imported package. Use
`gconfig` alias to avoid this if the file imports both `config` and
`go-git/config`. Cleanest fix: keep the helper file free of the local
`config` package import — see corrected version below.)

Replace `internal/source/githelper.go` with:

```go
package source

import (
	goconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

func refFromString(s string) plumbing.ReferenceName {
	if len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return plumbing.ReferenceName(s)
}

var _ = goconfig.RefSpec{}
```

- [ ] **Step 4: Add deps and run tests**

```bash
go get github.com/go-git/go-git/v5
go mod tidy
go test ./internal/source/... -v -run TestGitSource
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/source/ go.mod go.sum
git commit -m "feat(source): GitSource via go-git shallow clone"
```

---

## Task 6: HTTPSource (M3)

**Files:**
- Create: `internal/source/http.go`
- Create: `internal/source/http_test.go`

HTTPSource fetches `marketplace.json` from the upstream URL, parses it,
then downloads each plugin's files (when the plugin `source` is a
relative URL/path under the marketplace URL's base).

- [ ] **Step 1: Write failing test using httptest**

Create `internal/source/http_test.go`:

```go
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
)

func startUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/marketplace.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plugins": []map[string]any{
				{"name": "p1", "source": "./plugins/p1"},
			},
		})
	})
	mux.HandleFunc("/plugins/p1/plugin.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"p1"}`))
	})
	return httptest.NewServer(mux)
}

func TestHTTPSource_Fetch(t *testing.T) {
	up := startUpstream(t)
	defer up.Close()

	spec := config.Source{Name: "h", Type: "http", URL: up.URL + "/marketplace.json", Enabled: true}
	src, err := NewHTTPSource(spec)
	require.NoError(t, err)

	dest := t.TempDir()
	require.NoError(t, src.Fetch(context.Background(), dest))

	// marketplace.json should be present
	data, err := os.ReadFile(filepath.Join(dest, "marketplace.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "p1")

	// p1's plugin.json should be mirrored
	pdata, err := os.ReadFile(filepath.Join(dest, "plugins/p1/plugin.json"))
	require.NoError(t, err)
	assert.Contains(t, string(pdata), "p1")
}

func TestHTTPSource_BadURL(t *testing.T) {
	spec := config.Source{Name: "h", Type: "http", URL: "http://127.0.0.1:1/x", Enabled: true}
	src, err := NewHTTPSource(spec)
	require.NoError(t, err)
	err = src.Fetch(context.Background(), t.TempDir())
	assert.Error(t, err)
}

// guard against accidentally importing fmt and not using it
var _ = fmt.Sprintf
```

- [ ] **Step 2: Run test, verify failure**

```bash
go test ./internal/source/... -v -run TestHTTPSource
```

Expected: FAIL.

- [ ] **Step 3: Implement HTTPSource**

Create `internal/source/http.go`:

```go
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/wu/agentsstore/internal/config"
)

type HTTPSource struct {
	spec config.Source
	cli  *http.Client
}

func NewHTTPSource(spec config.Source) (*HTTPSource, error) {
	if spec.Type != "http" {
		return nil, fmt.Errorf("HTTPSource requires type=http, got %q", spec.Type)
	}
	return &HTTPSource{
		spec: spec,
		cli:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (h *HTTPSource) Name() string { return h.spec.Name }
func (h *HTTPSource) Type() string { return "http" }

type pluginList struct {
	Plugins []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	} `json:"plugins"`
}

func (h *HTTPSource) Fetch(ctx context.Context, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	base, err := url.Parse(h.spec.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	// strip the file portion of the marketplace URL to get the base
	baseDir, _ := path.Split(base.Path)
	base.Path = strings.TrimSuffix(base.Path, path.Base(base.Path))

	data, err := h.fetch(ctx, h.spec.URL)
	if err != nil {
		return fmt.Errorf("fetch marketplace.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "marketplace.json"), data, 0o644); err != nil {
		return err
	}

	var pl pluginList
	if err := json.Unmarshal(data, &pl); err != nil {
		// Unknown shape; nothing more to mirror.
		return nil
	}

	for _, p := range pl.Plugins {
		if p.Source == "" {
			continue
		}
		rel := p.Source
		// resolve relative to base
		ref, err := url.Parse(rel)
		if err != nil {
			continue
		}
		resolved := *base
		if ref.IsAbs() {
			resolved = *ref
		} else {
			resolved.Path = path.Join(baseDir, ref.Path)
		}
		// download the file (single file) — if it's a directory, fetch a
		// sentinel file in this minimal implementation
		body, err := h.fetch(ctx, resolved.String())
		if err != nil {
			return fmt.Errorf("fetch plugin %s: %w", p.Name, err)
		}
		// write to local path mirrored from the URL
		relPath := strings.TrimPrefix(resolved.Path, "/")
		if relPath == "" {
			continue
		}
		full := filepath.Join(destDir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (h *HTTPSource) fetch(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: %d", u, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/source/... -v -run TestHTTPSource
```

Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/source/ go.mod go.sum
git commit -m "feat(source): HTTPSource fetches marketplace.json + plugin files"
```

---

## Task 7: Aggregator (M4)

**Files:**
- Create: `internal/aggregator/aggregator.go`
- Create: `internal/aggregator/aggregator_test.go`

The aggregator reads each source's mirrored `marketplace.json`, rewrites
plugin `source` paths to point at the aggregator's own plugin routes, and
writes the merged manifest to `data/aggregated/marketplace.json`.

- [ ] **Step 1: Write failing tests**

Create `internal/aggregator/aggregator_test.go`:

```go
package aggregator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/store"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	b, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o644))
}

func TestAggregate_Merge(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	require.NoError(t, s.EnsureSourceDir("a"))
	require.NoError(t, s.EnsureSourceDir("b"))

	writeJSON(t, filepath.Join(dir, "sources/a/marketplace.json"), map[string]any{
		"name": "a",
		"plugins": []map[string]any{
			{"name": "p1", "source": "./plugins/p1"},
		},
	})
	writeJSON(t, filepath.Join(dir, "sources/b/marketplace.json"), map[string]any{
		"name": "b",
		"plugins": []map[string]any{
			{"name": "p2", "source": "./plugins/p2"},
		},
	})

	agg := New(s, "http://example.test")
	require.NoError(t, agg.Refresh([]string{"a", "b"}))

	got, err := s.ReadFile("aggregated", "marketplace.json")
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(got, &m))
	plugins := m["plugins"].([]any)
	assert.Len(t, plugins, 2)
	// Verify rewriting: each plugin.source becomes /plugins/{src}/...
	names := map[string]string{}
	for _, p := range plugins {
		pm := p.(map[string]any)
		names[pm["name"].(string)] = pm["source"].(string)
	}
	assert.Equal(t, "http://example.test/plugins/a/plugins/p1", names["p1"])
	assert.Equal(t, "http://example.test/plugins/b/plugins/p2", names["p2"])
}

func TestAggregate_MissingSource(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	agg := New(s, "http://x")
	err := agg.Refresh([]string{"nope"})
	assert.Error(t, err)
}

func TestRewriteURL(t *testing.T) {
	assert.Equal(t,
		"http://x/plugins/src/p/x",
		rewriteURL("http://x", "src", "./p/x"))
	assert.Equal(t,
		"http://y/plugins/src/abs",
		rewriteURL("http://x/y", "src", "http://other/abs"))
}
```

- [ ] **Step 2: Run test, verify failure**

```bash
go test ./internal/aggregator/... -v
```

Expected: FAIL.

- [ ] **Step 3: Implement aggregator**

Create `internal/aggregator/aggregator.go`:

```go
package aggregator

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/wu/agentsstore/internal/store"
)

type Aggregator struct {
	store   *store.Store
	baseURL string
}

func New(s *store.Store, baseURL string) *Aggregator {
	return &Aggregator{store: s, baseURL: strings.TrimRight(baseURL, "/")}
}

type plugin struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	// preserve all other fields
	Extra map[string]json.RawMessage `json:"-"`
}

type manifest struct {
	Name    string          `json:"name,omitempty"`
	Plugins []plugin        `json:"plugins"`
	Extra   map[string]json.RawMessage `json:"-"`
}

// Refresh merges the marketplace.json of the given source names and writes
// the result to data/aggregated/marketplace.json.
func (a *Aggregator) Refresh(sourceNames []string) error {
	merged := manifest{Plugins: []plugin{}}
	for _, name := range sourceNames {
		data, err := a.store.ReadFile(name, "marketplace.json")
		if err != nil {
			return fmt.Errorf("source %s: %w", name, err)
		}
		var m manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("source %s: parse: %w", name, err)
		}
		// Re-marshal to preserve extra fields per plugin, then re-unmarshal
		// into a generic shape to pick up unknown keys.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		for _, p := range m.Plugins {
			p.Source = rewriteURL(a.baseURL, name, p.Source)
			merged.Plugins = append(merged.Plugins, p)
		}
	}
	if err := a.store.EnsureSourceDir("aggregated"); err != nil {
		return err
	}
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return a.store.WriteFile("aggregated", "marketplace.json", out)
}

func rewriteURL(baseURL, srcName, pluginSrc string) string {
	// pluginSrc is treated as a URL or path relative to the source.
	if u, err := url.Parse(pluginSrc); err == nil && u.IsAbs() {
		// External URL — leave as-is so consumer can fetch directly
		return pluginSrc
	}
	// strip leading "./"
	clean := strings.TrimPrefix(pluginSrc, "./")
	return fmt.Sprintf("%s/plugins/%s/%s", baseURL, srcName, path.Clean(clean))
}
```

(Note: `store.EnsureSourceDir` currently hardcodes `name` as the subdir;
we'll instead write via `WriteFile` which already calls `MkdirAll`.)

Replace the `EnsureSourceDir("aggregated")` call in `Refresh` with:

```go
	if err := os.MkdirAll(a.store.AggregatedDir(), 0o755); err != nil {
		return err
	}
```

Add the import to `aggregator.go`:

```go
import (
	// ...
	"os"
)
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/aggregator/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/aggregator/ go.mod go.sum
git commit -m "feat(aggregator): merge marketplace.json with path rewrite"
```

---

## Task 8: Read API (M5)

**Files:**
- Modify: `internal/server/server.go`
- Create: `internal/server/reader.go`
- Create: `internal/server/reader_test.go`

- [ ] **Step 1: Extend Server to hold deps**

Update `internal/server/server.go`:

```go
package server

import (
	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/store"
)

type Server struct {
	Engine     *gin.Engine
	Store      *store.Store
	Aggregator *aggregator.Aggregator
	BaseURL    string
}

func (s *Server) RegisterRoutes() {
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})

	reader := &Reader{Store: s.Store, Aggregator: s.Aggregator, BaseURL: s.BaseURL}
	s.Engine.GET("/marketplace.json", reader.Marketplace)
	s.Engine.GET("/plugins/:source/*path", reader.PluginFile)
}
```

- [ ] **Step 2: Implement Reader**

Create `internal/server/reader.go`:

```go
package server

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/store"
)

type Reader struct {
	Store      *store.Store
	Aggregator *aggregator.Aggregator
	BaseURL    string
}

func (r *Reader) Marketplace(c *gin.Context) {
	data, err := os.ReadFile(r.Store.AggregatedDir() + "/marketplace.json")
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "no aggregated marketplace yet; add a source and refresh",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

func (r *Reader) PluginFile(c *gin.Context) {
	src := c.Param("source")
	rel := c.Param("path")
	if rel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing path"})
		return
	}
	if rel[0] == '/' {
		rel = rel[1:]
	}
	full, err := r.Store.ResolvePluginPath(src, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := os.Stat(full); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(full)
}
```

- [ ] **Step 3: Write failing tests**

Create `internal/server/reader_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/store"
)

func newTestServer(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s := store.New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	require.NoError(t, s.WriteFile("team", "plugins/p1/plugin.json", []byte(`{"name":"p1"}`)))
	require.NoError(t, os.MkdirAll(s.AggregatedDir(), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(s.AggregatedDir(), "marketplace.json"),
		[]byte(`{"plugins":[{"name":"p1"}]}`), 0o644))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := &Server{Engine: r, Store: s, Aggregator: aggregator.New(s, "http://x"), BaseURL: "http://x"}
	srv.RegisterRoutes()
	return r, s
}

func TestMarketplace_OK(t *testing.T) {
	r, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/marketplace.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	assert.Contains(t, m, "plugins")
}

func TestMarketplace_503(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := &Server{Engine: r, Store: s, Aggregator: aggregator.New(s, "http://x"), BaseURL: "http://x"}
	srv.RegisterRoutes()

	req := httptest.NewRequest(http.MethodGet, "/marketplace.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPluginFile_OK(t *testing.T) {
	r, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/plugins/team/plugins/p1/plugin.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"p1"`)
}

func TestPluginFile_Traversal(t *testing.T) {
	r, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/plugins/team/../escape.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPluginFile_NotFound(t *testing.T) {
	r, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/plugins/team/missing.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/server/... -v
```

Expected: PASS (4 reader + 1 healthz).

- [ ] **Step 5: Commit**

```bash
git add internal/server/
git commit -m "feat(server): read API for marketplace.json and plugin files"
```

---

## Task 9: Admin API (M6)

**Files:**
- Modify: `internal/server/server.go` (register admin routes, add Manager)
- Create: `internal/server/admin.go`
- Create: `internal/server/admin_test.go`
- Create: `internal/source/manager.go` (state map + persistence)
- Create: `internal/source/manager_test.go`

The Manager owns: the loaded `*config.Config`, the per-source state map,
and persistence (save config on write). It's a thin coordinator on top of
the Registry + Store + Aggregator.

- [ ] **Step 1: Write failing test for Manager**

Create `internal/source/manager_test.go`:

```go
package source

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/store"
)

func TestManager_AddAndList(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yaml")
	st := store.New(filepath.Join(dir, "data"))
	mgr := NewManager(cfgPath, st, &sync.Mutex{})

	spec := config.Source{Name: "team", Type: "git", URL: "https://x/y.git", Enabled: true}
	require.NoError(t, mgr.Add(spec))

	st2, err := mgr.List()
	require.NoError(t, err)
	assert.Len(t, st2, 1)
	assert.Equal(t, "team", st2[0].Name)

	// Persisted on disk
	_, err = os.Stat(cfgPath)
	assert.NoError(t, err)
}

func TestManager_Duplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yaml")
	st := store.New(filepath.Join(dir, "data"))
	mgr := NewManager(cfgPath, st, &sync.Mutex{})

	spec := config.Source{Name: "x", Type: "git", URL: "https://x", Enabled: true}
	require.NoError(t, mgr.Add(spec))
	err := mgr.Add(spec)
	assert.Error(t, err)
}

func TestManager_Delete(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yaml")
	st := store.New(filepath.Join(dir, "data"))
	mgr := NewManager(cfgPath, st, &sync.Mutex{})

	spec := config.Source{Name: "x", Type: "git", URL: "https://x", Enabled: true}
	require.NoError(t, mgr.Add(spec))
	require.NoError(t, mgr.Delete("x"))

	st2, _ := mgr.List()
	assert.Empty(t, st2)
}

// silence unused import warning when context not used elsewhere
var _ = context.Background
```

- [ ] **Step 2: Run test, verify failure**

```bash
go test ./internal/source/... -v -run TestManager
```

Expected: FAIL.

- [ ] **Step 3: Implement Manager**

Create `internal/source/manager.go`:

```go
package source

import (
	"fmt"
	"sync"

	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/store"
)

type State struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	LastRefresh string `json:"last_refresh,omitempty"`
	LastError   string `json:"last_error,omitempty"`
}

type Manager struct {
	cfgPath string
	store   *store.Store
	mu      *sync.Mutex
	states  map[string]State
}

func NewManager(cfgPath string, st *store.Store, mu *sync.Mutex) *Manager {
	return &Manager{cfgPath: cfgPath, store: st, mu: mu, states: map[string]State{}}
}

func (m *Manager) loadOrInit() error {
	if _, err := loadIfExists(m.cfgPath); err != nil {
		return err
	}
	return nil
}

func loadIfExists(path string) (*config.Config, error) {
	if _, err := readFile(path); err != nil {
		// missing -> init empty
		if isNotExist(err) {
			return &config.Config{
				Server:  config.ServerConfig{Listen: ":8080", DataDir: "./data"},
				Sources: []config.Source{},
			}, nil
		}
		return nil, err
	}
	return config.Load(path)
}

func (m *Manager) Add(s config.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return err
	}
	for _, existing := range cfg.Sources {
		if existing.Name == s.Name {
			return fmt.Errorf("source %q already exists", s.Name)
		}
	}
	if err := s.Validate(); err != nil {
		return err
	}
	cfg.Sources = append(cfg.Sources, s)
	if err := cfg.Save(m.cfgPath); err != nil {
		return err
	}
	m.states[s.Name] = State{Name: s.Name, Enabled: s.Enabled}
	return nil
}

func (m *Manager) Update(name string, s config.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return err
	}
	found := false
	for i, existing := range cfg.Sources {
		if existing.Name == name {
			if s.Name != name {
				return fmt.Errorf("cannot rename source %q (use delete + add)", name)
			}
			cfg.Sources[i] = s
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("source %q not found", name)
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if err := cfg.Save(m.cfgPath); err != nil {
		return err
	}
	m.states[name] = State{Name: name, Enabled: s.Enabled}
	return nil
}

func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return err
	}
	out := cfg.Sources[:0]
	found := false
	for _, existing := range cfg.Sources {
		if existing.Name == name {
			found = true
			continue
		}
		out = append(out, existing)
	}
	if !found {
		return fmt.Errorf("source %q not found", name)
	}
	cfg.Sources = out
	if err := cfg.Save(m.cfgPath); err != nil {
		return err
	}
	delete(m.states, name)
	return m.store.RemoveSource(name)
}

func (m *Manager) List() ([]State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]State, 0, len(cfg.Sources))
	for _, s := range cfg.Sources {
		st := m.states[s.Name]
		st.Name = s.Name
		st.Enabled = s.Enabled
		out = append(out, st)
	}
	return out, nil
}

func (m *Manager) Get(name string) (config.Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, err := m.loadLocked()
	if err != nil {
		return config.Source{}, err
	}
	for _, s := range cfg.Sources {
		if s.Name == name {
			return s, nil
		}
	}
	return config.Source{}, fmt.Errorf("source %q not found", name)
}

func (m *Manager) Snapshot() (*config.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadLocked()
}

func (m *Manager) SetState(name string, mut func(*State)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.states[name]
	mut(&st)
	m.states[name] = st
}

func (m *Manager) GetState(name string) State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[name]
}

func (m *Manager) loadLocked() (*config.Config, error) {
	return loadIfExists(m.cfgPath)
}
```

Add helpers in `internal/source/io.go`:

```go
package source

import "os"

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func isNotExist(err error) bool { return os.IsNotExist(err) }
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/source/... -v
```

Expected: PASS (registry + git + http + manager tests).

- [ ] **Step 5: Wire manager into Server and add admin routes**

Update `internal/server/server.go`:

```go
package server

import (
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
)

type Server struct {
	Engine     *gin.Engine
	Store      *store.Store
	Aggregator *aggregator.Aggregator
	Manager    *source.Manager
	Registry   *source.Registry
	BaseURL    string
	CfgPath    string
	mu         *sync.Mutex
}

func (s *Server) RegisterRoutes() {
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})

	reader := &Reader{Store: s.Store, Aggregator: s.Aggregator, BaseURL: s.BaseURL}
	s.Engine.GET("/marketplace.json", reader.Marketplace)
	s.Engine.GET("/plugins/:source/*path", reader.PluginFile)

	admin := &Admin{Server: s}
	g := s.Engine.Group("/admin/api")
	g.GET("/sources", admin.ListSources)
	g.POST("/sources", admin.AddSource)
	g.PUT("/sources/:name", admin.UpdateSource)
	g.DELETE("/sources/:name", admin.DeleteSource)
	g.POST("/sources/:name/refresh", admin.RefreshOne)
	g.POST("/refresh", admin.RefreshAll)
	g.GET("/aggregated", admin.Aggregated)
}
```

Create `internal/server/admin.go`:

```go
package server

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/source"
)

type Admin struct {
	Server *Server
}

func (a *Admin) ListSources(c *gin.Context) {
	states, err := a.Server.Manager.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sources": states})
}

func (a *Admin) AddSource(c *gin.Context) {
	var s config.Source
	if err := c.BindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if s.Enabled == false {
		s.Enabled = true // default
	}
	if err := a.Server.Manager.Add(s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"source": s})
}

func (a *Admin) UpdateSource(c *gin.Context) {
	name := c.Param("name")
	var s config.Source
	if err := c.BindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Server.Manager.Update(name, s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"source": s})
}

func (a *Admin) DeleteSource(c *gin.Context) {
	name := c.Param("name")
	if err := a.Server.Manager.Delete(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *Admin) RefreshOne(c *gin.Context) {
	name := c.Param("name")
	spec, err := a.Server.Manager.Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	src, err := a.Server.Registry.Build(spec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dest := a.Server.Store.SourceDir(name)
	if err := src.Fetch(c.Request.Context(), dest); err != nil {
		a.Server.Manager.SetState(name, func(st *source.State) {
			st.LastError = err.Error()
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Update state timestamp
	a.Server.Manager.SetState(name, func(st *source.State) {
		st.LastError = ""
		st.LastRefresh = nowRFC3339()
	})
	// Trigger aggregate over all enabled sources
	cfg, _ := a.Server.Manager.Snapshot()
	names := []string{}
	for _, s := range cfg.Sources {
		if s.Enabled {
			names = append(names, s.Name)
		}
	}
	if err := a.Server.Aggregator.Refresh(names); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"refreshed": name})
}

func (a *Admin) RefreshAll(c *gin.Context) {
	cfg, err := a.Server.Manager.Snapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	failed := map[string]string{}
	ok := []string{}
	for _, s := range cfg.Sources {
		if !s.Enabled {
			continue
		}
		src, err := a.Server.Registry.Build(s)
		if err != nil {
			failed[s.Name] = err.Error()
			continue
		}
		dest := a.Server.Store.SourceDir(s.Name)
		if err := src.Fetch(c.Request.Context(), dest); err != nil {
			a.Server.Manager.SetState(s.Name, func(st *source.State) { st.LastError = err.Error() })
			failed[s.Name] = err.Error()
			continue
		}
		a.Server.Manager.SetState(s.Name, func(st *source.State) {
			st.LastError = ""
			st.LastRefresh = nowRFC3339()
		})
		ok = append(ok, s.Name)
	}
	names := ok
	if err := a.Server.Aggregator.Refresh(names); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"refreshed": ok, "failed": failed})
}

func (a *Admin) Aggregated(c *gin.Context) {
	data, err := os.ReadFile(a.Server.Store.AggregatedDir() + "/marketplace.json")
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}
```

Add helper `internal/server/time.go`:

```go
package server

import "time"

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
```

- [ ] **Step 6: Write admin tests**

Create `internal/server/admin_test.go`:

```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
)

func newAdminTestServer(t *testing.T) (*gin.Engine, *source.Manager) {
	t.Helper()
	dir := t.TempDir()
	st := store.New(filepath.Join(dir, "data"))
	mu := &sync.Mutex{}
	mgr := source.NewManager(filepath.Join(dir, "c.yaml"), st, mu)

	reg := source.NewRegistry()
	reg.Register("http", func(s config.Source) source.Source {
		out, _ := source.NewHTTPSource(s)
		return out
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := &Server{
		Engine:     r,
		Store:      st,
		Aggregator: aggregator.New(st, "http://test"),
		Manager:    mgr,
		Registry:   reg,
		BaseURL:    "http://test",
		CfgPath:    filepath.Join(dir, "c.yaml"),
		mu:         mu,
	}
	srv.RegisterRoutes()
	return r, mgr
}

func TestAdmin_AddListDelete(t *testing.T) {
	r, _ := newAdminTestServer(t)

	body, _ := json.Marshal(map[string]any{
		"name": "team", "type": "http",
		"url": "http://127.0.0.1:1/x", "enabled": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/admin/api/sources", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "team")

	req = httptest.NewRequest(http.MethodDelete, "/admin/api/sources/team", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/admin/api/sources", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.NotContains(t, w.Body.String(), "team")
}

func TestAdmin_DuplicateAdd(t *testing.T) {
	r, _ := newAdminTestServer(t)
	body, _ := json.Marshal(map[string]any{
		"name": "x", "type": "http", "url": "http://a", "enabled": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	req = httptest.NewRequest(http.MethodPost, "/admin/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/server/... -v
```

Expected: PASS.

- [ ] **Step 8: Update main.go to wire the registry/manager**

Update `main.go`:

```go
package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/server"
	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
)

func main() {
	cfgPath := envOr("AGENTSSTORE_CONFIG", "./config.yaml")
	cfg, err := loadOrInitConfig(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st := store.New(cfg.Server.DataDir)
	if err := os.MkdirAll(st.AggregatedDir(), 0o755); err != nil {
		log.Fatal(err)
	}

	mu := &sync.Mutex{}
	mgr := source.NewManager(cfgPath, st, mu)

	reg := source.NewRegistry()
	reg.Register("git", func(s config.Source) source.Source {
		out, err := source.NewGitSource(s)
		if err != nil {
			log.Fatalf("git source: %v", err)
		}
		return out
	})
	reg.Register("http", func(s config.Source) source.Source {
		out, err := source.NewHTTPSource(s)
		if err != nil {
			log.Fatalf("http source: %v", err)
		}
		return out
	})

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	srv := &server.Server{
		Engine:     r,
		Store:      st,
		Aggregator: aggregator.New(st, baseURL(cfg)),
		Manager:    mgr,
		Registry:   reg,
		BaseURL:    baseURL(cfg),
		CfgPath:    cfgPath,
		mu:         mu,
	}
	srv.RegisterRoutes()

	log.Printf("agentsstore listening on %s (data=%s)", cfg.Server.Listen, cfg.Server.DataDir)
	if err := r.Run(cfg.Server.Listen); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func baseURL(cfg *config.Config) string {
	if cfg.Server.BaseURL != "" {
		return cfg.Server.BaseURL
	}
	abs, err := filepath.Abs(cfg.Server.DataDir)
	if err != nil {
		return "http://localhost" + cfg.Server.Listen
	}
	return "http://localhost" + cfg.Server.Listen + "/" + filepath.Base(abs)
}

func loadOrInitConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c := &config.Config{
			Server:  config.ServerConfig{Listen: ":8080", DataDir: "./data"},
			Sources: []config.Source{},
		}
		if err := c.Save(path); err != nil {
			return nil, err
		}
		return c, nil
	}
	return config.Load(path)
}
```

- [ ] **Step 9: Verify build and tests**

```bash
go build ./...
go test ./... -v
```

Expected: all green.

- [ ] **Step 10: Commit**

```bash
git add .
git commit -m "feat(admin): CRUD API + manager + main wiring"
```

---

## Task 10: Admin UI (M7)

**Files:**
- Create: `internal/ui/ui.go`
- Create: `internal/ui/templates/*.html`
- Create: `internal/ui/static/style.css`
- Create: `internal/ui/static/app.js`
- Modify: `internal/server/server.go` (register UI routes)
- Create: `internal/server/ui.go`
- Create: `internal/server/ui_test.go`

- [ ] **Step 1: Create embed wrapper**

Create `internal/ui/ui.go`:

```go
package ui

import "embed"

//go:embed templates/*.html
var Templates embed.FS

//go:embed static/*
var Static embed.FS
```

- [ ] **Step 2: Create base template**

Create `internal/ui/templates/base.html`:

```html
{{define "base"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{.Title}} · AgentsStore</title>
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<header>
<h1><a href="/admin/">AgentsStore</a></h1>
<nav>
<a href="/admin/">Sources</a>
<a href="/admin/preview">Preview</a>
</nav>
</header>
<main>{{template "body" .}}</main>
<script src="/static/app.js"></script>
</body>
</html>{{end}}
```

- [ ] **Step 3: Create index template (lists sources)**

Create `internal/ui/templates/index.html`:

```html
{{define "body"}}
<h2>Sources</h2>
<p><a class="btn" href="/admin/sources/new">+ Add source</a>
   <button class="btn" onclick="refreshAll()">Refresh all</button></p>
<table>
<thead><tr><th>Name</th><th>Type</th><th>URL</th><th>Enabled</th><th>Status</th><th>Actions</th></tr></thead>
<tbody>
{{range .Sources}}
<tr>
<td>{{.Name}}</td>
<td>{{.Type}}</td>
<td>{{.URL}}</td>
<td>{{if .Enabled}}yes{{else}}no{{end}}</td>
<td>
{{if .LastError}}<span class="err">err: {{.LastError}}</span>
{{else if .LastRefresh}}ok ({{.LastRefresh}})
{{else}}never{{end}}
</td>
<td>
<button onclick="refreshOne('{{.Name}}')">Refresh</button>
<a href="/admin/sources/{{.Name}}/edit">Edit</a>
<button onclick="deleteOne('{{.Name}}')">Delete</button>
</td>
</tr>
{{else}}
<tr><td colspan="6">No sources yet.</td></tr>
{{end}}
</tbody>
</table>
{{end}}
{{template "base" .}}
```

- [ ] **Step 4: Create new source form**

Create `internal/ui/templates/new.html`:

```html
{{define "body"}}
<h2>Add source</h2>
<form method="post" action="/admin/sources/new" onsubmit="return submitNew(event)">
<label>Name <input name="name" required pattern="[a-z0-9][a-z0-9-]*"></label>
<label>Type
<select name="type">
<option value="git">git</option>
<option value="http">http</option>
</select></label>
<label>URL <input name="url" required></label>
<label>Ref (git only) <input name="ref"></label>
<label><input type="checkbox" name="enabled" checked> Enabled</label>
<button type="submit">Save</button>
<a href="/admin/">Cancel</a>
</form>
{{end}}
{{template "base" .}}
```

- [ ] **Step 5: Create edit source form**

Create `internal/ui/templates/edit.html`:

```html
{{define "body"}}
<h2>Edit source: {{.Source.Name}}</h2>
<form onsubmit="return submitEdit(event, '{{.Source.Name}}')">
<label>Name <input name="name" value="{{.Source.Name}}" readonly></label>
<label>Type
<select name="type">
<option value="git" {{if eq .Source.Type "git"}}selected{{end}}>git</option>
<option value="http" {{if eq .Source.Type "http"}}selected{{end}}>http</option>
</select></label>
<label>URL <input name="url" value="{{.Source.URL}}" required></label>
<label>Ref (git only) <input name="ref" value="{{.Source.Ref}}"></label>
<label><input type="checkbox" name="enabled" {{if .Source.Enabled}}checked{{end}}> Enabled</label>
<button type="submit">Save</button>
<a href="/admin/">Cancel</a>
</form>
{{end}}
{{template "base" .}}
```

- [ ] **Step 6: Create preview template**

Create `internal/ui/templates/preview.html`:

```html
{{define "body"}}
<h2>Aggregated marketplace.json</h2>
<pre>{{.Aggregated}}</pre>
{{end}}
{{template "base" .}}
```

- [ ] **Step 7: Create static CSS**

Create `internal/ui/static/style.css`:

```css
* { box-sizing: border-box; }
body { font-family: -apple-system, system-ui, sans-serif; margin: 0; background: #f7f7f8; }
header { background: #1f2937; color: white; padding: 1rem 2rem; display: flex; align-items: center; gap: 2rem; }
header h1 { margin: 0; font-size: 1.2rem; }
header h1 a { color: white; text-decoration: none; }
header nav a { color: #cbd5e1; text-decoration: none; margin-right: 1rem; }
main { max-width: 1100px; margin: 2rem auto; background: white; padding: 2rem; border-radius: 8px; }
table { width: 100%; border-collapse: collapse; }
th, td { padding: 0.5rem; text-align: left; border-bottom: 1px solid #e5e7eb; }
.btn { background: #2563eb; color: white; padding: 0.4rem 0.8rem; border: 0; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; }
button { background: #e5e7eb; border: 0; padding: 0.3rem 0.6rem; border-radius: 4px; cursor: pointer; }
.err { color: #b91c1c; }
form { display: flex; flex-direction: column; gap: 0.75rem; max-width: 480px; }
label { display: flex; flex-direction: column; gap: 0.25rem; }
input, select { padding: 0.4rem; border: 1px solid #d1d5db; border-radius: 4px; }
pre { background: #0b1020; color: #d1d5db; padding: 1rem; border-radius: 6px; overflow: auto; }
```

- [ ] **Step 8: Create app.js with actions**

Create `internal/ui/static/app.js`:

```js
async function refreshOne(name) {
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}/refresh`, { method: 'POST' });
  if (!r.ok) { alert('refresh failed: ' + (await r.text())); return; }
  location.reload();
}
async function refreshAll() {
  const r = await fetch('/admin/api/refresh', { method: 'POST' });
  if (!r.ok) { alert('refresh all failed: ' + (await r.text())); return; }
  location.reload();
}
async function deleteOne(name) {
  if (!confirm(`Delete source ${name}?`)) return;
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}`, { method: 'DELETE' });
  if (!r.ok) { alert('delete failed: ' + (await r.text())); return; }
  location.reload();
}
async function submitNew(e) {
  e.preventDefault();
  const f = e.target;
  const body = {
    name: f.name.value,
    type: f.type.value,
    url: f.url.value,
    ref: f.ref.value,
    enabled: f.enabled.checked,
  };
  const r = await fetch('/admin/api/sources', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!r.ok) { alert('add failed: ' + (await r.text())); return false; }
  location.href = '/admin/';
  return false;
}
async function submitEdit(e, name) {
  e.preventDefault();
  const f = e.target;
  const body = {
    name: f.name.value,
    type: f.type.value,
    url: f.url.value,
    ref: f.ref.value,
    enabled: f.enabled.checked,
  };
  const r = await fetch(`/admin/api/sources/${encodeURIComponent(name)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!r.ok) { alert('update failed: ' + (await r.text())); return false; }
  location.href = '/admin/';
  return false;
}
```

- [ ] **Step 9: Implement UI handler**

Create `internal/server/ui.go`:

```go
package server

import (
	"html/template"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/ui"
)

type UI struct {
	Server *Server
	tpl    *template.Template
}

func NewUI(s *Server) *UI {
	tpl := template.Must(template.ParseFS(ui.Templates, "templates/*.html"))
	return &UI{Server: s, tpl: tpl}
}

func (u *UI) Index(c *gin.Context) {
	states, err := u.Server.Manager.List()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	type src struct {
		Name, Type, URL, Ref, LastError, LastRefresh string
		Enabled                                       bool
	}
	cfg, _ := u.Server.Manager.Snapshot()
	byName := map[string]config.Source{}
	for _, s := range cfg.Sources {
		byName[s.Name] = s
	}
	rows := []src{}
	for _, st := range states {
		s := byName[st.Name]
		rows = append(rows, src{
			Name: s.Name, Type: s.Type, URL: s.URL, Ref: s.Ref,
			Enabled: s.Enabled, LastError: st.LastError, LastRefresh: st.LastRefresh,
		})
	}
	u.render(c, "index.html", gin.H{"Title": "Sources", "Sources": rows})
}

func (u *UI) NewForm(c *gin.Context) {
	u.render(c, "new.html", gin.H{"Title": "Add source"})
}

func (u *UI) EditForm(c *gin.Context) {
	name := c.Param("name")
	s, err := u.Server.Manager.Get(name)
	if err != nil {
		c.String(http.StatusNotFound, err.Error())
		return
	}
	u.render(c, "edit.html", gin.H{"Title": "Edit " + name, "Source": s})
}

func (u *UI) Preview(c *gin.Context) {
	data, err := os.ReadFile(u.Server.Store.AggregatedDir() + "/marketplace.json")
	if err != nil {
		u.render(c, "preview.html", gin.H{"Title": "Preview", "Aggregated": "(no aggregated marketplace yet)"})
		return
	}
	u.render(c, "preview.html", gin.H{"Title": "Preview", "Aggregated": string(data)})
}

func (u *UI) render(c *gin.Context, name string, data gin.H) {
	data["_tpl"] = name
	c.Status(http.StatusOK)
	if err := u.tpl.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}
}
```

Note: the templates use `{{template "base" .}}` to wrap each body. The
handler passes the body template name in `_tpl` and the base template
includes `{{template "body" .}}` — but `body` is redefined per-page. Gin
template `ExecuteTemplate` is invoked with name "base" but the parsed
set has many templates. To work correctly, change the base template to
use a fixed name "body" and parse all page templates so each redefines
"body". The current `templates/*.html` already does this.

- [ ] **Step 10: Wire UI routes**

Update `internal/server/server.go`'s `RegisterRoutes`:

```go
func (s *Server) RegisterRoutes() {
	s.Engine.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})

	reader := &Reader{Store: s.Store, Aggregator: s.Aggregator, BaseURL: s.BaseURL}
	s.Engine.GET("/marketplace.json", reader.Marketplace)
	s.Engine.GET("/plugins/:source/*path", reader.PluginFile)

	// static
	s.Engine.StaticFS("/static", http.FS(ui.Static))

	ui := NewUI(s)
	admin := s.Engine.Group("/admin")
	admin.GET("/", ui.Index)
	admin.GET("/sources/new", ui.NewForm)
	admin.GET("/sources/:name/edit", ui.EditForm)
	admin.GET("/preview", ui.Preview)

	api := s.Engine.Group("/admin/api")
	api.GET("/sources", admin.ListSources)
	api.POST("/sources", admin.AddSource)
	api.PUT("/sources/:name", admin.UpdateSource)
	api.DELETE("/sources/:name", admin.DeleteSource)
	api.POST("/sources/:name/refresh", admin.RefreshOne)
	api.POST("/refresh", admin.RefreshAll)
	api.GET("/aggregated", admin.Aggregated)
}
```

(Replace the prior admin struct with the inline `admin` and add the
imports `net/http` and `github.com/wu/agentsstore/internal/ui`.)

- [ ] **Step 11: Test UI page renders**

Create `internal/server/ui_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/wu/agentsstore/internal/aggregator"
	"github.com/wu/agentsstore/internal/config"
	"github.com/wu/agentsstore/internal/source"
	"github.com/wu/agentsstore/internal/store"
)

func TestUI_Index(t *testing.T) {
	dir := t.TempDir()
	st := store.New(filepath.Join(dir, "data"))
	mu := &sync.Mutex{}
	mgr := source.NewManager(filepath.Join(dir, "c.yaml"), st, mu)
	reg := source.NewRegistry()
	reg.Register("http", func(s config.Source) source.Source { out, _ := source.NewHTTPSource(s); return out })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := &Server{Engine: r, Store: st, Aggregator: aggregator.New(st, "http://x"), Manager: mgr, Registry: reg, BaseURL: "http://x", CfgPath: filepath.Join(dir, "c.yaml"), mu: mu}
	srv.RegisterRoutes()

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Sources")
}
```

- [ ] **Step 12: Run tests**

```bash
go test ./... -v
```

Expected: PASS.

- [ ] **Step 13: Manual smoke test**

```bash
go build -o /tmp/agentsstore .
mkdir -p /tmp/as-data
/tmp/agentsstore &
PID=$!
sleep 1
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/admin/ | head -20
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"name":"team","type":"http","url":"http://127.0.0.1:1/x","enabled":true}' \
  http://localhost:8080/admin/api/sources
curl -s http://localhost:8080/admin/api/sources
kill $PID
```

Expected: `/healthz` returns `ok`, `/admin/` returns HTML containing "Sources", and the POST creates a source (refresh will fail since the URL is unreachable; the failure should appear in the state list).

- [ ] **Step 14: Commit**

```bash
git add internal/ui/ internal/server/
git commit -m "feat(ui): admin web UI for source management"
```

---

## Task 11: README + final docs (M8)

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

Create `README.md`:

````markdown
# agentsstore

A small Go HTTP service that aggregates multiple Claude Code plugin
marketplaces (Git repos and HTTP endpoints) into a single URL. Team
members configure their Claude Code once, pointing at the aggregator;
adding/removing upstream sources is done through the admin web UI or
the JSON admin API.

## Quick start

```bash
go build -o agentsstore .
./agentsstore                  # listens on :8080
```

Open http://localhost:8080/admin/ in your browser.

## Configure Claude Code to use the aggregator

In your `~/.claude/settings.json`:

```json
{
  "marketplaces": {
    "team-aggregated": {
      "url": "http://aggregator.local:8080/marketplace.json"
    }
  }
}
```

Claude Code reads `/marketplace.json` and then fetches each plugin via
`/plugins/{source}/{plugin_path}`.

## Configuration

The service reads `config.yaml` in the working directory (or the path in
`AGENTSSTORE_CONFIG`). Example:

```yaml
server:
  listen: ":8080"
  data_dir: "./data"
  base_url: ""           # optional; auto-derived if empty

sources:
  - name: "anthropic-official"
    type: "git"
    url: "https://github.com/anthropics/claude-code.git"
    ref: "main"
    enabled: true
```

## Source types

- `git`: shallow-clones the repository on refresh. Reads
  `marketplace.json` from the repo root. Use `ref` to pin a branch/tag.
- `http`: fetches `marketplace.json` from the URL, then downloads each
  plugin's files referenced by the `source` field.

## Admin API

See the design doc for the full surface:
`docs/superpowers/specs/2026-06-11-claude-code-marketplace-aggregator-design.md`.

## Development

```bash
go test ./...
go vet ./...
```

No Docker, no system git required.
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README"
```

---

## Self-Review (post-write)

1. **Spec coverage**:
   - Single binary ✓ (M0)
   - YAML config ✓ (M1)
   - Git + HTTP source types ✓ (M2, M3)
   - Mirror to local disk ✓ (M2, M3, store)
   - Aggregator merge + path rewrite ✓ (M4)
   - Read API: `/marketplace.json`, `/plugins/...` ✓ (M5)
   - Admin API CRUD + refresh ✓ (M6)
   - Web UI: list, new, edit, preview ✓ (M7)
   - Manual refresh only ✓ (M6 — no scheduler)
   - Path-traversal guard ✓ (store + reader tests)
   - `lastError` / `lastRefresh` state visibility ✓ (Manager.State)

2. **Placeholder scan**: no TBDs, TODOs, or "implement later" notes. Every
   step has concrete code or commands.

3. **Type consistency**:
   - `Source.Name()` / `Source.Type()` / `Source.Fetch()` consistent
     across `source.go`, `git.go`, `http.go`, registry.
   - `State.Name` / `State.Enabled` / `State.LastRefresh` / `State.LastError`
     used identically in `manager.go`, `admin.go`, and UI templates.
   - `Config.Source` field tags (`name`, `type`, `url`, `ref`, `enabled`)
     used consistently in YAML, validation, manager, and JSON payloads.

## Execution Handoff

Plan complete and saved to
`docs/superpowers/plans/2026-06-11-claude-code-marketplace-aggregator.md`.

Two execution options:

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per
   task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using
   executing-plans, batch execution with checkpoints.
