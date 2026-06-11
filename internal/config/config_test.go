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
