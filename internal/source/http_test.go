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
