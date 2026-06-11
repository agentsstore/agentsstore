package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wu/agentsstore/internal/config"
)

// startUpstream serves a Claude Code marketplace at the well-known path
// /.claude-plugin/marketplace.json. The handler also mirrors any plugin
// file referenced from the manifest.
func startUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.claude-plugin/marketplace.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plugins": []map[string]any{
				{"name": "p1", "source": "./plugins/p1"},
			},
		})
	})
	mux.HandleFunc("/.claude-plugin/plugins/p1/plugin.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"p1"}`))
	})
	return httptest.NewServer(mux)
}

func TestHTTPSource_Fetch(t *testing.T) {
	up := startUpstream(t)
	defer up.Close()

	// The source URL is the base directory; the fetcher appends the
	// well-known .claude-plugin/marketplace.json path itself.
	spec := config.Source{Name: "h", Type: "http", URL: up.URL + "/", Enabled: true}
	src, err := NewHTTPSource(spec)
	require.NoError(t, err)

	dest := t.TempDir()
	require.NoError(t, src.Fetch(context.Background(), dest))

	// Manifest should be at the well-known path
	data, err := os.ReadFile(filepath.Join(dest, ".claude-plugin/marketplace.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "p1")

	// p1's plugin.json should be mirrored under .claude-plugin/
	pdata, err := os.ReadFile(filepath.Join(dest, ".claude-plugin/plugins/p1/plugin.json"))
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

// TestHTTPSource_Fetch_AtomicRefresh verifies that calling Fetch twice replaces
// the destination contents atomically: files from the first run are removed if
// the second run has a different plugin set. A non-atomic implementation would
// leave stale files behind.
func TestHTTPSource_Fetch_AtomicRefresh(t *testing.T) {
	mux := http.NewServeMux()

	var callCount int32
	atomic.StoreInt32(&callCount, 0)
	mux.HandleFunc("/.claude-plugin/marketplace.json", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		var plugins []map[string]any
		if n == 1 {
			plugins = []map[string]any{
				{"name": "p1", "source": "./plugins/p1"},
			}
		} else {
			plugins = []map[string]any{
				{"name": "p2", "source": "./plugins/p2"},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"plugins": plugins})
	})
	mux.HandleFunc("/.claude-plugin/plugins/p1/plugin.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"p1"}`))
	})
	mux.HandleFunc("/.claude-plugin/plugins/p2/plugin.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"p2"}`))
	})
	up := httptest.NewServer(mux)
	defer up.Close()

	spec := config.Source{Name: "h", Type: "http", URL: up.URL + "/", Enabled: true}
	src, err := NewHTTPSource(spec)
	require.NoError(t, err)

	dest := t.TempDir()

	// First fetch: marketplace contains p1.
	require.NoError(t, src.Fetch(context.Background(), dest))
	_, err = os.Stat(filepath.Join(dest, ".claude-plugin/plugins/p1/plugin.json"))
	require.NoError(t, err, "p1/plugin.json should exist after first fetch")
	_, err = os.Stat(filepath.Join(dest, ".claude-plugin/plugins/p2/plugin.json"))
	assert.True(t, os.IsNotExist(err), "p2/plugin.json should NOT exist after first fetch")

	// Second fetch: marketplace now contains p2 only. The implementation must
	// NOT leave p1's files behind.
	require.NoError(t, src.Fetch(context.Background(), dest))
	_, err = os.Stat(filepath.Join(dest, ".claude-plugin/plugins/p2/plugin.json"))
	require.NoError(t, err, "p2/plugin.json should exist after second fetch")
	_, err = os.Stat(filepath.Join(dest, ".claude-plugin/plugins/p1/plugin.json"))
	assert.True(t, os.IsNotExist(err), "p1/plugin.json should have been removed by atomic refresh; got err=%v", err)

	// The .partial staging dir must not leak.
	_, err = os.Stat(dest + ".partial")
	assert.True(t, os.IsNotExist(err), "staging dir should not remain after successful fetch")
}

// TestHTTPSource_Fetch_PluginWithFilePath verifies that when a plugin source
// already ends in a .json manifest filename, the fetcher does NOT append
// "/plugin.json" a second time.
func TestHTTPSource_Fetch_PluginWithFilePath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.claude-plugin/marketplace.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plugins": []map[string]any{
				{"name": "p1", "source": "./plugins/p1/plugin.json"},
			},
		})
	})
	// Real manifest file path that the upstream serves.
	mux.HandleFunc("/.claude-plugin/plugins/p1/plugin.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"p1"}`))
	})
	// If the implementation double-appends, it will hit this path with a 404.
	mux.HandleFunc("/.claude-plugin/plugins/p1/plugin.json/plugin.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "should not be requested", http.StatusNotFound)
	})
	up := httptest.NewServer(mux)
	defer up.Close()

	spec := config.Source{Name: "h", Type: "http", URL: up.URL + "/", Enabled: true}
	src, err := NewHTTPSource(spec)
	require.NoError(t, err)

	dest := t.TempDir()
	require.NoError(t, src.Fetch(context.Background(), dest))

	// The manifest should be saved at the same path the upstream served it from.
	pdata, err := os.ReadFile(filepath.Join(dest, ".claude-plugin/plugins/p1/plugin.json"))
	require.NoError(t, err)
	assert.Contains(t, string(pdata), "p1")

	// And the doubled path should NOT exist. (The parent "plugins/p1/plugin.json"
	// is a regular file, so the doubled path can never be created — but we
	// verify that by checking the stat error, accepting either IsNotExist or
	// "not a directory" from the OS.)
	doublePath := filepath.Join(dest, ".claude-plugin/plugins/p1/plugin.json/plugin.json")
	_, err = os.Stat(doublePath)
	assert.Error(t, err, "double-suffixed path should not exist as a real file")
}

// TestHTTPSource_Fetch_PluginWithTraversal verifies that a plugin source
// containing ".." that would resolve outside the marketplace directory is
// rejected with an error.
func TestHTTPSource_Fetch_PluginWithTraversal(t *testing.T) {
	mux := http.NewServeMux()
	// Marketplace lives at a nested path so "../escape" from its directory
	// resolves to a sibling directory above it.
	mux.HandleFunc("/marketplace/sub/.claude-plugin/marketplace.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plugins": []map[string]any{
				{"name": "evil", "source": "../escape"},
			},
		})
	})
	up := httptest.NewServer(mux)
	defer up.Close()

	spec := config.Source{Name: "h", Type: "http", URL: up.URL + "/marketplace/sub/", Enabled: true}
	src, err := NewHTTPSource(spec)
	require.NoError(t, err)

	dest := t.TempDir()
	err = src.Fetch(context.Background(), dest)
	assert.Error(t, err, "traversal source must be rejected")
	assert.Contains(t, err.Error(), "escapes marketplace directory")
}

// guard against accidentally importing fmt and not using it
var _ = fmt.Sprintf
