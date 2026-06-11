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
