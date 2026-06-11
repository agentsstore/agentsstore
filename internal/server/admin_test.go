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
