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

	// Per Claude Code convention, the marketplace manifest lives at
	// .claude-plugin/marketplace.json inside each source.
	writeJSON(t, filepath.Join(dir, "sources/a/.claude-plugin/marketplace.json"), map[string]any{
		"name": "a",
		"plugins": []map[string]any{
			{"name": "p1", "source": "./plugins/p1"},
		},
	})
	writeJSON(t, filepath.Join(dir, "sources/b/.claude-plugin/marketplace.json"), map[string]any{
		"name": "b",
		"plugins": []map[string]any{
			{"name": "p2", "source": "./plugins/p2"},
		},
	})

	agg := New(s, "http://example.test")
	require.NoError(t, agg.Refresh([]string{"a", "b"}, ""))

	got, err := os.ReadFile(filepath.Join(s.AggregatedDir(), "marketplace.json"))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(got, &m))
	plugins := m["plugins"].([]any)
	assert.Len(t, plugins, 2)
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
	err := agg.Refresh([]string{"nope"}, "")
	assert.Error(t, err)
	// The error should mention the source name AND hint at the marketplace
	// format requirement so users get actionable feedback when they point
	// at a repo that isn't actually a marketplace.
	assert.Contains(t, err.Error(), "nope")
	assert.Contains(t, err.Error(), "marketplace.json")
	assert.Contains(t, err.Error(), "marketplace format")
}

func TestAggregate_RefreshOverrideBaseURL(t *testing.T) {
	// When the caller passes a non-empty baseURL, the per-refresh value
	// must override the constructor's baseURL.
	dir := t.TempDir()
	s := store.New(dir)
	require.NoError(t, s.EnsureSourceDir("a"))

	writeJSON(t, filepath.Join(dir, "sources/a/.claude-plugin/marketplace.json"), map[string]any{
		"name": "a",
		"plugins": []map[string]any{
			{"name": "p1", "source": "./plugins/p1"},
		},
	})

	agg := New(s, "http://constructor.test")
	require.NoError(t, agg.Refresh([]string{"a"}, "http://override.test"))

	got, err := os.ReadFile(filepath.Join(s.AggregatedDir(), "marketplace.json"))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(got, &m))
	plugins := m["plugins"].([]any)
	require.Len(t, plugins, 1)
	p := plugins[0].(map[string]any)
	assert.Equal(t, "http://override.test/plugins/a/plugins/p1", p["source"])
}

func TestRewriteURL(t *testing.T) {
	assert.Equal(t,
		"http://x/plugins/src/p/x",
		rewriteURL("http://x", "src", "./p/x"))
	// Absolute URL is left as-is so the consumer can fetch it directly.
	assert.Equal(t,
		"http://other/abs",
		rewriteURL("http://x", "src", "http://other/abs"))
}
