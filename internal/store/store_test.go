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

func TestWriteFile_RejectsTraversal(t *testing.T) {
	cases := []struct {
		name string
		rel  string
	}{
		{"parent_traversal", "../../../tmp/pwned"},
		{"absolute", "/etc/passwd"},
		{"empty", ""},
		{"name_with_slash", "../etc/passwd"},
	}
	dir := t.TempDir()
	s := New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.WriteFile("team", tc.rel, []byte("x"))
			assert.Error(t, err, "rel=%q should be rejected", tc.rel)
		})
	}
}

func TestReadFile_RejectsTraversal(t *testing.T) {
	cases := []struct {
		name string
		rel  string
	}{
		{"parent_traversal", "../../../etc/passwd"},
		{"absolute", "/etc/passwd"},
		{"empty", ""},
	}
	dir := t.TempDir()
	s := New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.ReadFile("team", tc.rel)
			assert.Error(t, err, "rel=%q should be rejected", tc.rel)
		})
	}
}

func TestRemoveSource_RejectsDotDot(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	require.NoError(t, s.EnsureSourceDir("team"))
	err := s.RemoveSource("..")
	assert.Error(t, err)
	// Verify the sources/ tree was not deleted.
	_, statErr := os.Stat(filepath.Join(dir, "sources"))
	assert.False(t, os.IsNotExist(statErr), "sources/ should still exist")
}

func TestEnsureSourceDir_RejectsDotDot(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	err := s.EnsureSourceDir("..")
	assert.Error(t, err)
}

func TestWriteFile_RejectsNameWithSlash(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	err := s.WriteFile("team/evil", "x.txt", []byte("y"))
	assert.Error(t, err)
}
