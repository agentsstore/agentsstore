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

func TestPluginFile_Directory(t *testing.T) {
	r, s := newTestServer(t)
	// Create a directory under the source
	require.NoError(t, os.MkdirAll(filepath.Join(s.SourceDir("team"), "plugins", "p2"), 0o755))

	req := httptest.NewRequest(http.MethodGet, "/plugins/team/plugins/p2/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPluginFile_Symlink(t *testing.T) {
	r, s := newTestServer(t)
	// Create a symlink inside the source pointing outside it
	target := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(target, []byte("secret"), 0o644))
	link := filepath.Join(s.SourceDir("team"), "plugins", "p1", "leak.txt")
	require.NoError(t, os.Symlink(target, link))

	req := httptest.NewRequest(http.MethodGet, "/plugins/team/plugins/p1/leak.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Should be rejected as 400 (not 200 with the secret content)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "secret")
}
