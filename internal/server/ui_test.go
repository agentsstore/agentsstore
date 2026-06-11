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
	srv := &Server{Engine: r, Store: st, Aggregator: aggregator.New(st, "http://x"), Manager: mgr, Registry: reg, BaseURL: "http://x", CfgPath: filepath.Join(dir, "c.yaml")}
	srv.RegisterRoutes()

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "源列表")
}

func TestUI_AllPages(t *testing.T) {
	dir := t.TempDir()
	st := store.New(filepath.Join(dir, "data"))
	mu := &sync.Mutex{}
	mgr := source.NewManager(filepath.Join(dir, "c.yaml"), st, mu)
	reg := source.NewRegistry()
	reg.Register("http", func(s config.Source) source.Source { out, _ := source.NewHTTPSource(s); return out })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	srv := &Server{Engine: r, Store: st, Aggregator: aggregator.New(st, "http://x"), Manager: mgr, Registry: reg, BaseURL: "http://x", CfgPath: filepath.Join(dir, "c.yaml")}
	srv.RegisterRoutes()

	cases := []struct {
		path, contains string
	}{
		{"/admin/", "暂无源"},
		{"/admin/sources/new", "添加源"},
		{"/admin/preview", "聚合后的 marketplace.json"},
		{"/static/style.css", "box-sizing"},
		{"/static/app.js", "refreshOne"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, tc.path)
		assert.Contains(t, w.Body.String(), tc.contains, tc.path)
	}
}
