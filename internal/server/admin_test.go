package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
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

func TestAdmin_AddWithEnabledFalse(t *testing.T) {
	r, _ := newAdminTestServer(t)

	body, _ := json.Marshal(map[string]any{
		"name": "team", "type": "http", "url": "http://a", "enabled": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// GET should show enabled=false (not silently flipped to true)
	req = httptest.NewRequest(http.MethodGet, "/admin/api/sources", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
}

func TestAdmin_UpdateWithEnabledFalse(t *testing.T) {
	r, _ := newAdminTestServer(t)

	// Add as enabled
	addBody, _ := json.Marshal(map[string]any{
		"name": "team", "type": "http", "url": "http://a", "enabled": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/sources", bytes.NewReader(addBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Update with enabled=false
	updBody, _ := json.Marshal(map[string]any{
		"name": "team", "type": "http", "url": "http://a", "enabled": false,
	})
	req = httptest.NewRequest(http.MethodPut, "/admin/api/sources/team", bytes.NewReader(updBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// GET should still show enabled=false
	req = httptest.NewRequest(http.MethodGet, "/admin/api/sources", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
}

// httpTestSource is a fake source that serves a marketplace.json from an
// httptest server. The Source implementation re-resolves the URL from the
// supplied config at fetch time, so the test server may be live only for
// the duration of the request.
type httpTestSource struct{ spec config.Source }

func (h *httpTestSource) Name() string             { return h.spec.Name }
func (h *httpTestSource) Type() string             { return "http" }
func (h *httpTestSource) Fetch(ctx context.Context, destDir string) error {
	// Re-parse the URL from the spec each time and fetch the body. This
	// makes the source self-contained and lets tests swap out the URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.spec.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("http source: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	st := store.New(filepath.Dir(filepath.Dir(destDir)))
	if err := st.EnsureSourceDir(h.spec.Name); err != nil {
		return err
	}
	// Claude Code convention: marketplace.json lives under .claude-plugin/.
	return st.WriteFile(h.spec.Name, source.MarketplaceManifestPath, body)
}

func newAdminTestServerWithFake(t *testing.T) (*gin.Engine, *source.Manager) {
	t.Helper()
	dir := t.TempDir()
	st := store.New(filepath.Join(dir, "data"))
	mu := &sync.Mutex{}
	mgr := source.NewManager(filepath.Join(dir, "c.yaml"), st, mu)

	reg := source.NewRegistry()
	// Use "http" type with our test source so config validation passes.
	reg.Register("http", func(s config.Source) source.Source {
		return &httpTestSource{spec: s}
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := &Server{
		Engine:     r,
		Store:      st,
		Aggregator: aggregator.New(st, "http://constructor.test"),
		Manager:    mgr,
		Registry:   reg,
		BaseURL:    "http://constructor.test",
		CfgPath:    filepath.Join(dir, "c.yaml"),
	}
	srv.RegisterRoutes()
	return r, mgr
}

// startMarketplaceServer starts an httptest server that serves a single
// marketplace.json containing one plugin. Returns the base URL and a
// cleanup function.
func startMarketplaceServer(t *testing.T, sourceName string) (string, func()) {
	t.Helper()
	mp := map[string]any{
		"name": sourceName,
		"plugins": []map[string]any{
			{"name": "p1", "source": "./plugins/p1"},
		},
	}
	body, _ := json.Marshal(mp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	return srv.URL, srv.Close
}

func addSource(t *testing.T, r *gin.Engine, name, url string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"name": name, "type": "http",
		"url": url, "enabled": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/sources", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestAdmin_RequestBaseURL_DerivesFromHost(t *testing.T) {
	r, _ := newAdminTestServerWithFake(t)
	url, stop := startMarketplaceServer(t, "team")
	defer stop()
	addSource(t, r, "team", url)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/refresh", nil)
	req.Host = "aggregator.team.local:8080"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Aggregated output must use the request's host, not localhost or
	// the constructor's baseURL.
	req = httptest.NewRequest(http.MethodGet, "/admin/api/aggregated", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http://aggregator.team.local:8080/plugins/team/plugins/p1")
	assert.NotContains(t, w.Body.String(), "http://constructor.test")
}

func TestAdmin_RequestBaseURL_PrefersForwarded(t *testing.T) {
	r, _ := newAdminTestServerWithFake(t)
	url, stop := startMarketplaceServer(t, "team")
	defer stop()
	addSource(t, r, "team", url)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/refresh", nil)
	req.Host = "127.0.0.1:9999"
	req.Header.Set("X-Forwarded-Host", "public.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/admin/api/aggregated", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "https://public.example.com/plugins/team/plugins/p1")
	assert.NotContains(t, w.Body.String(), "127.0.0.1")
}

func TestRequestBaseURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name       string
		host       string
		tls        bool
		fwdHost    string
		fwdProto   string
		want       string
	}{
		{"host-only", "host.example:8080", false, "", "", "http://host.example:8080"},
		{"forwarded-http", "internal:1", false, "pub.example", "", "http://pub.example"},
		{"forwarded-https", "internal:1", false, "pub.example", "https", "https://pub.example"},
		{"tls-implies-https", "secure:443", true, "", "", "https://secure:443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tc.host
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tc.fwdHost != "" {
				req.Header.Set("X-Forwarded-Host", tc.fwdHost)
			}
			if tc.fwdProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.fwdProto)
			}
			c.Request = req
			assert.Equal(t, tc.want, requestBaseURL(c))
		})
	}
}
